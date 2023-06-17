package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/citizenwallet/indexer/internal/storage"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type EventDB struct {
	path string
	db   *sql.DB
}

// NewTransferDB creates a new DB
func NewEventDB(path string) (*EventDB, error) {
	// check if db exists before opening, since we use rwc mode
	exists := storage.Exists(path)

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?%s", path, dbConfigString))
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

// Close closes the db
func (db *EventDB) Close() error {
	return db.db.Close()
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
		standard TEXT NOT NULL,
		name TEXT NOT NULL,
		symbol TEXT NOT NULL,
		UNIQUE(contract, standard)
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
	CREATE INDEX idx_events_address_signature ON t_events (contract, standard);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_events_address_signature_state ON t_events (contract, standard, state);
	`)
	if err != nil {
		return err
	}

	return nil
}

// GetEvents gets all events from the db
func (db *EventDB) GetEvents() ([]*indexer.Event, error) {
	rows, err := db.db.Query(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol
	FROM t_events
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*indexer.Event{}
	for rows.Next() {
		var event indexer.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetOutdatedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetOutdatedEvents(currentBlk int64) ([]*indexer.Event, error) {
	rows, err := db.db.Query(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol
	FROM t_events
	WHERE last_block < ?
	ORDER BY created_at ASC
	`, currentBlk)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*indexer.Event{}
	for rows.Next() {
		var event indexer.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetQueuedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetQueuedEvents() ([]*indexer.Event, error) {
	rows, err := db.db.Query(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol
	FROM t_events
	WHERE state = ?
	ORDER BY created_at ASC
	`, indexer.EventStateQueued)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*indexer.Event{}
	for rows.Next() {
		var event indexer.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// SetEventState sets the state of an event
func (db *EventDB) SetEventState(contract string, standard indexer.Standard, state indexer.EventState) error {
	_, err := db.db.Exec(`
	UPDATE t_events
	SET state = ?, updated_at = ?
	WHERE contract = ? AND standard = ?
	`, state, time.Now().Format(time.RFC3339), contract, standard)

	return err
}

// SetEventLastBlock sets the last block of an event
func (db *EventDB) SetEventLastBlock(contract string, standard indexer.Standard, lastBlock int64) error {
	_, err := db.db.Exec(`
	UPDATE t_events
	SET last_block = ?, updated_at = ?
	WHERE contract = ? AND standard = ?
	`, lastBlock, time.Now().Format(time.RFC3339), contract, standard)

	return err
}

// AddEvent adds an event to the db
func (db *EventDB) AddEvent(contract string, state indexer.EventState, startBlk, lastBlk int64, std indexer.Standard, name, symbol string) error {
	t := indexer.SQLiteTime(time.Now())

	_, err := db.db.Exec(`
	INSERT INTO t_events (contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(contract, standard) DO UPDATE SET
		state = excluded.state,
		updated_at = excluded.updated_at,
		start_block = excluded.start_block,
		last_block = excluded.last_block,
		name = excluded.name,
		symbol = excluded.symbol
	`, contract, state, t, t, startBlk, lastBlk, std, name, symbol)

	return err
}
