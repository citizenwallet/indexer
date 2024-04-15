package paymaster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
	pay "github.com/citizenwallet/smartcontracts/pkg/contracts/paymaster"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/tokenEntryPoint"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
)

var (
	// Allowed function signatures
	funcSigSingle = crypto.Keccak256([]byte("execute(address,uint256,bytes)"))[:4]
	funcSigBatch  = crypto.Keccak256([]byte("executeBatch(address[],uint256[],bytes[])"))[:4]

	// OO Signature limit in seconds
	ooSigLimit = int64(60 * 60 * 24 * 7)
)

type Service struct {
	evm indexer.EVMRequester

	db *db.DB
}

// NewService
func NewService(evm indexer.EVMRequester, db *db.DB) *Service {
	return &Service{
		evm,
		db,
	}
}

type paymasterType struct {
	Type string `json:"type"`
}

type paymasterData struct {
	PaymasterAndData     string `json:"paymasterAndData"`
	PreVerificationGas   string `json:"preVerificationGas"`
	VerificationGasLimit string `json:"verificationGasLimit"`
	CallGasLimit         string `json:"callGasLimit"`
}

func (s *Service) Sponsor(r *http.Request) (any, int) {
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
	var pt paymasterType

	for i, param := range params {
		switch i {
		case 0:
			v, ok := param.(map[string]interface{})
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
			v, ok := param.(map[string]interface{})
			if !ok {
				return nil, http.StatusBadRequest
			}

			b, err := json.Marshal(v)
			if err != nil {
				return nil, http.StatusBadRequest
			}

			err = json.Unmarshal(b, &pt)
			if err != nil {
				return nil, http.StatusBadRequest
			}
		}
	}

	if epAddr == "" {
		return nil, http.StatusBadRequest
	}

	// verify the user op

	ep, err := tokenEntryPoint.NewTokenEntryPoint(common.HexToAddress(epAddr), s.evm.Backend())
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	// verify that the paymaster is the correct one
	pmaddr, err := ep.Paymaster(nil)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	if pmaddr != addr {
		return nil, http.StatusBadRequest
	}

	// verify the nonce

	// get nonce using the account factory since we are not sure if the account has been created yet
	nonce := userop.Nonce

	// verify the init code
	initCode := hexutil.Encode(userop.InitCode)

	// if the nonce is not 0, then the init code should be empty
	if nonce.Cmp(big.NewInt(0)) == 1 && initCode != "0x" {
		return nil, http.StatusBadRequest
	}

	// if the nonce is 0, then check that the factory exists
	if nonce.Cmp(big.NewInt(0)) == 0 && len(userop.InitCode) > 20 {
		factoryaddr := common.BytesToAddress(userop.InitCode[:20])

		// Get the contract's bytecode
		bytecode, err := s.evm.CodeAt(context.Background(), factoryaddr, nil)
		if err != nil {
			fmt.Println(err)
			return nil, http.StatusInternalServerError
		}

		// Check if the contract is deployed
		if len(bytecode) == 0 {
			return nil, http.StatusBadRequest
		}
	}

	// verify the calldata, it should only be allowed to contain the function signatures we allow
	funcSig := userop.CallData[:4]
	if !bytes.Equal(funcSig, funcSigSingle) && !bytes.Equal(funcSig, funcSigBatch) {
		return nil, http.StatusBadRequest

	}

	addressArg, _ := abi.NewType("address", "address", nil)
	uint256Arg, _ := abi.NewType("uint256", "uint256", nil)
	bytesArg, _ := abi.NewType("bytes", "bytes", nil)
	callArgs := abi.Arguments{
		abi.Argument{
			Type: addressArg,
		},
		abi.Argument{
			Type: uint256Arg,
		},
		abi.Argument{
			Type: bytesArg,
		},
	}

	// Unpack the values
	callValues, err := callArgs.Unpack(userop.CallData[4:])
	if err != nil {
		return nil, http.StatusBadRequest
	}

	// destination address
	_, ok := callValues[0].(common.Address)
	if !ok {
		return nil, http.StatusBadRequest
	}

	// value in uint256
	callValue, ok := callValues[1].(*big.Int)
	if !ok || callValue.Cmp(big.NewInt(0)) != 0 {
		// shouldn't have any value
		return nil, http.StatusBadRequest
	}

	// data in bytes
	_, ok = callValues[2].([]byte)
	if !ok {
		return nil, http.StatusBadRequest
	}

	// validity period
	now := time.Now().Unix()
	validUntil := big.NewInt(now + 60)
	validAfter := big.NewInt(now - 10)

	// Ensure the values fit within 48 bits
	if validUntil.BitLen() > 48 || validAfter.BitLen() > 48 {
		return nil, http.StatusInternalServerError
	}

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
	validity, err := args.Pack(validUntil, validAfter)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	hash, err := pm.GetHash(nil, pay.UserOperation(userop), validUntil, validAfter)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	// Convert the hash to an Ethereum signed message hash
	hhash := accounts.TextHash(hash[:])

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

	sig, err := crypto.Sign(hhash, privateKey)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	// Ensure the v value is 27 or 28, this is because of the way Ethereum signature recovery works
	if sig[crypto.RecoveryIDOffset] == 0 || sig[crypto.RecoveryIDOffset] == 1 {
		sig[crypto.RecoveryIDOffset] += 27
	}

	data := append(addr.Bytes(), validity...)
	data = append(data, sig...)

	pd := &paymasterData{
		PaymasterAndData:     hexutil.Encode(data),
		PreVerificationGas:   hexutil.EncodeBig(userop.PreVerificationGas),
		VerificationGasLimit: hexutil.EncodeBig(userop.VerificationGasLimit),
		CallGasLimit:         hexutil.EncodeBig(userop.CallGasLimit),
	}

	return pd, http.StatusOK
}

