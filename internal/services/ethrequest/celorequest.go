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

type CeloService struct {
	rpc    *rpc.Client
	client *ethclient.Client
	ctx    context.Context
}

func (e *CeloService) Context() context.Context {
	return e.ctx
}

func NewCeloService(ctx context.Context, endpoint string) (*CeloService, error) {
	rpc, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	cl := ethclient.NewClient(rpc)

	return &CeloService{rpc, cl, ctx}, nil
}

func (e *CeloService) Close() {
	e.client.Close()
}

// BlockTime returns the timestamp of the block at the given number
func (e *CeloService) BlockTime(number *big.Int) (uint64, error) {
	// Celo Blocks has a slightly different format than Ethereum Blocks, so we need to use a custom Block struct
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

func (e *CeloService) ListenForLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) error {
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

func (e *CeloService) Backend() bind.ContractBackend {
	return e.client
}

func (e *CeloService) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return e.client.CodeAt(e.ctx, account, blockNumber)
}

func (e *CeloService) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return e.client.NonceAt(e.ctx, account, blockNumber)
}

func (e *CeloService) BaseFee() (*big.Int, error) {
	// Get the latest block header
	header, err := e.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return header.BaseFee, nil
}

func (e *CeloService) EstimateGasPrice() (*big.Int, error) {
	return e.client.SuggestGasPrice(e.ctx)
}

func (e *CeloService) EstimateGasLimit(msg ethereum.CallMsg) (uint64, error) {
	return e.client.EstimateGas(e.ctx, msg)
}

func (e *CeloService) NewTx(nonce uint64, from, to common.Address, data []byte) (*types.Transaction, error) {
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

	// Create a new dynamic fee transaction
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		GasFeeCap: maxFeePerGas,
		GasTipCap: maxPriorityFeePerGas,
		Gas:       gasLimit + (gasLimit / 2), // make sure there is some margin for spikes
		To:        &to,
		Value:     common.Big0,
		Data:      data,
	})
	return tx, nil
}

func (e *CeloService) SendTransaction(tx *types.Transaction) error {
	return e.client.SendTransaction(e.ctx, tx)
}

func (e *CeloService) MaxPriorityFeePerGas() (*big.Int, error) {
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

func (e *CeloService) StorageAt(addr common.Address, slot common.Hash) ([]byte, error) {
	return e.client.StorageAt(e.ctx, addr, slot, nil)
}

func (e *CeloService) ChainID() (*big.Int, error) {
	chid, err := e.client.ChainID(e.ctx)
	if err != nil {
		return nil, err
	}

	return chid, nil
}

func (e *CeloService) LatestBlock() (*big.Int, error) {
	var blk *EthBlock
	err := e.rpc.Call(&blk, "eth_getBlockByNumber", "latest", true)
	if err != nil {
		return common.Big0, err
	}

	v, err := hexutil.DecodeBig(blk.Number)
	if err != nil {
		return common.Big0, err
	}
	return v, nil
}

func (e *CeloService) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	return e.client.FilterLogs(e.ctx, q)
}

func (e *CeloService) WaitForTx(tx *types.Transaction) error {
	rcpt, err := bind.WaitMined(e.ctx, e.client, tx)
	if err != nil {
		return err
	}

	if rcpt.Status != types.ReceiptStatusSuccessful {
		return errors.New("tx failed")
	}

	return nil
}
