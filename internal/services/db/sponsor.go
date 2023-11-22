package db

import (
	"database/sql"
	"fmt"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

type SponsorDB struct {
	suffix string
	db     *sql.DB
	rdb    *sql.DB
}

// NewSponsorDB creates a new DB
func NewSponsorDB(db, rdb *sql.DB, name string) (*SponsorDB, error) {
	sdb := &SponsorDB{
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return sdb, nil
}

// Close closes the db
func (db *SponsorDB) Close() error {
	return db.db.Close()
}

func (db *SponsorDB) CloseR() error {
	return db.rdb.Close()
}

// createSponsorsTable creates a table to store sponsors in the given db
func (db *SponsorDB) CreateSponsorsTable(suffix string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	CREATE TABLE t_sponsors_%s(
		contract TEXT NOT NULL PRIMARY KEY,
		pk text NOT NULL,
		created_at timestamp NOT NULL,
		updated_at timestamp NOT NULL
	);
	`, suffix))

	return err
}

// createSponsorsTableIndexes creates the indexes for sponsors in the given db
func (db *SponsorDB) CreateSponsorsTableIndexes(suffix string) error {
	return nil
}

// GetSponsor gets a sponsor from the db by contract
func (db *SponsorDB) GetSponsor(contract string) (*indexer.Sponsor, error) {
	var sponsor indexer.Sponsor
	err := db.rdb.QueryRow(fmt.Sprintf(`
	SELECT contract, pk, created_at, updated_at
	FROM t_sponsors_%s
	WHERE contract = $1
	`, db.suffix), contract).Scan(&sponsor.Contract, &sponsor.PrivateKey, &sponsor.CreatedAt, &sponsor.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &sponsor, nil
}
