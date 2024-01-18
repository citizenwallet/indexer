package router

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/account"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
)

var (
	options sync.Map

	allMethods = []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
		http.MethodDelete,
	}

	acceptedHeaders = []string{
		"Origin",
		"Content-Type",
		"Content-Length",
		"X-Requested-With",
		"Accept-Encoding",
		"Authorization",
		indexer.SignatureHeader,
		indexer.AddressHeader,
		indexer.AppVersionHeader,
	}

	MAGIC_VALUE = [4]byte{0x16, 0x26, 0xba, 0x7e}
)

// HealthMiddleware is a middleware that responds to health checks
func HealthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// OptionsMiddleware ensures that we return the correct headers for CORS requests
func OptionsMiddleware(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := r.Context().Value(chi.RouteCtxKey).(*chi.Context)

		var path string
		if r.URL.RawPath != "" {
			path = r.URL.RawPath
		} else {
			path = r.URL.Path
		}

		var methodsStr string
		cached, ok := options.Load(path)
		if ok {
			methodsStr = cached.(string)
		} else {
			var methods []string
			for _, method := range allMethods {
				nctx := chi.NewRouteContext()
				if ctx.Routes.Match(nctx, method, path) {
					methods = append(methods, method)
				}
			}

			methods = append(methods, http.MethodOptions)
			methodsStr = strings.Join(methods, ", ")
			options.Store(path, methodsStr)
		}

		// allowed methods
		w.Header().Set("Allow", methodsStr)

		// allowed methods for CORS
		w.Header().Set("Access-Control-Allow-Methods", methodsStr)

		// allowed origins
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// allowed headers
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(acceptedHeaders, ", "))

		// actually handle the request
		if r.Method != http.MethodOptions {
			h.ServeHTTP(w, r)
			return
		}

		// handle OPTIONS requests
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func RequestSizeLimitMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

type BodyEncoding string

const (
	BodyEncodingBase64 BodyEncoding = "base64"
)

type signedBody struct {
	Data     []byte       `json:"data"`
	Encoding BodyEncoding `json:"encoding"`
	Expiry   int64        `json:"expiry"`
	Version  int          `json:"version"`
}

// withSignature is a middleware that checks the signature of the request against the request headers
func withSignature(evm indexer.EVMRequester, h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check signature
		signature := r.Header.Get(indexer.SignatureHeader)
		if signature == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req signedBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// get address
		addr := r.Header.Get(indexer.AddressHeader)
		if addr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		haccaddr := common.HexToAddress(addr)

		// check signature
		switch req.Version {
		case 0:
			// LEGACY: remove 3 months from 22/10/2023
			// reason: verifySignature only verifies the data and not the entire request, the expiry time can be manipulated
			if !verifySignature(req, haccaddr, signature) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		case 2:
			// DEPRECATED: remove 3 months from 14/11/2023
			// reason: does not support ERC1271
			if !verifyV2Signature(req, haccaddr, signature) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		default:
			if !verify1271Signature(evm, req, haccaddr, signature) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		r.Body = io.NopCloser(strings.NewReader(string(req.Data)))
		r.ContentLength = int64(len(req.Data))

		ctx := context.WithValue(r.Context(), indexer.ContextKeyAddress, addr)
		ctx = context.WithValue(ctx, indexer.ContextKeySignature, signature)

		h(w, r.WithContext(ctx))
		return
	})
}

// withMultiPartSignature is a middleware that checks the signature of the request against a multi-part request headers
func withMultiPartSignature(evm indexer.EVMRequester, h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check signature
		signature := r.Header.Get(indexer.SignatureHeader)
		if signature == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		body := r.FormValue("body")

		var req signedBody
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// get address
		addr := r.Header.Get(indexer.AddressHeader)
		if addr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		haccaddr := common.HexToAddress(addr)

		// check signature
		switch req.Version {
		case 0:
			// LEGACY: remove 3 months from 22/10/2023
			// reason: verifySignature only verifies the data and not the entire request, the expiry time can be manipulated
			if !verifySignature(req, haccaddr, signature) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		case 2:
			// DEPRECATED: remove 3 months from 14/11/2023
			// reason: does not support ERC1271
			if !verifyV2Signature(req, haccaddr, signature) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		default:
			if !verify1271Signature(evm, req, haccaddr, signature) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		r.MultipartForm.Value["body"] = []string{string(req.Data)}

		ctx := context.WithValue(r.Context(), indexer.ContextKeyAddress, addr)

		h(w, r.WithContext(ctx))
		return
	})
}

// with1271Signature is a middleware that checks the owner's signature of the request against the request headers and the actual account on-chain
func with1271Signature(evm indexer.EVMRequester, h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// parse signature from header
		signature := r.Header.Get(indexer.SignatureHeader)
		if signature == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req signedBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// get address
		addr := r.Header.Get(indexer.AddressHeader)
		if addr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		haccaddr := common.HexToAddress(addr)

		// check signature
		if !verify1271Signature(evm, req, haccaddr, signature) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Body = io.NopCloser(strings.NewReader(string(req.Data)))
		r.ContentLength = int64(len(req.Data))

		ctx := context.WithValue(r.Context(), indexer.ContextKeyAddress, addr)
		ctx = context.WithValue(ctx, indexer.ContextKeySignature, signature)

		h(w, r.WithContext(ctx))
		return
	})
}

// withJSONRPCRequest is a middleware that handles a JSON RPC request
func withJSONRPCRequest(hmap map[string]http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// parse request
		var req indexer.JsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// check if the method is available
		h, ok := hmap[req.Method]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		r.Body = io.NopCloser(strings.NewReader(string(req.Params)))
		r.ContentLength = int64(len([]byte(req.Params)))

		h(w, r)
		return
	})
}

