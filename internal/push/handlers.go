package push

import (
	"encoding/json"
	"net/http"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/ethereum/go-ethereum/common"
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

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	var pt indexer.PushToken
	err := json.NewDecoder(r.Body).Decode(&pt)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// make sure the addresses are EIP55 checksummed
	pt.Account = com.ChecksumAddress(pt.Account)

	// check that the push token is from the sender of the transaction
	if !com.IsSameHexAddress(pt.Account, acc.Hex()) {
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

	err = com.Body(w, pt, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) RemoveAccountToken(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	// parse token from url params
	token := chi.URLParam(r, "token")

	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tname := s.db.TransferName(contractAddr)

	pdb, ok := s.db.PushTokenDB[tname]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err := pdb.RemoveAccountPushToken(token, accaddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.Body(w, []byte("{}"), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
