package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type PushTokenDB struct {
	suffix string
	db     *sql.DB
	rdb    *sql.DB
}

// NewPushTokenDB creates a new DB
func NewPushTokenDB(db, rdb *sql.DB, name string) (*PushTokenDB, error) {
	txdb := &PushTokenDB{
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return txdb, nil
}

// Close closes the db
func (db *PushTokenDB) Close() error {
	return db.db.Close()
}

func (db *PushTokenDB) CloseR() error {
	return db.rdb.Close()
}

// CreatePushTable creates a table to store push tokens in the given db
func (db *PushTokenDB) CreatePushTable() error {
	_, err := db.db.Exec(fmt.Sprintf(`
	CREATE TABLE t_push_token_%s(
		token TEXT NOT NULL PRIMARY KEY,
		account text NOT NULL,
		created_at timestamp NOT NULL,
		updated_at timestamp NOT NULL,
		UNIQUE (token, account)
	);
	`, db.suffix))

	return err
}

// CreatePushTableIndexes creates the indexes for push in the given db
func (db *PushTokenDB) CreatePushTableIndexes() error {
	suffix := common.ShortenName(db.suffix, 6)

	// fetch tokens for an address
	_, err := db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_push_%s_account ON t_push_token_%s (account);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_push_%s_token_account ON t_push_token_%s (token, account);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	return nil
}

// AddToken adds a token to the db
func (db *PushTokenDB) AddToken(p *indexer.PushToken) error {
	now := time.Now().UTC()

	// insert transfer on conflict update
	_, err := db.db.Exec(fmt.Sprintf(`
	INSERT INTO t_push_token_%s (token, account, created_at, updated_at)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT(token, account) DO UPDATE SET
		updated_at = excluded.updated_at
	`, db.suffix), p.Token, p.Account, now, now)

	return err
}

// GetAccountTokens returns the push tokens for a given account
func (db *PushTokenDB) GetAccountTokens(account string) ([]*indexer.PushToken, error) {
	pt := []*indexer.PushToken{}

	rows, err := db.rdb.Query(fmt.Sprintf(`
		SELECT token, account
		FROM t_push_token_%s
		WHERE account = $1
		`, db.suffix), account)
	if err != nil {
		if err == sql.ErrNoRows {
			return pt, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p indexer.PushToken

		err := rows.Scan(&p.Token, &p.Account)
		if err != nil {
			return nil, err
		}

		pt = append(pt, &p)
	}

	return pt, nil
}

// RemoveAccountPushToken removes a push token for a given account from the db
func (db *PushTokenDB) RemoveAccountPushToken(token, account string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	DELETE FROM t_push_token_%s WHERE token = $1 AND account = $2
	`, db.suffix), token, account)

	return err
}

// RemovePushToken removes a push token from the db
func (db *PushTokenDB) RemovePushToken(token string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	DELETE FROM t_push_token_%s WHERE token = $1
	`, db.suffix), token)

	return err
}
