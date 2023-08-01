package logs

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	chainID *big.Int
	db      *db.DB

	comm *ethrequest.Community
}

func NewService(chainID *big.Int, db *db.DB, comm *ethrequest.Community) *Service {
	return &Service{
		chainID: chainID,
		db:      db,
		comm:    comm,
	}
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	addr := chi.URLParam(r, "addr")

	// parse maxDate from url query
	maxDateq, _ := url.QueryUnescape(r.URL.Query().Get("maxDate"))

	t, err := time.Parse(time.RFC3339, maxDateq)
	if err != nil {
		t = time.Now()
	}
	maxDate := t.UTC()

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

	tokenIdq := r.URL.Query().Get("tokenId")
	tokenId, err := strconv.Atoi(tokenIdq)
	if err != nil {
		tokenId = 0
	}

	tdb, ok := s.db.TransferDB[s.db.TransferName(contractAddr)]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// get logs from db
	logs, err := tdb.GetPaginatedTransfers(int64(tokenId), addr, maxDate, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: remove legacy support
	total := offset + 10

	err = common.BodyMultiple(w, logs, common.Pagination{Limit: limit, Offset: offset, Total: total})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) GetNew(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	addr := chi.URLParam(r, "addr")

	// parse fromDate from url query
	fromDateq, _ := url.QueryUnescape(r.URL.Query().Get("fromDate"))

	t, err := time.Parse(time.RFC3339, fromDateq)
	if err != nil {
		t = time.Now()
	}
	fromDate := t.UTC()

	// parse pagination params from url query
	limitq := r.URL.Query().Get("limit")

	limit, err := strconv.Atoi(limitq)
	if err != nil {
		limit = 10
	}

	tokenIdq := r.URL.Query().Get("tokenId")
	tokenId, err := strconv.Atoi(tokenIdq)
	if err != nil {
		tokenId = 0
	}

	tdb, ok := s.db.TransferDB[s.db.TransferName(contractAddr)]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// get logs from db
	logs, err := tdb.GetNewTransfers(int64(tokenId), addr, fromDate, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = common.BodyMultiple(w, logs, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) AddSending(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "addr")

	// the address in the url should match the one in the headers
	haddr, ok := indexer.GetAddressFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	acc, err := s.comm.GetAccount(haddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !common.IsSameHexAddress(acc.Hex(), accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var log indexer.Transfer
	err = json.NewDecoder(r.Body).Decode(&log)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// check that the log is from the sender of the transaction
	if !common.IsSameHexAddress(log.From, accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Status = indexer.TransferStatusSending

	// make sure the addresses are EIP55 checksummed
	log.To = common.ChecksumAddress(log.To)
	log.From = common.ChecksumAddress(log.From)
	log.FromTo = log.CombineFromTo()

	tdb, ok := s.db.TransferDB[s.db.TransferName(contractAddr)]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = tdb.AddTransfer(&log)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = common.Body(w, log, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type setStatusRequest struct {
	Status indexer.TransferStatus `json:"status"`
	UUID   string                 `json:"uuid"`
}

func (s *Service) SetStatus(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "addr")

	// parse hash from url params
	hash := chi.URLParam(r, "hash")

	// the address in the url should match the one in the headers
	haddr, ok := indexer.GetAddressFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	acc, err := s.comm.GetAccount(haddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !common.IsSameHexAddress(acc.Hex(), accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req setStatusRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.UUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tdb, ok := s.db.TransferDB[s.db.TransferName(contractAddr)]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = tdb.SetStatus(string(req.Status), hash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = common.Body(w, []byte("{}"), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