// OOSponsor generates multiple signatures that can be used to send user operations in the future
func (s *Service) OOSponsor(r *http.Request) (any, int) {
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
	var pt paymasterType
	var amount int

	for i, param := range params {
		switch i {
		case 0:
			v, ok := param.(map[string]interface{})
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
			v, ok := param.(map[string]interface{})
			if !ok {
				return nil, http.StatusBadRequest
			}

			b, err := json.Marshal(v)
			if err != nil {
				return nil, http.StatusBadRequest
			}

			err = json.Unmarshal(b, &pt)
			if err != nil {
				return nil, http.StatusBadRequest
			}
		case 3:
			v, ok := param.(float64) // json marshalling converts numbers to float64
			if !ok {
				vstr, ok := param.(string)
				if !ok {
					amount = 10
				} else {
					amount, err = strconv.Atoi(vstr)
					if err != nil {
						amount = 10
					}
				}
			} else {
				amount = int(v)
			}
		}
	}

	if epAddr == "" {
		return nil, http.StatusBadRequest
	}

	// verify the user op
	// sender := userop.Sender

	ep, err := tokenEntryPoint.NewTokenEntryPoint(common.HexToAddress(epAddr), s.evm.Backend())
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	pmaddr, err := ep.Paymaster(nil)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	if pmaddr != addr {
		return nil, http.StatusBadRequest
	}

	// verify the calldata, it should only be allowed to contain the function signatures we allow
	funcSig := userop.CallData[:4]
	if !bytes.Equal(funcSig, funcSigSingle) && !bytes.Equal(funcSig, funcSigBatch) {
		return nil, http.StatusBadRequest

	}

	addressArg, _ := abi.NewType("address", "address", nil)
	uint256Arg, _ := abi.NewType("uint256", "uint256", nil)
	bytesArg, _ := abi.NewType("bytes", "bytes", nil)
	callArgs := abi.Arguments{
		abi.Argument{
			Type: addressArg,
		},
		abi.Argument{
			Type: uint256Arg,
		},
		abi.Argument{
			Type: bytesArg,
		},
	}

	// Unpack the values
	callValues, err := callArgs.Unpack(userop.CallData[4:])
	if err != nil {
		return nil, http.StatusBadRequest
	}

	// destination address
	_, ok := callValues[0].(common.Address)
	if !ok {
		return nil, http.StatusBadRequest
	}

	// value in uint256
	callValue, ok := callValues[1].(*big.Int)
	if !ok || callValue.Cmp(big.NewInt(0)) != 0 {
		// shouldn't have any value
		return nil, http.StatusBadRequest
	}

	// data in bytes
	_, ok = callValues[2].([]byte)
	if !ok {
		return nil, http.StatusBadRequest
	}

	// validity period
	now := time.Now().Unix()

	validUntil := big.NewInt(now + ooSigLimit)
	validAfter := big.NewInt(now - 10)

	// Ensure the values fit within 48 bits
	if validUntil.BitLen() > 48 || validAfter.BitLen() > 48 {
		return nil, http.StatusInternalServerError
	}

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
	validity, err := args.Pack(validUntil, validAfter)
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

	userops := []*indexer.UserOp{}

	// generate an amount of nonces equivalent to the amount requested
	for i := 0; i < amount; i++ {
		op := userop.Copy()

		nonce, err := comm.NewNonce()
		if err != nil {
			return nil, http.StatusInternalServerError
		}

		op.Nonce = nonce.BigInt()

		hash, err := pm.GetHash(nil, pay.UserOperation(op), validUntil, validAfter)
		if err != nil {
			return nil, http.StatusInternalServerError
		}

		// Convert the hash to an Ethereum signed message hash
		hhash := accounts.TextHash(hash[:])

		sig, err := crypto.Sign(hhash, privateKey)
		if err != nil {
			return nil, http.StatusInternalServerError
		}

		// Ensure the v value is 27 or 28, this is because of the way Ethereum signature recovery works
		if sig[crypto.RecoveryIDOffset] == 0 || sig[crypto.RecoveryIDOffset] == 1 {
			sig[crypto.RecoveryIDOffset] += 27
		}

		data := append(addr.Bytes(), validity...)
		data = append(data, sig...)

		op.PaymasterAndData = data

		userops = append(userops, &op)
	}

	return userops, http.StatusOK
}
