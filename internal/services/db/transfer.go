package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type TransferDB struct {
	suffix string
	db     *sql.DB
	rdb    *sql.DB
}

// NewTransferDB creates a new DB
func NewTransferDB(db, rdb *sql.DB, name string) (*TransferDB, error) {
	txdb := &TransferDB{
		suffix: name,
		db:     db,
		rdb:    rdb,
	}

	return txdb, nil
}

// Close closes the db
func (db *TransferDB) Close() error {
	return db.db.Close()
}

func (db *TransferDB) CloseR() error {
	return db.rdb.Close()
}

// createTransferTable creates a table to store transfers in the given db
// from_to_addr is an optimization column to allow searching for transfers withouth using OR
func (db *TransferDB) CreateTransferTable() error {
	_, err := db.db.Exec(fmt.Sprintf(`
	CREATE TABLE t_transfers_%s(
		hash TEXT NOT NULL PRIMARY KEY,
		tx_hash text NOT NULL,
		token_id integer NOT NULL,
		created_at timestamp NOT NULL,
		from_to_addr text NOT NULL,
		from_addr text NOT NULL,
		to_addr text NOT NULL,
		nonce integer NOT NULL,
		value text NOT NULL,
		data bytea DEFAULT '{}',
		status text NOT NULL DEFAULT 'success'
	);
	`, db.suffix))

	return err
}

// createTransferTableIndexes creates the indexes for transfers in the given db
func (db *TransferDB) CreateTransferTableIndexes() error {
	suffix := common.ShortenName(db.suffix, 6)

	_, err := db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_transfers_%s_tx_hash ON t_transfers_%s (tx_hash);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// filtering by address
	_, err = db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_transfers_%s_to_addr ON t_transfers_%s (to_addr);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_transfers_%s_from_addr ON t_transfers_%s (from_addr);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// single-token queries
	_, err = db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_transfers_%s_date_from_token_id_from_addr_simple ON t_transfers_%s (created_at, token_id, from_addr);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	_, err = db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_transfers_%s_date_from_token_id_to_addr_simple ON t_transfers_%s (created_at, token_id, to_addr);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	// sending queries
	_, err = db.db.Exec(fmt.Sprintf(`
	CREATE INDEX idx_transfers_%s_status_date_from_tx_hash ON t_transfers_%s (status, created_at, tx_hash);
	`, suffix, db.suffix))
	if err != nil {
		return err
	}

	return nil
}

// AddTransfer adds a transfer to the db
func (db *TransferDB) AddTransfer(tx *indexer.Transfer) error {

	// insert transfer on conflict update
	_, err := db.db.Exec(fmt.Sprintf(`
	INSERT INTO t_transfers_%s (hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
	`, db.suffix), tx.Hash, tx.TxHash, tx.TokenID, tx.CreatedAt, tx.CombineFromTo(), tx.From, tx.To, tx.Nonce, tx.Value.String(), tx.Data, tx.Status)

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
		_, err := dbtx.Exec(fmt.Sprintf(`
			INSERT INTO t_transfers_%s (hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
			`, db.suffix), t.Hash, t.TxHash, t.TokenID, t.CreatedAt, t.CombineFromTo(), t.From, t.To, t.Nonce, t.Value.String(), t.Data, t.Status)
		if err != nil {
			return dbtx.Rollback()
		}
	}

	return dbtx.Commit()
}

// SetStatus sets the status of a transfer to pending
func (db *TransferDB) SetStatus(status, hash string) error {
	// if status is success, don't update
	_, err := db.db.Exec(fmt.Sprintf(`
	UPDATE t_transfers_%s SET status = $1 WHERE hash = $2 AND status != 'success'
	`, db.suffix), status, hash)

	return err
}

// SetStatusFromTxHash sets the status of a transfer to pending
func (db *TransferDB) SetStatusFromTxHash(status, txhash string) error {
	// if status is success, don't update
	_, err := db.db.Exec(fmt.Sprintf(`
	UPDATE t_transfers_%s SET status = $1 WHERE tx_hash = $2 AND status != 'success'
	`, db.suffix), status, txhash)

	return err
}

