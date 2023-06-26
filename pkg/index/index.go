package index

import (
	"errors"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/internal/ethrequest"
	"github.com/citizenwallet/indexer/internal/sc"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc1155"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc20"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc721"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type ErrIndexing error

var (
	ErrIndexingRecoverable ErrIndexing = errors.New("error indexing recoverable") // an error occurred while indexing but it is not fatal
)

type Indexer struct {
	rate    int
	chainID *big.Int
	db      *db.DB
	eth     *ethrequest.EthService
}

func New(rate int, chainID *big.Int, db *db.DB, eth *ethrequest.EthService) *Indexer {
	return &Indexer{
		rate:    rate,
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
		return ErrIndexingRecoverable
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
func (i *Indexer) Background(syncrate int) error {
	for {
		err := i.Start()
		if err != nil {
			// check if the error is recoverable
			if err == ErrIndexingRecoverable {
				log.Default().Println("[background] recoverable error: ", err)
				// wait a bit
				<-time.After(250 * time.Millisecond)
				// skip the event
				continue
			}
			return err
		}

		time.Sleep(time.Duration(syncrate) * time.Second)
	}
}

// Process events
func (i *Indexer) Process(evs []*indexer.Event, curr *big.Int) error {
	if len(evs) == 0 {
		// nothing to do
		return nil
	}

	log.Default().Println("indexing ", len(evs), " events")

	// iterate over events and index them
	for _, ev := range evs {
		err := i.Index(ev, curr)
		if err == nil {
			continue
		}

		// check if the error is recoverable
		if err == ErrIndexingRecoverable {
			log.Default().Println("[process] recoverable error: ", err)
			// wait a bit
			<-time.After(250 * time.Millisecond)
			// skip the event
			continue
		}
		return err
	}

	log.Default().Println("indexing done")

	return nil
}

func (i *Indexer) Index(ev *indexer.Event, curr *big.Int) error {
	log.Default().Println("indexing event: ", ev.Contract, ev.Standard, " from block: ", ev.LastBlock, " to block: ", curr.Int64(), " ...")

	// check if the event last block matches the latest block of the chain
	if ev.LastBlock >= curr.Int64() {
		// event is up to date
		err := i.db.EventDB.SetEventState(ev.Contract, ev.Standard, indexer.EventStateIndexed)
		if err != nil {
			return err
		}

		log.Default().Println("nothing to do")
		return nil
	}

	// set the event state to indexing
	err := i.db.EventDB.SetEventState(ev.Contract, ev.Standard, indexer.EventStateIndexing)
	if err != nil {
		return err
	}

	var contractAbi abi.ABI
	var topics [][]common.Hash

	switch ev.Standard {
	case indexer.ERC20:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc20.Erc20MetaData.ABI)))
		if err != nil {
			return err
		}

		topics = [][]common.Hash{{crypto.Keccak256Hash([]byte(sc.ERC20Transfer))}}
	case indexer.ERC721:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc721.Erc721MetaData.ABI)))
		if err != nil {
			return err
		}

		topics = [][]common.Hash{{crypto.Keccak256Hash([]byte(sc.ERC721Transfer))}}
	case indexer.ERC1155:
		contractAbi, err = abi.JSON(strings.NewReader(string(erc1155.Erc1155MetaData.ABI)))
		if err != nil {
			return err
		}
		topics = [][]common.Hash{{crypto.Keccak256Hash([]byte(sc.ERC1155TransferSingle)), crypto.Keccak256Hash([]byte(sc.ERC1155TransferBatch))}}
	default:
		return errors.New("unsupported token standard")
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
		startBlock := blockNum - int64(i.rate)
		if startBlock < ev.LastBlock {
			startBlock = ev.LastBlock
		}

		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(startBlock),
			ToBlock:   big.NewInt(blockNum),
			Addresses: []common.Address{contractAddr},
			Topics:    topics,
		}

		logs, err := i.eth.FilterLogs(query)
		if err != nil {
			return ErrIndexingRecoverable
		}

		if len(logs) > 0 {
			log.Default().Println("found ", len(logs), " logs between ", startBlock, " and ", blockNum, " ...")

			txs := []*indexer.Transfer{}

			for _, log := range logs {
				blk, err := i.eth.BlockByNumber(big.NewInt(int64(log.BlockNumber)))
				if err != nil {
					return ErrIndexingRecoverable
				}

				blktime := time.UnixMilli(int64(blk.Time()) * 1000).UTC()

				switch ev.Standard {
				case indexer.ERC20:
					tx, err := getERC20Log(blktime, contractAbi, log)
					if err != nil {
						return err
					}

					txs = append(txs, tx)
				case indexer.ERC721:
					tx, err := getERC721Log(blktime, contractAbi, log)
					if err != nil {
						return err
					}

					txs = append(txs, tx)
				case indexer.ERC1155:
					tx, err := getERC1155Logs(blktime, contractAbi, log)
					if err != nil {
						return err
					}

					txs = append(txs, tx...)
				}
			}

			if len(txs) > 0 {
				err = txdb.AddTransfers(txs)
				if err != nil {
					return err
				}
			}
		}

		blockNum = startBlock - 1
	}

	err = i.db.EventDB.SetEventLastBlock(ev.Contract, ev.Standard, curr.Int64())
	if err != nil {
		return err
	}

	// set the event state to indexed
	err = i.db.EventDB.SetEventState(ev.Contract, ev.Standard, indexer.EventStateIndexed)
	if err != nil {
		return err
	}

	log.Default().Println("done")

	return nil
}

