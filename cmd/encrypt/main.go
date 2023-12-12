package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/config"
)

func main() {
	log.Default().Println("generating...")
	log.Default().Println(" ")

	env := flag.String("env", "", "path to .env file")

	v := flag.String("v", "", "the value to be encrypted")

	flag.Parse()

	ctx := context.Background()

	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}

	s, err := base64.StdEncoding.DecodeString(conf.DBSecret)
	if err != nil {
		log.Fatal(err)
	}

	k, err := common.Encrypt(*v, s)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Printf("key: %s\n", k)
}
