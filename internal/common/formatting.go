package common

import "fmt"

func ShortenName(s string, length int) string {
	if len(s) <= length*2 {
		return s
	}

	firstSix := s[:length]
	lastSix := s[len(s)-length:]
	return fmt.Sprintf("%s__%s", firstSix, lastSix)
}
