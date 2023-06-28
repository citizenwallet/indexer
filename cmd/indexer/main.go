package main

import (
	"context"
	"flag"
	"log"

	"github.com/citizenwallet/indexer/internal/config"
	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/internal/ethrequest"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/router"
)

func main() {
	log.Default().Println("launching indexer...")

	env := flag.String("env", "", "path to .env file")

	port := flag.Int("port", 3000, "port to listen on")

	sync := flag.Int("sync", 5, "sync from block number (default: 5)")

	ws := flag.Bool("ws", false, "enable websocket")

	rate := flag.Int("rate", 99, "rate to sync (default: 99)")

	flag.Parse()

	ctx := context.Background()

	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("connecting to rpc...")

	rpcUrl := conf.RPCURL
	if *ws {
		log.Default().Println("running in websocket mode...")
		rpcUrl = conf.RPCWSURL
	} else {
		log.Default().Println("running in standard http mode...")
	}

	ethreq, err := ethrequest.NewEthService(ctx, rpcUrl)
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
	defer d.Close()

	quitAck := make(chan error)

	log.Default().Println("starting index service...")

	i := index.New(*rate, chid, d, ethreq)

	go func() {
		quitAck <- i.Background(*sync)
	}()

	log.Default().Println("starting api service...")

	api := router.NewServer(chid, conf.APIKEY, conf.AccountFactoryAddress, ethreq, d)

	go func() {
		quitAck <- api.Start(*port)
	}()

	log.Default().Println("listening on port: ", *port)

	for err := range quitAck {
		if err != nil {
			log.Fatal(err)
		}
	}
}
