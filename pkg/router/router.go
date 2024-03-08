package router

import (
	"crypto/ecdsa"
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
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/internal/services/firebase"
	"github.com/citizenwallet/indexer/internal/userop"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/indexer/pkg/queue"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Router struct {
	chainId      *big.Int
	apiKey       string
	epAddr       string
	accFactAddr  string
	prfAddr      string
	evm          indexer.EVMRequester
	db           *db.DB
	useropq      *queue.Service
	b            *bucket.Bucket
	firebase     *firebase.PushService
	paymasterKey *ecdsa.PrivateKey
}

func NewServer(chainId *big.Int, apiKey string, epAddr, accFactAddr, prfAddr string, evm indexer.EVMRequester, db *db.DB, useropq *queue.Service, b *bucket.Bucket, firebase *firebase.PushService, pk *ecdsa.PrivateKey) *Router {
	return &Router{
		chainId,
		apiKey,
		epAddr,
		accFactAddr,
		prfAddr,
		evm,
		db,
		useropq,
		b,
		firebase,
		pk,
	}
}

func (r *Router) CreateHandler() (http.Handler, error) {
	cr := chi.NewRouter()

	comm, err := ethrequest.NewCommunity(r.evm, r.epAddr, r.accFactAddr, r.prfAddr)
	if err != nil {
		return nil, err
	}

	// configure middleware
	cr.Use(middleware.RequestID)
	cr.Use(middleware.Logger)

	// configure custom middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(RequestSizeLimitMiddleware(10 << 20)) // Limit request bodies to 10MB
	cr.Use(middleware.Compress(9))

	// instantiate handlers
	l := logs.NewService(r.chainId, r.db, r.evm)
	ev := events.NewService(r.db)
	pr := profiles.NewService(r.b, r.evm, comm)
	pu := push.NewService(r.db, comm)
	acc := accounts.NewService(r.evm, r.accFactAddr, r.db, r.paymasterKey)

	pm := paymaster.NewService(r.evm, r.db)
	uop := userop.NewService(r.evm, r.db, r.useropq, r.chainId)
	ch := chain.NewService(r.evm, r.chainId)

	// instantiate legacy handlers
	legl := logs.NewLegacyService(r.chainId, r.db, comm)
	legpr := profiles.NewLegacyService(r.b, comm)

	// configure routes
	cr.Route("/logs/v2/transfers", func(cr chi.Router) {
		cr.Route("/{token_address}", func(cr chi.Router) {
			cr.Get("/", l.GetAll)
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

	// configure legacy routes
	cr.Route("/logs/transfers", func(cr chi.Router) {
		// legacy support: for versions < 1.0.37
		cr.Route("/{contract_address}", func(cr chi.Router) {
			cr.Get("/{addr}", legl.Get)
			cr.Get("/{addr}/new", legl.GetNew)

			cr.Post("/{addr}", withSignature(r.evm, legl.AddSending))

			cr.Patch("/{addr}/{hash}", withSignature(r.evm, legl.SetStatus))
		})
	})

	cr.Route("/profiles", func(cr chi.Router) {
		// legacy support: for versions < 1.0.37
		cr.Put("/{acc_addr}", withMultiPartSignature(r.evm, legpr.PinMultiPartProfile))
		cr.Patch("/{acc_addr}", withSignature(r.evm, legpr.PinProfile))
		cr.Delete("/{acc_addr}", withSignature(r.evm, legpr.Unpin))
	})

	return cr, nil
}

func (r *Router) Start(port int, handler http.Handler) error {
	// start the server
	return http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}

func (r *Router) StartTLS(domain string, handler http.Handler) error {
	certFile := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	keyFile := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain)

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
