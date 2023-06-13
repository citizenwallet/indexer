package node

type Transfer struct {
	Hash    string     `json:"hash"`
	TokenID int64      `json:"token_id"`
	Date    SQLiteTime `json:"created_at"`
	From    string     `json:"from_addr"`
	To      string     `json:"to_addr"`
	Value   int64      `json:"value"`
	Data    []byte     `json:"data"`
}
