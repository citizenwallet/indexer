package node

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type EventState string

const (
	EventStateQueued   EventState = "queued"
	EventStateIndexing EventState = "indexing"
	EventStateIndexed  EventState = "indexed"
)

type Event struct {
	Address    common.Address `json:"address"`
	ChainID    int            `json:"chain_id"`
	State      EventState     `json:"state"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	StartBlock string         `json:"start_block"`
	LastBlock  string         `json:"last_block"`
	Signature  string         `json:"signature"`
	Name       string         `json:"name"`
	Symbol     string         `json:"symbol"`
}
