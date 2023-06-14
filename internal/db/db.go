package db

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/citizenwallet/node/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

const (
	dbBaseFolder = ".cw"
)

type DB struct {
	chainID *big.Int
	mu      sync.Mutex

	EventDB    *EventDB
	TransferDB map[string]*TransferDB
}

// NewDB instantiates a new DB
func NewDB(chainID *big.Int) (*DB, error) {
	basePath := storage.GetUserHomeDir()
	folderPath := fmt.Sprintf("%s/%s", basePath, dbBaseFolder)
	path := fmt.Sprintf("%s/events_%v.db", folderPath, chainID)

	// check if directory exists
	if !storage.Exists(folderPath) {
		err := storage.CreateDir(folderPath)
		if err != nil {
			return nil, err
		}
	}

	eventDB, err := NewEventDB(path)
	if err != nil {
		return nil, err
	}

	txdb := map[string]*TransferDB{}

	// evs, err := eventDB.GetEvents()
	// if err != nil {
	// 	return nil, err
	// }

	return &DB{
		chainID:    chainID,
		EventDB:    eventDB,
		TransferDB: txdb,
	}, nil

	// for _, ev := range evs {
	// 	name := d.TransferName(ev.Contract)
	// 	log.Default().Println("creating transfer db for: ", name)
	// 	txdb[name], err = NewTransferDB(name)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// d.TransferDB = txdb

	// return d, nil
}

// TransferName returns the name of the transfer db for the given contract
func (d *DB) TransferName(contract string) string {
	return fmt.Sprintf("%v_%s", d.chainID, contract)
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
	txdb, err := NewTransferDB(name)
	if err != nil {
		return nil, err
	}
	d.TransferDB[name] = txdb
	return txdb, nil
}
