package common

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/citizenwallet/indexer/pkg/indexer"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var testCases = []string{
	"0x",
	"0xb61d27f60000000000000000000000005815e61ef72c9e6107b5c5a05fd121f334f7a7f1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000044a9059cbb00000000000000000000000029d755c17df3ed2ecae6e42d694fb4f7e2ff6010000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000",
	"0xb61d27f6000000000000000000000000eec0f3257369c6bcd2fd8755cbef8a95b12bc4c90000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c49d91dd7d000000000000000000000000d5e60a846ab25f73a5b405dfca83de1ba98fe99720202020202020202020202020202020202020202020202020207861766965720000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000002e516d50316d786637354250794a76367434657833666248626153784874716d4470444b454d55514b44504d48707600000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
	"0xb61d27f6000000000000000000000000c0f9e0907c8de79fd5902b61e463dfedc5dc85700000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000848e0cc176437a77d18f8574e019c1e2754adcd7848f6b9b701d8d6db261ea6b979bf6c80d0000000000000000000000005815e61ef72c9e6107b5c5a05fd121f334f7a7f1000000000000000000000000cfa21b33d304d57c4e964e3819588eb5ac06b4d900000000000000000000000000000000000000000000000000000000000f424000000000000000000000000000000000000000000000000000000000",
}

type caseResult struct {
	Dest   common.Address
	From   common.Address
	To     common.Address
	Amount *big.Int
	Err    error
}

var expected = []caseResult{
	{
		Dest:   common.HexToAddress("0x"),
		From:   common.Address{},
		To:     common.HexToAddress("0x"),
		Amount: big.NewInt(0),
		Err:    ErrInvalidCalldata,
	},
	{
		Dest:   common.HexToAddress("0x5815E61eF72c9E6107b5c5A05FD121F334f7a7f1"),
		From:   common.Address{},
		To:     common.HexToAddress("0x29d755C17df3ED2eCAE6e42d694fb4F7E2ff6010"),
		Amount: big.NewInt(1),
	},
	{
		Dest:   common.HexToAddress("0x"),
		From:   common.Address{},
		To:     common.HexToAddress("0x"),
		Amount: big.NewInt(0),
		Err:    ErrNotTransfer,
	},
	{
		Dest:   common.HexToAddress("0x5815E61eF72c9E6107b5c5A05FD121F334f7a7f1"),
		From:   common.HexToAddress("0x3A5b94BB05083Bd3Ac33AfADa5c42Fb232C5020e"),
		To:     common.HexToAddress("0xcfa21B33D304D57c4E964e3819588Eb5ac06B4D9"),
		Amount: big.NewInt(1000000),
	},
}

func TestParseERC20Transfer(t *testing.T) {
	for i, tc := range testCases {
		data := common.FromHex(tc)

		evm := NewMockEVMRequester()

		dest, from, to, amount, err := ParseERC20Transfer(data, evm)
		if err != nil {
			if err != expected[i].Err {
				t.Errorf("err = %s, want %s", err, expected[i].Err)
			}
			continue
		}

		if dest != expected[i].Dest {
			t.Errorf("dest = %s, want %s", dest, expected[i].Dest)
		}

		if from != expected[i].From {
			t.Errorf("from = %s, want %s", from, expected[i].From)
		}

		if to != expected[i].To {
			t.Errorf("to = %s, want %s", to, expected[i].To)
		}

		if amount.Cmp(expected[i].Amount) != 0 {
			t.Errorf("amount = %s, want %s", amount, expected[i].Amount)
		}
	}
}

type MockEVMRequester struct{}

func NewMockEVMRequester() indexer.EVMRequester {
	return &MockEVMRequester{}
}

// Backend implements indexer.EVMRequester.
func (m *MockEVMRequester) Backend() bind.ContractBackend {
	panic("unimplemented")
}

// BaseFee implements indexer.EVMRequester.
func (m *MockEVMRequester) BaseFee() (*big.Int, error) {
	panic("unimplemented")
}

// BlockTime implements indexer.EVMRequester.
func (m *MockEVMRequester) BlockTime(number *big.Int) (uint64, error) {
	panic("unimplemented")
}

// CallContract implements indexer.EVMRequester.
func (m *MockEVMRequester) CallContract(call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	result := "0000000000000000000000003A5b94BB05083Bd3Ac33AfADa5c42Fb232C5020e"

	print(result)

	// Decode the hex string into a byte slice
	decodedBytes, err := hex.DecodeString(result)
	if err != nil {
		return nil, err
	}

	return decodedBytes, nil
}

// ChainID implements indexer.EVMRequester.
func (m *MockEVMRequester) ChainID() (*big.Int, error) {
	panic("unimplemented")
}

// Close implements indexer.EVMRequester.
func (m *MockEVMRequester) Close() {
	panic("unimplemented")
}

// CodeAt implements indexer.EVMRequester.
func (m *MockEVMRequester) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	panic("unimplemented")
}

// Context implements indexer.EVMRequester.
func (m *MockEVMRequester) Context() context.Context {
	panic("unimplemented")
}

// EstimateGasLimit implements indexer.EVMRequester.
func (m *MockEVMRequester) EstimateGasLimit(msg ethereum.CallMsg) (uint64, error) {
	panic("unimplemented")
}

// EstimateGasPrice implements indexer.EVMRequester.
func (m *MockEVMRequester) EstimateGasPrice() (*big.Int, error) {
	panic("unimplemented")
}

// FilterLogs implements indexer.EVMRequester.
func (m *MockEVMRequester) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	panic("unimplemented")
}

// LatestBlock implements indexer.EVMRequester.
func (m *MockEVMRequester) LatestBlock() (*big.Int, error) {
	panic("unimplemented")
}

// ListenForLogs implements indexer.EVMRequester.
func (m *MockEVMRequester) ListenForLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) error {
	panic("unimplemented")
}

// NewTx implements indexer.EVMRequester.
func (m *MockEVMRequester) NewTx(nonce uint64, from common.Address, to common.Address, data []byte, extraGas bool) (*types.Transaction, error) {
	panic("unimplemented")
}

// NonceAt implements indexer.EVMRequester.
func (m *MockEVMRequester) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	panic("unimplemented")
}

// SendTransaction implements indexer.EVMRequester.
func (m *MockEVMRequester) SendTransaction(tx *types.Transaction) error {
	panic("unimplemented")
}

// StorageAt implements indexer.EVMRequester.
func (m *MockEVMRequester) StorageAt(addr common.Address, slot common.Hash) ([]byte, error) {
	panic("unimplemented")
}

// WaitForTx implements indexer.EVMRequester.
func (m *MockEVMRequester) WaitForTx(tx *types.Transaction, timeout int) error {
	panic("unimplemented")
}
