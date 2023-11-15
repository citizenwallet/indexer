package accounts

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/accfactory"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/account"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	b   *bucket.Bucket
	evm indexer.EVMRequester

	paymasterKey *ecdsa.PrivateKey
}

func NewService(b *bucket.Bucket, evm indexer.EVMRequester, paymasterKey *ecdsa.PrivateKey) *Service {
	return &Service{
		b:            b,
		evm:          evm,
		paymasterKey: paymasterKey,
	}
}

type creationRequest struct {
	Owner string  `json:"owner"`
	Salt  big.Int `json:"salt"`
}

type creationResponse struct {
	AccountAddress string `json:"account_address"`
}

// Create handler for publishing an account
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	// ensure that the address in the request body matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	// parse owner address from url params
	var req creationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	owner := common.HexToAddress(req.Owner)

	if haccaddr != owner {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse account factory address from url params
	afaddr := chi.URLParam(r, "factory_address")

	af := common.HexToAddress(afaddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), af, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account factory contract is deployed
	if len(bytecode) == 0 {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// instantiate account factory contract
	afcontract, err := accfactory.NewAccfactory(af, s.evm.Backend())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// check if it already exists, don't allow to create again
	accaddr, err := afcontract.GetAddress(nil, owner, &req.Salt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the contract's bytecode
	bytecode, err = s.evm.CodeAt(context.Background(), accaddr, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account contract is already deployed
	if len(bytecode) > 0 {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// create account
	chainId, err := s.evm.ChainID()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	transactor, err := bind.NewKeyedTransactorWithChainID(s.paymasterKey, chainId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err := afcontract.CreateAccount(transactor, owner, common.Big0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// wait for tx to be mined
	err = s.evm.WaitForTx(tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.Body(w, &creationResponse{AccountAddress: accaddr.Hex()}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type upgradeRequest struct {
	Owner           string  `json:"owner"`
	Salt            big.Int `json:"salt"`
	EntryPoint      string  `json:"entry_point"`
	TokenEntryPoint string  `json:"token_entry_point"`
}

// Upgrade handler for upgrading an account
func (s *Service) Upgrade(w http.ResponseWriter, r *http.Request) {
	// ensure that the address in the url matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	if haccaddr != acc {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse owner address from url params
	var req upgradeRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	owner := common.HexToAddress(req.Owner)

	// parse account factory address from url params
	afaddr := chi.URLParam(r, "factory_address")

	af := common.HexToAddress(afaddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), af, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account factory contract is deployed
	if len(bytecode) == 0 {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// instantiate account factory contract
	afcontract, err := accfactory.NewAccfactory(af, s.evm.Backend())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// check if it already exists, don't allow to create again
	accaddrv2, err := afcontract.GetAddress(nil, owner, &req.Salt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the contract's bytecode
	bytecode, err = s.evm.CodeAt(context.Background(), accaddrv2, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account contract is already deployed
	if len(bytecode) > 0 {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// upgrade account
	chainId, err := s.evm.ChainID()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	transactor, err := bind.NewKeyedTransactorWithChainID(s.paymasterKey, chainId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(account.AccountMetaData.ABI)))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	caddr, tx, _, err := bind.DeployContract(transactor, parsedABI, []byte(account.AccountMetaData.Bin), s.evm.Backend(), common.HexToAddress(req.EntryPoint), common.HexToAddress(req.TokenEntryPoint))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// wait for tx to be mined
	err = s.evm.WaitForTx(tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ccontract, err := account.NewAccount(acc, s.evm.Backend())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err = ccontract.Initialize(transactor, owner)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// wait for tx to be mined
	err = s.evm.WaitForTx(tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	acccontract, err := account.NewAccount(acc, s.evm.Backend())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err = acccontract.UpgradeTo(transactor, caddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// wait for tx to be mined
	err = s.evm.WaitForTx(tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.Body(w, &creationResponse{AccountAddress: acc.Hex()}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
