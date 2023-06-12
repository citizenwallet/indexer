package db

import (
	_ "github.com/mattn/go-sqlite3"
)

const (
	dbBaseFolder = ".cw"
)

type DB struct {
	EventDB    *EventDB
	TransferDB map[string]*TransferDB
}

// NewDB creates a new DB
func NewDB(chainID int64) (*DB, error) {
	eventDB, err := NewEventDB(chainID)
	if err != nil {
		return nil, err
	}

	return &DB{
		EventDB:    eventDB,
		TransferDB: make(map[string]*TransferDB),
	}, nil
}

// AddEvent adds an event to the db
