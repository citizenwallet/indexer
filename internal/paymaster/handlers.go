package paymaster

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/index"
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
)

type Service struct {
	evm index.EVMRequester

	paymasterKey *ecdsa.PrivateKey
}

// NewService
func NewService(evm index.EVMRequester, pk *ecdsa.PrivateKey) *Service {
	return &Service{
		evm,
		pk,
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

func (s *Service) Sponsor(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	addr := common.HexToAddress(contractAddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.Client().CodeAt(context.Background(), addr, nil)
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
	pm, err := pay.NewPaymaster(addr, s.evm.Client())
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
	var pt paymasterType

	for i, param := range params {
		switch i {
		case 0:
			v, ok := param.(map[string]interface{})
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
		case 2:
			v, ok := param.(map[string]interface{})
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			b, err := json.Marshal(v)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = json.Unmarshal(b, &pt)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
	}

	if epAddr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// verify the user op
	sender := userop.Sender

	ep, err := tokenEntryPoint.NewTokenEntryPoint(common.HexToAddress(epAddr), s.evm.Client())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// verify the nonce

	// get nonce using the account factory since we are not sure if the account has been created yet
	nonce, err := ep.GetNonce(nil, sender, common.Big0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// make sure that the nonce is correct
	if nonce.Cmp(userop.Nonce) != 0 {
		// nonce is wrong
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// verify the init code
	initCode := hexutil.Encode(userop.InitCode)

	// if the nonce is not 0, then the init code should be empty
	if nonce.Cmp(big.NewInt(0)) == 1 && initCode != "0x" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// if the nonce is 0, then check that the factory returns the sender
	if nonce.Cmp(big.NewInt(0)) == 0 {
		factoryaddr := common.BytesToAddress(userop.InitCode[:20])

		// Get the contract's bytecode
		bytecode, err := s.evm.Client().CodeAt(context.Background(), factoryaddr, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Check if the contract is deployed
		if len(bytecode) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// verify the calldata, it should only be allowed to contain the function signatures we allow
	funcSig := userop.CallData[:4]
	if !bytes.Equal(funcSig, funcSigSingle) && !bytes.Equal(funcSig, funcSigBatch) {
		w.WriteHeader(http.StatusBadRequest)
		return

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
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// destination address
	_, ok := callValues[0].(common.Address)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// value in uint256
	callValue, ok := callValues[1].(*big.Int)
	if !ok || callValue.Cmp(big.NewInt(0)) != 0 {
		// shouldn't have any value
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// data in bytes
	_, ok = callValues[2].([]byte)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// validity period
	now := time.Now().Unix()
	validUntil := big.NewInt(now + 30)
	validAfter := big.NewInt(now - 10)

	// Ensure the values fit within 48 bits
	if validUntil.BitLen() > 48 || validAfter.BitLen() > 48 {
		w.WriteHeader(http.StatusInternalServerError)
		return
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hash, err := pm.GetHash(nil, pay.UserOperation(userop), validUntil, validAfter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Convert the hash to an Ethereum signed message hash
	hhash := accounts.TextHash(hash[:])

	sig, err := crypto.Sign(hhash, s.paymasterKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
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

	comm.JSONRPCBody(w, pd, nil)
}
