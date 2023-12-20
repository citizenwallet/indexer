package common

import (
	"context"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

// GetContextAddress returns the indexer.ContextKeyAddress from the context
func GetContextAddress(ctx context.Context) (string, bool) {
	addr, ok := ctx.Value(indexer.ContextKeyAddress).(string)
	return addr, ok
}
