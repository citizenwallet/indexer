package router

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestSignatureVerification(t *testing.T) {
	// generate a key pair
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("legacy", func(t *testing.T) {
		// make a v0 signed body
		data := []byte("eyJoZWxsbyI6IndvcmxkIn0") // base64: '{"hello":"world"}'

		body := signedBody{
			Data:     data,
			Encoding: BodyEncodingBase64,
			Expiry:   time.Now().Add(time.Second * 5).UnixMilli(),
		}

		// sign the body
		sig, err := crypto.Sign(crypto.Keccak256(body.Data), k)
		if err != nil {
			t.Fatal(err)
		}

		compactedSig := compactSignature(sig)

		addr := crypto.PubkeyToAddress(k.PublicKey).Hex()

		// verify the signature
		if !verifySignature(body, addr, compactedSig) {
			t.Errorf("verifySignature(%v, %s, %s) = false, want true", body, addr, compactedSig)
		}
	})

	t.Run("v2", func(t *testing.T) {
		// make a v0 signed body
		data := []byte("eyJoZWxsbyI6IndvcmxkIn0") // base64: '{"hello":"world"}'

		body := signedBody{
			Data:     data,
			Encoding: BodyEncodingBase64,
			Expiry:   time.Now().Add(time.Second * 5).UnixMilli(),
			Version:  2,
		}

		// sign the body
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}

		sig, err := crypto.Sign(crypto.Keccak256(b), k)
		if err != nil {
			t.Fatal(err)
		}

		compactedSig := compactSignature(sig)

		addr := crypto.PubkeyToAddress(k.PublicKey).Hex()

		// verify the signature
		if !verifyV2Signature(body, addr, compactedSig) {
			t.Errorf("verifySignature(%v, %s, %s) = false, want true", body, addr, compactedSig)
		}
	})
}
