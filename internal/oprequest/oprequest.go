package oprequest

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	ETHEstimateGas        = "eth_estimateGas"
	ETHSendRawTransaction = "eth_sendRawTransaction"
	ETHSign               = "eth_sign"
	ETHChainID            = "eth_chainId"
)

type OPService struct {
	client *ethclient.Client
	ctx    context.Context
}

func (e *OPService) Client() *ethclient.Client {
	return e.client
}

func (e *OPService) Context() context.Context {
	return e.ctx
}

func NewEthService(ctx context.Context, endpoint string) (*OPService, error) {
	cl, err := client.DialEthClientWithTimeout(ctx, endpoint, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &OPService{cl, ctx}, nil
}

func (e *OPService) Close() {
	e.client.Close()
}

func (e *OPService) ChainID() (*big.Int, error) {
	chid, err := e.client.ChainID(e.ctx)
	if err != nil {
		return nil, err
	}

	return chid, nil
}

func (e *OPService) LatestBlock() (*types.Block, error) {
	return e.client.BlockByNumber(e.ctx, nil)
}

func (e *OPService) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	return e.client.FilterLogs(e.ctx, q)
}

func (e *OPService) BlockByNumber(number *big.Int) (*types.Block, error) {
	return e.client.BlockByNumber(e.ctx, number)
}
