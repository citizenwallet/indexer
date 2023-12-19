package userop

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	pay "github.com/citizenwallet/smartcontracts/pkg/contracts/paymaster"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	evm     indexer.EVMRequester
	db      *db.DB
	chainId *big.Int
}

// NewService
func NewService(evm indexer.EVMRequester, db *db.DB, chid *big.Int) *Service {
	return &Service{
		evm,
		db,
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
	publicKey := privateKey.Public().(*ecdsa.PublicKey)

	// Convert the public key to an Ethereum address
	sponsor := crypto.PubkeyToAddress(*publicKey)

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

	nonce, err := s.evm.NonceAt(context.Background(), sponsor, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err := s.evm.NewTx(nonce, sponsor, entryPoint, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	chainId, err := s.evm.ChainID()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainId), privateKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// detect if this userop is a transfer using the calldata
	// Parse the contract ABI
	var tdb *db.TransferDB
	var log *indexer.Transfer

	dest, toaddr, amount, parseErr := comm.ParseERC20Transfer(userop.CallData)
	if parseErr == nil {
		// this is an erc20 transfer
		log = &indexer.Transfer{
			TokenID:   0,
			CreatedAt: time.Now(),
			From:      userop.Sender.Hex(),
			To:        toaddr.Hex(), //
			Nonce:     userop.Nonce.Int64(),
			Value:     amount,
			Status:    indexer.TransferStatusSending,
		}

		log.FromTo = log.CombineFromTo()

		log.GenerateHash(s.chainId.Int64())

		tdb, ok = s.db.TransferDB[s.db.TransferName(dest.Hex())]
		if ok {
			tdb.AddTransfer(log)
		}
	}

	err = s.evm.SendTransaction(signedTx)
	if err != nil {
		e, ok := err.(rpc.Error)
		if ok && e.ErrorCode() != -32000 {
			if parseErr == nil && tdb != nil && log != nil {
				tdb.RemoveSendingTransfer(log.Hash)
			}

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !strings.Contains(e.Error(), "insufficient funds") {
			if parseErr == nil && tdb != nil && log != nil {
				tdb.RemoveSendingTransfer(log.Hash)
			}

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if parseErr == nil && tdb != nil && log != nil {
			tdb.RemoveSendingTransfer(log.Hash)
		}

		// insufficient funds
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}

	if parseErr == nil && tdb != nil && log != nil {
		err = tdb.SetStatus(string(indexer.TransferStatusSending), signedTx.Hash().Hex())
		if err != nil {
			tdb.RemoveSendingTransfer(log.Hash)
		}
	}

	comm.JSONRPCBody(w, tx.Hash().Hex(), nil)
}