// verifySignature verifies the signature of the request against the request body
//
// Deprecated: verifySignature incorrectly verifies only the data and not the entire request
func verifySignature(req signedBody, addr common.Address, signature string) bool {
	// verify that the signature is a legacy signature
	if req.Version != 0 {
		return false
	}

	// verify if the signature has expired
	if req.Expiry < time.Now().UTC().Unix() {
		return false
	}

	// hash the request data
	h := crypto.Keccak256Hash(req.Data)

	// decode the signature
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return false
	}

	// recover the public key from the signature
	pubkey, _, err := ecdsa.RecoverCompact(sig, h.Bytes())
	if err != nil {
		return false
	}

	// derive the address from the public key
	address := crypto.PubkeyToAddress(*pubkey.ToECDSA())

	// the address in the request must match the address derived from the signature
	if address != addr {
		return false
	}

	// create ModNScalars from the signature manually
	sr, ss := secp256k1.ModNScalar{}, secp256k1.ModNScalar{}

	// set the byteslices manually from the signature
	sr.SetByteSlice(sig[1:33])
	ss.SetByteSlice(sig[33:65])

	// create a new signature from the ModNScalars
	ns := ecdsa.NewSignature(&sr, &ss)
	if err != nil {
		return false
	}

	// verify the signature
	return ns.Verify(h.Bytes(), pubkey)
}

// verifyV2Signature verifies the signature of the request against the entire request body
func verifyV2Signature(req signedBody, addr common.Address, signature string) bool {
	// verify that the signature is v2
	if req.Version != 2 {
		return false
	}

	// verify if the signature has expired
	if req.Expiry < time.Now().UTC().Unix() {
		return false
	}

	// hash the entire request data
	b, err := json.Marshal(req)
	if err != nil {
		return false
	}

	h := crypto.Keccak256Hash(b)

	// decode the signature
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return false
	}

	// recover the public key from the signature
	pubkey, _, err := ecdsa.RecoverCompact(sig, h.Bytes())
	if err != nil {
		return false
	}

	// derive the address from the public key
	address := crypto.PubkeyToAddress(*pubkey.ToECDSA())

	// the address in the request must match the address derived from the signature
	if address != addr {
		return false
	}

	// create ModNScalars from the signature manually
	sr, ss := secp256k1.ModNScalar{}, secp256k1.ModNScalar{}

	// set the byteslices manually from the signature
	sr.SetByteSlice(sig[1:33])
	ss.SetByteSlice(sig[33:65])

	// create a new signature from the ModNScalars
	ns := ecdsa.NewSignature(&sr, &ss)
	if err != nil {
		return false
	}

	// verify the signature
	return ns.Verify(h.Bytes(), pubkey)
}

// verify1271Signature verifies the signature of the request against the actual account on-chain if local fails
func verify1271Signature(evm indexer.EVMRequester, req signedBody, accaddr common.Address, signature string) bool {
	// verify that the signature is v3
	if req.Version != 3 {
		return false
	}

	// verify if the signature has expired
	if req.Expiry < time.Now().UTC().Unix() {
		return false
	}

	// decode the signature
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return false
	}

	if sig[crypto.RecoveryIDOffset] == 27 || sig[crypto.RecoveryIDOffset] == 28 {
		sig[crypto.RecoveryIDOffset] -= 27
	}

	// hash the entire request data
	b, err := json.Marshal(req)
	if err != nil {
		return false
	}

	h := accounts.TextHash(crypto.Keccak256(b))

	var h32 [32]byte
	copy(h32[:], h)

	// check if the signature belongs to the owner
	pkey, err := crypto.SigToPub(h, sig)
	if err != nil {
		return false
	}

	// derive the address from the public key
	address := crypto.PubkeyToAddress(*pkey)

	// classic signature verification
	if address == accaddr {
		return true
	}

	// check on chain if it is a valid account and the signer is the owner

	// Get the contract's bytecode
	bytecode, err := evm.CodeAt(context.Background(), accaddr, nil)
	if err != nil {
		return false
	}

	// Check if the account is deployed
	if len(bytecode) == 0 {
		return false
	}

	acc, err := account.NewAccount(accaddr, evm.Backend())
	if err != nil {
		return false
	}

	if sig[crypto.RecoveryIDOffset] == 0 || sig[crypto.RecoveryIDOffset] == 1 {
		sig[crypto.RecoveryIDOffset] += 27
	}

	// verify the signature
	v, err := acc.IsValidSignature(nil, h32, sig)
	if err == nil {
		return v == MAGIC_VALUE
	}

	// an error occured, check if it is because the method is not implemented
	e, ok := err.(rpc.Error)
	if ok && e.ErrorCode() != -32000 {
		return false
	}

	// not implemented, check the owner manually
	owner, err := acc.Owner(nil)
	if err != nil {
		return false
	}

	return owner == address
}

// compactSignature gets the v, r, and s values and compacts them into a 65 byte array
// 0x - padding
// v - 1 byte
// r - 32 bytes
// s - 32 bytes
func compactSignature(sig []byte) string {
	rsig := make([]byte, 65)

	// v is the last byte of the signature plus 27
	integer := big.NewInt(0).SetBytes(sig[64:65]).Uint64()

	rsig[0] = byte(integer + 27)
	copy(rsig[1:33], sig[0:32])
	copy(rsig[33:65], sig[32:64])

	return hexutil.Encode(rsig)
}
