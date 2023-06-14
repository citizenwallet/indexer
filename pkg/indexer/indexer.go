package indexer

import (
	"errors"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/citizenwallet/node/internal/db"
	"github.com/citizenwallet/node/internal/ethrequest"
	"github.com/citizenwallet/node/internal/sc"
	"github.com/citizenwallet/node/pkg/node"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc1155"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc20"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc721"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	rate            = 999 // how many blocks to process at a time
	backgroundSleep = 10  // how many seconds to sleep between background checks
)

type Indexer struct {
	chainID *big.Int
	db      *db.DB
	eth     *ethrequest.EthService
}

func New(chainID *big.Int, db *db.DB, eth *ethrequest.EthService) *Indexer {
	return &Indexer{
		chainID: chainID,
		db:      db,
		eth:     eth,
	}
}

// Start starts the indexer service
func (i *Indexer) Start() error {
	// get the latest block
	latestBlock, err := i.eth.LatestBlock()
	if err != nil {
		return err
	}
	curr := latestBlock.Number()

	// check if there are any queued events
	evs, err := i.db.EventDB.GetOutdatedEvents(curr.Int64())
	if err != nil {
		return err
	}

	return i.Process(evs, curr)
}

// Background starts an indexer service in the background
func (i *Indexer) Background() error {
	err := i.Start()
	if err != nil {
		return err
	}

	for {
		// get the latest block
		latestBlock, err := i.eth.LatestBlock()
		if err != nil {
			return err
		}
		curr := latestBlock.Number()

		// check if there are any queued events
		evs, err := i.db.EventDB.GetQueuedEvents()
		if err != nil {
			return err
		}

		err = i.Process(evs, curr)
		if err != nil {
			log.Default().Println("indexer error: ", err)
		}

		time.Sleep(backgroundSleep * time.Second)
	}
}

// Process events
func (i *Indexer) Process(evs []*node.Event, curr *big.Int) error {
	if len(evs) == 0 {
		// nothing to do
		return nil
	}

	log.Default().Println("indexing ", len(evs), " events")

	// iterate over events and index them
	for _, ev := range evs {
		err := i.Index(ev, curr)
		if err != nil {
			return err
		}
	}

	log.Default().Println("indexing done")

	return nil
}

