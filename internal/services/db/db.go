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
	rdb     *sql.DB

	EventDB     *EventDB
	SponsorDB   *SponsorDB
	TransferDB  map[string]*TransferDB
	PushTokenDB map[string]*PushTokenDB
}

// NewDB instantiates a new DB
func NewDB(chainID *big.Int, username, password, name, host, rhost, secret string) (*DB, error) {
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, host)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	rconnStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, rhost)
	rdb, err := sql.Open("postgres", rconnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = rdb.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	evname := chainID.String()

	eventDB, err := NewEventDB(db, rdb, evname)
	if err != nil {
		return nil, err
	}

	sponsorDB, err := NewSponsorDB(db, rdb, evname, secret)
	if err != nil {
		return nil, err
	}

	d := &DB{
		chainID:   chainID,
		db:        db,
		rdb:       rdb,
		EventDB:   eventDB,
		SponsorDB: sponsorDB,
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

	// check if db exists before opening, since we use rwc mode
	exists, err = d.SponsorTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = sponsorDB.CreateSponsorsTable(evname)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = sponsorDB.CreateSponsorsTableIndexes(evname)
		if err != nil {
			return nil, err
		}
	}

	txdb := map[string]*TransferDB{}
	ptdb := map[string]*PushTokenDB{}

	evs, err := eventDB.GetEvents()
	if err != nil {
		return nil, err
	}

	for _, ev := range evs {
		name := d.TransferName(ev.Contract)
		log.Default().Println("creating transfer db for: ", name)

		txdb[name], err = NewTransferDB(db, rdb, name)
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

		log.Default().Println("creating push token db for: ", name)

		ptdb[name], err = NewPushTokenDB(db, rdb, name)
		if err != nil {
			return nil, err
		}

		// check if db exists before opening, since we use rwc mode
		exists, err = d.PushTokenTableExists(name)
		if err != nil {
			return nil, err
		}

		if !exists {
			// create table
			err = ptdb[name].CreatePushTable()
			if err != nil {
				return nil, err
			}

			// create indexes
			err = ptdb[name].CreatePushTableIndexes()
			if err != nil {
				return nil, err
			}
		}
	}

	d.TransferDB = txdb
	d.PushTokenDB = ptdb

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

// SponsorTableExists checks if a table exists in the database
func (db *DB) SponsorTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_sponsors_%s'
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

// PushTokenTableExists checks if a table exists in the database
func (db *DB) PushTokenTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_push_token_%s'
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

// GetPushTokenDB returns true if the push token db for the given contract exists, returns the db if it exists
func (d *DB) GetPushTokenDB(contract string) (*PushTokenDB, bool) {
	name := d.TransferName(contract)
	d.mu.Lock()
	defer d.mu.Unlock()
	ptdb, ok := d.PushTokenDB[name]
	if !ok {
		return nil, false
	}
	return ptdb, true
}

// AddTransferDB adds a new transfer db for the given contract
func (d *DB) AddTransferDB(contract string) (*TransferDB, error) {
	name := d.TransferName(contract)
	d.mu.Lock()
	defer d.mu.Unlock()
	if txdb, ok := d.TransferDB[name]; ok {
		return txdb, nil
	}
	txdb, err := NewTransferDB(d.db, d.rdb, name)
	if err != nil {
		return nil, err
	}
	d.TransferDB[name] = txdb
	return txdb, nil
}

// AddPushTokenDB adds a new push token db for the given contract
func (d *DB) AddPushTokenDB(contract string) (*PushTokenDB, error) {
	name := d.TransferName(contract)
	d.mu.Lock()
	defer d.mu.Unlock()
	if ptdb, ok := d.PushTokenDB[name]; ok {
		return ptdb, nil
	}
	ptdb, err := NewPushTokenDB(d.db, d.rdb, name)
	if err != nil {
		return nil, err
	}
	d.PushTokenDB[name] = ptdb
	return ptdb, nil
}

// Close closes the db and all its transfer and push dbs
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

	for i, ptdb := range d.PushTokenDB {
		err := ptdb.Close()
		if err != nil {
			return err
		}

		delete(d.PushTokenDB, i)
	}

	err := d.SponsorDB.Close()
	if err != nil {
		return err
	}

	return d.EventDB.Close()
}
