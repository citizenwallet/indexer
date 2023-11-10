package router

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net/http"

	"github.com/citizenwallet/indexer/internal/auth"
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
	b            *bucket.Bucket
	firebase     *firebase.PushService
	paymasterKey *ecdsa.PrivateKey
}

func NewServer(chainId *big.Int, apiKey string, epAddr, accFactAddr, prfAddr string, evm indexer.EVMRequester, db *db.DB, b *bucket.Bucket, firebase *firebase.PushService, pk *ecdsa.PrivateKey) *Router {
	return &Router{
		chainId,
		apiKey,
		epAddr,
		accFactAddr,
		prfAddr,
		evm,
		db,
		b,
		firebase,
		pk,
	}
}

// implement the Server interface
func (r *Router) Start(port int) error {
	cr := chi.NewRouter()

	a := auth.New(r.apiKey)
	comm, err := ethrequest.NewCommunity(r.evm, r.epAddr, r.accFactAddr, r.prfAddr)
	if err != nil {
		return err
	}

	// configure middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(a.AuthMiddleware)
	cr.Use(middleware.Compress(9))

	// instantiate handlers
	l := logs.NewService(r.chainId, r.db, comm)
	ev := events.NewService(r.db)
	pr := profiles.NewService(r.b, comm)
	pu := push.NewService(r.db, comm)

	pm := paymaster.NewService(r.evm, r.paymasterKey)
	uop := userop.NewService(r.evm, r.paymasterKey)

	// configure routes
	cr.Route("/logs/transfers", func(cr chi.Router) {
		cr.Route("/{contract_address}", func(cr chi.Router) {
			cr.Get("/{addr}", l.Get)
			cr.Get("/{addr}/new", l.GetNew)

			cr.Post("/{addr}", withSignature(l.AddSending))

			cr.Patch("/{addr}/{hash}", withSignature(l.SetStatus))
		})
	})

	cr.Route("/events", func(cr chi.Router) {
		cr.Post("/", ev.AddEvent)
	})

	cr.Route("/profiles", func(cr chi.Router) {
		cr.Put("/{addr}", withMultiPartSignature(pr.PinMultiPartProfile))
		cr.Patch("/{addr}", withSignature(pr.PinProfile))
		cr.Delete("/{addr}", withSignature(pr.Unpin))
	})

	cr.Route("/push/{contract_address}", func(cr chi.Router) {
		cr.Put("/", withSignature(pu.AddToken))
		cr.Delete("/{addr}/{token}", withSignature(pu.RemoveAccountToken))
	})

	cr.Route("/rpc/{contract_address}", func(cr chi.Router) {
		cr.Post("/", withJSONRPCRequest(map[string]http.HandlerFunc{
			"pm_sponsorUserOperation": pm.Sponsor,
			"eth_sendUserOperation":   uop.Send,
		}))
	})

	// start the server
	return http.ListenAndServe(fmt.Sprintf(":%v", port), cr)
}
