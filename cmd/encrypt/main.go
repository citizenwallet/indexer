package main

import (
	"encoding/base64"
	"flag"
	"log"

	"github.com/citizenwallet/indexer/internal/common"
)

func main() {
	log.Default().Println("generating...")
	log.Default().Println(" ")

	secret := flag.String("s", "", "the key to be used to encrypt the value")

	v := flag.String("v", "", "the value to be encrypted")

	flag.Parse()

	s, err := base64.StdEncoding.DecodeString(*secret)
	if err != nil {
		log.Fatal(err)
	}

	k, err := common.Encrypt(*v, s)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Printf("key: %s\n", k)
}
