package ethrequest

import (
	"context"
	"errors"
	"math/big"

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
	err := e.rpc.Call(&blk, "eth_getBlockByNumber", "0x"+common.Bytes2Hex(number.Bytes()), true)
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

func (e *OPService) NewTx(nonce uint64, from, to common.Address, data []byte) (*types.Transaction, error) {
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

func (e *OPService) WaitForTx(tx *types.Transaction) error {
	rcpt, err := bind.WaitMined(e.ctx, e.client, tx)
	if err != nil {
		return err
	}

	if rcpt.Status != types.ReceiptStatusSuccessful {
		return errors.New("tx failed")
	}

	return nil
}
