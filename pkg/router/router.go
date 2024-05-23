package router

import (
	"crypto/tls"
	"fmt"
	"math/big"
	"net/http"

	"github.com/citizenwallet/indexer/internal/accounts"
	"github.com/citizenwallet/indexer/internal/chain"
	"github.com/citizenwallet/indexer/internal/events"
	"github.com/citizenwallet/indexer/internal/logs"
	"github.com/citizenwallet/indexer/internal/paymaster"
	"github.com/citizenwallet/indexer/internal/profiles"
	"github.com/citizenwallet/indexer/internal/push"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/firebase"
	"github.com/citizenwallet/indexer/internal/userop"
	"github.com/citizenwallet/indexer/internal/version"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/indexer/pkg/queue"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Router struct {
	chainId  *big.Int
	evm      indexer.EVMRequester
	db       *db.DB
	useropq  *queue.Service
	b        *bucket.Bucket
	firebase *firebase.PushService
}

func NewServer(chainId *big.Int, evm indexer.EVMRequester, db *db.DB, useropq *queue.Service, b *bucket.Bucket, firebase *firebase.PushService) *Router {
	return &Router{
		chainId,
		evm,
		db,
		useropq,
		b,
		firebase,
	}
}

func (r *Router) CreateHandler() (http.Handler, error) {
	cr := chi.NewRouter()

	// configure middleware
	cr.Use(middleware.RequestID)
	cr.Use(middleware.Logger)

	// configure custom middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(RequestSizeLimitMiddleware(10 << 20)) // Limit request bodies to 10MB
	cr.Use(middleware.Compress(9))

	// instantiate handlers
	v := version.NewService()
	l := logs.NewService(r.chainId, r.db, r.evm)
	ev := events.NewService(r.db)
	pr := profiles.NewService(r.b, r.evm)
	pu := push.NewService(r.db)
	acc := accounts.NewService(r.evm, r.db)

	pm := paymaster.NewService(r.evm, r.db)
	uop := userop.NewService(r.evm, r.db, r.useropq, r.chainId)
	ch := chain.NewService(r.evm, r.chainId)

	// configure routes
	cr.Route("/version", func(cr chi.Router) {
		cr.Get("/", v.Current)
	})

	cr.Route("/logs/v2/transfers", func(cr chi.Router) {
		cr.Route("/{token_address}", func(cr chi.Router) {
			cr.Get("/", l.GetAll)
			cr.Get("/new", l.GetAllNew)
			cr.Get("/{acc_addr}", l.Get)
			cr.Get("/{acc_addr}/new", l.GetNew)

			cr.Post("/{acc_addr}", withSignature(r.evm, l.AddSending))

			cr.Patch("/{acc_addr}/{hash}", withSignature(r.evm, l.SetStatus))
		})
	})

	cr.Route("/events", func(cr chi.Router) {
		cr.Post("/", ev.AddEvent) // TODO: add auth
	})

	cr.Route("/profiles/v2", func(cr chi.Router) {
		cr.Route("/{contract_address}", func(cr chi.Router) {
			cr.Put("/{acc_addr}", withMultiPartSignature(r.evm, pr.PinMultiPartProfile))
			cr.Patch("/{acc_addr}", withSignature(r.evm, pr.PinProfile))
			cr.Delete("/{acc_addr}", withSignature(r.evm, pr.Unpin))
		})
	})

	cr.Route("/push/{contract_address}", func(cr chi.Router) {
		cr.Put("/{acc_addr}", withSignature(r.evm, pu.AddToken))
		cr.Delete("/{acc_addr}/{token}", withSignature(r.evm, pu.RemoveAccountToken))
	})

	cr.Route("/accounts", func(cr chi.Router) {
		cr.Get("/{acc_addr}/exists", acc.Exists)
		cr.Route("/factory/{factory_address}", func(cr chi.Router) {
			cr.Post("/", with1271Signature(r.evm, acc.Create))
			cr.Patch("/sca/{acc_addr}", with1271Signature(r.evm, acc.Upgrade))
		})
	})

	cr.Route("/rpc/{pm_address}", func(cr chi.Router) {
		cr.Post("/", withJSONRPCRequest(map[string]indexer.RPCHandlerFunc{
			"pm_sponsorUserOperation":   pm.Sponsor,
			"pm_ooSponsorUserOperation": pm.OOSponsor,
			"eth_sendUserOperation":     uop.Send,
			"eth_chainId":               ch.ChainId,
		}))
	})

	return cr, nil
}

func (r *Router) Start(port int, handler http.Handler) error {
	// start the server
	return http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}

func (r *Router) StartTLS(certpath string, handler http.Handler) error {
	certFile := fmt.Sprintf("%s/fullchain.pem", certpath)
	keyFile := fmt.Sprintf("%s/privkey.pem", certpath)

	config := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			crt, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, err
			}

			return &crt, err
		},
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%v", 443),
		Handler:   handler,
		TLSConfig: config,
	}

	// start the server
	return server.ListenAndServeTLS("", "")
}
