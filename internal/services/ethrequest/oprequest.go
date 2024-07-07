package ethrequest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type OPService struct {
	rpc    *rpc.Client
	client *ethclient.Client
	ctx    context.Context
}

func (e *OPService) Context() context.Context {
	return e.ctx
}

func NewOpService(ctx context.Context, endpoint string) (*OPService, error) {
	rpc, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	cl := ethclient.NewClient(rpc)

	return &OPService{rpc, cl, ctx}, nil
}

func (e *OPService) Close() {
	e.client.Close()
}

func (e *OPService) BlockTime(number *big.Int) (uint64, error) {
	var blk *EthBlock
	err := e.rpc.Call(&blk, "eth_getBlockByNumber", fmt.Sprintf("0x%s", number.Text(16)), true)
	if err != nil {
		return 0, err
	}

	if blk == nil {
		return 0, errors.New("block not found")
	}

	v, err := hexutil.DecodeUint64(blk.Timestamp)
	if err != nil {
		return 0, err
	}

	return v, nil
}

func (e *OPService) CallContract(call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return e.client.CallContract(e.ctx, call, blockNumber)
}

func (e *OPService) ListenForLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) error {
	for {
		sub, err := e.client.SubscribeFilterLogs(ctx, q, ch)
		if err != nil {
			log.Default().Println("error subscribing to logs", err.Error())

			<-time.After(1 * time.Second)

			continue
		}

		select {
		case <-ctx.Done():
			log.Default().Println("context done, unsubscribing")
			sub.Unsubscribe()

			return ctx.Err()
		case err := <-sub.Err():
			// subscription error, try and re-subscribe
			log.Default().Println("subscription error", err.Error())
			sub.Unsubscribe()

			<-time.After(1 * time.Second)

			continue
		}
	}
}

func (e *OPService) Backend() bind.ContractBackend {
	return e.client
}

func (e *OPService) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return e.client.CodeAt(e.ctx, account, blockNumber)
}

func (e *OPService) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return e.client.NonceAt(e.ctx, account, blockNumber)
}

func (e *OPService) BaseFee() (*big.Int, error) {
	// Get the latest block header
	header, err := e.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return header.BaseFee, nil
}

func (e *OPService) EstimateGasPrice() (*big.Int, error) {
	return e.client.SuggestGasPrice(e.ctx)
}

func (e *OPService) EstimateGasLimit(msg ethereum.CallMsg) (uint64, error) {
	return e.client.EstimateGas(e.ctx, msg)
}

func (e *OPService) NewTx(nonce uint64, from, to common.Address, data []byte, extraGas bool) (*types.Transaction, error) {
	baseFee, err := e.BaseFee()
	if err != nil {
		return nil, err
	}

	// Set the priority fee per gas (miner tip)
	tip, err := e.MaxPriorityFeePerGas()
	if err != nil {
		return nil, err
	}

	buffer := new(big.Int).Div(tip, big.NewInt(100))

	maxPriorityFeePerGas := new(big.Int).Add(tip, buffer)

	maxFeePerGas := new(big.Int).Add(maxPriorityFeePerGas, new(big.Int).Mul(baseFee, big.NewInt(2)))

	// Prepare the call message
	msg := ethereum.CallMsg{
		From:     from, // the account executing the function
		To:       &to,
		Gas:      0,    // set to 0 for estimation
		GasPrice: nil,  // set to nil for estimation
		Value:    nil,  // set to nil for estimation
		Data:     data, // the function call data
	}

	gasLimit, err := e.EstimateGasLimit(msg)
	if err != nil {
		return nil, err
	}

	gasFeeCap := new(big.Int).Add(maxFeePerGas, new(big.Int).Div(maxFeePerGas, big.NewInt(10)))
	gasTipCap := new(big.Int).Add(maxPriorityFeePerGas, new(big.Int).Div(maxPriorityFeePerGas, big.NewInt(10)))
	if extraGas {
		gasFeeCap = new(big.Int).Add(maxFeePerGas, new(big.Int).Div(maxFeePerGas, big.NewInt(5)))
		gasTipCap = new(big.Int).Add(maxPriorityFeePerGas, new(big.Int).Div(maxPriorityFeePerGas, big.NewInt(5)))
	}

	// Create a new dynamic fee transaction
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit + (gasLimit / 2), // make sure there is some margin for spikes
		To:        &to,
		Value:     common.Big0,
		Data:      data,
	})
	return tx, nil
}

func (e *OPService) SendTransaction(tx *types.Transaction) error {
	return e.client.SendTransaction(e.ctx, tx)
}

func (e *OPService) MaxPriorityFeePerGas() (*big.Int, error) {
	var hexFee string
	err := e.rpc.Call(&hexFee, "eth_maxPriorityFeePerGas")
	if err != nil {
		return common.Big0, err
	}

	fee := new(big.Int)
	_, ok := fee.SetString(hexFee[2:], 16) // remove the "0x" prefix and parse as base 16
	if !ok {
		return nil, errors.New("invalid hex string")
	}

	return fee, nil
}

func (e *OPService) StorageAt(addr common.Address, slot common.Hash) ([]byte, error) {
	return e.client.StorageAt(e.ctx, addr, slot, nil)
}

func (e *OPService) ChainID() (*big.Int, error) {
	chid, err := e.client.ChainID(e.ctx)
	if err != nil {
		return nil, err
	}

	return chid, nil
}

func (e *OPService) LatestBlock() (*big.Int, error) {
	blk, err := e.client.BlockByNumber(e.ctx, nil)
	if err != nil {
		return common.Big0, err
	}

	return blk.Number(), nil
}

func (e *OPService) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	return e.client.FilterLogs(e.ctx, q)
}

func (e *OPService) WaitForTx(tx *types.Transaction, timeout int) error {
	// Create a context that will be canceled after 4 seconds
	ctx, cancel := context.WithTimeout(e.ctx, time.Duration(timeout)*time.Second)
	defer cancel() // Cancel the context when the function returns

	rcpt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return err
	}

	if rcpt.Status != types.ReceiptStatusSuccessful {
		return errors.New("tx failed")
	}

	return nil
}
