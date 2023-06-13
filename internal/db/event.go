package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/citizenwallet/node/internal/storage"
	"github.com/citizenwallet/node/pkg/node"
)

type EventDB struct {
	path string
	db   *sql.DB
}

// NewTransferDB creates a new DB
func NewEventDB(path string) (*EventDB, error) {
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
		path: path,
		db:   db,
	}, nil
}

// createEventsTable creates a table to store events in the given db
func createEventsTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE t_events (
		contract TEXT NOT NULL,
		state TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		start_block INTEGER NOT NULL,
		last_block INTEGER NOT NULL,
		function TEXT NOT NULL,
		name TEXT NOT NULL,
		symbol TEXT NOT NULL,
		UNIQUE(contract, function)
	)
	`)

	return err
}

// createEventsTableIndexes creates the indexes for events in the given db
func createEventsTableIndexes(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE INDEX idx_events_state ON t_events (state);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_events_address_signature ON t_events (contract, function);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_events_address_signature_state ON t_events (contract, function, state);
	`)
	if err != nil {
		return err
	}

	return nil
}

// GetEvents gets all events from the db
func (db *EventDB) GetEvents() ([]*node.Event, error) {
	rows, err := db.db.Query(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, function, name, symbol
	FROM t_events
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*node.Event{}
	for rows.Next() {
		var event node.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Function, &event.Name, &event.Symbol)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetQueuedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetQueuedEvents() ([]*node.Event, error) {
	rows, err := db.db.Query(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, function, name, symbol
	FROM t_events
	WHERE state = 'queued'
	ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*node.Event{}
	for rows.Next() {
		var event node.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Function, &event.Name, &event.Symbol)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// SetEventState sets the state of an event
func (db *EventDB) SetEventState(contract, function string, state node.EventState) error {
	_, err := db.db.Exec(`
	UPDATE t_events
	SET state = ?, updated_at = ?
	WHERE contract = ? AND function = ?
	`, state, time.Now().Format(time.RFC3339), contract, function)

	return err
}

// SetEventLastBlock sets the last block of an event
func (db *EventDB) SetEventLastBlock(contract, function string, lastBlock int64) error {
	_, err := db.db.Exec(`
	UPDATE t_events
	SET last_block = ?, updated_at = ?
	WHERE contract = ? AND function = ?
	`, lastBlock, time.Now().Format(time.RFC3339), contract, function)

	return err
}
