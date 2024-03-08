package index

import (
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

func reconcileTransfersWithDB(txdb *db.TransferDB, txs []*indexer.Transfer) error {
	newTxs := []*indexer.Transfer{}

	for _, tx := range txs {
		// check if the transfer already exists
		exists, err := txdb.TransferExists(tx.TxHash, tx.From, tx.To, tx.Value.String())
		if err != nil {
			return err
		}

		if !exists {
			// there can be optimistic transactions already in the db
			// attempt to find a similar transaction
			hash, _ := txdb.TransferSimilarExists(tx.From, tx.To, tx.Value.String())

			if hash != "" {
				// there is an optimistic transaction, set its tx_hash and status
				err = txdb.ReconcileTx(tx.TxHash, hash, tx.Nonce)
				if err != nil {
					return err
				}

				continue
			}

			newTxs = append(newTxs, tx)
			continue
		}

		err = txdb.SetStatusFromHash(string(indexer.TransferStatusSuccess), tx.Hash)
		if err != nil {
			return err
		}
	}

	if len(newTxs) > 0 {
		// add the new transfers to the db
		err := txdb.AddTransfers(newTxs)
		if err != nil {
			return err
		}
	}

	return nil
}
