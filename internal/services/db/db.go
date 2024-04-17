package db

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strings"
	"sync"

	"database/sql"

	"github.com/citizenwallet/indexer/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

const (
	dbBaseFolder   = "data"
	dbConfigString = "cache=private&_journal=WAL&mode=rwc&_txlock=immediate&_busy_timeout=10000"
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
func NewDB(chainID *big.Int, basePath, secret string) (*DB, error) {
	// basePath := "."
	folderPath := fmt.Sprintf("%s/%s", basePath, dbBaseFolder)
	path := fmt.Sprintf("%s/cw.db", folderPath)

	// check if directory exists
	if !storage.Exists(folderPath) {
		err := storage.CreateDir(folderPath)
		if err != nil {
			return nil, err
		}
	}

	// check if db exists before opening, since we use rwc mode
	// exists := storage.Exists(path)

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?%s", path, dbConfigString))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, host)
	// db, err := sql.Open("postgres", connStr)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to connect to database: %w", err)
	// }

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(1)

	// rconnStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, rhost)
	// rdb, err := sql.Open("postgres", rconnStr)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to connect to database: %w", err)
	// }

	// err = rdb.Ping()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to ping database: %w", err)
	// }

	evname := chainID.String()

	eventDB, err := NewEventDB(db, db, evname)
	if err != nil {
		return nil, err
	}

	sponsorDB, err := NewSponsorDB(db, db, evname, secret)
	if err != nil {
		return nil, err
	}

	d := &DB{
		chainID:   chainID,
		db:        db,
		rdb:       db,
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
		name, err := d.TableNameSuffix(ev.Contract)
		if err != nil {
			return nil, err
		}

		log.Default().Println("creating transfer db for: ", name)

		txdb[name], err = NewTransferDB(db, db, name)
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

		ptdb[name], err = NewPushTokenDB(db, db, name)
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
	tableName := fmt.Sprintf("t_events_%s", suffix)
	row := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Table does not exist
			return false, nil
		} else {
			// A database error occurred
			return false, err
		}
	}

	return true, nil
}

// SponsorTableExists checks if a table exists in the database
func (db *DB) SponsorTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_sponsors_%s", suffix)
	row := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Table does not exist
			return false, nil
		} else {
			// A database error occurred
			return false, err
		}
	}

	return true, nil
}

// TransferTableExists checks if a table exists in the database
func (db *DB) TransferTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_transfers_%s", suffix)
	row := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Table does not exist
			return false, nil
		} else {
			// A database error occurred
			return false, err
		}
	}

	return true, nil
}

// PushTokenTableExists checks if a table exists in the database
func (db *DB) PushTokenTableExists(suffix string) (bool, error) {
	tableName := fmt.Sprintf("t_push_token_%s", suffix)
	row := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Table does not exist
			return false, nil
		} else {
			// A database error occurred
			return false, err
		}
	}

	return true, nil
}

// TableNameSuffix returns the name of the transfer db for the given contract
func (d *DB) TableNameSuffix(contract string) (string, error) {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")

	suffix := fmt.Sprintf("%v_%s", d.chainID, strings.ToLower(contract))

	if !re.MatchString(contract) {
		return suffix, errors.New("bad contract address")
	}

	return suffix, nil
}

// GetTransferDB returns true if the transfer db for the given contract exists, returns the db if it exists
func (d *DB) GetTransferDB(contract string) (*TransferDB, bool) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, false
	}

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
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, false
	}

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
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, err
	}

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
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, err
	}
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
