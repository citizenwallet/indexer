package common

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Nonce struct {
	Seq uint64
	Key *big.Int
}

// GenerateNonce generates a nonce which is a unique number that can only be used once.
// This nonce is a uint256 composed of a uint64 and a uint192.
// The uin192 contains an epoch timestamp in the last 10 digits
func NewNonce() (*Nonce, error) {
	// Initialize a big.Int with the value of 0. This represents the uint64 part of the nonce.
	seq := big.NewInt(0)

	// Create a byte slice with a length of 24. Since each byte is 8 bits, this byte slice can hold a uint192.
	key := make([]byte, 24)

	// Generate a random uint192 by reading 24 random bytes into the key byte slice.
	// If an error occurs during this operation, the function returns the error.
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	// Convert the random uint192 to a big.Int by calling SetBytes on a new big.Int,
	// which interprets the byte slice as a big-endian integer.
	keyInt := new(big.Int).SetBytes(key)

	// replace the last 10 digits with an epoch timestamp
	// this is to avoid nonce collision
	// Get the current epoch timestamp
	timestamp := big.NewInt(time.Now().Unix())

	// Shift the timestamp 10 digits to the left
	timestamp.Mul(timestamp, big.NewInt(1e10))

	// Create a mask with the last 10 digits as 1 and the rest as 0
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 10*10), big.NewInt(1))

	// Clear the last 10 digits of the uint192
	keyInt.And(keyInt, mask)

	// Set the last 10 digits to the epoch timestamp
	keyInt.Or(keyInt, timestamp)

	// Return the nonce and no error.
	return &Nonce{Seq: seq.Uint64(), Key: keyInt}, nil
}

// ParseNonce parses a nonce from a big.Int containing a uint256
// with the first 64 bits being the key and the last 192 bits being the seq.
func ParseNonce(nonce *big.Int) *Nonce {
	// Create a big.Int with 1 followed by 64 zeros
	mask := new(big.Int).Lsh(big.NewInt(1), 64)

	// Subtract 1 to get a big.Int with 64 ones
	mask.Sub(mask, big.NewInt(1))

	// Bitwise AND the nonce with the mask to get the seq
	seq := new(big.Int).And(nonce, mask)

	// Shift the nonce 192 bits to the right to get the key
	keyInt := new(big.Int).Rsh(nonce, 192)

	return &Nonce{Seq: seq.Uint64(), Key: keyInt}
}

func (n *Nonce) BigInt() *big.Int {
	seq := big.NewInt(int64(n.Seq))

	// Convert the random uint192 to a big.Int by calling SetBytes on a new big.Int,
	// which interprets the byte slice as a big-endian integer.
	keyInt := new(big.Int).Set(n.Key)

	// Combine the uint64 and the uint192 into a uint256 by shifting the uint192 64 bits to the left
	// to make room for the uint64, and then performing a bitwise OR operation to combine the two numbers.
	keyInt.Lsh(keyInt, 64).Or(keyInt, seq)

	// Return the nonce and no error.
	return keyInt
}

func (n *Nonce) String() string {
	return hexutil.EncodeBig(n.BigInt())
}
