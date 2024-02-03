package indexer

import "net/http"

type RPCHandlerFunc func(r *http.Request) (any, int)
