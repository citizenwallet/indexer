package chain

import (
	"math/big"
	"net/http"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

type Service struct {
	evm     indexer.EVMRequester
	chainId *big.Int
}

// NewService
func NewService(evm indexer.EVMRequester, chid *big.Int) *Service {
	return &Service{
		evm,
		chid,
	}
}

func (s *Service) ChainId(r *http.Request) (any, int) {
	// Return the message ID
	return s.chainId, http.StatusOK
}
