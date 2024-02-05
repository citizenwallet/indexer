package govindex

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
