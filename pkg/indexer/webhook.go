package indexer

import "context"

type WebhookMessager interface {
	Notify(ctx context.Context, message string) error
	NotifyError(ctx context.Context, errorMessage error) error
}
