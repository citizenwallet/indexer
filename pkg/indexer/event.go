package indexer

type EventState string

const (
	EventStateQueued   EventState = "queued"
	EventStateIndexing EventState = "indexing"
	EventStateIndexed  EventState = "indexed"
)

type Standard string

const (
	ERC20   Standard = "ERC20"
	ERC721  Standard = "ERC721"
	ERC1155 Standard = "ERC1155"
)

type Event struct {
	Contract   string     `json:"contract"`
	State      EventState `json:"state"`
	CreatedAt  SQLiteTime `json:"created_at"`
	UpdatedAt  SQLiteTime `json:"updated_at"`
	StartBlock int64      `json:"start_block"`
	LastBlock  int64      `json:"last_block"`
	Standard   Standard   `json:"standard"`
	Name       string     `json:"name"`
	Symbol     string     `json:"symbol"`
}
