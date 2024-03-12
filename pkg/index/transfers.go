package index

import (
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

// reconcileTransfersWithDB tries to reconcile the transfers with optimistic ones in the db
func reconcileTransfersWithDB(txdb *db.TransferDB, txs []*indexer.Transfer) error {
	if len(txs) > 0 {
		// add the new transfers to the db
		err := txdb.AddTransfers(txs)
		if err != nil {
			return err
		}
	}

	return nil
}
