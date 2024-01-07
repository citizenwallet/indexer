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

func (i *Indexer) EventsFromBlock(ev *indexer.Event, blk *block, ptdb *db.PushTokenDB) error {
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

	fromBlock := ev.LastBlock
	blocksToIndex := blk.Number - uint64(fromBlock)
	if blocksToIndex > uint64(i.rate) {
		fromBlock = int64(blk.Number) - int64(i.rate)
	}

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(int64(blk.Number)),
		Addresses: []common.Address{contractAddr},
		Topics:    topics,
	}

	logs, err := i.evm.FilterLogs(query)
	if err != nil {
		return ErrIndexingRecoverable
	}

	if len(logs) > 0 {
		txs, err := parseTransfersFromLogs(i.evm, ev, contractAbi, blk, logs)
		if err != nil {
			return err
		}

		if len(txs) > 0 {
			err = reconcileTransfersWithDB(txdb, txs)
			if err != nil {
				return err
			}

			// TODO: move to a queue in a separate service
			if ptdb != nil && i.fb != nil {
				go sendPushForTxs(ptdb, i.fb, ev, txs)
			}
			// end TODO
		}
	}

	err = i.db.EventDB.SetEventLastBlock(ev.Contract, ev.Standard, int64(blk.Number))
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
