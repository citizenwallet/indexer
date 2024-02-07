package governance

import (
	com "github.com/citizenwallet/indexer/internal/common"
	"github.com/citizenwallet/indexer/internal/services/db/govdb"
	"github.com/citizenwallet/indexer/pkg/govindexer"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

type Service struct {
	gdb *govdb.DB
}

func NewService(gdb *govdb.DB) *Service {
	return &Service{
		gdb: gdb,
	}
}

// TODO !!!

// GetGov godoc
//
//		@Summary		Fetch transfer logs
//		@Description	get transfer logs for a given token and account
//		@Tags			logs
//		@Accept			json
//		@Produce		json
//		@Param			token_address	path		string	true	"Token Contract Address"
//	 	@Param			acc_address	path		string	true	"Address of the account"
//		@Success		200	{object}	common.Response
//		@Failure		400
//		@Failure		404
//		@Failure		500
//		@Router			/gov/{contract_address} [get]
func (s *Service) GetGov(w http.ResponseWriter, r *http.Request) {
	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	g := govindexer.Governor{
		Contract:    contractAddr,
		State:       "",
		CreatedAt:   time.Time{},
		UpdatedAt:   time.Time{},
		StartBlock:  10000,
		LastBlock:   10000,
		Name:        "Test Governor",
		Votes:       "",
		Description: "Something",
	}

	err := com.Body(w, g, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) GetGovProposals(w http.ResponseWriter, r *http.Request) {

	contractAddr := chi.URLParam(r, "contract_address")

	now := time.Now()

	props := []govindexer.Proposal{
		{
			Governor:    contractAddr,
			ProposalId:  "DEADBEEF",
			Proposer:    "CAFEBABE",
			State:       "All good",
			Targets:     nil,
			Values:      nil,
			Signatures:  nil,
			Calldatas:   nil,
			CreatedAt:   now,
			UpdatedAt:   now,
			VoteStart:   now,
			VoteEnd:     now,
			Name:        "The Name",
			Description: "The Description",
		},
	}

	err := com.BodyMultiple(w, props, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
