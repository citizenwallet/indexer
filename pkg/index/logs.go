package index

import (
	"errors"
	"math/big"
	"time"

	"github.com/citizenwallet/indexer/internal/sc"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc1155"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc20"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/erc721"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func parseERC20Log(blktime time.Time, contractAbi abi.ABI, log types.Log) (*indexer.Transfer, error) {
	var trsf erc20.Erc20Transfer

	err := contractAbi.UnpackIntoInterface(&trsf, "Transfer", log.Data)
	if err != nil {
		return nil, err
	}

	trsf.From = common.HexToAddress(log.Topics[1].Hex())
	trsf.To = common.HexToAddress(log.Topics[2].Hex())

	return &indexer.Transfer{
		Hash:      log.TxHash.Hex(),
		TxHash:    log.TxHash.Hex(),
		TokenID:   0,
		CreatedAt: blktime,
		From:      trsf.From.Hex(),
		To:        trsf.To.Hex(),
		Nonce:     int64(trsf.Raw.Index),
		Value:     trsf.Value,
		Status:    indexer.TransferStatusSuccess,
	}, nil
}

func parseERC721Log(blktime time.Time, contractAbi abi.ABI, log types.Log) (*indexer.Transfer, error) {
	var trsf erc721.Erc721Transfer

	err := contractAbi.UnpackIntoInterface(&trsf, "Transfer", log.Data)
	if err != nil {
		return nil, err
	}

	trsf.From = common.HexToAddress(log.Topics[1].Hex())
	trsf.To = common.HexToAddress(log.Topics[2].Hex())

	return &indexer.Transfer{
		Hash:      log.TxHash.Hex(),
		TxHash:    log.TxHash.Hex(),
		TokenID:   trsf.TokenId.Int64(),
		CreatedAt: blktime,
		From:      trsf.From.Hex(),
		To:        trsf.To.Hex(),
		Nonce:     int64(trsf.Raw.Index),
		Value:     common.Big1,
		Status:    indexer.TransferStatusSuccess,
	}, nil
}

func parseERC1155Log(blktime time.Time, contractAbi abi.ABI, log types.Log) ([]*indexer.Transfer, error) {
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
			TxHash:    log.TxHash.Hex(),
			TokenID:   trsf.Id.Int64(),
			CreatedAt: blktime,
			From:      trsf.From.Hex(),
			To:        trsf.To.Hex(),
			Nonce:     int64(trsf.Raw.Index),
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
				Hash:      log.TxHash.Hex(),
				TxHash:    log.TxHash.Hex(),
				TokenID:   id.Int64(),
				CreatedAt: blktime,
				From:      trsf.From.Hex(),
				To:        trsf.To.Hex(),
				Nonce:     int64(trsf.Raw.Index),
				Value:     trsf.Values[i],
				Status:    indexer.TransferStatusSuccess,
			})
		}
	default:
		return nil, errors.New("unknown function signature")
	}

	return txs, nil
}

// parseLogs function takes an EVM requester, an event, a contract ABI, and a slice of logs,
// and returns a slice of transfers and an error if any.
func parseTransfersFromLogs(evm indexer.EVMRequester, ev *indexer.Event, contractAbi *abi.ABI, logs []types.Log) ([]*indexer.Transfer, error) {
	// Initialize an empty slice of transfers
	txs := []*indexer.Transfer{}

	// Initialize a map to store blocks by block number
	blks := make(map[int64]uint64)

	// Iterate over the logs
	for _, l := range logs {
		// To reduce API consumption, cache blocks by number

		// Check if the block was already fetched
		blk, ok := blks[int64(l.BlockNumber)]
		if !ok {
			// If the block was not fetched yet, fetch it
			blk, err := evm.BlockTime(big.NewInt(int64(l.BlockNumber)))
			if err != nil {
				// If there's an error, return the transfers and a recoverable error
				return txs, ErrIndexingRecoverable
			}

			// Save the block in our map for later
			blks[int64(l.BlockNumber)] = blk
		}

		// Convert the block time to a time.Time value
		blktime := time.UnixMilli(int64(blk) * 1000).UTC()

		// Switch on the standard of the event
		switch ev.Standard {
		case indexer.ERC20:
			// If the event is an ERC20 event, parse the log as an ERC20 log
			tx, err := parseERC20Log(blktime, *contractAbi, l)
			if err != nil {
				// If there's an error, return the transfers and the error
				return txs, err
			}

			// Append the transfer to the slice of transfers
			txs = append(txs, tx)
		case indexer.ERC721:
			// If the event is an ERC721 event, parse the log as an ERC721 log
			tx, err := parseERC721Log(blktime, *contractAbi, l)
			if err != nil {
				// If there's an error, return the transfers and the error
				return txs, err
			}

			// Append the transfer to the slice of transfers
			txs = append(txs, tx)
		case indexer.ERC1155:
			// If the event is an ERC1155 event, parse the log as an ERC1155 log
			tx, err := parseERC1155Log(blktime, *contractAbi, l)
			if err != nil {
				// If there's an error, return the transfers and the error
				return txs, err
			}

			// Append the transfers to the slice of transfers
			txs = append(txs, tx...)
		}
	}

	// Return the slice of transfers and no error
	return txs, nil
}
