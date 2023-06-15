package indexer

import "math/big"

type Transfer struct {
	Hash      string     `json:"hash"`
	TokenID   int64      `json:"token_id"`
	CreatedAt SQLiteTime `json:"created_at"`
	FromTo    string     `json:"-"`
	From      string     `json:"from"`
	To        string     `json:"to"`
	Value     *big.Int   `json:"value"`
	Data      []byte     `json:"data"`
}
