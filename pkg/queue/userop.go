package queue

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"strings"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type UserOpService struct {
	db  *db.DB
	evm indexer.EVMRequester
}

func NewUserOpService(db *db.DB,
	evm indexer.EVMRequester) *UserOpService {
	return &UserOpService{
		db,
		evm,
	}
}

// Process method processes a message of type indexer.Message and returns a processed message and an error if any.
func (s *UserOpService) Process(message indexer.Message) (indexer.Message, error) {
	// Type assertion to check if the message is of type indexer.UserOpMessage
	txm, ok := message.Message.(indexer.UserOpMessage)
	if !ok {
		// If the message is not of type indexer.UserOpMessage, return an error
		return message, errors.New("invalid tx message")
	}

	// Fetch the sponsor's corresponding private key from the database
	sponsorKey, err := s.db.SponsorDB.GetSponsor(txm.Paymaster.Hex())
	if err != nil {
		return message, err
	}

	// Generate ecdsa.PrivateKey from bytes
	privateKey, err := comm.HexToPrivateKey(sponsorKey.PrivateKey)
	if err != nil {
		return message, err
	}

	// Get the public key from the private key
	publicKey := privateKey.Public().(*ecdsa.PublicKey)

	// Convert the public key to an Ethereum address
	sponsor := crypto.PubkeyToAddress(*publicKey)

	// Get the nonce for the sponsor's address
	nonce, err := s.evm.NonceAt(context.Background(), sponsor, nil)
	if err != nil {
		return message, err
	}

	// Create a new transaction
	tx, err := s.evm.NewTx(nonce, sponsor, txm.To, txm.Data)
	if err != nil {
		return message, err
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(txm.ChainId), privateKey)
	if err != nil {
		return message, err
	}

	// Detect if this user operation is a transfer using the call data
	// Parse the contract ABI
	var tdb *db.TransferDB
	var log *indexer.Transfer

	userop := txm.UserOp
	txdata, ok := txm.ExtraData.(*indexer.TransferData)
	if !ok {
		// if it's invalid, set it to nil to avoid errors and corrupted json
		txdata = nil
	}

	// Parse the ERC20 transfer from the call data
	dest, toaddr, amount, parseErr := comm.ParseERC20Transfer(userop.CallData)
	if parseErr == nil {
		// If the parsing is successful, this is an ERC20 transfer
		// Create a new transfer log
		log = &indexer.Transfer{
			Hash:      signedTx.Hash().Hex(),
			TxHash:    signedTx.Hash().Hex(),
			TokenID:   0,
			CreatedAt: time.Now(),
			From:      userop.Sender.Hex(),
			To:        toaddr.Hex(),
			Nonce:     userop.Nonce.Int64(),
			Value:     amount,
			Data:      txdata,
			Status:    indexer.TransferStatusSending,
		}

		// Combine the From and To addresses into a single string
		log.FromTo = log.CombineFromTo()

		// Get the transfer database for the destination address
		name, flag := s.db.TransferName(dest.Hex())
		if flag {
			err := errors.New("Bad contract address")
			return message, err
		}

		tdb, ok = s.db.TransferDB[name]
		if ok {
			// If the transfer database exists, add the transfer log to it
			tdb.AddTransfer(log)
		}
	}

	// Send the signed transaction
	err = s.evm.SendTransaction(signedTx)
	if err != nil {
		// If there's an error, check if it's an RPC error
		e, ok := err.(rpc.Error)
		if ok && e.ErrorCode() != -32000 {
			// If it's an RPC error and the error code is not -32000, remove the sending transfer and return the error
			if parseErr == nil && tdb != nil && log != nil {
				tdb.RemoveTransfer(log.Hash)
			}

			return message, err
		}

		if !strings.Contains(e.Error(), "insufficient funds") {
			// If the error is not about insufficient funds, remove the sending transfer and return the error
			if parseErr == nil && tdb != nil && log != nil {
				tdb.SetStatus(log.Hash, string(indexer.TransferStatusFail))
			}

			return message, err
		}

		if parseErr == nil && tdb != nil && log != nil {
			// If there are insufficient funds, set the status of the transfer to fail
			tdb.SetStatus(log.Hash, string(indexer.TransferStatusFail))
		}

		// Return the error about insufficient funds
		return message, err
	}

	if parseErr == nil && tdb != nil && log != nil {
		err = tdb.SetStatus(string(indexer.TransferStatusPending), signedTx.Hash().Hex())
		if err != nil {
			tdb.RemoveTransfer(log.Hash)
		}
	}

	err = s.evm.WaitForTx(signedTx)
	if err != nil {
		if parseErr == nil && tdb != nil && log != nil {
			tdb.RemoveTransfer(log.Hash)
		}

		return message, err
	}

	return indexer.Message{}, nil
}
