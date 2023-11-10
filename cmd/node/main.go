//go:build (darwin && cgo) || linux
// +build darwin,cgo linux

package main

import (
	"context"
	"encoding/hex"
	"flag"
	"log"
	"time"

	"github.com/citizenwallet/indexer/internal/config"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/internal/services/firebase"
	"github.com/citizenwallet/indexer/internal/services/webhook"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/indexer/pkg/router"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/getsentry/sentry-go"
)

func main() {
	log.Default().Println("launching indexer...")

	env := flag.String("env", "", "path to .env file")

	port := flag.Int("port", 3000, "port to listen on")

	sync := flag.Int("sync", 5, "sync from block number (default: 5)")

	ws := flag.Bool("ws", false, "enable websocket")

	onlyAPI := flag.Bool("onlyApi", false, "only run api service")

	rate := flag.Int("rate", 99, "rate to sync (default: 99)")

	evmtype := flag.String("evm", string(indexer.EVMTypeEthereum), "which evm to use (default: ethereum)")

	fbpath := flag.String("fbpath", "firebase.json", "path to firebase credentials")

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

	d, err := db.NewDB(chid, conf.DBUsername, conf.DBPassword, conf.DBName, conf.DBHost, conf.DBReaderHost)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	quitAck := make(chan error)

	fb := firebase.NewPushService(ctx, *fbpath)

	if !*onlyAPI {
		log.Default().Println("starting index service...")

		i, err := index.New(*rate, chid, d, evm, fb)
		if err != nil {
			log.Fatal(err)
		}
		defer i.Close()

		go func() {
			quitAck <- i.Background(*sync)
		}()
	}

	log.Default().Println("starting api service...")

	bu := bucket.NewBucket(conf.PinataBaseURL, conf.PinataAPIKey, conf.PinataAPISecret)

	pkBytes, err := hex.DecodeString(conf.PaymasterKey)
	if err != nil {
		log.Fatal(err)
	}

	// Generate ecdsa.PrivateKey from bytes
	privateKey, err := crypto.ToECDSA(pkBytes)
	if err != nil {
		log.Fatal(err)
	}

	api := router.NewServer(chid, conf.APIKEY, conf.EntryPointAddress, conf.AccountFactoryAddress, conf.ProfileAddress, evm, d, bu, fb, privateKey)

	go func() {
		quitAck <- api.Start(*port)
	}()

	log.Default().Println("listening on port: ", *port)

	w := webhook.NewMessager(conf.DiscordURL, conf.RPCChainName)

	for err := range quitAck {
		if err != nil {
			w.NotifyError(ctx, err)
			sentry.CaptureException(err)
			log.Fatal(err)
		}
	}
}
