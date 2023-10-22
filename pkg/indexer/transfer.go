package indexer

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

type TransferStatus string

const (
	TransferStatusUnknown TransferStatus = ""
	TransferStatusSending TransferStatus = "sending"
	TransferStatusPending TransferStatus = "pending"
	TransferStatusSuccess TransferStatus = "success"
	TransferStatusFail    TransferStatus = "fail"
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
	Data      []byte         `json:"data"`
	Status    TransferStatus `json:"status"`
}

func (t *Transfer) CombineFromTo() string {
	return fmt.Sprintf("%s_%s", t.From, t.To)
}

// generate hash for transfer using chainID, tokenID, nonce, from, to, value
func (t *Transfer) GenerateHash(chainID int64) {
	hash := crypto.Keccak256Hash([]byte(fmt.Sprintf("%d_%d_%s_%s_%s_%d_%d", chainID, t.TokenID, t.CreatedAt, t.From, t.To, t.Nonce, t.Value)))
	t.Hash = hash.Hex()
}
