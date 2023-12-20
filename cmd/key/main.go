package main

import (
	"encoding/base64"
	"log"

	"github.com/citizenwallet/indexer/internal/common"
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

	log.Default().Printf("key: %s\n", keyBase64)
}
