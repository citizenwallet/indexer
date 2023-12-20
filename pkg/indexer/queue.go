package indexer

import (
	"math/big"
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
	ChainId   *big.Int
	UserOp    UserOp
}

func newMessage(id string, message any) *Message {
	return &Message{
		ID:         id,
		CreatedAt:  time.Now(),
		RetryCount: 0,
		Message:    message,
	}
}

func NewTxMessage(pm, to common.Address, data []byte, chainId *big.Int, userop UserOp) *Message {
	op := UserOpMessage{
		Paymaster: pm,
		To:        to,
		Data:      data,
		ChainId:   chainId,
		UserOp:    userop,
	}
	return newMessage(common.Bytes2Hex(userop.Signature), op)
}
