package profiles

import (
	"encoding/json"
	"net/http"

	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/pkg/indexer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	b    *bucket.Bucket
	comm *ethrequest.Community
}

func NewService(b *bucket.Bucket, comm *ethrequest.Community) *Service {
	return &Service{
		b:    b,
		comm: comm,
	}
}

type pinResponse struct {
	IpfsURL string `json:"ipfs_url"`
}

// PinProfile handler for pinning profile to ipfs
func (s *Service) PinProfile(w http.ResponseWriter, r *http.Request) {
	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	var profile indexer.Profile
	err := json.NewDecoder(r.Body).Decode(&profile)
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

	go func(acchex string) {
		// update was successful, we can delete the old one
		// get the hash from the profile contract
		hash, err := s.comm.GetProfile(acchex)
		if err == nil {
			err = s.b.Unpin(r.Context(), hash)
			if err != nil {
				// not sure here if we should return an error or not
				// pinning the new one was successful, but unpinning the old one failed
			}
		}
	}(acc.Hex())

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

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

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

	go func(acchex string) {
		// update was successful, we can delete the old one
		// get the hash from the profile contract
		hash, err := s.comm.GetProfile(acchex)
		if err == nil {
			err = s.b.Unpin(r.Context(), hash)
			if err != nil {
				// not sure here if we should return an error or not
				// pinning the new one was successful, but unpinning the old one failed
			}
		}
	}(acc.Hex())

	err = com.Body(w, &pinResponse{IpfsURL: uri}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Unpin handler for unpinning profile from ipfs
func (s *Service) Unpin(w http.ResponseWriter, r *http.Request) {
	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	// get the hash from the profile contract, makes sure that users can only delete their own profile
	hash, err := s.comm.Profile.Get(nil, acc)
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
