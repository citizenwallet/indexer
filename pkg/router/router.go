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
	chainId *big.Int
	apiKey  string
	es      *ethrequest.EthService
	db      *db.DB
}

func NewServer(chainId *big.Int, apiKey string, es *ethrequest.EthService, db *db.DB) *Router {
	return &Router{
		chainId,
		apiKey,
		es,
		db,
	}
}

// implement the Server interface
func (r *Router) Start(port int) error {
	cr := chi.NewRouter()

	a := auth.New(r.apiKey)

	// configure middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(a.AuthMiddleware)
	cr.Use(middleware.Compress(9))

	// instantiate handlers
	l := logs.NewService(r.db)
	ev := events.NewService(r.db)

	// configure routes
	cr.Route("/logs", func(cr chi.Router) {
		cr.Route("/{contractAddr}", func(cr chi.Router) {
			cr.Get("/{addr}", l.GetLogs)
		})
	})

	cr.Route("/events", func(cr chi.Router) {
		cr.Post("/", ev.AddEvent)
	})

	// start the server
	return http.ListenAndServe(fmt.Sprintf(":%v", port), cr)
}
