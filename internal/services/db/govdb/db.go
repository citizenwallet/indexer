package govdb

import (
	"database/sql"
	"github.com/citizenwallet/indexer/internal/services/db"
	"math/big"
	"sync"
)

// TODO: could share base struct with DB a/o be collapsed together

type GovDB struct {
	chainID *big.Int
	mu      sync.Mutex
	db      *sql.DB
	rdb     *sql.DB
}

func NewGovDB(chainID *big.Int, username, password, name, host, rhost string) (*GovDB, error) {
	db, rdb, err := db.NewDBConnection(username, password, name, host, rhost)
	if err != nil {
		return nil, err
	}

	gdb := GovDB{
		chainID: chainID,
		mu:      sync.Mutex{},
		db:      db,
		rdb:     rdb,
	}

	return &gdb, nil
}
