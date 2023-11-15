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
	err := e.rpc.Call(&blk, "eth_getBlockByNumber", "0x"+common.Bytes2Hex(number.Bytes()), true)
	if err != nil {
		return 0, err
	}

	v, err := hexutil.DecodeUint64(blk.Timestamp)
	if err != nil {
		return 0, err
	}

	return v, nil
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
