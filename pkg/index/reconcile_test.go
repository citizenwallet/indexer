package index

import (
	"testing"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

func TestReconcileTransactions(t *testing.T) {
	incomingTxs := []*indexer.Transfer{}
	// expectedTxs := []*indexer.Transfer{}

	for _, tx := range incomingTxs {
		t.Run(tx.Hash, func(t *testing.T) {

		})
	}
}
