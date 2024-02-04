//go:build gov_test

package govindex

import (
	"context"
	"github.com/citizenwallet/indexer/internal/services/db/govdb"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/stretchr/testify/require"
	"math/big"
	"os"
	"testing"
)

var polygonMainId = big.NewInt(137)

func TestIndexerBasics(t *testing.T) {

	gdb, err := govdb.NewDB(polygonMainId, "postgres", "", "poleary", "localhost", "localhost")
	require.NoError(t, err)
	gdb.SetTesting()
	defer gdb.Close()

	evm, err := ethrequest.NewEthService(context.Background(), os.Getenv("RPC_URL"))
	require.NoError(t, err)

	gidx, err := New(99, polygonMainId, gdb, evm)
	require.NoError(t, err)
	_ = gidx
}
