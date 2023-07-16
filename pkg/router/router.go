package router

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/citizenwallet/indexer/internal/auth"
	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/internal/ethrequest"
	"github.com/citizenwallet/indexer/internal/events"
	"github.com/citizenwallet/indexer/internal/logs"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Router struct {
	chainId     *big.Int
	apiKey      string
	epAddr      string
	accFactAddr string
	es          *ethrequest.EthService
	db          *db.DB
}

func NewServer(chainId *big.Int, apiKey string, epAddr, accFactAddr string, es *ethrequest.EthService, db *db.DB) *Router {
	return &Router{
		chainId,
		apiKey,
		epAddr,
		accFactAddr,
		es,
		db,
	}
}

// implement the Server interface
func (r *Router) Start(port int) error {
	cr := chi.NewRouter()

	a := auth.New(r.apiKey)
	comm, err := ethrequest.NewCommunity(r.es, r.epAddr, r.accFactAddr)
	if err != nil {
		return err
	}

	// configure middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(a.AuthMiddleware)
	cr.Use(middleware.Compress(9))

	// instantiate handlers
	l := logs.NewService(r.chainId, r.db, r.es, comm)
	ev := events.NewService(r.db)

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

	// start the server
	return http.ListenAndServe(fmt.Sprintf(":%v", port), cr)
}
