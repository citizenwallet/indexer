package common

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrInvalidCalldata = errors.New("invalid calldata")
	ErrNotTransfer     = errors.New("not a transfer")

	executeSigSingle = crypto.Keccak256([]byte("execute(address,uint256,bytes)"))[:4]

	transferSig = crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	mintSig     = crypto.Keccak256([]byte("mint(address,uint256)"))[:4]
	withdrawSig = crypto.Keccak256([]byte("withdraw(bytes32,address,address,uint256)"))[:4]
)

// ParseERC20Transfer parses the calldata of an ERC20 transfer from a smart contract Execute function
func ParseERC20Transfer(calldata []byte, evm indexer.EVMRequester) (common.Address, common.Address, common.Address, *big.Int, error) {
	if len(calldata) < 228 {
		return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The function selector is the first 4 bytes of the calldata
	funcSelector := calldata[:4]
	if !bytes.Equal(funcSelector, executeSigSingle) { // TODO: implement batch execute
		return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The rest of the calldata is the encoded arguments
	args := calldata[4:]

	// The first argument is the dest address, which is 32 bytes offset from the start of the args
	dest := common.BytesToAddress(args[32-20 : 32])
	if len(dest.Bytes()) == 0 {
		return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
	}

	// The third argument is the funcData, which starts 96 bytes offset from the start of the args
	funcData := args[128:132]

	// The first 4 bytes of the funcData is the transfer function selector
	trfFuncSelector := funcData[:4]

	// Depending on the function selector, the arguments are in different positions
	switch string(trfFuncSelector) {
	case string(transferSig), string(mintSig):
		// Standard ERC20 transfer
		funcArgs := args[132:]

		// The first argument of the funcData is the to address, which is 32 bytes offset from the start of the funcData
		to := common.BytesToAddress(funcArgs[32-20 : 32])
		if len(to.Bytes()) == 0 {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// The second argument of the funcData is the amount, which is 64 bytes offset from the start of the funcData
		amount := new(big.Int).SetBytes(funcArgs[64-32 : 64])
		if amount.Cmp(big.NewInt(0)) == 0 {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		return dest, common.Address{}, to, amount, nil
	case string(withdrawSig):
		// Withdraw function from the Card Manager
		funcArgs := args[132:]

		// The first argument of the funcData is the card hash, which is 32 bytes offset from the start of the funcData
		cardHash := [32]byte(funcArgs[0:32])
		if len(cardHash) == 0 {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// Load the contract ABI
		contractAbi, err := abi.JSON(strings.NewReader(string(CardManagerABI)))
		if err != nil {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// Set the contract address
		contractAddress := dest

		// Prepare the call
		callData, err := contractAbi.Pack("getCardAddress", cardHash)
		if err != nil {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// Create a call message
		msg := ethereum.CallMsg{
			To:   &contractAddress,
			Data: callData,
		}

		result, err := evm.CallContract(msg, nil)
		if err != nil {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// Parse the result
		var cardAddress common.Address
		err = contractAbi.UnpackIntoInterface(&cardAddress, "getCardAddress", result)
		if err != nil {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// The second argument of the funcData is the token address, which is 64 bytes offset from the start of the funcData
		dest := common.BytesToAddress(funcArgs[64-20 : 64])
		if len(dest.Bytes()) == 0 {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// The third argument of the funcData is the to address, which is 96 bytes offset from the start of the funcData
		to := common.BytesToAddress(funcArgs[96-20 : 96])
		if len(to.Bytes()) == 0 {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		// The fourth argument of the funcData is the amount, which is 128 bytes offset from the start of the funcData
		amount := new(big.Int).SetBytes(funcArgs[128-32 : 128])
		if amount.Cmp(big.NewInt(0)) == 0 {
			return common.Address{}, common.Address{}, common.Address{}, nil, ErrInvalidCalldata
		}

		return dest, cardAddress, to, amount, nil
	}

	return common.Address{}, common.Address{}, common.Address{}, nil, ErrNotTransfer
}

const CardManagerABI = `[{"inputs":[{"internalType":"address","name":"_owner","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[],"name":"AlreadyInitializing","type":"error"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"voucher","type":"address"}],"name":"CardCreated","type":"event"},{"inputs":[],"name":"cardImplementation","outputs":[{"internalType":"contract Card","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"contractAddress","type":"address"}],"name":"contractExists","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"cardHash","type":"bytes32"}],"name":"createCard","outputs":[{"internalType":"contract Card","name":"ret","type":"address"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"cardHash","type":"bytes32"}],"name":"getCardAddress","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"serial","type":"uint256"}],"name":"getCardHash","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"contract IEntryPoint","name":"_entryPoint","type":"address"},{"internalType":"contract ITokenEntryPoint","name":"_tokenEntryPoint","type":"address"},{"internalType":"address[]","name":"_whitelistAddresses","type":"address[]"}],"name":"initialize","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"addr","type":"address"}],"name":"isWhitelisted","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"cardHash","type":"bytes32"},{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferCardOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address[]","name":"addresses","type":"address[]"}],"name":"updateWhitelist","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"cardHash","type":"bytes32"},{"internalType":"contract IERC20","name":"token","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"withdraw","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
