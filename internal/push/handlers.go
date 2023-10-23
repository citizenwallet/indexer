package push

import (
	"encoding/json"
	"net/http"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	db   *db.DB
	comm *ethrequest.Community
}

func NewService(db *db.DB, comm *ethrequest.Community) *Service {
	return &Service{
		db:   db,
		comm: comm,
	}
}

func (s *Service) AddToken(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// the account should match the one in the headers
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

	var pt indexer.PushToken
	err = json.NewDecoder(r.Body).Decode(&pt)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// make sure the addresses are EIP55 checksummed
	pt.Account = common.ChecksumAddress(pt.Account)

	// check that the push token is from the sender of the transaction
	if !common.IsSameHexAddress(pt.Account, acc.Hex()) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tname := s.db.TransferName(contractAddr)

	pdb, ok := s.db.PushTokenDB[tname]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = pdb.AddToken(&pt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = common.Body(w, pt, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) RemoveAccountToken(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "addr")

	// parse token from url params
	token := chi.URLParam(r, "token")

	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// the account should match the one in the headers
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

	tname := s.db.TransferName(contractAddr)

	pdb, ok := s.db.PushTokenDB[tname]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = pdb.RemoveAccountPushToken(token, accaddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = common.Body(w, []byte("{}"), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
