package govdb

import (
	"database/sql"
	"fmt"
	"github.com/citizenwallet/indexer/internal/services/db"
	"math/big"
	"sync"
)

// TODO: could share base struct with DB a/o be collapsed together

type DB struct {
	chainID *big.Int
	mu      sync.Mutex
	db      *sql.DB
	rdb     *sql.DB

	GovernorsDB *GovernorDB
	ProposalsDB *ProposalDB

	testing bool
}

func NewDB(chainID *big.Int, username, password, name, host, rhost string) (*DB, error) {
	db, rdb, err := db.NewDBConnection(username, password, name, host, rhost)
	if err != nil {
		return nil, err
	}

	gdb := DB{
		chainID: chainID,
		mu:      sync.Mutex{},
		db:      db,
		rdb:     rdb,
	}
	gdb.GovernorsDB = &GovernorDB{p: &gdb}
	gdb.ProposalsDB = &ProposalDB{p: &gdb}

	if err = gdb.GovernorsDB.ensureExists(); err != nil {
		return nil, err
	}

	if err = gdb.ProposalsDB.ensureExists(); err != nil {
		return nil, err
	}

	return &gdb, nil
}

func (gdb *DB) SetTesting() {
	gdb.testing = true
}

func (gdb *DB) Close() {
	if gdb.testing {
		gdb.GovernorsDB.drop()
		gdb.ProposalsDB.drop()
	}

	gdb.db.Close()
	gdb.db = nil
	gdb.rdb.Close()
	gdb.rdb = nil

	return
}

func (gdb *DB) governorsTableName() string {
	return fmt.Sprintf("t_governors_%s", gdb.chainID.String())
}

func (gdb *DB) proposalsTableName() string {
	return fmt.Sprintf("t_proposals_%s", gdb.chainID.String())
}

func (gdb *DB) checkTableExists(tname string) (bool, error) {
	var exists bool
	err := gdb.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = '%s'
    );
    `, tname)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
