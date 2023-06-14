package events

import (
	"encoding/json"
	"net/http"

	"github.com/citizenwallet/indexer/internal/db"
	"github.com/citizenwallet/indexer/pkg/indexer"
)

type Service struct {
	db *db.DB
}

func NewService(db *db.DB) *Service {
	return &Service{
		db: db,
	}
}

// AddEvent adds an event to the database for future indexing
func (s *Service) AddEvent(w http.ResponseWriter, r *http.Request) {
	// parse event from request body
	ev := &indexer.Event{}

	err := json.NewDecoder(r.Body).Decode(ev)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// if we are adding an event, it should be queued for indexing
	ev.State = indexer.EventStateQueued

	// add event to database
	err = s.db.EventDB.AddEvent(ev.Contract, ev.State, ev.StartBlock, ev.LastBlock, ev.Function, ev.Name, ev.Symbol)
	if err != nil {
		println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
