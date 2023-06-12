package node

import (
	"time"
)

type EventState string

const (
	EventStateQueued   EventState = "queued"
	EventStateIndexing EventState = "indexing"
	EventStateIndexed  EventState = "indexed"
)

type Event struct {
	Address    string     `json:"address"`
	State      EventState `json:"state"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	StartBlock int64      `json:"start_block"`
	LastBlock  int64      `json:"last_block"`
	Signature  string     `json:"signature"`
	Name       string     `json:"name"`
	Symbol     string     `json:"symbol"`
}
