package firebase

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/citizenwallet/indexer/internal/storage"
	"github.com/citizenwallet/indexer/pkg/indexer"
	"google.golang.org/api/option"
)

type PushService struct {
	ctx       context.Context
	Messaging *messaging.Client
}

func NewPushService(ctx context.Context, path string) *PushService {
	// file exists
	exists := storage.Exists(path)
	if !exists {
		log.Default().Println("firebase credentials file not found, push notifications will be disabled.")
		// return a new PushService with a nil Messaging client
		return &PushService{ctx: ctx, Messaging: nil}
	}

	opt := option.WithCredentialsFile(path)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		log.Fatalf("error initializing messaging client: %v\n", err)
	}

	return &PushService{
		ctx:       ctx,
		Messaging: client,
	}
}

// Send sends a push notification to the given tokens. Returns the tokens to be removed.
func (s *PushService) Send(push *indexer.PushMessage) ([]string, error) {
	if s.Messaging == nil {
		return []string{}, nil
	}

	tokens := []string{}

	for _, t := range push.Tokens {
		tokens = append(tokens, t.Token)
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: push.Title,
			Body:  push.Body,
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "tx_notification.wav",
				},
			},
		},
	}

	br, err := s.Messaging.SendEachForMulticast(s.ctx, message)
	if err != nil {
		return []string{}, err
	}

	if br.FailureCount > 0 {
		var failedTokens []string
		for idx, resp := range br.Responses {
			if !resp.Success {
				// The order of responses corresponds to the order of the registration tokens.
				failedTokens = append(failedTokens, tokens[idx])
			}
		}

		return failedTokens, nil
	}

	return []string{}, nil
}
