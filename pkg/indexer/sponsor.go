package indexer

type Sponsor struct {
	Contract   string `json:"contract"`
	PrivateKey string `json:"private_key"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
