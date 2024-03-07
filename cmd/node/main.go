//go:generate swagger generate spec

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
	"github.com/citizenwallet/indexer/pkg/queue"
	"github.com/citizenwallet/indexer/pkg/router"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/getsentry/sentry-go"
)

// @title           Citizen Wallet Indexer API
// @version         1.0
// @description     This is a server which handles token contract indexing, user operations, and other support functions for the app.
// @termsOfService  https://citizenwallet.xyz

// @contact.name   API Support
// @contact.url    https://github.com/citizenwallet
// @contact.email  support@citizenspring.earth

// @license.name  MIT
// @license.url   https://raw.githubusercontent.com/citizenwallet/indexer/main/LICENSE

// @host      localhost:3000
// @BasePath  /

// @securityDefinitions.basic  Authorization Bearer

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	log.Default().Println("launching indexer...")

	env := flag.String("env", ".env", "path to .env file")

	port := flag.Int("port", 3000, "port to listen on")

	sync := flag.Int("sync", 1, "sync from block number (default: 1)")

	useropqbf := flag.Int("buffer", 100, "userop queue buffer size (default: 100)")

	ws := flag.Bool("ws", false, "enable websocket")

	notify := flag.Bool("notify", true, "enable notifications")

	onlyAPI := flag.Bool("onlyApi", false, "only run api service")

	rate := flag.Int("rate", 10, "rate to sync (default: 10)")

	evmtype := flag.String("evm", string(indexer.EVMTypeEthereum), "which evm to use (default: ethereum)")

	fbpath := flag.String("fbpath", "firebase.json", "path to firebase credentials")

	dbpath := flag.String("dbpath", ".", "path to db")

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

	d, err := db.NewDB(chid, *dbpath, conf.DBUsername, conf.DBPassword, conf.DBName, conf.DBHost, conf.DBReaderHost, conf.DBSecret)
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

	w := webhook.NewMessager(conf.DiscordURL, conf.RPCChainName, *notify)

	op := queue.NewUserOpService(d, evm)

	useropq := queue.NewService("userop", 3, *useropqbf, ctx, w)

	go func() {
		quitAck <- useropq.Start(op)
	}()

	api := router.NewServer(chid, conf.APIKEY, conf.EntryPointAddress, conf.AccountFactoryAddress, conf.ProfileAddress, evm, d, useropq, bu, fb, privateKey)

	go func() {
		quitAck <- api.Start(*port)
	}()

	log.Default().Println("listening on port: ", *port)

	for err := range quitAck {
		if err != nil {
			w.NotifyError(ctx, err)
			sentry.CaptureException(err)
			log.Fatal(err)
		}
	}
}
