package db

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type SponsorDB struct {
	suffix string
	secret []byte
	db     *sql.DB
	rdb    *sql.DB
}

// NewSponsorDB creates a new DB
func NewSponsorDB(db, rdb *sql.DB, name, secret string) (*SponsorDB, error) {
	// parse base64 secret
	s, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, errors.New("failed to decode secret")
	}

	sdb := &SponsorDB{
		suffix: name,
		secret: s,
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

	decrypted, err := common.Decrypt(sponsor.PrivateKey, db.secret)
	if err != nil {
		return nil, err
	}

	sponsor.PrivateKey = decrypted

	return &sponsor, nil
}

// AddSponsor adds a sponsor to the db
func (db *SponsorDB) AddSponsor(sponsor *indexer.Sponsor) error {
	encrypted, err := common.Encrypt(sponsor.PrivateKey, db.secret)
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
	INSERT INTO t_sponsors_%s(contract, pk, created_at, updated_at)
	VALUES($1, $2, $3, $4)
	`, db.suffix), sponsor.Contract, encrypted, sponsor.CreatedAt, sponsor.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

// UpdateSponsor updates a sponsor in the db
func (db *SponsorDB) UpdateSponsor(sponsor *indexer.Sponsor) error {
	encrypted, err := common.Encrypt(sponsor.PrivateKey, db.secret)
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
	UPDATE t_sponsors_%s
	SET pk = $1, updated_at = $2
	WHERE contract = $3
	`, db.suffix), encrypted, sponsor.UpdatedAt, sponsor.Contract)
	if err != nil {
		return err
	}

	return nil
}
