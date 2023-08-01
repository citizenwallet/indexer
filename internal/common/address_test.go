package common

import (
	"testing"
)

func TestChecksumAddress(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "valid address",
			addr:     "0x1234567890123456789012345678901234567890",
			expected: "0x1234567890123456789012345678901234567890",
		},
		{
			name:     "invalid address",
			addr:     "not_an_address",
			expected: "0x0000000000000000000000000000000000000000",
		},
		{
			name:     "checksum address",
			addr:     "0x480fbe37526226b6c6e2a7afa449cdf661939d2f",
			expected: "0x480Fbe37526226b6c6E2a7AfA449cDf661939D2f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ChecksumAddress(tt.addr)
			if actual != tt.expected {
				t.Errorf("checksumAddress(%s): expected %s, but got %s", tt.addr, tt.expected, actual)
			}
		})
	}
}
