//go:build gov_test

package govdb

import (
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

var polygonMainId = big.NewInt(137)

func TestProposals(t *testing.T) {
	propIdBytes := make([]byte, 32)
	propIdB64 := base64.StdEncoding.EncodeToString(propIdBytes)
	println(propIdB64)
}

func TestDBBasics(t *testing.T) {
	// TODO: pull params from env?
	gdb, err := NewDB(polygonMainId, "postgres", "", "poleary", "localhost", "localhost")
	require.NoError(t, err)

	// governors
	exists, err := gdb.checkTableExists(gdb.governorsTableName())
	require.NoError(t, err)
	require.True(t, exists)

	err = gdb.GovernorsDB.drop()
	require.NoError(t, err)

	exists, err = gdb.checkTableExists(gdb.governorsTableName())
	require.NoError(t, err)
	require.False(t, exists)

	// proposals
	exists, err = gdb.checkTableExists(gdb.proposalsTableName())
	require.NoError(t, err)
	require.True(t, exists)

	err = gdb.ProposalsDB.drop()
	require.NoError(t, err)

	exists, err = gdb.checkTableExists(gdb.proposalsTableName())
	require.NoError(t, err)
	require.False(t, exists)

}
