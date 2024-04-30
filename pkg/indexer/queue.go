package indexer

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type MessageResponse struct {
	Data any
	Err  error
}

type Message struct {
	ID         string
	CreatedAt  time.Time
	RetryCount int
	Message    any
	Response   *chan MessageResponse
}

func (m *Message) Respond(data any, err error) {
	if m.Response == nil {
		return
	}

	*m.Response <- MessageResponse{
		Data: data,
		Err:  err,
	}
}

func (m *Message) WaitForResponse() (any, error) {
	defer m.Close()

	select {
	case resp, ok := <-*m.Response:
		if !ok {
			return nil, fmt.Errorf("response channel is closed")
		}
		// handle response
		if resp.Err != nil {
			return nil, resp.Err
		}

		return resp.Data, nil
	case <-time.After(time.Second * 12): // timeout so that we don't block the request forever in case the queue is stuck
		return nil, fmt.Errorf("request timeout")
	}
}

func (m *Message) Close() {
	if m.Response == nil {
		return
	}

	close(*m.Response)
}

type UserOpMessage struct {
	Paymaster  common.Address
	EntryPoint common.Address
	ChainId    *big.Int
	UserOp     UserOp
	ExtraData  any
}

func newMessage(id string, message any, response *chan MessageResponse) *Message {
	return &Message{
		ID:         id,
		CreatedAt:  time.Now(),
		RetryCount: 0,
		Message:    message,
		Response:   response,
	}
}

func NewTxMessage(pm, entrypoint common.Address, chainId *big.Int, userop UserOp, txdata *TransferData) *Message {
	op := UserOpMessage{
		Paymaster:  pm,
		EntryPoint: entrypoint,
		ChainId:    chainId,
		UserOp:     userop,
		ExtraData:  txdata,
	}

	respch := make(chan MessageResponse)
	return newMessage(common.Bytes2Hex(userop.Signature), op, &respch)
}
