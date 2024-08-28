//go:generate swagger generate spec

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
	"github.com/citizenwallet/indexer/internal/services/firebase"
	"github.com/citizenwallet/indexer/internal/services/webhook"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/indexer/pkg/queue"
	"github.com/citizenwallet/indexer/pkg/router"
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
	log.Default().Println("launching bundler...")

	env := flag.String("env", "", "path to .env file")

	confpath := flag.String("confpath", "./config", "path to config file")

	certpath := flag.String("certpath", "./certs", "cert folder path")

	port := flag.Int("port", 3001, "port to listen on")

	useropqbf := flag.Int("buffer", 1000, "userop queue buffer size (default: 1000)")

	ws := flag.Bool("ws", false, "enable websocket")

	notify := flag.Bool("notify", true, "enable notifications")

	evmtype := flag.String("evm", string(indexer.EVMTypeEthereum), "which evm to use (default: ethereum)")

	fbpath := flag.String("fbpath", "firebase.json", "path to firebase credentials")

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

	fb := firebase.NewPushService(ctx, *fbpath)

	w := webhook.NewMessager(conf.DiscordURL, conf.RPCChainName, *notify)
	defer func() {
		if r := recover(); r != nil {
			// in case of a panic, notify the webhook messager with an error notification
			err := fmt.Errorf("recovered from panic: %v", r)
			log.Default().Println(err)
			w.NotifyError(ctx, err)
			sentry.CaptureException(err)
		}
	}()

	log.Default().Println("starting bundler service...")

	op := queue.NewUserOpService(d, evm, fb)

	useropq := queue.NewService("userop", 3, *useropqbf, ctx, w)

	go func() {
		quitAck <- useropq.Start(op)
	}()

	api := router.NewServer(chid, evm, d)

	go func() {
		router := api.CreateBaseRouter()

		api.AddMiddleware(router)
		api.AddBundlerRoutes(router, useropq)

		if *port == 443 {
			quitAck <- api.StartTLS(*certpath, router)
			return
		}
		quitAck <- api.Start(*port, router)
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
