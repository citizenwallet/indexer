package db

import (
	"database/sql"
	"fmt"

	"github.com/citizenwallet/node/internal/storage"
)

type EventDB struct {
	ChainID int64

	path string
	db   *sql.DB
}

// NewTransferDB creates a new DB
func NewEventDB(chainID int64) (*EventDB, error) {

	basePath := storage.GetUserHomeDir()
	path := fmt.Sprintf("%s/%s/events_%v.db", basePath, dbBaseFolder, chainID)

	// check if db exists before opening, since we use rwc mode
	exists := storage.Exists(path)

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", path))
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = createEventsTable(db)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = createEventsTableIndexes(db)
		if err != nil {
			return nil, err
		}
	}

	return &EventDB{
		ChainID: chainID,
		path:    path,
		db:      db,
	}, nil
}

// createEventsTable creates a table to store events in the given db
func createEventsTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE t_events (
		address TEXT NOT NULL,
		state TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		start_block INTEGER NOT NULL,
		last_block INTEGER NOT NULL,
		signature TEXT NOT NULL,
		name TEXT NOT NULL,
		symbol TEXT NOT NULL,
		UNIQUE(address, signature)
	)
	`)

	return err
}

// createEventsTableIndexes creates the indexes for events in the given db
func createEventsTableIndexes(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE INDEX idx_events_address_signature ON t_transfers (address, signature);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_events_address_signature_state ON t_transfers (address, signature, state);
	`)
	if err != nil {
		return err
	}

	return nil
}
