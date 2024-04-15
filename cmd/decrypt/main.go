package main

import (
	"encoding/base64"
	"flag"
	"log"

	"github.com/citizenwallet/indexer/internal/common"
)

func main() {
	log.Default().Println("decrypting...")
	log.Default().Println(" ")

	secret := flag.String("s", "", "the key to be used to decrypt the value")

	v := flag.String("v", "", "the value to be decrypted")

	flag.Parse()

	s, err := base64.StdEncoding.DecodeString(*secret)
	if err != nil {
		log.Fatal(err)
	}

	k, err := common.Decrypt(*v, s)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Printf("original value: %s\n", k)

}
