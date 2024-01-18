package indexer

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type TransferStatus string

const (
	TransferStatusUnknown TransferStatus = ""
	TransferStatusSending TransferStatus = "sending"
	TransferStatusPending TransferStatus = "pending"
	TransferStatusSuccess TransferStatus = "success"
	TransferStatusFail    TransferStatus = "fail"

	TEMP_HASH_PREFIX = "TEMP_HASH"
)

func TransferStatusFromString(s string) (TransferStatus, error) {
	switch s {
	case "sending":
		return TransferStatusSending, nil
	case "pending":
		return TransferStatusPending, nil
	case "success":
		return TransferStatusSuccess, nil
	case "fail":
		return TransferStatusFail, nil
	}

	return TransferStatusUnknown, errors.New("unknown role: " + s)
}

type Transfer struct {
	Hash      string         `json:"hash"`
	TxHash    string         `json:"tx_hash"`
	TokenID   int64          `json:"token_id"`
	CreatedAt time.Time      `json:"created_at"`
	FromTo    string         `json:"-"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Nonce     int64          `json:"nonce"`
	Value     *big.Int       `json:"value"`
	Data      *TransferData  `json:"data"`
	Status    TransferStatus `json:"status"`
}

type TransferData struct {
	Description string `json:"description"`
}

// TransferData implements the sql.Scanner interface
func (td *TransferData) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("Type assertion .([]byte) failed.")
	}

	if len(b) == 0 {
		return nil
	}

	return json.Unmarshal(b, td)
}

// TransferData implements the driver.Valuer interface
func (td TransferData) Value() (driver.Value, error) {
	return json.Marshal(td)
}

func (t *Transfer) CombineFromTo() string {
	return fmt.Sprintf("%s_%s", t.From, t.To)
}

// generate hash for transfer using chainID, tokenID, nonce, from, to, value
func (t *Transfer) GenerateTempHash(chainID int64) {
	buf := new(bytes.Buffer)

	// Write each value to the buffer as bytes
	binary.Write(buf, binary.BigEndian, chainID)
	binary.Write(buf, binary.BigEndian, t.TokenID)
	binary.Write(buf, binary.BigEndian, t.Nonce)
	buf.Write(common.Hex2Bytes(t.From))
	buf.Write(common.Hex2Bytes(t.To))
	buf.Write(t.Value.Bytes())

	hash := crypto.Keccak256Hash(buf.Bytes())
	t.Hash = fmt.Sprintf("%s_%s", TEMP_HASH_PREFIX, hash.Hex())
}

func (t *Transfer) ToRounded(decimals int64) float64 {
	v, _ := t.Value.Float64()

	if decimals == 0 {
		return v
	}

	// Calculate value * 10^x
	multiplier, _ := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil).Float64()

	result, _ := new(big.Float).Quo(big.NewFloat(v), big.NewFloat(multiplier)).Float64()

	return result
}

// Update updates the transfer using the given transfer
func (t *Transfer) Update(tx *Transfer) {
	// update all fields
	t.Hash = tx.Hash
	t.TxHash = tx.TxHash
	t.TokenID = tx.TokenID
	t.CreatedAt = tx.CreatedAt
	t.FromTo = tx.FromTo
	t.From = tx.From
	t.To = tx.To
	t.Nonce = tx.Nonce
	t.Value = tx.Value
	t.Data = tx.Data
	t.Status = tx.Status
}
