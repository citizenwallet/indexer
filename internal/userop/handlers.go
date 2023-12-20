package userop

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/indexer/pkg/queue"
	pay "github.com/citizenwallet/smartcontracts/pkg/contracts/paymaster"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	evm     indexer.EVMRequester
	db      *db.DB
	useropq *queue.Service
	chainId *big.Int
}

// NewService
func NewService(evm indexer.EVMRequester, db *db.DB, useropq *queue.Service, chid *big.Int) *Service {
	return &Service{
		evm,
		db,
		useropq,
		chid,
	}
}

func (s *Service) Send(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "pm_address")

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

	// fetch the sponsor's corresponding private key from the db
	sponsorKey, err := s.db.SponsorDB.GetSponsor(addr.Hex())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Generate ecdsa.PrivateKey from bytes
	privateKey, err := comm.HexToPrivateKey(sponsorKey.PrivateKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	publicKeyBytes := crypto.FromECDSAPub(&privateKey.PublicKey)

	// check if the public key matches the recovered public key
	matches := bytes.Equal(sigPublicKey, publicKeyBytes)
	if !matches {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get the public key from the private key
	// publicKey := privateKey.Public().(*ecdsa.PublicKey)

	// Convert the public key to an Ethereum address
	// sponsor := crypto.PubkeyToAddress(*publicKey)

	entryPoint := common.HexToAddress(epAddr)

	// Parse the contract ABI
	parsedABI, err := tokenEntryPoint.TokenEntryPointMetaData.GetAbi()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Pack the function name and arguments into calldata
	data, err := parsedABI.Pack("handleOps", []tokenEntryPoint.UserOperation{tokenEntryPoint.UserOperation(userop)}, entryPoint)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create a new message
	message := indexer.NewTxMessage(addr, entryPoint, data, s.chainId, userop)

	// Enqueue the message
	s.useropq.Enqueue(*message)

	// Return the message ID
	comm.JSONRPCBody(w, message.ID, nil)
}