// ReconcileTxHash updates transfers to ensure that there are no duplicates
func (db *TransferDB) ReconcileTxHash(tx *indexer.Transfer) error {
	// check if there are multiple transfers with the same tx_hash
	var count int
	row := db.rdb.QueryRow(fmt.Sprintf(`
	SELECT COUNT(*) FROM t_transfers_%s WHERE tx_hash = $1
	`, db.suffix), tx.TxHash)

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
	_, err = db.db.Exec(fmt.Sprintf(`
	DELETE FROM t_transfers_%s WHERE tx_hash = $1
	`, db.suffix), tx.TxHash)
	if err != nil {
		return err
	}

	// insert the confirmed transfer
	_, err = db.db.Exec(fmt.Sprintf(`
	INSERT INTO t_transfers_%s (hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'success')
	ON CONFLICT (hash) DO UPDATE SET 
		tx_hash = EXCLUDED.tx_hash,
		token_id = EXCLUDED.token_id,
		created_at = EXCLUDED.created_at,
		from_to_addr = EXCLUDED.from_to_addr,
		from_addr = EXCLUDED.from_addr,
		to_addr = EXCLUDED.to_addr,
		nonce = EXCLUDED.nonce,
		value = EXCLUDED.value,
		data = EXCLUDED.data,
		status = EXCLUDED.status;
	`, db.suffix), tx.Hash, tx.TxHash, tx.TokenID, tx.CreatedAt, tx.CombineFromTo(), tx.From, tx.To, tx.Nonce, tx.Value.String(), tx.Data)

	return err
}

// SetTxHash sets the tx hash of a transfer with no tx_hash
func (db *TransferDB) SetTxHash(txHash, hash string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	UPDATE t_transfers_%s SET tx_hash = $1 WHERE hash = $2 AND tx_hash = ''
	`, db.suffix), txHash, hash)

	return err
}

// TransferExists returns true if the transfer tx_hash exists in the db
func (db *TransferDB) TransferExists(txHash string) (bool, error) {
	var count int
	row := db.rdb.QueryRow(fmt.Sprintf(`
	SELECT COUNT(*) FROM t_transfers_%s WHERE tx_hash = $1
	`, db.suffix), txHash)

	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RemoveSendingTransfer removes a sending transfer from the db
func (db *TransferDB) RemoveSendingTransfer(hash string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	DELETE FROM t_transfers_%s WHERE hash = $1 AND tx_hash = '' AND status = 'sending'
	`, db.suffix), hash)

	return err
}

// RemovePendingTransfer removes a pending transfer from the db
func (db *TransferDB) RemovePendingTransfer(hash string) error {
	_, err := db.db.Exec(fmt.Sprintf(`
	DELETE FROM t_transfers_%s WHERE hash = $1 AND tx_hash = '' AND status = 'pending'
	`, db.suffix), hash)

	return err
}

// GetPaginatedTransfers returns the transfers for a given from_addr or to_addr paginated
func (db *TransferDB) GetPaginatedTransfers(tokenId int64, addr string, maxDate time.Time, limit, offset int) ([]*indexer.Transfer, error) {
	transfers := []*indexer.Transfer{}

	rows, err := db.rdb.Query(fmt.Sprintf(`
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		WHERE created_at <= $1 AND token_id = $2 AND from_addr = $3
		UNION ALL
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		WHERE created_at <= $4 AND token_id = $5 AND to_addr = $6
		ORDER BY created_at DESC
		LIMIT $7 OFFSET $8
		`, db.suffix, db.suffix), maxDate, tokenId, addr, maxDate, tokenId, addr, limit, offset)
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
func (db *TransferDB) GetNewTransfers(tokenId int64, addr string, fromDate time.Time, limit int) ([]*indexer.Transfer, error) {
	transfers := []*indexer.Transfer{}

	rows, err := db.rdb.Query(fmt.Sprintf(`
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		WHERE created_at >= $1 AND token_id = $2 AND from_addr = $3
		UNION ALL
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		WHERE created_at >= $4 AND token_id = $5 AND to_addr = $6
		ORDER BY created_at DESC
		LIMIT $7
		`, db.suffix, db.suffix), fromDate, tokenId, addr, fromDate, tokenId, addr, limit)
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
	fromDate := time.Now().UTC()
	transfers := []*indexer.Transfer{}

	rows, err := db.rdb.Query(fmt.Sprintf(`
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		WHERE status = $1 AND created_at <= $2 AND tx_hash = ''
		UNION ALL
		SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
		FROM t_transfers_%s
		WHERE status = $3 AND created_at <= $4 AND tx_hash = ''
		ORDER BY created_at DESC
		LIMIT $5
		`, db.suffix, db.suffix), indexer.TransferStatusPending, fromDate, indexer.TransferStatusSending, fromDate, limit)
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
