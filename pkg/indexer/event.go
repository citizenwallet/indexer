package indexer

type EventState string

const (
	EventStateQueued   EventState = "queued"
	EventStateIndexing EventState = "indexing"
	EventStateIndexed  EventState = "indexed"
)

type Event struct {
	Contract   string     `json:"contract"`
	State      EventState `json:"state"`
	CreatedAt  SQLiteTime `json:"created_at"`
	UpdatedAt  SQLiteTime `json:"updated_at"`
	StartBlock int64      `json:"start_block"`
	LastBlock  int64      `json:"last_block"`
	Function   string     `json:"function"`
	Name       string     `json:"name"`
	Symbol     string     `json:"symbol"`
}
