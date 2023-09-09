package index

import (
	"context"
	"errors"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/citizenwallet/indexer/internal/sc"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc1155"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc20"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc721"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type ErrIndexing error

var (
	ErrIndexingRecoverable ErrIndexing = errors.New("error indexing recoverable") // an error occurred while indexing but it is not fatal
)

type EVMType string

const (
	EVMTypeEthereum EVMType = "ethereum"
	EVMTypeOptimism EVMType = "optimism"
)

type EVMRequester interface {
	Context() context.Context
	Client() *ethclient.Client

	ChainID() (*big.Int, error)
	LatestBlock() (*types.Block, error)
	FilterLogs(q ethereum.FilterQuery) ([]types.Log, error)
	BlockByNumber(number *big.Int) (*types.Block, error)

	Close()
}

type Indexer struct {
	rate    int
	chainID *big.Int
	db      *db.DB
	evm     EVMRequester

	re *Reconciler
}

func New(rate int, chainID *big.Int, db *db.DB, evm EVMRequester, ctx context.Context, rpcUrl, origin string) (*Indexer, error) {
	re, err := NewReconciler(rate, chainID, db, ctx, rpcUrl, origin)
	if err != nil {
		return nil, err
	}

	return &Indexer{
		rate:    rate,
		chainID: chainID,
		db:      db,
		evm:     evm,
		re:      re,
	}, nil
}

func (i *Indexer) IndexERC20From(contract string, from int64) error {
	// get the latest block
	latestBlock, err := i.evm.LatestBlock()
	if err != nil {
		return ErrIndexingRecoverable
	}
	curr := latestBlock.Number()

	ev, err := i.db.EventDB.GetEvent(contract, indexer.ERC20)
	if err != nil {
		return err
	}

	return i.IndexFrom(ev, curr, big.NewInt(from))
}

// Start starts the indexer service
func (i *Indexer) Start() error {
	// get the latest block
	latestBlock, err := i.evm.LatestBlock()
	if err != nil {
		return ErrIndexingRecoverable
	}
	curr := latestBlock.Number()

	// check if there are any queued events
	evs, err := i.db.EventDB.GetOutdatedEvents(curr.Int64())
	if err != nil {
		return err
	}

	err = i.re.Process(evs)
	if err != nil && err != ErrReconcilingRecoverable {
		return err
	}

	return i.Process(evs, curr)
}

func (e *Indexer) Close() {
	e.re.Close()
}

// Background starts an indexer service in the background
func (i *Indexer) Background(syncrate int) error {
	for {
		err := i.Start()
		if err != nil {
			// check if the error is recoverable
			if err == ErrIndexingRecoverable {
				log.Default().Println("indexer [background] recoverable error: ", err)
				// wait a bit
				<-time.After(250 * time.Millisecond)
				// skip the event
				continue
			}
			return err
		}

		<-time.After(time.Duration(syncrate) * time.Second)
	}
}

// Process events
func (i *Indexer) Process(evs []*indexer.Event, curr *big.Int) error {
	if len(evs) == 0 {
		// nothing to do
		return nil
	}

	// iterate over events and index them
	for _, ev := range evs {
		err := i.Index(ev, curr)
		if err == nil {
			continue
		}

		// check if the error is recoverable
		if err == ErrIndexingRecoverable {
			log.Default().Println("indexer [process] recoverable error: ", err)
			// wait a bit
			<-time.After(250 * time.Millisecond)
			// skip the event
			continue
		}
		return err
	}

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

		log.Default().Println("indexing block: ", startBlock, " to block: ", blockNum, " ...")

		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(startBlock),
			ToBlock:   big.NewInt(blockNum),
			Addresses: []common.Address{contractAddr},
			Topics:    topics,
		}

		logs, err := i.evm.FilterLogs(query)
		if err != nil {
			return ErrIndexingRecoverable
		}

		if len(logs) > 0 {
			txs := []*indexer.Transfer{}

			// store a map of blocks by block number
			blks := map[int64]*types.Block{}

			for index, l := range logs {
				// to reduce API consumption, cache blocks by number

				// check if it was already fetched
				blk, ok := blks[int64(l.BlockNumber)]
				if !ok {
					// was not fetched yet, fetch it
					blk, err = i.evm.BlockByNumber(big.NewInt(int64(l.BlockNumber)))
					if err != nil {
						return ErrIndexingRecoverable
					}

					// save in our map for later
					blks[int64(l.BlockNumber)] = blk
				}

				blktime := time.UnixMilli(int64(blk.Time()) * 1000).UTC()

				if index == 0 {
					log.Default().Println("found ", len(logs), " logs between ", startBlock, " and ", blockNum, " [", blktime, "] ...")
				}

				switch ev.Standard {
				case indexer.ERC20:
					tx, err := getERC20Log(blktime, contractAbi, l)
					if err != nil {
						return err
					}

					txs = append(txs, tx)
				case indexer.ERC721:
					tx, err := getERC721Log(blktime, contractAbi, l)
					if err != nil {
						return err
					}

					txs = append(txs, tx)
				case indexer.ERC1155:
					tx, err := getERC1155Logs(blktime, contractAbi, l)
					if err != nil {
						return err
					}

					txs = append(txs, tx...)
				}
			}

			if len(txs) > 0 {
				// filter out existing transfers
				newTxs := []*indexer.Transfer{}
				for _, tx := range txs {
					// check if the transfer already exists
					exists, err := txdb.TransferExists(tx.TxHash)
					if err != nil {
						return err
					}

					if !exists {
						// generate a hash
						tx.GenerateHash(i.chainID.Int64())

						newTxs = append(newTxs, tx)
						continue
					}

					err = txdb.SetStatusFromTxHash(string(indexer.TransferStatusSuccess), tx.TxHash)
					if err != nil {
						return err
					}
				}

				if len(newTxs) > 0 {
					// add the new transfers to the db
					err = txdb.AddTransfers(newTxs)
					if err != nil {
						return err
					}
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

func (i *Indexer) IndexFrom(ev *indexer.Event, curr, from *big.Int) error {
	log.Default().Println("indexing event: ", ev.Contract, ev.Standard, " from block: ", ev.LastBlock, " to block: ", curr.Int64(), " ...")

	// check if the event last block matches the latest block of the chain
	if from.Int64() >= curr.Int64() {
		// event is up to date

		log.Default().Println("nothing to do")
		return nil
	}

	var err error

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
	for blockNum > from.Int64() {
		startBlock := blockNum - int64(i.rate)
		if startBlock < from.Int64() {
			startBlock = from.Int64()
		}

		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(startBlock),
			ToBlock:   big.NewInt(blockNum),
			Addresses: []common.Address{contractAddr},
			Topics:    topics,
		}

		logs, err := i.evm.FilterLogs(query)
		if err != nil {
			return ErrIndexingRecoverable
		}

		if len(logs) > 0 {
			txs := []*indexer.Transfer{}

			// store a map of blocks by block number
			blks := map[int64]*types.Block{}

			for index, l := range logs {
				// to reduce API consumption, cache blocks by number

				// check if it was already fetched
				blk, ok := blks[int64(l.BlockNumber)]
				if !ok {
					// was not fetched yet, fetch it
					blk, err = i.evm.BlockByNumber(big.NewInt(int64(l.BlockNumber)))
					if err != nil {
						return ErrIndexingRecoverable
					}

					// save in our map for later
					blks[int64(l.BlockNumber)] = blk
				}

				blktime := time.UnixMilli(int64(blk.Time()) * 1000).UTC()

				if index == 0 {
					log.Default().Println("found ", len(logs), " logs between ", startBlock, " and ", blockNum, " [", blktime, "] ...")
				}

				switch ev.Standard {
				case indexer.ERC20:
					tx, err := getERC20Log(blktime, contractAbi, l)
					if err != nil {
						return err
					}

					txs = append(txs, tx)
				case indexer.ERC721:
					tx, err := getERC721Log(blktime, contractAbi, l)
					if err != nil {
						return err
					}

					txs = append(txs, tx)
				case indexer.ERC1155:
					tx, err := getERC1155Logs(blktime, contractAbi, l)
					if err != nil {
						return err
					}

					txs = append(txs, tx...)
				}
			}

			if len(txs) > 0 {
				// filter out existing transfers
				newTxs := []*indexer.Transfer{}
				for _, tx := range txs {
					// check if the transfer already exists
					exists, err := txdb.TransferExists(tx.TxHash)
					if err != nil {
						return err
					}

					if !exists {
						// there can be optimistic transactions already in the db
						// attempt to find a similar transaction
						exists, err = txdb.TransferSimilarExists(tx.From, tx.To, tx.Value.String())
						if err != nil {
							return err
						}

						if exists {
							log.Default().Println("found a potential optimistic transaction: ", tx.From, tx.To, tx.Value.String())
						}
					}

					if !exists {
						// generate a hash
						tx.GenerateHash(i.chainID.Int64())

						newTxs = append(newTxs, tx)
						continue
					}

					err = txdb.SetStatusFromTxHash(string(indexer.TransferStatusSuccess), tx.TxHash)
					if err != nil {
						return err
					}
				}

				if len(newTxs) > 0 {
					// add the new transfers to the db
					err = txdb.AddTransfers(newTxs)
					if err != nil {
						return err
					}
				}
			}
		}

		blockNum = startBlock - 1
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
		TxHash:    log.TxHash.Hex(),
		TokenID:   0,
		CreatedAt: blktime,
		From:      trsf.From.Hex(),
		To:        trsf.To.Hex(),
		Value:     trsf.Value,
		Status:    indexer.TransferStatusSuccess,
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
		TxHash:    log.TxHash.Hex(),
		TokenID:   trsf.TokenId.Int64(),
		CreatedAt: blktime,
		From:      trsf.From.Hex(),
		To:        trsf.To.Hex(),
		Value:     common.Big1,
		Status:    indexer.TransferStatusSuccess,
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
			TxHash:    log.TxHash.Hex(),
			TokenID:   trsf.Id.Int64(),
			CreatedAt: blktime,
			From:      trsf.From.Hex(),
			To:        trsf.To.Hex(),
			Value:     trsf.Value,
			Status:    indexer.TransferStatusSuccess,
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
				TxHash:    log.TxHash.Hex(),
				TokenID:   id.Int64(),
				CreatedAt: blktime,
				From:      trsf.From.Hex(),
				To:        trsf.To.Hex(),
				Value:     trsf.Values[i],
				Status:    indexer.TransferStatusSuccess,
			})
		}
	default:
		return nil, errors.New("unknown function signature")
	}

	return txs, nil
}
