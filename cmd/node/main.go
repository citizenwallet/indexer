package main

import (
	"context"
	"flag"
	"log"

	"github.com/citizenwallet/node/internal/config"
	"github.com/citizenwallet/node/internal/db"
	"github.com/citizenwallet/node/internal/ethrequest"
	"github.com/citizenwallet/node/pkg/indexer"
)

func main() {
	log.Default().Println("launching node...")

	env := flag.String("env", "", "path to .env file")

	port := flag.Int("port", 3000, "port to listen on")

	flag.Parse()

	ctx := context.Background()

	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("connecting to rpc...")

	ethreq, err := ethrequest.NewEthService(ctx, conf.RPCWSURL)
	if err != nil {
		log.Fatal(err)
	}
	defer ethreq.Close()

	log.Default().Println("fetching chain id...")

	chid, err := ethreq.ChainID()
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("node running for chain: ", chid.String())

	log.Default().Println("starting internal db service...")

	d, err := db.NewDB(chid)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("starting indexer service...")
	i := indexer.New(chid, d, ethreq)

	quitAck := make(chan error)

	go func() {
		quitAck <- i.Start()
	}()

	log.Default().Println("starting rpc listener service...")

	log.Default().Println("starting api service...")

	log.Default().Println("listening on port: ", *port)

	for err := range quitAck {
		if err != nil {
			log.Fatal(err)
		}
	}
}
