package indexer

import "encoding/json"

type JsonRPCRequest struct {
	Version string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (r *JsonRPCRequest) isValid() bool {
	return r.Version == "2.0" && r.ID > 0 && r.Method != ""
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type JsonRPCResponse struct {
	Version string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Result  any           `json:"result"`
	Error   *JSONRPCError `json:"error,omitempty"`
}
