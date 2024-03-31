package profiles

import (
	"context"
	"encoding/json"
	"net/http"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/citizenwallet/smartcontracts/pkg/contracts/profile"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	b   *bucket.Bucket
	evm indexer.EVMRequester
}

func NewService(b *bucket.Bucket, evm indexer.EVMRequester) *Service {
	return &Service{
		b:   b,
		evm: evm,
	}
}

type pinResponse struct {
	IpfsURL string `json:"ipfs_url"`
}

// PinProfile handler for pinning profile to ipfs
func (s *Service) PinProfile(w http.ResponseWriter, r *http.Request) {
	// ensure that the address in the url matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	if haccaddr != acc {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse profile address from url params
	prfaddr := chi.URLParam(r, "contract_address")

	prf := common.HexToAddress(prfaddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), prf, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the profile contract is deployed
	if len(bytecode) == 0 {
		http.Error(w, "profile contract is missing", http.StatusBadRequest)
		return
	}

	// instantiate profile contract
	prfcontract, err := profile.NewProfile(prf, s.evm.Backend())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var profile indexer.Profile
	err = json.NewDecoder(r.Body).Decode(&profile)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	praddr := common.HexToAddress(profile.Account)

	if acc != praddr {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// pin profile to ipfs
	b, err := json.Marshal(profile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uri, err := s.b.PinJSONToIPFS(r.Context(), b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func(acchex common.Address) {
		// update was successful, we can delete the old one
		// get the hash from the profile contract
		hash, err := prfcontract.Get(nil, acchex)
		if err == nil {
			err = s.b.Unpin(r.Context(), hash)
			if err != nil {
				// not sure here if we should return an error or not
				// pinning the new one was successful, but unpinning the old one failed
			}
		}
	}(acc)

	err = com.Body(w, &pinResponse{IpfsURL: uri}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// PinMultiPartProfile handler for pinning profile to ipfs
func (s *Service) PinMultiPartProfile(w http.ResponseWriter, r *http.Request) {
	// Parse the form data to get the uploaded file
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit (adjust as needed)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// ensure that the address in the url matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	if haccaddr != acc {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse profile address from url params
	prfaddr := chi.URLParam(r, "contract_address")

	prf := common.HexToAddress(prfaddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), prf, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the profile contract is deployed
	if len(bytecode) == 0 {
		http.Error(w, "profile contract is missing", http.StatusBadRequest)
		return
	}

	// instantiate profile contract
	prfcontract, err := profile.NewProfile(prf, s.evm.Backend())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// parse image
	si, err := com.ParseImage(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	strbody := r.MultipartForm.Value["body"][0]
	if strbody == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var profile indexer.Profile
	if err := json.Unmarshal([]byte(strbody), &profile); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	praddr := common.HexToAddress(profile.Account)

	if acc != praddr {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// pin image to ipfs
	uri, err := s.b.PinFileToIPFS(r.Context(), si.Big, "big.jpg")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile.Image = uri

	// pin medium image to ipfs
	uri, err = s.b.PinFileToIPFS(r.Context(), si.Medium, "medium.jpg")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile.ImageMedium = uri

	// pin small image to ipfs
	uri, err = s.b.PinFileToIPFS(r.Context(), si.Small, "small.jpg")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile.ImageSmall = uri

	// pin profile to ipfs
	b, err := json.Marshal(profile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uri, err = s.b.PinJSONToIPFS(r.Context(), b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func(acchex common.Address) {
		// update was successful, we can delete the old one
		// get the hash from the profile contract
		hash, err := prfcontract.Get(nil, acchex)
		if err == nil {
			err = s.b.Unpin(r.Context(), hash)
			if err != nil {
				// not sure here if we should return an error or not
				// pinning the new one was successful, but unpinning the old one failed
			}
		}
	}(acc)

	err = com.Body(w, &pinResponse{IpfsURL: uri}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Unpin handler for unpinning profile from ipfs
func (s *Service) Unpin(w http.ResponseWriter, r *http.Request) {
	// ensure that the address in the url matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	if haccaddr != acc {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse profile address from url params
	prfaddr := chi.URLParam(r, "contract_address")

	prf := common.HexToAddress(prfaddr)

	// Get the contract's bytecode
	bytecode, err := s.evm.CodeAt(context.Background(), prf, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the profile contract is deployed
	if len(bytecode) == 0 {
		http.Error(w, "profile contract is missing", http.StatusBadRequest)
		return
	}

	// instantiate profile contract
	prfcontract, err := profile.NewProfile(prf, s.evm.Backend())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// get the hash from the profile contract, makes sure that users can only delete their own profile
	hash, err := prfcontract.Get(nil, acc)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = s.b.Unpin(r.Context(), hash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
