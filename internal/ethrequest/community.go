package ethrequest

import (
	"math/big"

	"github.com/daobrussels/smartcontracts/pkg/contracts/simpleaccountfactory"
	"github.com/ethereum/go-ethereum/common"
)

type Community struct {
	es *EthService

	EntryPointAddr common.Address

	AccountFactoryAddr common.Address
	AccountFactory     *simpleaccountfactory.Simpleaccountfactory
}

func NewCommunity(es *EthService, entryPointAddr, accountFactoryAddr string) (*Community, error) {
	eaddr := common.HexToAddress(entryPointAddr)
	addr := common.HexToAddress(accountFactoryAddr)

	// instantiate account factory contract
	acc, err := simpleaccountfactory.NewSimpleaccountfactory(addr, es.Client())
	if err != nil {
		return nil, err
	}

	return &Community{
		es:                 es,
		EntryPointAddr:     eaddr,
		AccountFactoryAddr: addr,
		AccountFactory:     acc,
	}, nil
}

// EntryPointNextNonce returns the next nonce for the entry point address
func (c *Community) EntryPointNextNonce() (*big.Int, error) {
	n, err := c.es.Client().NonceAt(c.es.Context(), c.EntryPointAddr, nil)
	if err != nil {
		return nil, err
	}

	return big.NewInt(int64(n)), nil
}

// GetAccount returns the account at the given address
func (c *Community) GetAccount(owner string) (*common.Address, error) {
	addr := common.HexToAddress(owner)

	acc, err := c.AccountFactory.GetAddress(nil, addr, common.Big0)
	if err != nil {
		return nil, err
	}

	return &acc, nil
}
