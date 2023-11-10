package ethrequest

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	ETHEstimateGas        = "eth_estimateGas"
	ETHSendRawTransaction = "eth_sendRawTransaction"
	ETHSign               = "eth_sign"
	ETHChainID            = "eth_chainId"
)

type EthBlock struct {
	Number    string `json:"number"`
	Timestamp string `json:"timestamp"`
}

type EthService struct {
	rpc    *rpc.Client
	client *ethclient.Client
	ctx    context.Context
}

func (e *EthService) Context() context.Context {
	return e.ctx
}

func NewEthService(ctx context.Context, endpoint string) (*EthService, error) {
	rpc, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	client := ethclient.NewClient(rpc)

	return &EthService{rpc, client, ctx}, nil
}

func (e *EthService) Close() {
	e.client.Close()
}

func (e *EthService) BlockTime(number *big.Int) (uint64, error) {
	blk, err := e.client.BlockByNumber(e.ctx, number)
	if err != nil {
		return 0, err
	}

	return blk.Time(), nil
}

func (e *EthService) Backend() bind.ContractBackend {
	return e.client
}

func (e *EthService) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return e.client.CodeAt(e.ctx, account, blockNumber)
}

func (e *EthService) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return e.client.NonceAt(e.ctx, account, blockNumber)
}

func (e *EthService) EstimateFullGas(from common.Address, tx *types.Transaction) (uint64, error) {

	msg := ethereum.CallMsg{
		From:       from,
		To:         tx.To(),
		Gas:        tx.Gas(),
		GasPrice:   tx.GasPrice(),
		GasFeeCap:  tx.GasFeeCap(),
		GasTipCap:  tx.GasTipCap(),
		Value:      tx.Value(),
		Data:       tx.Data(),
		AccessList: tx.AccessList(),
	}

	return e.client.EstimateGas(e.ctx, msg)
}

func (e *EthService) EstimateGas(from, to string, value uint64) (uint64, error) {
	t := common.HexToAddress(to)

	msg := ethereum.CallMsg{
		From:  common.HexToAddress(from),
		To:    &t,
		Value: big.NewInt(int64(value)),
		Gas:   0,
	}

	return e.client.EstimateGas(e.ctx, msg)
}

func (e *EthService) EstimateGasPrice(from string, value uint64, data []byte) (uint64, error) {
	msg := ethereum.CallMsg{
		From:  common.HexToAddress(from),
		Value: big.NewInt(int64(value)),
		Data:  data,
		Gas:   0,
	}

	return e.client.EstimateGas(e.ctx, msg)
}

func (e *EthService) EstimateContractGasPrice(data []byte) (uint64, error) {
	msg := ethereum.CallMsg{
		Data: data,
		Gas:  0,
	}

	return e.client.EstimateGas(e.ctx, msg)
}

func (e *EthService) Sign(addr string, message string) (string, error) {

	var sig string
	err := e.rpc.Call(&sig, ETHSign, addr, message)
	if err != nil {
		return "", err
	}

	return sig, err
}

func (e *EthService) SendRawTransaction(tx string) ([]byte, error) {

	err := e.rpc.Call(nil, ETHSendRawTransaction, tx)

	return nil, err
}

func (e *EthService) LatestBlock() (*big.Int, error) {
	blk, err := e.client.BlockByNumber(e.ctx, nil)
	if err != nil {
		return common.Big0, err
	}

	return blk.Number(), nil
}

func (e *EthService) BlockByNumber(number *big.Int) (*types.Block, error) {
	return e.client.BlockByNumber(e.ctx, number)
}

func (e *EthService) TransactionByHash(hash common.Hash) (tx *types.Transaction, isPending bool, err error) {
	return e.client.TransactionByHash(e.ctx, hash)
}

func (e *EthService) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	return e.client.FilterLogs(e.ctx, q)
}

func (e *EthService) ChainID() (*big.Int, error) {
	var id string
	err := e.rpc.Call(&id, ETHChainID)
	if err != nil {
		return nil, err
	}

	chid, ok := big.NewInt(0).SetString(strip0x(id), 16)
	if !ok {
		return nil, errors.New("invalid chain id")
	}

	return chid, nil
}

func (e *EthService) NextNonce(address string) (uint64, error) {
	return e.client.PendingNonceAt(e.ctx, common.HexToAddress(address))
}

func (e *EthService) GetCode(address common.Address) ([]byte, error) {
	return e.client.CodeAt(e.ctx, address, nil)
}

func makeValidEvenHex(h string) string {
	h = strip0x(h)
	h = evenHex(h)
	return "0x" + h
}

func strip0x(h string) string {
	if len(h) > 2 && h[:2] == "0x" {
		return h[2:]
	}

	return h
}

func evenHex(h string) string {
	if len(h)%2 == 0 {
		return h
	}

	return "0" + h
}
