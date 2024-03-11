package index

import (
	"context"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/firebase"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type ErrIndexing error

var (
	ErrIndexingRecoverable ErrIndexing = errors.New("error indexing recoverable") // an error occurred while indexing but it is not fatal
)

type Indexer struct {
	rate    int
	chainID *big.Int
	db      *db.DB
	evm     indexer.EVMRequester
	fb      *firebase.PushService
}

func New(rate int, chainID *big.Int, db *db.DB, evm indexer.EVMRequester, fb *firebase.PushService) (*Indexer, error) {
	return &Indexer{
		rate:    rate,
		chainID: chainID,
		db:      db,
		evm:     evm,
		fb:      fb,
	}, nil
}

func (i *Indexer) IndexERC20From(contract string, from int64) error {
	// get the latest block
	curr, err := i.evm.LatestBlock()
	if err != nil {
		return ErrIndexingRecoverable
	}

	ev, err := i.db.EventDB.GetEvent(contract, indexer.ERC20)
	if err != nil {
		return err
	}

	for curr.Int64() > from {
		t, err := i.evm.BlockTime(curr)
		if err != nil {
			return ErrIndexingRecoverable
		}

		blk := &block{Number: curr.Uint64(), Time: t}

		err = i.EventsFromBlock(ev, blk)
		if err != nil {
			return err
		}

		curr.Sub(curr, big.NewInt(1))
	}

	return nil
}

// Start starts the indexer service
func (i *Indexer) Start() error {
	// get the latest block
	curr, err := i.evm.LatestBlock()
	if err != nil {
		return ErrIndexingRecoverable
	}

	// check if there are any queued events
	evs, err := i.db.EventDB.GetOutdatedEvents(curr.Int64())
	if err != nil {
		return err
	}

	t, err := i.evm.BlockTime(curr)
	if err != nil {
		return ErrIndexingRecoverable
	}

	blk := &block{Number: curr.Uint64(), Time: t}

	return i.Process(evs, blk)
}

func (e *Indexer) Close() {
	//
}

// Background starts an indexer service in the background
func (i *Indexer) Background(syncrate int) error {
	for {
		err := i.Start()
		if err != nil {
			// check if the error is recoverable
			if err == ErrIndexingRecoverable {
				log.Default().Println("indexer [background] recoverable error: ", err)
				// wait a bit
				<-time.After(250 * time.Millisecond)
				// skip the event
				continue
			}
			return err
		}

		<-time.After(time.Duration(syncrate) * time.Second)
	}
}

// ListenBackground starts an indexer service that listens for blocks in the background
func (i *Indexer) ListenBackground(ctx context.Context) error {
	evs, err := i.db.EventDB.GetEvents()
	if err != nil {
		return err
	}

	quitAck := make(chan error)

	for _, ev := range evs {
		go func() {
			err := i.EventsFromLogStream(ctx, quitAck, ev)
			if err != nil {
				quitAck <- err
			}
		}()
	}

	return <-quitAck
}

// Process events
func (i *Indexer) Process(evs []*indexer.Event, blk *block) error {
	if len(evs) == 0 {
		// nothing to do
		return nil
	}

	// iterate over events and index them
	for _, ev := range evs {
		var err error

		err = i.EventsFromBlock(ev, blk)

		if err == nil {
			continue
		}

		if err == ErrIndexingRecoverable {
			log.Default().Println("indexer [process] recoverable error: ", err)
			// wait a bit
			<-time.After(250 * time.Millisecond)
			// skip the event
			continue
		}
		return err
	}

	return nil
}
