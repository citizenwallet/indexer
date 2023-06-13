package router

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/citizenwallet/node/internal/db"
	"github.com/citizenwallet/node/internal/ethrequest"
	"github.com/citizenwallet/node/internal/logs"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Router struct {
	chainId *big.Int
	es      *ethrequest.EthService
	db      *db.DB
}

func NewServer(chainId *big.Int, es *ethrequest.EthService, db *db.DB) *Router {
	return &Router{
		chainId,
		es,
		db,
	}
}

// implement the Server interface
func (r *Router) Start(port int) error {
	cr := chi.NewRouter()

	// configure middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(middleware.Compress(9))

	// instantiate handlers
	logs := logs.NewService(r.db)

	// configure routes
	cr.Route("/logs", func(cr chi.Router) {
		cr.Route("/{contractAddr}", func(cr chi.Router) {
			cr.Get("/", logs.GetLogs)
		})
	})

	// start the server
	return http.ListenAndServe(fmt.Sprintf(":%v", port), cr)
}
