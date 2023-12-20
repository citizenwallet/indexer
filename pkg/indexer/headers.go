package indexer

import (
	"context"
)

const (
	// SignatureHeader is the header that contains the signature of the request
	SignatureHeader = "X-Signature"
	// AddressHeader is the header that contains the address of the sender
	AddressHeader = "X-Address"
	// AppVersionHeader is the header that contains the app version of the sender
	AppVersionHeader = "X-App-Version"
)

type ContextKey string

const (
	ContextKeyAddress   ContextKey = AddressHeader
	ContextKeySignature ContextKey = SignatureHeader
)

// get address from context if exists
func GetAddressFromContext(ctx context.Context) (string, bool) {
	addr, ok := ctx.Value(ContextKeyAddress).(string)
	return addr, ok
}
