package common

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

func IsSameHexAddress(a, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b)
}

func ChecksumAddress(addr string) string {
	address := common.HexToAddress(addr)

	return address.Hex()
}
