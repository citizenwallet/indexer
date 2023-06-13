package db

import (
	"database/sql"
	"fmt"

	"github.com/citizenwallet/node/internal/storage"
)

type TransferDB struct {
	path string
	db   *sql.DB
}

// NewTransferDB creates a new DB
func NewTransferDB(name string) (*TransferDB, error) {

	basePath := storage.GetUserHomeDir()
	path := fmt.Sprintf("%s/%s/logs_%s.db", basePath, dbBaseFolder, name)

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
			println("error creating transfer table")
			return nil, err
		}

		// create indexes
		err = createTransferTableIndexes(db)
		if err != nil {
			println("error creating transfer table indexes")
			return nil, err
		}
	}

	return &TransferDB{
		path: path,
		db:   db,
	}, nil
}

// createTransferTable creates a table to store transfers in the given db
func createTransferTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE t_transfers (
		hash TEXT NOT NULL PRIMARY KEY,
		token_id INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		from_addr TEXT NOT NULL,
		to_addr TEXT NOT NULL,
		value INTEGER NOT NULL,
		data BLOB
	)
	`)

	return err
}

// createTransferTableIndexes creates the indexes for transfers in the given db
func createTransferTableIndexes(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE INDEX idx_transfers_token_id_date ON t_transfers (token_id, created_at);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_transfers_token_id_from_date ON t_transfers (token_id, from_addr, created_at);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_transfers_token_id_to_date ON t_transfers (token_id, to_addr, created_at);
	`)
	if err != nil {
		return err
	}

	return nil
}

// AddTransfer adds a transfer to the db
func (db *TransferDB) AddTransfer(hash string, tokenID int64, createdAt string, fromAddr string, toAddr string, value int64, data []byte) error {

	// insert transfer on conflict update
	_, err := db.db.Exec(`
	INSERT INTO t_transfers (hash, token_id, created_at, from_addr, to_addr, value, data)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hash) DO UPDATE SET
		token_id = excluded.token_id,
		created_at = excluded.created_at,
		from_addr = excluded.from_addr,
		to_addr = excluded.to_addr,
		value = excluded.value,
		data = excluded.data
	`, hash, tokenID, createdAt, fromAddr, toAddr, value, data)

	return err
}
