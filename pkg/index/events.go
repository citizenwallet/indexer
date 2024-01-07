package index

import (
	"fmt"
	"math/big"

	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/firebase"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

func (i *Indexer) EventsFromBlock(ev *indexer.Event, curr *big.Int) error {
	// check if the event last block matches the latest block of the chain
	if ev.LastBlock >= curr.Int64() {
		// event is up to date
		err := i.db.EventDB.SetEventState(ev.Contract, ev.Standard, indexer.EventStateIndexed)
		if err != nil {
			return err
		}

		return nil
	}

	// set the event state to indexing
	err := i.db.EventDB.SetEventState(ev.Contract, ev.Standard, indexer.EventStateIndexing)
	if err != nil {
		return err
	}

	contractAbi, err := GetContractABI(ev.Standard)
	topics := GetContractTopics(ev.Standard)

	txdb, ok := i.db.GetTransferDB(ev.Contract)
	if !ok {
		txdb, err = i.db.AddTransferDB(ev.Contract)
		if err != nil {
			return err
		}
	}

	ptdb, ok := i.db.GetPushTokenDB(ev.Contract)
	if !ok {
		ptdb, err = i.db.AddPushTokenDB(ev.Contract)
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
			startBlock = ev.LastBlock + 1
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
			txs, err := parseTransfersFromLogs(i.evm, ev, contractAbi, logs)
			if err != nil {
				return err
			}

			if len(txs) > 0 {
				err = reconcileTransfersWithDB(txdb, txs)
				if err != nil {
					return err
				}

				// TODO: move to a queue in a separate service
				go sendPushForTxs(ptdb, i.fb, ev, txs)
				// end TODO
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

	return nil
}

func sendPushForTxs(ptdb *db.PushTokenDB, fb *firebase.PushService, ev *indexer.Event, txs []*indexer.Transfer) {
	accTokens := map[string][]*indexer.PushToken{}

	messages := []*indexer.PushMessage{}

	for _, tx := range txs {
		if tx.Status != indexer.TransferStatusSuccess {
			continue
		}

		if _, ok := accTokens[tx.To]; !ok {
			// get the push tokens for the recipient
			pt, err := ptdb.GetAccountTokens(tx.To)
			if err != nil {
				return
			}

			if len(pt) == 0 {
				// no push tokens for this account
				continue
			}

			accTokens[tx.To] = pt
		}

		value := tx.ToRounded(ev.Decimals)

		messages = append(messages, indexer.NewAnonymousPushMessage(accTokens[tx.To], ev.Name, fmt.Sprintf("%.2f", value), ev.Symbol))
	}

	if len(messages) > 0 {
		for _, push := range messages {
			badTokens, err := fb.Send(push)
			if err != nil {
				continue
			}

			if len(badTokens) > 0 {
				// remove the bad tokens
				for _, token := range badTokens {
					err = ptdb.RemovePushToken(token)
					if err != nil {
						continue
					}
				}
			}
		}
	}
}

func (i *Indexer) AllEventsFromBlock(ev *indexer.Event, curr, from *big.Int) error {
	// check if the event last block matches the latest block of the chain
	if from.Int64() >= curr.Int64() {
		// event is up to date

		return nil
	}

	contractAbi, err := GetContractABI(ev.Standard)
	topics := GetContractTopics(ev.Standard)

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
			txs, err := parseTransfersFromLogs(i.evm, ev, contractAbi, logs)
			if err != nil {
				return err
			}

			if len(txs) > 0 {
				err = reconcileTransfersWithDB(txdb, txs)
				if err != nil {
					return err
				}
			}
		}

		blockNum = startBlock - 1
	}

	return nil
}
