package files

import (
	"encoding/json"
	"net/http"

	"github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/bucket"
	"github.com/citizenwallet/indexer/internal/services/ethrequest"
	"github.com/citizenwallet/indexer/pkg/indexer"
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
	// Parse the form data to get the uploaded file
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit (adjust as needed)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// parse address from url params
	accaddr := chi.URLParam(r, "addr")

	// the address in the url should match the one in the headers
	haddr, ok := indexer.GetAddressFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	acc, err := s.comm.GetAccount(haddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !common.IsSameHexAddress(acc.Hex(), accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// parse image
	si, err := common.ParseImage(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	strbody := r.FormValue("body")
	if strbody == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var profile indexer.Profile
	if err := json.Unmarshal([]byte(strbody), &profile); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !common.IsSameHexAddress(acc.Hex(), profile.Address) {
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

	err = common.Body(w, &pinResponse{IpfsURL: uri}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Unpin handler for unpinning profile from ipfs
func (s *Service) Unpin(w http.ResponseWriter, r *http.Request) {
	// parse address from url params
	accaddr := chi.URLParam(r, "addr")

	// the address in the url should match the one in the headers
	haddr, ok := indexer.GetAddressFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	acc, err := s.comm.GetAccount(haddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !common.IsSameHexAddress(acc.Hex(), accaddr) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	hash := chi.URLParam(r, "hash")

	err = s.b.Unpin(r.Context(), hash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
