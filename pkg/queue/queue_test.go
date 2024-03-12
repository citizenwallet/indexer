package queue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
	"github.com/ethereum/go-ethereum/common"
)

type TestTxProcessor struct {
	t             *testing.T
	expectedCount int
	count         int

	expectedError error
}

func (p *TestTxProcessor) Process(messages []indexer.Message) ([]indexer.Message, []error) {
	invalidMessages := []indexer.Message{}
	messageErrors := []error{}

	for _, m := range messages {
		p.count++
		_, ok := m.Message.(indexer.UserOpMessage)
		if !ok {
			invalidMessages = append(invalidMessages, m)
			messageErrors = append(messageErrors, p.expectedError)
		}
	}

	return invalidMessages, messageErrors
}

type TestTxMessager struct {
	t             *testing.T
	expectedError error
}

func (m *TestTxMessager) Notify(ctx context.Context, message string) error {
	return nil
}

func (m *TestTxMessager) NotifyWarning(ctx context.Context, errorMessage error) error {
	return nil
}

func (m *TestTxMessager) NotifyError(ctx context.Context, errorMessage error) error {
	if strings.Contains(errorMessage.Error(), "queue is full") || strings.Contains(errorMessage.Error(), "queue is almost full") {
		return nil
	}

	if errorMessage != m.expectedError {
		m.t.Fatalf("expected %s, got %s", m.expectedError, errorMessage)
	}
	return nil
}

func TestProcessMessages(t *testing.T) {
	expectedTxError := errors.New("invalid tx message")

	t.Run("TxMessages", func(t *testing.T) {
		testCases := []indexer.Message{
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
		}

		m := &TestTxMessager{t, expectedTxError}
		q := NewService("tx", 3, 10, nil, m)

		p := &TestTxProcessor{t, len(testCases), 0, expectedTxError}

		go func() {
			for _, tc := range testCases {
				q.Enqueue(tc)
			}

			for {
				if p.count >= p.expectedCount {
					break
				}

				time.Sleep(100 * time.Millisecond)
			}
			q.Close()
		}()

		err := q.Start(p)
		if err != nil {
			t.Fatal(err)
		}

		if p.count != p.expectedCount {
			t.Fatalf("expected %d, got %d", p.expectedCount, p.count)
		}
	})

	t.Run("TxMessages with 1 invalid", func(t *testing.T) {
		testCases := []indexer.Message{
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
			{ID: "invalid", CreatedAt: time.Now(), RetryCount: 0, Message: "invalid"},
			*indexer.NewTxMessage(common.Address{}, common.Address{}, common.Big0, indexer.UserOp{}, nil),
		}

		m := &TestTxMessager{t, expectedTxError}
		q := NewService("tx", 3, 10, nil, m)

		p := &TestTxProcessor{t, len(testCases) + 3, 0, expectedTxError}

		go func() {
			for _, tc := range testCases {
				q.Enqueue(tc)
			}

			for {
				if p.count >= p.expectedCount {
					break
				}

				time.Sleep(100 * time.Millisecond)
			}
			q.Close()
		}()

		err := q.Start(p)
		if err != nil {
			t.Fatal(err)
		}

		if p.count != p.expectedCount {
			t.Fatalf("expected %d, got %d", p.expectedCount, p.count)
		}
	})

	t.Run("Push Notifications", func(t *testing.T) {
		// TODO: implement
	})
}