func (i *Indexer) Index(ev *node.Event, curr *big.Int) error {
	log.Default().Println("indexing event: ", ev.Contract, ev.Function, " from block: ", ev.LastBlock, " to block: ", curr.Int64(), " ...")

	// check if the event last block matches the latest block of the chain
	if ev.LastBlock >= curr.Int64() {
		// event is up to date
		err := i.db.EventDB.SetEventState(ev.Contract, ev.Function, node.EventStateIndexed)
		if err != nil {
			return err
		}

		log.Default().Println("nothing to do")
		return nil
	}

	// set the event state to indexing
	err := i.db.EventDB.SetEventState(ev.Contract, ev.Function, node.EventStateIndexing)
	if err != nil {
		return err
	}

	fnsig := crypto.Keccak256Hash([]byte(ev.Function))

	erc20sig := crypto.Keccak256Hash([]byte(sc.ERC20Transfer))
	erc721sig := crypto.Keccak256Hash([]byte(sc.ERC721Transfer))
	erc1155sig := crypto.Keccak256Hash([]byte(sc.ERC1155Transfer))

	var contractAbi abi.ABI

	switch fnsig {
	case erc20sig:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc20.Erc20MetaData.ABI)))
		if err != nil {
			return err
		}
	case erc721sig:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc721.Erc721MetaData.ABI)))
		if err != nil {
			return err
		}
	case erc1155sig:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc1155.Erc1155MetaData.ABI)))
		if err != nil {
			return err
		}
	default:
		return errors.New("unknown function signature")
	}

	txdb, ok := i.db.GetTransferDB(ev.Contract)
	if !ok {
		txdb, err = i.db.AddTransferDB(ev.Contract)
		if err != nil {
			return err
		}
	}

	contractAddr := common.HexToAddress(ev.Contract)

	blockNum := curr.Int64()
	// index from the latest block to the last block
	for blockNum > ev.LastBlock {
		startBlock := blockNum - rate
		if startBlock < ev.LastBlock {
			startBlock = ev.LastBlock
		}

		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(startBlock),
			ToBlock:   big.NewInt(blockNum),
			Addresses: []common.Address{contractAddr},
			Topics:    [][]common.Hash{{fnsig}},
		}

		logs, err := i.eth.FilterLogs(query)
		if err != nil {
			return err
		}

		if len(logs) > 0 {
			log.Default().Println("found ", len(logs), " logs between ", startBlock, " and ", blockNum, " ...")
			for _, log := range logs {
				blk, err := i.eth.BlockByNumber(big.NewInt(int64(log.BlockNumber)))
				if err != nil {
					return err
				}

				blktime := time.UnixMilli(int64(blk.Time()) * 1000)

				switch fnsig {
				case erc20sig:
					err = addERC20Log(txdb, blktime, contractAbi, log)
					if err != nil {
						return err
					}
				case erc721sig:
					err = addERC721Log(txdb, blktime, contractAbi, log)
					if err != nil {
						return err
					}
				case erc1155sig:
					err = addERC1155Log(txdb, blktime, contractAbi, log)
					if err != nil {
						return err
					}

					// TODO: parse batch transfers
				}
			}
		}

		blockNum = startBlock - 1
	}

	err = i.db.EventDB.SetEventLastBlock(ev.Contract, ev.Function, curr.Int64())
	if err != nil {
		return err
	}

	// set the event state to indexed
	err = i.db.EventDB.SetEventState(ev.Contract, ev.Function, node.EventStateIndexed)
	if err != nil {
		return err
	}

	log.Default().Println("done")

	return nil
}

func addERC20Log(txdb *db.TransferDB, blktime time.Time, contractAbi abi.ABI, log types.Log) error {
	var trsf erc20.Erc20Transfer

	err := contractAbi.UnpackIntoInterface(&trsf, "Transfer", log.Data)
	if err != nil {
		return err
	}

	trsf.From = common.HexToAddress(log.Topics[1].Hex())
	trsf.To = common.HexToAddress(log.Topics[2].Hex())

	return txdb.AddTransfer(log.TxHash.Hex(), 0, blktime.Format(time.RFC3339), trsf.From.Hex(), trsf.To.Hex(), trsf.Value, nil)
}

func addERC721Log(txdb *db.TransferDB, blktime time.Time, contractAbi abi.ABI, log types.Log) error {
	var trsf erc721.Erc721Transfer

	err := contractAbi.UnpackIntoInterface(&trsf, "Transfer", log.Data)
	if err != nil {
		return err
	}

	trsf.From = common.HexToAddress(log.Topics[1].Hex())
	trsf.To = common.HexToAddress(log.Topics[2].Hex())

	return txdb.AddTransfer(log.TxHash.Hex(), trsf.TokenId.Int64(), blktime.Format(time.RFC3339), trsf.From.Hex(), trsf.To.Hex(), common.Big1, nil)
}

func addERC1155Log(txdb *db.TransferDB, blktime time.Time, contractAbi abi.ABI, log types.Log) error {
	var trsf erc1155.Erc1155TransferSingle

	err := contractAbi.UnpackIntoInterface(&trsf, "TransferSingle", log.Data)
	if err != nil {
		return err
	}

	trsf.From = common.HexToAddress(log.Topics[2].Hex())
	trsf.To = common.HexToAddress(log.Topics[3].Hex())

	return txdb.AddTransfer(log.TxHash.Hex(), trsf.Id.Int64(), blktime.Format(time.RFC3339), trsf.From.Hex(), trsf.To.Hex(), trsf.Value, nil)
}
