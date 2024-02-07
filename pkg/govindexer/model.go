package govindexer

import "time"

type Governor struct {
	Contract string `json:"contract"`
	State    string `json:"state"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	StartBlock int64 `json:"start_block"`
	LastBlock  int64 `json:"last_block"`

	Name        string `json:"name"`
	Votes       string `json:"votes"`
	Description string `json:"description"`
}

type Proposal struct {
	Governor   string `json:"governor"`
	ProposalId string `json:"proposal_id"`
	Proposer   string `json:"proposer"`
	State      string `json:"state"`

	Targets    []string `json:"targets"`
	Values     []string `json:"valuez"`
	Signatures []string `json:"signatures"`
	Calldatas  []string `json:"calldatas"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	VoteStart time.Time `json:"vote_start"`
	VoteEnd   time.Time `json:"vote_end"`

	Name        string `json:"name"`
	Description string `json:"description"`
}
