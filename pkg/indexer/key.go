package indexer

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
)

// generate a new private key
func GeneratePrivateKey() (*ecdsa.PrivateKey, error) {
	return crypto.GenerateKey()
}

// generate a new private key
func GenerateHexPrivateKey() (string, error) {
	pk, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}

	// Convert the private key to bytes
	privateKeyBytes := crypto.FromECDSA(pk)

	// Convert the bytes to a hexadecimal string
	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	return privateKeyHex, nil
}
