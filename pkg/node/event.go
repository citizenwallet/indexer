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
	Address     common.Address `json:"address"`
	State       EventState     `json:"state"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	BlockNumber string         `json:"block_number"`
	Signature   string         `json:"signature"`
	Name        string         `json:"name"`
	Symbol      string         `json:"symbol"`
}
