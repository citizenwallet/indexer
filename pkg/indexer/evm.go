package indexer

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type EVMType string

const (
	EVMTypeEthereum EVMType = "ethereum"
	EVMTypeOptimism EVMType = "optimism"
	EVMTypeCelo     EVMType = "celo"
)

type EVMRequester interface {
	Context() context.Context
	Backend() bind.ContractBackend

	CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	BaseFee() (*big.Int, error)
	EstimateGasPrice() (*big.Int, error)
	EstimateGasLimit(msg ethereum.CallMsg) (uint64, error)
	NewTx(nonce uint64, from, to common.Address, data []byte) (*types.Transaction, error)
	SendTransaction(tx *types.Transaction) error
	StorageAt(addr common.Address, slot common.Hash) ([]byte, error)

	ChainID() (*big.Int, error)
	LatestBlock() (*big.Int, error)
	FilterLogs(q ethereum.FilterQuery) ([]types.Log, error)
	BlockTime(number *big.Int) (uint64, error)
	ListenForLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) error

	WaitForTx(tx *types.Transaction) error

	Close()
}
