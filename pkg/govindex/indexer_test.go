//go:build gov_test

package govindex

import (
	"context"
	"github.com/citizenwallet/indexer/internal/services/db/govdb"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/pkg/govindexer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"math/big"
	"os"
	"testing"
	"time"
)

var polygonMumbaiId = big.NewInt(80001)

func TestIndexerBasics(t *testing.T) {

	gdb, err := govdb.NewDB(polygonMumbaiId, "postgres", "", "poleary", "localhost", "localhost")
	require.NoError(t, err)
	gdb.SetTesting()
	defer gdb.Close()

	evm, err := ethrequest.NewEthService(context.Background(), os.Getenv("RPC_URL"))
	require.NoError(t, err)

	gidx, err := New(100, polygonMumbaiId, gdb, evm)
	require.NoError(t, err)

	govMumbaiAddr := common.HexToAddress("0xeEDbe595DDCFB5AfDbA7E16B3a36B885CbA81A4A")
	govCreateMumbaiBlock := int64(41478833)

	latest, err := evm.LatestBlock()
	require.NoError(t, err)

	g := govindexer.Governor{
		Contract:    govMumbaiAddr.String(),
		State:       "",
		CreatedAt:   time.Time{},
		UpdatedAt:   time.Time{},
		StartBlock:  govCreateMumbaiBlock,
		LastBlock:   govCreateMumbaiBlock - 1,
		Name:        "",
		Votes:       "",
		Description: "",
	}
	err = gidx.fromBlock(&g, latest.Uint64())
	require.NoError(t, err)
}
