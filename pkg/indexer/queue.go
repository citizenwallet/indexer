package indexer

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Message struct {
	ID         string
	CreatedAt  time.Time
	RetryCount int
	Message    any
}

type TxMessage struct {
	From common.Address
	To   common.Address
	Data []byte
}

func newMessage(message any) *Message {
	return &Message{
		ID:         RandomString(32),
		CreatedAt:  time.Now(),
		RetryCount: 0,
		Message:    message,
	}
}

func NewTxMessage(from, to common.Address, data []byte) *Message {
	tx := &TxMessage{
		From: from,
		To:   to,
		Data: data,
	}
	return newMessage(tx)
}
