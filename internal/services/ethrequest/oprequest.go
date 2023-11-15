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
