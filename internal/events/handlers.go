package events

import (
	"encoding/json"
	"net/http"

	"github.com/citizenwallet/indexer/internal/services/db"
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

	// create transfer db for event
	txdb, err := s.db.AddTransferDB(ev.Contract)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = txdb.CreateTransferTable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// add event to database
	err = s.db.EventDB.AddEvent(ev.Contract, ev.State, ev.StartBlock, ev.LastBlock, ev.Standard, ev.Name, ev.Symbol)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
