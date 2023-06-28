package ethrequest

import (
	"github.com/daobrussels/smartcontracts/pkg/contracts/simpleaccountfactory"
	"github.com/ethereum/go-ethereum/common"
)

type Community struct {
	es *EthService

	AccountFactoryAddr common.Address `json:"accountFactory"`
	AccountFactory     *simpleaccountfactory.Simpleaccountfactory
}

func NewCommunity(es *EthService, accountFactoryAddr string) (*Community, error) {
	addr := common.HexToAddress(accountFactoryAddr)

	// instantiate account factory contract
	acc, err := simpleaccountfactory.NewSimpleaccountfactory(addr, es.Client())
	if err != nil {
		return nil, err
	}

	return &Community{
		es:                 es,
		AccountFactoryAddr: addr,
		AccountFactory:     acc,
	}, nil
}

// GetAccount returns the account at the given address
func (c *Community) GetAccount(owner string) (*common.Address, error) {
	addr := common.HexToAddress(owner)

	acc, err := c.AccountFactory.GetAddress(nil, addr, common.Big0)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return &acc, nil
}
