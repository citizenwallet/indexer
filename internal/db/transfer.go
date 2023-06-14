package db

import (
	"database/sql"
	"fmt"
	"math/big"

	"github.com/citizenwallet/indexer/internal/storage"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type TransferDB struct {
	path string
	db   *sql.DB
}

// NewTransferDB creates a new DB
func NewTransferDB(name string) (*TransferDB, error) {

	basePath := storage.GetUserHomeDir()
	path := fmt.Sprintf("%s/%s/logs_%s.db", basePath, dbBaseFolder, name)

	// check if db exists before opening, since we use rwc mode
	exists := storage.Exists(path)

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc", path))
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = createTransferTable(db)
		if err != nil {
			println("error creating transfer table")
			return nil, err
		}

		// create indexes
		err = createTransferTableIndexes(db)
		if err != nil {
			println("error creating transfer table indexes")
			return nil, err
		}
	}

	return &TransferDB{
		path: path,
		db:   db,
	}, nil
}

// createTransferTable creates a table to store transfers in the given db
// from_to_addr is an optimization column to allow searching for transfers withouth using OR
func createTransferTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE t_transfers (
		hash TEXT NOT NULL PRIMARY KEY,
		token_id INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		from_to_addr TEXT NOT NULL,
		from_addr TEXT NOT NULL,
		to_addr TEXT NOT NULL,
		value TEXT NOT NULL,
		data BLOB
	)
	`)

	return err
}

// createTransferTableIndexes creates the indexes for transfers in the given db
func createTransferTableIndexes(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE INDEX idx_transfers_token_id_date ON t_transfers (token_id, created_at);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_transfers_date_from_to_addr ON t_transfers (created_at, from_to_addr COLLATE NOCASE);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE INDEX idx_transfers_token_id_from_to_addr_date ON t_transfers (token_id, from_to_addr COLLATE NOCASE, created_at);
	`)
	if err != nil {
		return err
	}

	return nil
}

// AddTransfer adds a transfer to the db
func (db *TransferDB) AddTransfer(hash string, tokenID int64, createdAt string, fromAddr string, toAddr string, value *big.Int, data []byte) error {

	// insert transfer on conflict update
	_, err := db.db.Exec(`
	INSERT INTO t_transfers (hash, token_id, created_at, from_to_addr, from_addr, to_addr, value, data)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hash) DO UPDATE SET
		token_id = excluded.token_id,
		created_at = excluded.created_at,
		from_to_addr = excluded.from_to_addr,
		from_addr = excluded.from_addr,
		to_addr = excluded.to_addr,
		value = excluded.value,
		data = excluded.data
	`, hash, tokenID, createdAt, combineFromTo(fromAddr, toAddr), fromAddr, toAddr, value.String(), data)

	return err
}

// GetTransfers returns the transfers for a given from_to_addr between a created_at range
func (db *TransferDB) GetTransfers(tokenID int64, addr string, fromCreatedAt string, toCreatedAt string) ([]*indexer.Transfer, error) {
	var transfers []*indexer.Transfer

	rows, err := db.db.Query(`
		SELECT hash, token_id, created_at, from_addr, to_addr, value, data
		FROM t_transfers
		WHERE token_id = ? AND from_to_addr LIKE ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
		`, tokenID, addr, fromCreatedAt, toCreatedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var transfer indexer.Transfer
		var value string

		err := rows.Scan(&transfer.Hash, &transfer.TokenID, &transfer.CreatedAt, &transfer.From, &transfer.To, &value, &transfer.Data)
		if err != nil {
			return nil, err
		}

		transfer.Value = new(big.Int)
		transfer.Value.SetString(value, 10)

		transfers = append(transfers, &transfer)
	}

	return transfers, nil
}

// GetPaginatedTransfers returns the transfers for a given from_addr or to_addr paginated
func (db *TransferDB) GetPaginatedTransfers(addr string, maxDate indexer.SQLiteTime, limit, offset int) ([]*indexer.Transfer, int, error) {
	likePattern := fmt.Sprintf("%%%s%%", addr)

	// get the total count of transfers for a from_to_addr
	var total int
	row := db.db.QueryRow(`
		SELECT COUNT(*)
		FROM t_transfers
		WHERE created_at <= ? AND from_to_addr LIKE ?
		`, maxDate, likePattern)

	err := row.Scan(&total)
	if err != nil {
		return nil, total, err
	}

	transfers := []*indexer.Transfer{}

	if total == 0 {
		return transfers, total, nil
	}

	rows, err := db.db.Query(`
		SELECT hash, token_id, created_at, from_to_addr, from_addr, to_addr, value, data
		FROM t_transfers
		WHERE created_at <= ? AND from_to_addr LIKE ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
		`, maxDate, likePattern, limit, offset)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	for rows.Next() {
		var transfer indexer.Transfer
		var value string

		err := rows.Scan(&transfer.Hash, &transfer.TokenID, &transfer.CreatedAt, &transfer.FromTo, &transfer.From, &transfer.To, &value, &transfer.Data)
		if err != nil {
			return nil, total, err
		}

		transfer.Value = new(big.Int)
		transfer.Value.SetString(value, 10)

		transfers = append(transfers, &transfer)
	}

	return transfers, total, nil
}

func combineFromTo(fromAddr string, toAddr string) string {
	return fmt.Sprintf("%s_%s", fromAddr, toAddr)
}
