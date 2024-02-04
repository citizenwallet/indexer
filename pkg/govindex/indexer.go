package govindex

import (
	"github.com/citizenwallet/indexer/internal/services/db/govdb"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"math/big"
)

type Indexer struct {
	rate    int
	chainID *big.Int
	gdb     *govdb.DB
	evm     indexer.EVMRequester
}

func New(rate int, chainID *big.Int, gdb *govdb.DB, evm indexer.EVMRequester) (*Indexer, error) {
	return &Indexer{
		rate:    rate,
		chainID: chainID,
		gdb:     gdb,
		evm:     evm,
	}, nil
}
