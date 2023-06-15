package sc

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const (
	ERC20Transfer         = "Transfer(address,address,uint256)"
	ERC721Transfer        = "Transfer(address, address,uint256)"
	ERC1155TransferSingle = "TransferSingle(address,address,address,uint256,uint256)"
	ERC1155TransferBatch  = "TransferBatch(address,address,address,uint256[],uint256[])"
)

type LogERC20Transfer struct {
	From   common.Address
	To     common.Address
	Tokens *big.Int
}

type LogERC721Transfer struct {
	From    common.Address
	To      common.Address
	TokenID *big.Int
}

type LogERC1155Transfer struct {
	From    common.Address
	To      common.Address
	TokenID *big.Int
	Tokens  *big.Int
}
