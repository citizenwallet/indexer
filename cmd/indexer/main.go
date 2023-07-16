package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/citizenwallet/indexer/internal/config"
	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/internal/ethrequest"
	"github.com/citizenwallet/indexer/internal/oprequest"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/router"
	"github.com/getsentry/sentry-go"
)

func main() {
	log.Default().Println("launching indexer...")

	env := flag.String("env", "", "path to .env file")

	port := flag.Int("port", 3000, "port to listen on")

	sync := flag.Int("sync", 5, "sync from block number (default: 5)")

	ws := flag.Bool("ws", false, "enable websocket")

	rate := flag.Int("rate", 99, "rate to sync (default: 99)")

	evmtype := flag.String("evm", string(index.EVMTypeEthereum), "which evm to use (default: ethereum)")

	flag.Parse()

	ctx := context.Background()

	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}

	if conf.SentryURL != "" && conf.SentryURL != "x" {
		err = sentry.Init(sentry.ClientOptions{
			Dsn: conf.SentryURL,
			// Set TracesSampleRate to 1.0 to capture 100%
			// of transactions for performance monitoring.
			// We recommend adjusting this value in production,
			TracesSampleRate: 1.0,
		})
		if err != nil {
			log.Fatalf("sentry.Init: %s", err)
		}
		// Flush buffered events before the program terminates.
		defer sentry.Flush(2 * time.Second)
	}

	log.Default().Println("connecting to rpc...")

	rpcUrl := conf.RPCURL
	if *ws {
		log.Default().Println("running in websocket mode...")
		rpcUrl = conf.RPCWSURL
	} else {
		log.Default().Println("running in standard http mode...")
	}

	var evm index.EVMRequester
	switch index.EVMType(*evmtype) {
	case index.EVMTypeEthereum:
		evm, err = ethrequest.NewEthService(ctx, rpcUrl)
		if err != nil {
			log.Fatal(err)
		}
	case index.EVMTypeOptimism:
		evm, err = oprequest.NewEthService(ctx, rpcUrl)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unsupported evm type (must be one of: ethereum, optimism))")
	}

	defer evm.Close()

	log.Default().Println("fetching chain id...")

	chid, err := evm.ChainID()
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

	i, err := index.New(*rate, chid, d, evm, ctx, conf.BundlerRPCURL, conf.BundlerOriginHeader)
	if err != nil {
		log.Fatal(err)
	}
	defer i.Close()

	go func() {
		quitAck <- i.Background(*sync)
	}()

	log.Default().Println("starting api service...")

	api := router.NewServer(chid, conf.APIKEY, conf.EntryPointAddress, conf.AccountFactoryAddress, evm, d)

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
