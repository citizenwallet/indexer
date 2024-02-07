package govindex

import (
	"github.com/citizenwallet/indexer/internal/services/db/govdb"
	"github.com/citizenwallet/indexer/pkg/govindexer"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"log"
	"math/big"
)

type Indexer struct {
	rate    int64
	chainID *big.Int
	gdb     *govdb.DB
	evm     indexer.EVMRequester

	handlers map[common.Hash]func(l types.Log) error
}

func New(rate int64, chainID *big.Int, gdb *govdb.DB, evm indexer.EVMRequester) (*Indexer, error) {
	i := &Indexer{
		rate:    rate,
		chainID: chainID,
		gdb:     gdb,
		evm:     evm,
	}
	i.handlers = i.makeHandlers()
	return i, nil
}

// TODO: this is cribbed + adapted from index/index.go
func (i *Indexer) makeIndexFilter(contractAddrHex string, lastBlock, targetBlock int64) (query ethereum.FilterQuery) {

	contractAddr := common.HexToAddress(contractAddrHex)

	// Calculate the starting block for the filter query
	// It's the last block that was indexed plus one
	fromBlock := lastBlock + 1

	// Calculate the number of blocks to index
	// It's the current block number minus the starting block
	blocksToIndex := targetBlock - fromBlock

	// TODO: different than other indexing code. i prefer to go forwards ...?
	if blocksToIndex > i.rate {
		targetBlock = fromBlock + i.rate - 1 // filter block range is inclusive
	}

	topics := govindexer.makeGovTopics()

	query = ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(targetBlock),
		Addresses: []common.Address{contractAddr},
		Topics:    topics,
	}

	return
}

func (i *Indexer) makeHandlers() map[common.Hash]func(l types.Log) error {
	return map[common.Hash]func(l types.Log) error{
		govindexer.GovVotingDelaySetId:       i.handleVotingDelaySet,
		govindexer.GovVotingPeriodSetId:      i.handleVotingPeriodSet,
		govindexer.GovProposalThresholdSetId: i.handleProposalThresholdSet,
	}
}

func (i *Indexer) handleVotingDelaySet(l types.Log) error {
	return nil
}

func (i *Indexer) handleVotingPeriodSet(l types.Log) error {
	return nil
}

func (i *Indexer) handleProposalThresholdSet(l types.Log) error {
	return nil
}

func (i *Indexer) fromBlock(gv *govindexer.Governor, targetBlock uint64) error {

	query := i.makeIndexFilter(gv.Contract, gv.LastBlock, int64(targetBlock))
	logs, err := i.evm.FilterLogs(query)
	if err != nil {
		return index.ErrIndexingRecoverable
	}

	if len(logs) > 0 {
		for _, l := range logs {
			h, ok := i.handlers[l.Topics[0]]
			if !ok {
				log.Fatalf("should not happen - found unregistered event")
			}
			if err = h(l); err != nil {
				return err // TODO: error handling ???
			}
		}
	}

	return nil
}
