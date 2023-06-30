package bundler

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"
)

const (
	methodGetUserOperationByHash = "eth_getUserOperationByHash"
)

type Bundler struct {
	ctx context.Context
	rpc *rpc.Client
}

func New(ctx context.Context, endpoint, origin string) (*Bundler, error) {
	rpc, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	rpc.SetHeader("Origin", origin)

	return &Bundler{ctx, rpc}, nil
}

func (e *Bundler) Close() {
	e.rpc.Close()
}

type UserOperation struct {
	Sender               string `json:"sender"`
	Nonce                string `json:"nonce"`
	InitCode             string `json:"initCode"`
	CallData             string `json:"callData"`
	CallGasLimit         string `json:"callGasLimit"`
	VerificationGasLimit string `json:"verificationGasLimit"`
	PreVerificationGas   string `json:"preVerificationGas"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	PaymasterAndData     string `json:"paymasterAndData"`
	Signature            string `json:"signature"`
}

type UserOperationResult struct {
	UserOperation   UserOperation `json:"userOperation"`
	EntryPoint      string        `json:"entryPoint"`
	BlockNumber     int64         `json:"blockNumber"`
	BlockHash       string        `json:"blockHash"`
	TransactionHash string        `json:"transactionHash"`
}

func (b *Bundler) GetUserOperationByHash(hash string) (*UserOperationResult, error) {
	var result UserOperationResult
	err := b.rpc.CallContext(b.ctx, &result, methodGetUserOperationByHash, hash)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
