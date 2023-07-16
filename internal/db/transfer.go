package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

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

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?%s", path, dbConfigString))
	if err != nil {
		return nil, err
	}

	if !exists {
		// create table
		err = createTransferTable(db)
		if err != nil {
			return nil, err
		}

		// create indexes
		err = createTransferTableIndexes(db)
		if err != nil {
			return nil, err
		}
	}

	return &TransferDB{
		path: path,
		db:   db,
	}, nil
}

// Close closes the db
func (db *TransferDB) Close() error {
	return db.db.Close()
}

// createTransferTable creates a table to store transfers in the given db
// from_to_addr is an optimization column to allow searching for transfers withouth using OR
func createTransferTable(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE t_transfers (
		hash TEXT NOT NULL PRIMARY KEY,
		tx_hash TEXT NOT NULL,
		token_id INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		from_to_addr TEXT NOT NULL,
		from_addr TEXT NOT NULL,
		to_addr TEXT NOT NULL,
		nonce INTEGER NOT NULL,
		value TEXT NOT NULL,
		data BLOB,
		status TEXT NOT NULL DEFAULT 'success'
	)
	`)

	return err
}

// createTransferTableIndexes creates the indexes for transfers in the given db
func createTransferTableIndexes(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE INDEX idx_transfers_tx_hash ON t_transfers (tx_hash);
	`)
	if err != nil {
		return err
	}

	// multi-token queries
	_, err = db.Exec(`
	CREATE INDEX idx_transfers_date_from_to_addr ON t_transfers (created_at, from_to_addr COLLATE NOCASE);
	`)
	if err != nil {
		return err
	}

	// single-token queries
	_, err = db.Exec(`
	CREATE INDEX idx_transfers_date_from_token_id_to_addr ON t_transfers (created_at, token_id, from_to_addr COLLATE NOCASE);
	`)
	if err != nil {
		return err
	}

	// sending queries
	_, err = db.Exec(`
	CREATE INDEX idx_transfers_status_date_from_tx_hash ON t_transfers (status, created_at, tx_hash);
	`)
	if err != nil {
		return err
	}

	return nil
}

// AddTransfer adds a transfer to the db
func (db *TransferDB) AddTransfer(tx *indexer.Transfer) error {

	// insert transfer on conflict update
	_, err := db.db.Exec(`
	INSERT INTO t_transfers (hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hash) DO UPDATE SET
		tx_hash = excluded.tx_hash,
		token_id = excluded.token_id,
		created_at = excluded.created_at,
		from_to_addr = excluded.from_to_addr,
		from_addr = excluded.from_addr,
		to_addr = excluded.to_addr,
		nonce = excluded.nonce,
		value = excluded.value,
		data = excluded.data,
		status = excluded.status
	`, tx.Hash, tx.TxHash, tx.TokenID, tx.CreatedAt, tx.CombineFromTo(), tx.From, tx.To, tx.Nonce, tx.Value.String(), tx.Data, tx.Status)

	return err
}

// AddTransfers adds a list of transfers to the db
func (db *TransferDB) AddTransfers(tx []*indexer.Transfer) error {

	dbtx, err := db.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	for _, t := range tx {
		// insert transfer on conflict update
		_, err := dbtx.Exec(`
			INSERT INTO t_transfers (hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(hash) DO UPDATE SET
				tx_hash = excluded.tx_hash,
				token_id = excluded.token_id,
				created_at = excluded.created_at,
				from_to_addr = excluded.from_to_addr,
				from_addr = excluded.from_addr,
				to_addr = excluded.to_addr,
				nonce = excluded.nonce,
				value = excluded.value,
				data = excluded.data,
				status = excluded.status
			`, t.Hash, t.TxHash, t.TokenID, t.CreatedAt, t.CombineFromTo(), t.From, t.To, t.Nonce, t.Value.String(), t.Data, t.Status)
		if err != nil {
			return dbtx.Rollback()
		}
	}

	return dbtx.Commit()
}

// SetStatus sets the status of a transfer to pending
func (db *TransferDB) SetStatus(status, hash string) error {
	// if status is success, don't update
	_, err := db.db.Exec(`
	UPDATE t_transfers SET status = ? WHERE hash = ? AND status != 'success'
	`, status, hash)

	return err
}

// SetStatusFromTxHash sets the status of a transfer to pending
func (db *TransferDB) SetStatusFromTxHash(status, txhash string) error {
	// if status is success, don't update
	_, err := db.db.Exec(`
	UPDATE t_transfers SET status = ? WHERE tx_hash = ? AND status != 'success'
	`, status, txhash)

	return err
}

// ReconcileTxHash updates transfers to ensure that there are no duplicates
func (db *TransferDB) ReconcileTxHash(tx *indexer.Transfer) error {
	// check if there are multiple transfers with the same tx_hash
	var count int
	row := db.db.QueryRow(`
	SELECT COUNT(*) FROM t_transfers WHERE tx_hash = ?
	`, tx.TxHash)

	err := row.Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// should be impossible
		return errors.New("no transfer with tx_hash")
	}

	// handle the scenario with multiple transfers with the same tx_hash

	// we can assume that the reason there are multiple transfers with the same tx_hash
	// is because one was inserted due tu indexing, meaning it is confirmed

	// delete all transfers with a given tx_hash
	_, err = db.db.Exec(`
	DELETE FROM t_transfers WHERE tx_hash = ?
	`, tx.TxHash)
	if err != nil {
		return err
	}

	// insert the confirmed transfer
	_, err = db.db.Exec(`
	INSERT OR REPLACE INTO t_transfers (hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'success')
	`, tx.Hash, tx.TxHash, tx.TokenID, tx.CreatedAt, tx.CombineFromTo(), tx.From, tx.To, tx.Nonce, tx.Value.String(), tx.Data)

	return err
}

