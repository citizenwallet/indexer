//go:build db_test
// +build db_test

package govdb

import (
	"math/big"
	"testing"
)

func TestDBBasics(t *testing.T) {
	gdb, err := NewGovDB(big.NewInt(137), "")
}
