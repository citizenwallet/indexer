package common

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrInvalidCalldata = errors.New("invalid calldata")
	ErrNotTransfer     = errors.New("not a transfer")

	executeSigSingle = crypto.Keccak256([]byte("execute(address,uint256,bytes)"))[:4]
	transferSig      = crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
)

// ParseERC20Transfer parses the calldata of an ERC20 transfer from a smart contract Execute function
func ParseERC20Transfer(calldata []byte) (common.Address, common.Address, *big.Int, error) {
	if len(calldata) < 228 {
		return common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The function selector is the first 4 bytes of the calldata
	funcSelector := calldata[:4]
	if !bytes.Equal(funcSelector, executeSigSingle) {
		return common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The rest of the calldata is the encoded arguments
	args := calldata[4:]

	// The first argument is the dest address, which is 32 bytes offset from the start of the args
	dest := common.BytesToAddress(args[32-20 : 32])
	if len(dest.Bytes()) == 0 {
		return common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The third argument is the funcData, which starts 96 bytes offset from the start of the args
	funcData := args[128:132]

	trfFuncSelector := funcData[:4]
	if !bytes.Equal(trfFuncSelector, transferSig) {
		return common.Address{}, common.Address{}, nil, ErrNotTransfer
	}

	funcArgs := args[132:]

	// The first argument of the funcData is the to address, which is 32 bytes offset from the start of the funcData
	to := common.BytesToAddress(funcArgs[32-20 : 32])
	if len(to.Bytes()) == 0 {
		return common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The second argument of the funcData is the amount, which is 64 bytes offset from the start of the funcData
	amount := new(big.Int).SetBytes(funcArgs[64-32 : 64])
	if amount.Cmp(big.NewInt(0)) == 0 {
		return common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	return dest, to, amount, nil
}
