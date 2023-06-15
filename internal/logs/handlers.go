package logs

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
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

	// parse address from url params
	addr := chi.URLParam(r, "addr")

	// parse maxDate from url query
	maxDateq, _ := url.QueryUnescape(r.URL.Query().Get("maxDate"))

	t, err := time.Parse(time.RFC3339, maxDateq)
	if err != nil {
		println(err.Error())
		t = time.Now()
	}
	maxDate := indexer.SQLiteTime(t.UTC())

	// parse pagination params from url query
	limitq := r.URL.Query().Get("limit")
	offsetq := r.URL.Query().Get("offset")

	limit, err := strconv.Atoi(limitq)
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetq)
	if err != nil {
		offset = 0
	}

	tdb, ok := s.db.TransferDB[s.db.TransferName(contractAddr)]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// get logs from db
	logs, total, err := tdb.GetPaginatedTransfers(addr, maxDate, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = common.BodyMultiple(w, logs, common.Pagination{Limit: limit, Offset: offset, Total: total})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
