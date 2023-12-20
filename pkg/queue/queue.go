package queue

import (
	"context"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

// Service struct represents a queue service with a queue channel, quit channel, maximum retries, context and a webhook messager.
type Service struct {
	queue      chan indexer.Message // Channel to enqueue messages
	quit       chan bool            // Channel to signal service to stop
	maxRetries int                  // Maximum number of retries for processing a message

	ctx context.Context         // Context to carry deadlines, cancellation signals, and other request-scoped values across API boundaries and between processes
	wm  indexer.WebhookMessager // Webhook messager to notify errors
}

// Processor is an interface that must be implemented by the consumer of the queue
type Processor interface {
	Process(indexer.Message) (indexer.Message, error) // Process method to process a message
}

// NewService function initializes a new Service with provided maximum retries, context and webhook messager.
func NewService(maxRetries int, ctx context.Context, wm indexer.WebhookMessager) *Service {
	return &Service{
		queue:      make(chan indexer.Message), // Initialize the queue channel
		quit:       make(chan bool),            // Initialize the quit channel
		maxRetries: maxRetries,                 // Set the maximum retries
		ctx:        ctx,                        // Set the context
		wm:         wm,                         // Set the webhook messager
	}
}

// Enqueue method enqueues a message to the queue channel.
func (s *Service) Enqueue(message indexer.Message) {
	s.queue <- message
}

// Close method sends a signal to the quit channel to stop the service.
func (s *Service) Close() {
	s.quit <- true
}

// Start method starts the service and processes messages from the queue channel.
// If processing a message fails, it requeues the message until the maximum retries is reached.
// If the queue was empty, it waits for a duration before continuing to avoid a busy loop.
// It also notifies errors using the webhook messager.
// The service can be stopped by sending a signal to the quit channel.
func (s *Service) Start(p Processor) error {
	for {
		select {
		case message := <-s.queue:
			msg, err := p.Process(message)
			if err != nil {
				if msg.RetryCount < s.maxRetries {
					msg.RetryCount++
					s.queue <- msg
					if len(s.queue) == 1 {
						extraWait := time.Duration(msg.RetryCount) * time.Second
						time.Sleep(extraWait)
					}
					continue
				}

				s.wm.NotifyError(s.ctx, err)
			}
		case <-s.quit:
			return nil
		}
	}
}
