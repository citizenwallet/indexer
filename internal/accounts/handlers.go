package accounts

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/accfactory"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	b   *bucket.Bucket
	evm indexer.EVMRequester

	entryPoint   common.Address
	paymasterKey *ecdsa.PrivateKey
}

func NewService(b *bucket.Bucket, evm indexer.EVMRequester, entryPoint string, paymasterKey *ecdsa.PrivateKey) *Service {
	return &Service{
		b:            b,
		evm:          evm,
		entryPoint:   common.HexToAddress(entryPoint),
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
		http.Error(w, "account contract is missing", http.StatusBadRequest)
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
		http.Error(w, "account contract is already deployed", http.StatusConflict)
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
	TokenEntryPoint string  `json:"token_entry_point"`
}

type upgradeResponse struct {
	AccountImplementation string `json:"account_implementation"`
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
		http.Error(w, "account factory contract is missing", http.StatusBadRequest)
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

	// Check if the account contract is already deployed and deploy if missing
	if len(bytecode) == 0 {
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

		tx, err := afcontract.CreateAccount(transactor, owner, &req.Salt)
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
	}

	// Get the contract's bytecode
	bytecode, err = s.evm.CodeAt(context.Background(), accaddrv2, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account contract was deployed
	if len(bytecode) == 0 {
		http.Error(w, "account contract is missing", http.StatusInternalServerError)
		return
	}

	slot := common.HexToHash(indexer.ImplementationStorageSlotKey)

	// Read the storage slot
	data, err := s.evm.StorageAt(accaddrv2, slot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert the data to a common.Hash
	implementation := common.BytesToHash(data)

	impladdr := common.HexToAddress(implementation.Hex())

	err = com.Body(w, &upgradeResponse{AccountImplementation: impladdr.Hex()}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
