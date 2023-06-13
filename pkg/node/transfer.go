package node

import "math/big"

type Transfer struct {
	Hash      string     `json:"hash"`
	TokenID   int64      `json:"token_id"`
	CreatedAt SQLiteTime `json:"created_at"`
	From      string     `json:"from_addr"`
	To        string     `json:"to_addr"`
	Value     *big.Int   `json:"value"`
	Data      []byte     `json:"data"`
}
