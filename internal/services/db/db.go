package db

import (
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"

	"database/sql"

	_ "github.com/lib/pq"
)

type DB struct {
	chainID *big.Int
	mu      sync.Mutex
	db      *sql.DB

	EventDB    *EventDB
	TransferDB map[string]*TransferDB
}

// NewDB instantiates a new DB
func NewDB(chainID *big.Int, username, password, name, host string) (*DB, error) {
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, host)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	evname := chainID.String()

	eventDB, err := NewEventDB(db, evname)
	if err != nil {
		return nil, err
	}

	d := &DB{
		chainID: chainID,
		db:      db,
		EventDB: eventDB,
	}

	// check if db exists before opening, since we use rwc mode
	exists, err := d.EventTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = eventDB.CreateEventsTable(evname)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = eventDB.CreateEventsTableIndexes(evname)
		if err != nil {
			return nil, err
		}
	}

	txdb := map[string]*TransferDB{}

	evs, err := eventDB.GetEvents()
	if err != nil {
		return nil, err
	}

	for _, ev := range evs {
		name := d.TransferName(ev.Contract)
		log.Default().Println("creating transfer db for: ", name)

		txdb[name], err = NewTransferDB(db, name)
		if err != nil {
			return nil, err
		}

		// check if db exists before opening, since we use rwc mode
		exists, err := d.TransferTableExists(name)
		if err != nil {
			return nil, err
		}

		if !exists {
			// create table
			err = txdb[name].CreateTransferTable()
			if err != nil {
				return nil, err
			}

			// create indexes
			err = txdb[name].CreateTransferTableIndexes()
			if err != nil {
				return nil, err
			}
		}
	}

	d.TransferDB = txdb

	return d, nil
}

// EventTableExists checks if a table exists in the database
func (db *DB) EventTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_events_%s'
    );
    `, suffix)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// TransferTableExists checks if a table exists in the database
func (db *DB) TransferTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_transfers_%s'
    );
    `, suffix)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// TransferName returns the name of the transfer db for the given contract
func (d *DB) TransferName(contract string) string {
	return fmt.Sprintf("%v_%s", d.chainID, strings.ToLower(contract))
}

// GetTransferDB returns true if the transfer db for the given contract exists, returns the db if it exists
func (d *DB) GetTransferDB(contract string) (*TransferDB, bool) {
	name := d.TransferName(contract)
	d.mu.Lock()
	defer d.mu.Unlock()
	txdb, ok := d.TransferDB[name]
	if !ok {
		return nil, false
	}
	return txdb, true
}

// AddTransferDB adds a new transfer db for the given contract
func (d *DB) AddTransferDB(contract string) (*TransferDB, error) {
	name := d.TransferName(contract)
	d.mu.Lock()
	defer d.mu.Unlock()
	if txdb, ok := d.TransferDB[name]; ok {
		return txdb, nil
	}
	txdb, err := NewTransferDB(d.db, name)
	if err != nil {
		return nil, err
	}
	d.TransferDB[name] = txdb
	return txdb, nil
}

// Close closes the db and all its transfer dbs
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, txdb := range d.TransferDB {
		err := txdb.Close()
		if err != nil {
			return err
		}

		delete(d.TransferDB, i)
	}
	return d.EventDB.Close()
}
