package sc

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const (
	ERC20Transfer         = "Transfer(address,address,uint256)"
	ERC721Transfer        = "Transfer(address indexed from, address indexed to, uint256 indexed tokenId)"
	ERC1155TransferSingle = "TransferSingle(address indexed operator, address indexed from, address indexed to, uint id, uint value)"
	ERC1155TransferBatch  = "TransferBatch(address indexed operator, address indexed from, address indexed to, uint[] ids, uint[] values)"
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
