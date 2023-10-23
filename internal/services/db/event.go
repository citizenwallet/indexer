package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

type EventDB struct {
	suffix string
	db     *sql.DB
	rdb    *sql.DB
}

// NewTransferDB creates a new DB
func NewEventDB(db, rdb *sql.DB, name string) (*EventDB, error) {
	evdb := &EventDB{
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return evdb, nil
}

// Close closes the db
func (db *EventDB) Close() error {
	return db.db.Close()
}

func (db *EventDB) CloseR() error {
	return db.rdb.Close()
}

// createEventsTable creates a table to store events in the given db
func (db *EventDB) CreateEventsTable(suffix string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	CREATE TABLE t_events_%s(
		contract text NOT NULL,
		state text NOT NULL,
		created_at timestamp NOT NULL,
		updated_at timestamp NOT NULL,
		start_block integer NOT NULL,
		last_block integer NOT NULL,
		standard text NOT NULL,
		name text NOT NULL,
		symbol text NOT NULL,
		decimals integer NOT NULL DEFAULT 6,
		UNIQUE (contract, standard)
	);
	`, suffix))

	return err
}

// createEventsTableIndexes creates the indexes for events in the given db
func (db *EventDB) CreateEventsTableIndexes(suffix string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
    CREATE INDEX idx_events_%s_state ON t_events_%s (state);
    `, suffix, suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
    CREATE INDEX idx_events_%s_address_signature ON t_events_%s (contract, standard);
    `, suffix, suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
    CREATE INDEX idx_events_%s_address_signature_state ON t_events_%s (contract, standard, state);
    `, suffix, suffix))
	if err != nil {
		return err
	}

	return nil
}

// GetEvent gets an event from the db by contract and standard
func (db *EventDB) GetEvent(contract string, standard indexer.Standard) (*indexer.Event, error) {
	var event indexer.Event
	err := db.rdb.QueryRow(fmt.Sprintf(`
	SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
	FROM t_events_%s
	WHERE contract = $1 AND standard = $2
	`, db.suffix), contract, standard).Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

// GetEvents gets all events from the db
func (db *EventDB) GetEvents() ([]*indexer.Event, error) {
	rows, err := db.rdb.Query(fmt.Sprintf(`
    SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
    FROM t_events_%s
    ORDER BY created_at ASC
    `, db.suffix))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*indexer.Event{}
	for rows.Next() {
		var event indexer.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetOutdatedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetOutdatedEvents(currentBlk int64) ([]*indexer.Event, error) {
	rows, err := db.rdb.Query(fmt.Sprintf(`
    SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
    FROM t_events_%s
    WHERE last_block < $1
    ORDER BY created_at ASC
    `, db.suffix), currentBlk)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*indexer.Event{}
	for rows.Next() {
		var event indexer.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// GetQueuedEvents gets all queued events from the db sorted by created_at
func (db *EventDB) GetQueuedEvents() ([]*indexer.Event, error) {
	rows, err := db.rdb.Query(fmt.Sprintf(`
    SELECT contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals
    FROM t_events_%s
    WHERE state = $1
    ORDER BY created_at ASC
    `, db.suffix), indexer.EventStateQueued)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []*indexer.Event{}
	for rows.Next() {
		var event indexer.Event
		err = rows.Scan(&event.Contract, &event.State, &event.CreatedAt, &event.UpdatedAt, &event.StartBlock, &event.LastBlock, &event.Standard, &event.Name, &event.Symbol, &event.Decimals)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

// SetEventState sets the state of an event
func (db *EventDB) SetEventState(contract string, standard indexer.Standard, state indexer.EventState) error {
	_, err := db.db.Exec(fmt.Sprintf(`
    UPDATE t_events_%s
    SET state = $1, updated_at = $2
    WHERE contract = $3 AND standard = $4
    `, db.suffix), state, time.Now().UTC(), contract, standard)

	return err
}

// SetEventLastBlock sets the last block of an event
func (db *EventDB) SetEventLastBlock(contract string, standard indexer.Standard, lastBlock int64) error {
	_, err := db.db.Exec(fmt.Sprintf(`
    UPDATE t_events_%s
    SET last_block = $1, updated_at = $2
    WHERE contract = $3 AND standard = $4
    `, db.suffix), lastBlock, time.Now().UTC(), contract, standard)

	return err
}

// AddEvent adds an event to the db
func (db *EventDB) AddEvent(contract string, state indexer.EventState, startBlk, lastBlk int64, std indexer.Standard, name, symbol string, decimals int64) error {
	t := time.Now()

	_, err := db.db.Exec(fmt.Sprintf(`
    INSERT INTO t_events_%s (contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    ON CONFLICT(contract, standard) DO UPDATE SET
        state = excluded.state,
        updated_at = excluded.updated_at,
        start_block = excluded.start_block,
        last_block = excluded.last_block,
        name = excluded.name,
        symbol = excluded.symbol
		decimals = excluded.decimals
    `, db.suffix), contract, state, t, t, startBlk, lastBlk, std, name, symbol, decimals)

	return err
}
