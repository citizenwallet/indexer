package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/citizenwallet/indexer/internal/config"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/internal/services/webhook"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/getsentry/sentry-go"
)

func main() {
	log.Default().Println("launching indexer...")

	env := flag.String("env", "", "path to .env file")

	confpath := flag.String("confpath", "./config", "path to config file")

	contract := flag.String("contract", "", "contract address to sync")

	startBlk := flag.Int64("start", 0, "which block to start from")

	ws := flag.Bool("ws", false, "enable websocket")

	notify := flag.Bool("notify", false, "enable notifications")

	rate := flag.Int("rate", 99, "rate to sync (default: 99)")

	evmtype := flag.String("evm", string(indexer.EVMTypeEthereum), "which evm to use (default: ethereum)")

	dbpath := flag.String("dbpath", ".", "path to db")

	flag.Parse()

	ctx := context.Background()

	conf, err := config.New(ctx, *env, *confpath)
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

	var evm indexer.EVMRequester
	switch indexer.EVMType(*evmtype) {
	case indexer.EVMTypeEthereum:
		evm, err = ethrequest.NewEthService(ctx, rpcUrl)
		if err != nil {
			log.Fatal(err)
		}
	case indexer.EVMTypeOptimism:
		evm, err = ethrequest.NewOpService(ctx, rpcUrl)
		if err != nil {
			log.Fatal(err)
		}
	case indexer.EVMTypeCelo:
		evm, err = ethrequest.NewCeloService(ctx, rpcUrl)
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

	d, err := db.NewDB(chid, *dbpath, conf.DBSecret)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	quitAck := make(chan error)

	log.Default().Println("starting index service...")

	i, err := index.New(*rate, chid, d, evm, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer i.Close()

	w := webhook.NewMessager(conf.DiscordURL, conf.RPCChainName, *notify)

	go func() {
		w.Notify(ctx, fmt.Sprintf("⚙️ indexing started for contract: %s", *contract))

		quitAck <- i.IndexERC20From(*contract, *startBlk)
	}()

	for err := range quitAck {
		if err == nil {
			break
		}

		if err != nil {
			w.NotifyError(ctx, err)
			log.Fatal(err)
		}
	}

	w.Notify(ctx, fmt.Sprintf("✅ indexing done for contract: %s", *contract))
	log.Default().Println("indexing done, shutting down...")
}
