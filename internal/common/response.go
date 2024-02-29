package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

type ResponseType string

const (
	ResponseTypeObject ResponseType = "object"
	ResponseTypeArray  ResponseType = "array"
	ResponseTypeSecure ResponseType = "secure"
)

type AddressResponse struct {
	Address string `json:"address"`
}

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// Response is the default response object
// swagger:response defaultResponse
type Response struct {
	// The response type
	// in: body
	ResponseType ResponseType `json:"response_type"`
	Object       any          `json:"object,omitempty"`
	Array        any          `json:"array,omitempty"`
	Meta         any          `json:"meta,omitempty"`
}

func Body(w http.ResponseWriter, body any, meta any) error {

	b, err := json.Marshal(&Response{
		ResponseType: ResponseTypeObject,
		Object:       body,
		Meta:         meta,
	})
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(b)

	return nil
}

func BodyMultiple(w http.ResponseWriter, body any, meta any) error {

	b, err := json.Marshal(&Response{
		ResponseType: ResponseTypeArray,
		Array:        body,
		Meta:         meta,
	})
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(b)

	return nil
}

func StreamedBody(w http.ResponseWriter, body string) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("stearming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	fmt.Fprintf(w, "%s", body)
	flusher.Flush()

	return nil
}

func JSONRPCBody(w http.ResponseWriter, id int, body any, meta any) error {

	b, err := json.Marshal(&indexer.JsonRPCResponse{
		Version: "2.0",
		ID:      id,
		Result:  body,
	})
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(b)

	return nil
}

func JSONRPCMultiBody(w http.ResponseWriter, ids []int, bodies []any, meta any) error {

	if len(ids) != len(bodies) {
		return errors.New("ids and bodies must have the same length")
	}

	responses := make([]indexer.JsonRPCResponse, len(ids))

	for i, id := range ids {
		responses[i] = indexer.JsonRPCResponse{
			Version: "2.0",
			ID:      id,
			Result:  bodies[i],
		}
	}

	b, err := json.Marshal(&responses)
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(b)

	return nil
}