// SetTxHash sets the tx hash of a transfer with no tx_hash
func (db *TransferDB) SetTxHash(txHash, hash string) error {
	_, err := db.db.Exec(`
	UPDATE t_transfers SET tx_hash = ? WHERE hash = ? AND tx_hash = ''
	`, txHash, hash)

	return err
}

// TransferExists returns true if the transfer tx_hash exists in the db
func (db *TransferDB) TransferExists(txHash string) (bool, error) {
	var count int
	row := db.db.QueryRow(`
	SELECT COUNT(*) FROM t_transfers WHERE tx_hash = ?
	`, txHash)

	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RemoveSendingTransfer removes a sending transfer from the db
func (db *TransferDB) RemoveSendingTransfer(hash string) error {
	_, err := db.db.Exec(`
	DELETE FROM t_transfers WHERE hash = ? AND tx_hash = '' AND status = 'sending'
	`, hash)

	return err
}

// RemovePendingTransfer removes a pending transfer from the db
func (db *TransferDB) RemovePendingTransfer(hash string) error {
	_, err := db.db.Exec(`
	DELETE FROM t_transfers WHERE hash = ? AND tx_hash = '' AND status = 'pending'
	`, hash)

	return err
}

// GetPaginatedTransfers returns the transfers for a given from_addr or to_addr paginated
func (db *TransferDB) GetPaginatedTransfers(tokenId int64, addr string, maxDate indexer.SQLiteTime, limit, offset int) ([]*indexer.Transfer, error) {
	likePattern := fmt.Sprintf("%%%s%%", addr)
	transfers := []*indexer.Transfer{}

	rows, err := db.db.Query(`
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers
		WHERE created_at <= ? AND token_id = ? AND from_to_addr LIKE ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
		`, maxDate, tokenId, likePattern, limit, offset)
	if err != nil {
		if err == sql.ErrNoRows {
			return transfers, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var transfer indexer.Transfer
		var value string

		err := rows.Scan(&transfer.Hash, &transfer.TxHash, &transfer.TokenID, &transfer.CreatedAt, &transfer.FromTo, &transfer.From, &transfer.To, &transfer.Nonce, &value, &transfer.Data, &transfer.Status)
		if err != nil {
			return nil, err
		}

		transfer.Value = new(big.Int)
		transfer.Value.SetString(value, 10)

		transfers = append(transfers, &transfer)
	}

	return transfers, nil
}

// GetNewTransfers returns the transfers for a given from_addr or to_addr from a given date
func (db *TransferDB) GetNewTransfers(tokenId int64, addr string, fromDate indexer.SQLiteTime, limit int) ([]*indexer.Transfer, error) {
	likePattern := fmt.Sprintf("%%%s%%", addr)
	transfers := []*indexer.Transfer{}

	rows, err := db.db.Query(`
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers
		WHERE created_at >= ? AND token_id = ? AND from_to_addr LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
		`, fromDate, tokenId, likePattern, limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return transfers, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var transfer indexer.Transfer
		var value string

		err := rows.Scan(&transfer.Hash, &transfer.TxHash, &transfer.TokenID, &transfer.CreatedAt, &transfer.FromTo, &transfer.From, &transfer.To, &transfer.Nonce, &value, &transfer.Data, &transfer.Status)
		if err != nil {
			return nil, err
		}

		transfer.Value = new(big.Int)
		transfer.Value.SetString(value, 10)

		transfers = append(transfers, &transfer)
	}

	return transfers, nil
}

// GetProcessingTransfers returns the transfers for a given from_addr or to_addr from a given date
func (db *TransferDB) GetProcessingTransfers(limit int) ([]*indexer.Transfer, error) {
	fromDate := indexer.SQLiteTime(time.Now().UTC())
	transfers := []*indexer.Transfer{}

	rows, err := db.db.Query(`
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers
		WHERE (status = ? OR status = ?) AND created_at <= ? AND tx_hash = ''
		ORDER BY created_at DESC
		LIMIT ?
		`, indexer.TransferStatusPending, indexer.TransferStatusSending, fromDate, limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return transfers, nil
		}

		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var transfer indexer.Transfer
		var value string

		err := rows.Scan(&transfer.Hash, &transfer.TxHash, &transfer.TokenID, &transfer.CreatedAt, &transfer.FromTo, &transfer.From, &transfer.To, &transfer.Nonce, &value, &transfer.Data, &transfer.Status)
		if err != nil {
			return nil, err
		}

		transfer.Value = new(big.Int)
		transfer.Value.SetString(value, 10)

		transfers = append(transfers, &transfer)
	}

	return transfers, nil
}
