package common

import "math/big"

func HexToBigInt(hex string) *big.Int {
	i, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return big.NewInt(0)
	}

	return i
}
