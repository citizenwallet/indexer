package queue

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"
	"sync"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type UserOpService struct {
	inProgress map[common.Address][]string
	mu         sync.Mutex
	db         *db.DB
	evm        indexer.EVMRequester
}

func NewUserOpService(db *db.DB,
	evm indexer.EVMRequester) *UserOpService {
	return &UserOpService{
		inProgress: map[common.Address][]string{},
		db:         db,
		evm:        evm,
	}
}

// Process method processes messages of type []indexer.Message and returns processed messages and an errors if any.
func (s *UserOpService) Process(messages []indexer.Message) (invalid []indexer.Message, errors []error) {
	invalid = []indexer.Message{}
	errors = []error{}

	messagesByEntryPoint := map[common.Address][]indexer.Message{}
	txmByEntryPoint := map[common.Address][]indexer.UserOpMessage{}

	// first organize messages by txm.EntryPoint
	for _, message := range messages {
		// Type assertion to check if the msgs... is of type indexer.UserOpMessage
		txm, ok := message.Message.(indexer.UserOpMessage)
		if !ok {
			// If the message is not of type indexer.UserOpMessage, return an error
			invalid = append(invalid, message)
			errors = append(errors, fmt.Errorf("invalid tx msgs..."))
			continue
		}

		messagesByEntryPoint[txm.EntryPoint] = append(messagesByEntryPoint[txm.EntryPoint], message)
		txmByEntryPoint[txm.EntryPoint] = append(txmByEntryPoint[txm.EntryPoint], txm)
	}

	// go through each entry point and process the messages
	for entrypoint, txms := range txmByEntryPoint {
		sampleTxm := txms[0] // use the first txm to get information we need to process the messages
		msgs := messagesByEntryPoint[entrypoint]

		// Fetch the sponsor's corresponding private key from the database
		sponsorKey, err := s.db.SponsorDB.GetSponsor(sampleTxm.Paymaster.Hex())
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				for range msgs {
					errors = append(errors, err)
				}
			}
			continue
		}

		// Generate ecdsa.PrivateKey from bytes
		privateKey, err := comm.HexToPrivateKey(sponsorKey.PrivateKey)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Get the public key from the private key
		publicKey := privateKey.Public().(*ecdsa.PublicKey)

		// Convert the public key to an Ethereum address
		sponsor := crypto.PubkeyToAddress(*publicKey)

		// Get the nonce for the sponsor's address
		nonce, err := s.evm.NonceAt(context.Background(), sponsor, nil)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Get the in progress transactions for the entrypoint and increment the nonce
		inProgress := s.inProgress[entrypoint]
		nonce += uint64(len(inProgress))

		// Parse the contract ABI
		parsedABI, err := tokenEntryPoint.TokenEntryPointMetaData.GetAbi()
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		ops := []tokenEntryPoint.UserOperation{}

		for _, txm := range txms {
			ops = append(ops, tokenEntryPoint.UserOperation(txm.UserOp))
		}

		// Pack the function name and arguments into calldata
		data, err := parsedABI.Pack("handleOps", ops, sampleTxm.EntryPoint)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Create a new transaction
		tx, err := s.evm.NewTx(nonce, sponsor, sampleTxm.EntryPoint, data)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		// Sign the transaction
		signedTx, err := types.SignTx(tx, types.NewLondonSigner(sampleTxm.ChainId), privateKey)
		if err != nil {
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}
			continue
		}

		signedTxHash := signedTx.Hash().Hex()

		// update inProgress
		s.mu.Lock()
		s.inProgress[entrypoint] = append(s.inProgress[entrypoint], signedTxHash)
		s.mu.Unlock()

		insertedTransfers := map[common.Address][]string{}

		for _, txm := range txms {
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
					TxHash:    signedTxHash,
					TokenID:   0,
					CreatedAt: time.Now().UTC(),
					From:      userop.Sender.Hex(),
					To:        toaddr.Hex(),
					Nonce:     userop.Nonce.Int64(),
					Value:     amount,
					Data:      txdata,
					Status:    indexer.TransferStatusSending,
				}

				log.Hash = log.GenerateUniqueHash()

				// Combine the From and To addresses into a single string
				log.FromTo = log.CombineFromTo()

				suffix, err := s.db.TableNameSuffix(dest.Hex())
				if err == nil {
					// Get the transfer database for the destination address
					tdb, ok = s.db.TransferDB[suffix]
					if ok {
						// If the transfer database exists, add the transfer log to it
						tdb.AddTransfer(log)

						insertedTransfers[dest] = append(insertedTransfers[dest], log.Hash)
					}
				}
			}
		}

		// Send the signed transaction
		err = s.evm.SendTransaction(signedTx)
		if err != nil {
			// If there's an error, check if it's an RPC error
			e, ok := err.(rpc.Error)
			if ok && e.ErrorCode() != -32000 {
				// If it's an RPC error and the error code is not -32000, remove the sending transfer and return the error
				for dest, hashes := range insertedTransfers {
					suffix, err := s.db.TableNameSuffix(dest.Hex())
					if err == nil {
						tdb, ok := s.db.TransferDB[suffix]
						if ok {
							for _, hash := range hashes {
								tdb.RemoveTransfer(hash)
							}
						}
					}
				}

				invalid = append(invalid, msgs...)
				for range msgs {
					errors = append(errors, err)
				}

				// remove from inProgress
				s.mu.Lock()
				s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
					return s != signedTxHash
				})
				s.mu.Unlock()
				continue
			}

			if !strings.Contains(e.Error(), "insufficient funds") {
				// If the error is not about insufficient funds, remove the sending transfer and return the error
				for dest, hashes := range insertedTransfers {
					suffix, err := s.db.TableNameSuffix(dest.Hex())
					if err == nil {
						tdb, ok := s.db.TransferDB[suffix]
						if ok {
							for _, hash := range hashes {
								tdb.SetStatus(hash, string(indexer.TransferStatusFail))
							}
						}
					}
				}

				invalid = append(invalid, msgs...)
				for range msgs {
					errors = append(errors, err)
				}

				// remove from inProgress
				s.mu.Lock()
				s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
					return s != signedTxHash
				})
				s.mu.Unlock()
				continue
			}

			for dest, hashes := range insertedTransfers {
				suffix, err := s.db.TableNameSuffix(dest.Hex())
				if err == nil {
					tdb, ok := s.db.TransferDB[suffix]

					if ok {
						for _, hash := range hashes {
							tdb.SetStatus(hash, string(indexer.TransferStatusFail))
						}
					}
				}
			}

			// Return the error about insufficient funds
			invalid = append(invalid, msgs...)
			for range msgs {
				errors = append(errors, err)
			}

			// remove from inProgress
			s.mu.Lock()
			s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
				return s != signedTxHash
			})
			s.mu.Unlock()
			continue
		}

		for dest, hashes := range insertedTransfers {
			suffix, err := s.db.TableNameSuffix(dest.Hex())
			if err == nil {
				tdb, ok := s.db.TransferDB[suffix]
				if ok {
					for _, hash := range hashes {
						err := tdb.SetStatus(hash, string(indexer.TransferStatusPending))
						if err != nil {
							tdb.RemoveTransfer(hash)
						}
					}
				}
			}
		}

		go func() {
			// async wait for the transaction to be mined
			err = s.evm.WaitForTx(signedTx)
			if err != nil {
				for dest, hashes := range insertedTransfers {
					suffix, err := s.db.TableNameSuffix(dest.Hex())
					if err == nil {
						tdb, ok := s.db.TransferDB[suffix]
						if ok {
							for _, hash := range hashes {
								tdb.RemoveTransfer(hash)
							}
						}
					}
				}
			}

			// remove from inProgress
			s.mu.Lock()
			s.inProgress[entrypoint] = comm.Filter(s.inProgress[entrypoint], func(s string) bool {
				return s != signedTxHash
			})
			s.mu.Unlock()
		}()
	}

	return invalid, errors
}
