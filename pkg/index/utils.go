package index

import (
	"errors"
	"strings"

	"github.com/citizenwallet/indexer/internal/sc"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc1155"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc20"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc721"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func GetContractABI(std indexer.Standard) (*abi.ABI, error) {
	var contractAbi abi.ABI
	var err error

	switch std {
	case indexer.ERC20:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc20.Erc20MetaData.ABI)))
		if err != nil {
			return nil, err
		}
	case indexer.ERC721:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc721.Erc721MetaData.ABI)))
		if err != nil {
			return nil, err
		}
	case indexer.ERC1155:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc1155.Erc1155MetaData.ABI)))
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported token standard")
	}

	return &contractAbi, nil
}

func GetContractTopics(std indexer.Standard) [][]common.Hash {
	var topics [][]common.Hash

	switch std {
	case indexer.ERC20:
		topics = [][]common.Hash{{crypto.Keccak256Hash([]byte(sc.ERC20Transfer))}}
	case indexer.ERC721:
		topics = [][]common.Hash{{crypto.Keccak256Hash([]byte(sc.ERC721Transfer))}}
	case indexer.ERC1155:
		topics = [][]common.Hash{{crypto.Keccak256Hash([]byte(sc.ERC1155TransferSingle)), crypto.Keccak256Hash([]byte(sc.ERC1155TransferBatch))}}
	default:
		topics = [][]common.Hash{}
	}

	return topics
}
