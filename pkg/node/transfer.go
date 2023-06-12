package node

import (
	"time"
)

type Transfer struct {
	Hash    string    `json:"hash"`
	TokenID int64     `json:"token_id"`
	Date    time.Time `json:"date"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	Value   int64     `json:"value"`
	Data    []byte    `json:"data"`
}
