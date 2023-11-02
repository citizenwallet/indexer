package paymaster

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/authorizer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
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

	// instantiate account factory contract
	// af, err := accfactory.NewAccfactory(addr, s.evm.Client())
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// Get the contract's bytecode
	bytecode, err := s.evm.Client().CodeAt(context.Background(), addr, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Check if the contract is deployed
	if len(bytecode) == 0 {
		w.WriteHeader(http.StatusBadRequest)
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

	auth, err := authorizer.NewAuthorizer(common.HexToAddress(epAddr), s.evm.Client())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// verify the user op
	sender := common.HexToAddress(userop.Sender)

	// get nonce using the account factory since we are not sure if the account has been created yet
	nonce, err := auth.GetNonce(nil, sender, common.Big0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Decode hex string to bytes
	bytes := common.FromHex(userop.Nonce)

	// Get *big.Int from bytes
	opnonce := new(big.Int)
	opnonce.SetBytes(bytes)

	// make sure that the nonce is correct
	if nonce.Cmp(opnonce) != 0 {
		// nonce is wrong
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// if the nonce is not 0, then the init code should be empty
	if nonce.Cmp(big.NewInt(0)) == 1 && userop.InitCode != "0x" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	op := &authorizer.UserOperation{
		Sender:               common.HexToAddress(userop.Sender),
		Nonce:                comm.HexToBigInt(userop.Nonce),
		InitCode:             common.FromHex(userop.InitCode),
		CallData:             common.FromHex(userop.CallData),
		CallGasLimit:         comm.HexToBigInt(userop.CallGasLimit),
		VerificationGasLimit: comm.HexToBigInt(userop.VerificationGasLimit),
		PreVerificationGas:   comm.HexToBigInt(userop.PreVerificationGas),
		MaxFeePerGas:         comm.HexToBigInt(userop.MaxFeePerGas),
		MaxPriorityFeePerGas: comm.HexToBigInt(userop.MaxPriorityFeePerGas),
		PaymasterAndData:     common.FromHex("0x"),
		Signature:            common.FromHex("0x"),
	}

	now := time.Now().Unix()

	hash, err := auth.GetHash(nil, *op, big.NewInt(now+30), big.NewInt(now))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sig, err := crypto.Sign(hash[:], s.paymasterKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Example uint48 values
	validUntil := uint64(now + 30)
	validAfter := uint64(now)

	// Create byte slices
	bytes1 := make([]byte, 8)
	bytes2 := make([]byte, 8)

	// Convert uint48 to bytes
	binary.BigEndian.PutUint64(bytes1, validUntil)
	binary.BigEndian.PutUint64(bytes2, validAfter)

	// Concatenate the byte slices
	data := append(bytes1[2:], bytes2[2:]...)

	// a, err := account.Authorizer(nil)
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// println("authorizer address: " + a.Hex())

	pk := crypto.PubkeyToAddress(s.paymasterKey.PublicKey)

	// we should be able to derive the entrypoint address from the account
	// sender, err := af.GetAddress(nil, o, common.Big0)
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// println(epAddr)
	// println(pk.Hex())
	// println(userop.Sender)

	pd := &paymasterData{
		PaymasterAndData:     fmt.Sprintf("%s%x%s", pk.Hex(), data, common.Bytes2Hex(sig)),
		PreVerificationGas:   "0x0",
		VerificationGasLimit: "0x0",
		CallGasLimit:         "0x0",
	}

	println("paymaster approved!")

	comm.JSONRPCBody(w, pd, nil)
}
