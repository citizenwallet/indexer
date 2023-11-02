package userop

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http"

	comm "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/pkg/index"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/authorizer"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
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

func (s *Service) Send(w http.ResponseWriter, r *http.Request) {
	println("userop send")
	// parse contract address from url params
	// contractAddr := chi.URLParam(r, "contract_address")

	// addr := common.HexToAddress(contractAddr)

	// instantiate account factory contract
	// af, err := accfactory.NewAccfactory(addr, s.evm.Client())
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// println(af)

	// parse the incoming params

	var params []any
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var userop indexer.UserOp
	var epAddr string

	println(userop.CallData)
	println(epAddr)

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
		}
	}

	if epAddr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: check paymaster address of signature

	// TODO: check user op signature recovered address against account address on factory

	// verify the signature of the user op

	// verify the user op
	// sender := common.HexToAddress(userop.Sender)

	auth, err := authorizer.NewAuthorizer(common.HexToAddress(epAddr), s.evm.Client())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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
		PaymasterAndData:     common.FromHex(userop.PaymasterAndData),
		Signature:            common.FromHex(userop.Signature),
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

	tx, err := auth.HandleOps(transactor, []authorizer.UserOperation{*op}, common.HexToAddress(epAddr))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Wait for the transaction to be mined
	receipt, err := bind.WaitMined(context.Background(), s.evm.Client(), tx)
	if err != nil || receipt.Status != 1 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	println("userop sent!")

	// get nonce using the account factory since we are not sure if the account has been created yet
	// nonce, err := af.GetNonce(nil, sender, common.Big0)
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }

	// Decode hex string to bytes
	// 	bytes := common.FromHex(userop.Nonce)

	// 	// Get *big.Int from bytes
	// 	opnonce := new(big.Int)
	// 	opnonce.SetBytes(bytes)

	// 	// println("nonce: ", nonce.String())
	// 	// println("opnonce: ", opnonce.String())

	// 	// make sure that the nonce is correct
	// 	// if nonce.Cmp(opnonce) != 0 {
	// 	// 	// nonce is wrong
	// 	// 	w.WriteHeader(http.StatusBadRequest)
	// 	// 	return
	// 	// }

	// 	// if nonce.Cmp(big.NewInt(0)) == 0 { // TODO: put this back
	// 	if opnonce.Cmp(big.NewInt(0)) == 0 {
	// 		// needs an account
	// 		// Check if there is contract code at the address
	// 		code, err := s.evm.Client().CodeAt(context.Background(), sender, nil)
	// 		if err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}
	// 		if len(code) > 0 {
	// 			// there is already code at the address even though the nonce is 0
	// 			w.WriteHeader(http.StatusBadRequest)
	// 			return
	// 		}

	// 		pkBytes, err := hex.DecodeString("fa714a855c94ac395c3b85c337bc9d84b4e2234b147fdd3dafb22f5a1c02bf2c")
	// 		if err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		// Generate ecdsa.PrivateKey from bytes
	// 		privateKey, err := crypto.ToECDSA(pkBytes)
	// 		if err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		// create the account
	// 		tx, err := af.CreateAccount(auth, crypto.PubkeyToAddress(privateKey.PublicKey), common.Big0)
	// 		if err != nil {
	// 			println(err.Error())
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		println(tx.Hash().Hex())

	// 		// Wait for the transaction to be mined
	// 		receipt, err := bind.WaitMined(context.Background(), s.evm.Client(), tx)
	// 		if err != nil || receipt.Status != 1 {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}
	// 		println("receipt of create account")
	// 		println(receipt.Status)

	// 		// allow the token
	// 		accountABI, err := abi.JSON(strings.NewReader(account.AccountMetaData.ABI))
	// 		if err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		data, err := accountABI.Pack("updateWhitelist", []common.Address{common.HexToAddress("0x765DE816845861e75A25fCA122bb6898B8B1282a")})
	// 		if err != nil {
	// 			println(err.Error())
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		acc, err := account.NewAccount(sender, s.evm.Client())
	// 		if err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		tx, err = acc.Execute(auth, sender, common.Big0, data)
	// 		if err != nil || receipt.Status != 1 {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}

	// 		// Wait for the transaction to be mined
	// 		receipt, err = bind.WaitMined(context.Background(), s.evm.Client(), tx)
	// 		if err != nil || receipt.Status != 1 {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			return
	// 		}
	// 		println("receipt of execute whitelist")
	// 		println(receipt.Status)
	// 	}

	// 	// check if the account factory we are calling is the same as the one that was used to create the account
	// 	// if strings.ToLower(a.Hex()) != strings.ToLower(pk.Hex()) {
	// 	// 	w.WriteHeader(http.StatusUnauthorized)
	// 	// 	return
	// 	// }

	// 	acc, err := account.NewAccount(sender, s.evm.Client())
	// 	if err != nil {
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		return
	// 	}

	// 	// accountABI, err := abi.JSON(strings.NewReader(account.AccountABI))
	// 	// if err != nil {
	// 	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	// 	return
	// 	// }

	// 	// data, err := accountABI.Pack("updateWhitelist", []common.Address{common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")})
	// 	// if err != nil {
	// 	// 	println(err.Error())
	// 	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	// 	return
	// 	// }

	// 	// tx, err := acc.Execute(auth, sender, common.Big0, data)
	// 	// if err != nil {
	// 	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	// 	return
	// 	// }

	// 	abiString := `[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"}]`

	// 	contractABI, err := abi.JSON(strings.NewReader(abiString))
	// 	if err != nil {
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		return
	// 	}

	// 	println(erc20.Erc20MetaData.ABI)

	// 	functionSignature := contractABI.Methods["transfer"].Sig

	// 	println(functionSignature)

	// 	data, err := contractABI.Pack("transfer", common.HexToAddress("0xf5D0181b80E1C793D98b95BFF0E6CDe1D27a3ef8"), big.NewInt(100000000000000000))
	// 	if err != nil {
	// 		println(err.Error())
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		return
	// 	}

	// 	tx, err := acc.Execute(auth, common.HexToAddress("0x765DE816845861e75A25fCA122bb6898B8B1282a"), common.Big0, data)
	// 	if err != nil {
	// 		println(err.Error())
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		return
	// 	}

	// 	// Wait for the transaction to be mined
	// 	receipt, err := bind.WaitMined(context.Background(), s.evm.Client(), tx)
	// 	if err != nil || receipt.Status != 1 {
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		return
	// 	}

	// 	print(tx.Hash().Hex())

	// println(
	//
	//	"sender: ", sender.Hex(),
	//
	// )
}
