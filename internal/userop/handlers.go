package userop

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/indexer"
	pay "github.com/citizenwallet/smartcontracts/pkg/contracts/paymaster"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	evm indexer.EVMRequester

	paymasterKey *ecdsa.PrivateKey
}

// NewService
func NewService(evm indexer.EVMRequester, pk *ecdsa.PrivateKey) *Service {
	return &Service{
		evm,
		pk,
	}
}

func (s *Service) Send(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	addr := common.HexToAddress(contractAddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), addr, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Check if the contract is deployed
	if len(bytecode) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// instantiate paymaster contract
	pm, err := pay.NewPaymaster(addr, s.evm.Backend())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// parse the incoming params

	var params []any
	err = json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var userop indexer.UserOp
	var epAddr string

	for i, param := range params {
		switch i {
		case 0:
			v, ok := param.(map[string]any)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			b, err := json.Marshal(v)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = json.Unmarshal(b, &userop)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		case 1:
			v, ok := param.(string)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			epAddr = v
		}
	}

	if epAddr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// check the paymaster signature, make sure it matches the paymaster address

	// unpack the validity and check if it is valid
	// Define the arguments
	uint48Ty, _ := abi.NewType("uint48", "uint48", nil)
	args := abi.Arguments{
		abi.Argument{
			Type: uint48Ty,
		},
		abi.Argument{
			Type: uint48Ty,
		},
	}

	// Encode the values
	validity, err := args.Unpack(userop.PaymasterAndData[20:84])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	validUntil, ok := validity[0].(*big.Int)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	validAfter, ok := validity[1].(*big.Int)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// check if the signature is theoretically still valid
	now := time.Now().Unix()
	if validUntil.Int64() < now || validAfter.Int64() > now {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get the hash of the message that was signed
	hash, err := pm.GetHash(nil, pay.UserOperation(userop), validUntil, validAfter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Convert the hash to an Ethereum signed message hash
	hhash := accounts.TextHash(hash[:])

	sig := make([]byte, len(userop.PaymasterAndData[84:]))
	copy(sig, userop.PaymasterAndData[84:])

	// update the signature v to undo the 27/28 addition
	sig[crypto.RecoveryIDOffset] -= 27

	// recover the public key from the signature
	sigPublicKey, err := crypto.Ecrecover(hhash, sig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	publicKeyBytes := crypto.FromECDSAPub(&s.paymasterKey.PublicKey)

	// check if the public key matches the recovered public key
	matches := bytes.Equal(sigPublicKey, publicKeyBytes)
	if !matches {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

	ep, err := tokenEntryPoint.NewTokenEntryPoint(common.HexToAddress(epAddr), s.evm.Backend())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err := ep.HandleOps(transactor, []tokenEntryPoint.UserOperation{tokenEntryPoint.UserOperation(userop)}, common.HexToAddress(epAddr))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	comm.JSONRPCBody(w, tx.Hash().Hex(), nil)
}
