package main

import (
	"context"
	"flag"
	"log"

	"github.com/citizenwallet/node/internal/config"
	"github.com/citizenwallet/node/internal/db"
	"github.com/citizenwallet/node/internal/ethrequest"
)

func main() {
	log.Default().Println("launching node...")

	env := flag.String("env", "", "path to .env file")

	port := flag.Int("port", 3000, "port to listen on")

	flag.Parse()

	log.Default().Println("listening on port: ", *port)

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

	log.Default().Println("fetching chain id...")

	chid, err := ethreq.ChainID()
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("node running for chain: ", chid.String())

	log.Default().Println("starting internal db service...")

	_, err = db.NewDB(chid)
	if err != nil {
		log.Fatal(err)
	}
}
