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

	// check whether event already exists
	name := s.db.TransferName(ev.Contract)
	exists, err := s.db.TransferTableExists(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	exists, err = s.db.PushTokenTableExists(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

	err = txdb.CreateTransferTableIndexes()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// create push token db for event
	ptdb, err := s.db.AddPushTokenDB(ev.Contract)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = ptdb.CreatePushTable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = ptdb.CreatePushTableIndexes()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// add event to database
	err = s.db.EventDB.AddEvent(ev.Contract, ev.State, ev.StartBlock, ev.LastBlock, ev.Standard, ev.Name, ev.Symbol, ev.Decimals)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
