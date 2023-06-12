package db

import (
	"database/sql"
	"fmt"

	"github.com/citizenwallet/node/internal/storage"
)

type TransferDB struct {
	ChainID  int64
	Contract string

	path string
	db   *sql.DB
}

// NewTransferDB creates a new DB
func NewTransferDB(chainID int64, contract string) (*TransferDB, error) {

	basePath := storage.GetUserHomeDir()
	path := fmt.Sprintf("%s/%s/transfers_%v_%s.db", basePath, dbBaseFolder, chainID, contract)

	// check if db exists before opening, since we use rwc mode
	exists := storage.Exists(path)

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", path))
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = createTransferTable(db)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = createTransferTableIndexes(db)
		if err != nil {
			return nil, err
		}
	}

	return &TransferDB{
		ChainID:  chainID,
		Contract: contract,
		path:     path,
		db:       db,
	}, nil
}

// createTransferTable creates a table to store transfers in the given db
func createTransferTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE t_transfers (
		hash TEXT NOT NULL PRIMARY KEY,
		token_id INTEGER NOT NULL,
		date TEXT NOT NULL,
		from_address TEXT NOT NULL,
		to_address TEXT NOT NULL,
		value INTEGER NOT NULL,
		data BLOB NOT NULL
	)
	`)

	return err
}

// createTransferTableIndexes creates the indexes for transfers in the given db
func createTransferTableIndexes(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE INDEX idx_transfers_token_id_date ON t_transfers (token_id, date);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_transfers_token_id_from_date ON t_transfers (token_id, from, date);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_transfers_token_id_to_date ON t_transfers (token_id, to, date);
	`)
	if err != nil {
		return err
	}

	return nil
}
