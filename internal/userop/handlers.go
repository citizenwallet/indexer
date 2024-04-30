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

func (s *Service) Send(r *http.Request) (any, int) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "pm_address")

	addr := common.HexToAddress(contractAddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), addr, nil)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	// Check if the contract is deployed
	if len(bytecode) == 0 {
		return nil, http.StatusBadRequest
	}

	// instantiate paymaster contract
	pm, err := pay.NewPaymaster(addr, s.evm.Backend())
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	// parse the incoming params

	var params []any
	err = json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		return nil, http.StatusBadRequest
	}

	var userop indexer.UserOp
	var epAddr string
	var txdata *indexer.TransferData

	for i, param := range params {
		switch i {
		case 0:
			v, ok := param.(map[string]any)
			if !ok {
				return nil, http.StatusBadRequest
			}
			b, err := json.Marshal(v)
			if err != nil {
				return nil, http.StatusBadRequest
			}

			err = json.Unmarshal(b, &userop)
			if err != nil {
				return nil, http.StatusBadRequest
			}
		case 1:
			v, ok := param.(string)
			if !ok {
				return nil, http.StatusBadRequest
			}

			epAddr = v
		case 2:
			v, ok := param.(map[string]any)
			if !ok {
				return nil, http.StatusBadRequest
			}

			b, err := json.Marshal(v)
			if err != nil {
				return nil, http.StatusBadRequest
			}

			err = json.Unmarshal(b, &txdata)
			if err != nil {
				return nil, http.StatusBadRequest
			}
		}
	}

	if epAddr == "" {
		return nil, http.StatusBadRequest
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
		return nil, http.StatusInternalServerError
	}

	validUntil, ok := validity[0].(*big.Int)
	if !ok {
		return nil, http.StatusInternalServerError
	}

	validAfter, ok := validity[1].(*big.Int)
	if !ok {
		return nil, http.StatusInternalServerError
	}

	// check if the signature is theoretically still valid
	now := time.Now().Unix()
	if validUntil.Int64() < now || validAfter.Int64() > now {
		return nil, http.StatusBadRequest
	}

	// Get the hash of the message that was signed
	hash, err := pm.GetHash(nil, pay.UserOperation(userop), validUntil, validAfter)
	if err != nil {
		return nil, http.StatusInternalServerError
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
		return nil, http.StatusInternalServerError
	}

	// fetch the sponsor's corresponding private key from the db
	sponsorKey, err := s.db.SponsorDB.GetSponsor(addr.Hex())
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	// Generate ecdsa.PrivateKey from bytes
	privateKey, err := comm.HexToPrivateKey(sponsorKey.PrivateKey)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	publicKeyBytes := crypto.FromECDSAPub(&privateKey.PublicKey)

	// check if the public key matches the recovered public key
	matches := bytes.Equal(sigPublicKey, publicKeyBytes)
	if !matches {
		return nil, http.StatusBadRequest
	}

	entryPoint := common.HexToAddress(epAddr)

	// Create a new message
	message := indexer.NewTxMessage(addr, entryPoint, s.chainId, userop, txdata)

	// Enqueue the message
	s.useropq.Enqueue(*message)

	resp, err := message.WaitForResponse()
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	txHash, ok := resp.(string)
	if !ok {
		return nil, http.StatusInternalServerError
	}

	// Return the message ID
	return txHash, http.StatusOK
}
