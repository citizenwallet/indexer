package router

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
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
	}
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

type BodyEncoding string

const (
	BodyEncodingBase64 BodyEncoding = "base64"
)

type signedBody struct {
	Data     []byte       `json:"data"`
	Encoding BodyEncoding `json:"encoding"`
	Expiry   int64        `json:"expiry"`
}

// SignatureMiddleware is a middleware that checks the signature of the request against the request body
func SignatureMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(indexer.ProtectedPaths, r.URL.Path) || (r.Method == http.MethodGet || r.Method == http.MethodOptions) {
			next.ServeHTTP(w, r)
			return
		}

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

		// get address
		addr := r.Header.Get(indexer.AddressHeader)
		if addr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// check signature
		if !verifySignature(req, addr, signature) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		err := r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(strings.NewReader(string(req.Data)))
		r.ContentLength = int64(len(req.Data))

		ctx := context.WithValue(r.Context(), indexer.ContextKeyAddress, addr)

		next.ServeHTTP(w, r.WithContext(ctx))
		return
	})
}

func verifySignature(req signedBody, addr string, signature string) bool {
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

	recoveredaddr := address.Hex()

	// the address in the request must match the address derived from the signature
	if strings.ToLower(recoveredaddr) != strings.ToLower(addr) {
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
