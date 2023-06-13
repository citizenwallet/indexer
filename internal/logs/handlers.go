package logs

import (
	"net/http"

	"github.com/citizenwallet/node/internal/db"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	db *db.DB
}

func NewService(db *db.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) GetLogs(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contractAddr")

	println("get logs for: ", contractAddr)
}
