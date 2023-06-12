package node

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Transfer struct {
	Hash    string         `json:"hash"`
	TokenID int            `json:"token_id"`
	Date    time.Time      `json:"date"`
	From    common.Address `json:"from"`
	To      common.Address `json:"to"`
	Value   big.Int        `json:"value"`
	Data    []byte         `json:"data"`
}
