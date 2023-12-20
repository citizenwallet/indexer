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

type UserOpMessage struct {
	Paymaster common.Address
	To        common.Address
	Data      []byte
	UserOp    UserOp
}

func newMessage(message any) *Message {
	return &Message{
		ID:         RandomString(32),
		CreatedAt:  time.Now(),
		RetryCount: 0,
		Message:    message,
	}
}

func NewTxMessage(pm, to common.Address, data []byte, userop UserOp) *Message {
	tx := UserOpMessage{
		Paymaster: pm,
		To:        to,
		Data:      data,
		UserOp:    userop,
	}
	return newMessage(tx)
}
