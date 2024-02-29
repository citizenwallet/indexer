package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	log.Default().Println("generating...")
	log.Default().Println(" ")

	k, err := common.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	// Encode the key as a base64 string
	keyBase64 := base64.StdEncoding.EncodeToString(k)

	hexKey := hex.EncodeToString(k)

	// key address
	ecdsaKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		log.Fatal(err)
	}

	keyAddress := crypto.PubkeyToAddress(ecdsaKey.PublicKey).Hex()

	println()
	println((fmt.Sprintf("key address: %s\n", keyAddress)))
	println((fmt.Sprintf("hex key: %s\n", hexKey)))
	println((fmt.Sprintf("b64 key: %s\n", keyBase64)))
	println()
}
