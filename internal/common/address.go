package common

import "strings"

func IsSameHexAddress(a, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b)
}
