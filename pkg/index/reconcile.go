package index

import (
	"context"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/citizenwallet/indexer/internal/bundler"
	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type ErrReconciling error

var (
	ErrReconcilingRecoverable ErrReconciling = errors.New("error reconciling recoverable") // an error occurred while reconciling but it is not fatal
	ErrInvalidUserOp          ErrReconciling = errors.New("Missing/invalid userOpHash")
)

type Reconciler struct {
	rate    int
	chainID *big.Int
	db      *db.DB
	bundler *bundler.Bundler
}

func NewReconciler(rate int, chainID *big.Int, db *db.DB, ctx context.Context, rpcUrl, origin string) (*Reconciler, error) {
	b, err := bundler.New(ctx, rpcUrl, origin)
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		rate:    rate,
		chainID: chainID,
		db:      db,
		bundler: b,
	}, nil
}

func (e *Reconciler) Close() {
	e.bundler.Close()
}

// Start starts the reconciler service
func (r *Reconciler) Start() error {
	// get all events
	evs, err := r.db.EventDB.GetEvents()
	if err != nil {
		return err
	}

	return r.Process(evs)
}

// Background starts a reconciler service in the background
func (r *Reconciler) Background(syncrate int) error {
	for {
		err := r.Start()
		if err != nil {
			// check if the error is recoverable
			if err == ErrReconcilingRecoverable {
				log.Default().Println("reconciler [background] recoverable error: ", err)
				// sleep for a bit and try again
				<-time.After(250 * time.Millisecond)
				// start again
				continue
			}
			return err
		}

		<-time.After(time.Duration(syncrate) * time.Second)
	}
}

// Process processes a batch of events
func (r *Reconciler) Process(evs []*indexer.Event) error {
	if len(evs) == 0 {
		// no events to process
		return nil
	}

	var err error

	for _, ev := range evs {
		log.Default().Println("reconciling event: ", ev.Contract, ev.Standard, " ...")

		txdb, ok := r.db.GetTransferDB(ev.Contract)
		if !ok {
			txdb, err = r.db.AddTransferDB(ev.Contract)
			if err != nil {
				return err
			}
		}

		// get pending transfers
		txs, err := txdb.GetPendingTransfers(r.rate)
		if err != nil {
			return err
		}

		if len(txs) == 0 {
			continue
		}

		for _, tx := range txs {
			log.Default().Println("found ", len(txs), " logs that need to be reconciled...")
			r, err := r.bundler.GetUserOperationByHash(tx.Hash)
			if err != nil && err.Error() != ErrInvalidUserOp.Error() {
				// probably a network error
				return ErrReconcilingRecoverable
			}
			if err != nil && err.Error() == ErrInvalidUserOp.Error() {
				// probably an unsubmitted user op
				log.Default().Println("user op not found: ", err.Error())

				// remove the transaction from the db
				// err = txdb.RemovePendingTransfer(tx.Hash)
				// if err != nil {
				// 	println("error removing pending transfer: ", err.Error())
				// 	return err
				// }
				// println("removed pending transfer: ", tx.Hash)
				continue
			}

			// check if it this tx_hash already exists
			exists, err := txdb.TransferExists(r.TransactionHash)
			if err != nil {
				return err
			}

			if exists {
				// set hash
				err = txdb.SetHash(tx.Hash, r.TransactionHash)
				if err != nil {
					return err
				}

				continue
			}

			// set tx_hash
			err = txdb.SetTxHash(r.TransactionHash, tx.Hash)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
