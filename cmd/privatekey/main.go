package main

import (
	"log"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

func main() {
	log.Default().Println("generating...")
	log.Default().Println(" ")

	pk, address, err := indexer.GenerateHexPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Printf("private key: %s\n", pk)
	log.Default().Printf("address: %s\n", address)
}
