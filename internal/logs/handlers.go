package logs

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	chainID *big.Int
	db      *db.DB

	evm indexer.EVMRequester
}

func NewService(chainID *big.Int, db *db.DB, evm indexer.EVMRequester) *Service {
	return &Service{
		chainID: chainID,
		db:      db,
		evm:     evm,
	}
}

func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

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

	chkaddr := com.ChecksumAddress(accaddr)

	// get logs from db
	logs, err := tdb.GetPaginatedTransfers(int64(tokenId), chkaddr, maxDate, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: remove legacy support
	total := offset + 10

	err = com.BodyMultiple(w, logs, com.Pagination{Limit: limit, Offset: offset, Total: total})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) GetNew(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

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

	chkaddr := com.ChecksumAddress(accaddr)

	// get logs from db
	logs, err := tdb.GetNewTransfers(int64(tokenId), chkaddr, fromDate, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.BodyMultiple(w, logs, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) AddSending(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	var log indexer.Transfer
	err := json.NewDecoder(r.Body).Decode(&log)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// check that the log is from the sender of the transaction
	if !com.IsSameHexAddress(log.From, accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Status = indexer.TransferStatusSending

	// make sure the addresses are EIP55 checksummed
	log.To = com.ChecksumAddress(log.To)
	log.From = com.ChecksumAddress(log.From)
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

	err = com.Body(w, log, nil)
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
	accaddr := chi.URLParam(r, "acc_addr")

	// parse hash from url params
	hash := chi.URLParam(r, "hash")

	var req setStatusRequest
	err := json.NewDecoder(r.Body).Decode(&req)
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

	tx, err := tdb.GetTransfer(req.UUID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// check that the log is from the sender of the transaction
	if !com.IsSameHexAddress(tx.From, accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = tdb.SetStatus(string(req.Status), hash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.Body(w, []byte("{}"), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