func getERC20Log(blktime time.Time, contractAbi abi.ABI, log types.Log) (*indexer.Transfer, error) {
	var trsf erc20.Erc20Transfer

	err := contractAbi.UnpackIntoInterface(&trsf, "Transfer", log.Data)
	if err != nil {
		return nil, err
	}

	trsf.From = common.HexToAddress(log.Topics[1].Hex())
	trsf.To = common.HexToAddress(log.Topics[2].Hex())

	return &indexer.Transfer{
		Hash:      log.TxHash.Hex(),
		TokenID:   0,
		CreatedAt: indexer.SQLiteTime(blktime),
		From:      trsf.From.Hex(),
		To:        trsf.To.Hex(),
		Value:     trsf.Value,
	}, nil
}

func getERC721Log(blktime time.Time, contractAbi abi.ABI, log types.Log) (*indexer.Transfer, error) {
	var trsf erc721.Erc721Transfer

	err := contractAbi.UnpackIntoInterface(&trsf, "Transfer", log.Data)
	if err != nil {
		return nil, err
	}

	trsf.From = common.HexToAddress(log.Topics[1].Hex())
	trsf.To = common.HexToAddress(log.Topics[2].Hex())

	return &indexer.Transfer{
		Hash:      log.TxHash.Hex(),
		TokenID:   trsf.TokenId.Int64(),
		CreatedAt: indexer.SQLiteTime(blktime),
		From:      trsf.From.Hex(),
		To:        trsf.To.Hex(),
		Value:     common.Big1,
	}, nil
}

func getERC1155Logs(blktime time.Time, contractAbi abi.ABI, log types.Log) ([]*indexer.Transfer, error) {
	evsig := log.Topics[0].Hex()

	txs := []*indexer.Transfer{}

	switch evsig {
	case crypto.Keccak256Hash([]byte(sc.ERC1155TransferSingle)).Hex():
		var trsf erc1155.Erc1155TransferSingle

		err := contractAbi.UnpackIntoInterface(&trsf, "TransferSingle", log.Data)
		if err != nil {
			return nil, err
		}

		trsf.From = common.HexToAddress(log.Topics[2].Hex())
		trsf.To = common.HexToAddress(log.Topics[3].Hex())

		txs = append(txs, &indexer.Transfer{
			Hash:      log.TxHash.Hex(),
			TokenID:   trsf.Id.Int64(),
			CreatedAt: indexer.SQLiteTime(blktime),
			From:      trsf.From.Hex(),
			To:        trsf.To.Hex(),
			Value:     trsf.Value,
		})
	case crypto.Keccak256Hash([]byte(sc.ERC1155TransferBatch)).Hex():
		var trsf erc1155.Erc1155TransferBatch

		err := contractAbi.UnpackIntoInterface(&trsf, "TransferBatch", log.Data)
		if err != nil {
			return nil, err
		}

		if len(trsf.Ids) != len(trsf.Values) {
			return nil, errors.New("ids and values length mismatch")
		}

		trsf.From = common.HexToAddress(log.Topics[2].Hex())
		trsf.To = common.HexToAddress(log.Topics[3].Hex())

		for i, id := range trsf.Ids {
			txs = append(txs, &indexer.Transfer{
				Hash:      log.TxHash.Hex(),
				TokenID:   id.Int64(),
				CreatedAt: indexer.SQLiteTime(blktime),
				From:      trsf.From.Hex(),
				To:        trsf.To.Hex(),
				Value:     trsf.Values[i],
			})
		}
	default:
		return nil, errors.New("unknown function signature")
	}

	return txs, nil
}
