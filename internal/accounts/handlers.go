package accounts

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/accfactory"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/account"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	evm indexer.EVMRequester

	db *db.DB
}

func NewService(evm indexer.EVMRequester, db *db.DB) *Service {
	return &Service{
		evm: evm,
		db:  db,
	}
}

// Create handler for publishing an account
func (s *Service) Exists(w http.ResponseWriter, r *http.Request) {
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), acc, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account contract is already deployed
	if len(bytecode) == 0 {
		http.Error(w, "account contract does not exist", http.StatusNotFound)
		return
	}

	err = com.Body(w, nil, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type creationRequest struct {
	Owner string  `json:"owner"`
	Salt  big.Int `json:"salt"`
}

type creationResponse struct {
	AccountAddress string `json:"account_address"`
}

// Deprecated: This is handled by initCode transactions now
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

	// fetch the sponsor address from the paymaster contract
	accimpl, err := afcontract.AccountImplementation(nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	accContract, err := account.NewAccount(accimpl, s.evm.Backend())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var privateKey *ecdsa.PrivateKey

	tep, err := accContract.TokenEntryPoint(nil)
	if err != nil {
		e, ok := err.(rpc.Error)
		if ok && e.ErrorCode() != -32000 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// legacy account with a token entrypoint
		// A hard migration was conducted in November 2023 for all accounts to have a token entrypoint
		// this allows for 4337 transactions without the need for a full node
		http.Error(w, err.Error(), http.StatusBadRequest)
		return

	} else {
		tepContract, err := tokenEntryPoint.NewTokenEntryPoint(tep, s.evm.Backend())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		paddr, err := tepContract.Paymaster(nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// fetch corresponding private key from the db
		sponsorKey, err := s.db.SponsorDB.GetSponsor(paddr.Hex())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Generate ecdsa.PrivateKey from bytes
		privateKey, err = com.HexToPrivateKey(sponsorKey.PrivateKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// Get the public key from the private key
	publicKey := privateKey.Public().(*ecdsa.PublicKey)

	// Convert the public key to an Ethereum address
	sponsor := crypto.PubkeyToAddress(*publicKey)

	// Get the nonce for the sponsor's address
	nonce, err := s.evm.NonceAt(context.Background(), sponsor, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Parse the contract ABI
	parsedABI, err := accfactory.AccfactoryMetaData.GetAbi()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data, err := parsedABI.Pack("createAccount", owner, &req.Salt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create a new transaction
	tx, err := s.evm.NewTx(nonce, sponsor, af, data, true)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainId), privateKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.evm.SendTransaction(signedTx)
	if err != nil {
		e, ok := err.(rpc.Error)
		if ok && e.ErrorCode() != -32000 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !strings.Contains(e.Error(), "insufficient funds") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// insufficient funds
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}

	// TODO: use the queue here
	// wait for tx to be mined
	err = s.evm.WaitForTx(tx, 3)
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
	Owner string  `json:"owner"`
	Salt  big.Int `json:"salt"`
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

	// check if the account is already upgraded
	impladdr, err := get1967ProxyImplementation(s.evm, acc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	implebytecode, err := s.evm.CodeAt(context.Background(), *impladdr, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if the account contract was deployed
	if len(implebytecode) == 0 {
		http.Error(w, "account implementation is missing", http.StatusInternalServerError)
		return
	}

	afimpl, err := afcontract.AccountImplementation(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the contract's bytecode
	afbytecode, err := s.evm.CodeAt(context.Background(), afimpl, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account factory contract is deployed
	if len(afbytecode) == 0 {
		http.Error(w, "account factory contract implementation is missing", http.StatusBadRequest)
		return
	}

	if bytes.Equal(implebytecode, afbytecode) {
		http.Error(w, "account is already upgraded", http.StatusConflict)
		return
	}

	// check if it already exists, create if needed
	accaddrv2, err := afcontract.GetAddress(nil, owner, &req.Salt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the contract's bytecode
	acc2bytecode, err := s.evm.CodeAt(context.Background(), accaddrv2, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the account contract is already deployed and deploy if missing
	if len(acc2bytecode) == 0 {
		// TODO: re-evaluate the necessity of this. If an account does not exist, how can it be upgraded?

		// upgrade account
		chainId, err := s.evm.ChainID()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		accContract, err := account.NewAccount(afimpl, s.evm.Backend())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tep, err := accContract.TokenEntryPoint(nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tepContract, err := tokenEntryPoint.NewTokenEntryPoint(tep, s.evm.Backend())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		paddr, err := tepContract.Paymaster(nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// fetch corresponding private key from the db
		sponsorKey, err := s.db.SponsorDB.GetSponsor(paddr.Hex())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Generate ecdsa.PrivateKey from bytes
		privateKey, err := com.HexToPrivateKey(sponsorKey.PrivateKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		transactor, err := bind.NewKeyedTransactorWithChainID(privateKey, chainId)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tx, err := afcontract.CreateAccount(transactor, owner, &req.Salt)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: use the queue here
		// wait for tx to be mined
		err = s.evm.WaitForTx(tx, 10)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	v2Impladdr, err := get1967ProxyImplementation(s.evm, accaddrv2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = com.Body(w, &upgradeResponse{AccountImplementation: v2Impladdr.Hex()}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func get1967ProxyImplementation(evm indexer.EVMRequester, addr common.Address) (*common.Address, error) {
	bytecode, err := evm.CodeAt(context.Background(), addr, nil)
	if err != nil {
		return nil, err
	}

	// Check if the account contract was deployed
	if len(bytecode) == 0 {
		return nil, errors.New("account contract is missing")
	}

	slot := common.HexToHash(indexer.ImplementationStorageSlotKey)

	// Read the storage slot
	data, err := evm.StorageAt(addr, slot)
	if err != nil {
		return nil, err
	}

	// Convert the data to a common.Hash
	implementation := common.BytesToHash(data)

	impl := common.HexToAddress(implementation.Hex())

	return &impl, nil
}
