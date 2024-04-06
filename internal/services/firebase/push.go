package firebase

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/citizenwallet/indexer/internal/services/db"
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

	data := "{}"
	if push.Data != nil {
		data = string(push.Data)
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Data: map[string]string{
			"tx_data": data,
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:            "tx_notification.wav",
					ContentAvailable: push.Silent,
				},
			},
			Headers: map[string]string{
				"apns-priority": "10", // Set the priority to 10
			},
		},
		Android: &messaging.AndroidConfig{
			Priority: "high", // Set the priority to high
		},
	}

	if !push.Silent {
		message.Notification = &messaging.Notification{
			Title: push.Title,
			Body:  push.Body,
		}
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

func SendPushForTxs(ptdb *db.PushTokenDB, fb *PushService, ev *indexer.Event, txs []*indexer.Transfer) {
	accTokens := map[string][]*indexer.PushToken{}

	messages := []*indexer.PushMessage{}

	for _, tx := range txs {
		if tx.Status == indexer.TransferStatusSuccess {
			// silent notifications for successful transfers for the senders
			if _, ok := accTokens[tx.From]; !ok {
				// get the push tokens for the recipient
				pt, err := ptdb.GetAccountTokens(tx.From)
				if err != nil {
					return
				}

				if len(pt) == 0 {
					// no push tokens for this account
					continue
				}

				accTokens[tx.From] = pt
			}

			messages = append(messages, indexer.NewSilentPushMessage(accTokens[tx.From], tx))
		}

		if _, ok := accTokens[tx.To]; !ok {
			// get the push tokens for the recipient
			pt, err := ptdb.GetAccountTokens(tx.To)
			if err != nil {
				return
			}

			if len(pt) == 0 {
				// no push tokens for this account
				continue
			}

			accTokens[tx.To] = pt
		}

		value := tx.ToRounded(ev.Decimals)

		messages = append(messages, indexer.NewAnonymousPushMessage(accTokens[tx.To], ev.Name, fmt.Sprintf("%.2f", value), ev.Symbol, tx))
	}

	if len(messages) > 0 {
		for _, push := range messages {
			badTokens, err := fb.Send(push)
			if err != nil {
				continue
			}

			if len(badTokens) > 0 {
				// remove the bad tokens
				for _, token := range badTokens {
					err = ptdb.RemovePushToken(token)
					if err != nil {
						continue
					}
				}
			}
		}
	}
}
