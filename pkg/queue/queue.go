package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

const batchSize = 10 // Size of each batch

// Service struct represents a queue service with a queue channel, quit channel, maximum retries, context and a webhook messager.
type Service struct {
	name       string               // Name of the queue service
	queue      chan indexer.Message // Channel to enqueue messages
	quit       chan bool            // Channel to signal service to stop
	maxRetries int                  // Maximum number of retries for processing a message
	bufferSize int                  // Buffer size of the queue channel

	ctx context.Context         // Context to carry deadlines, cancellation signals, and other request-scoped values across API boundaries and between processes
	wm  indexer.WebhookMessager // Webhook messager to notify errors
}

// Processor is an interface that must be implemented by the consumer of the queue
type Processor interface {
	Process([]indexer.Message) ([]indexer.Message, []error) // Process method to process a message
}

// NewService function initializes a new Service with provided maximum retries, context and webhook messager.
func NewService(name string, maxRetries, bufferSize int, ctx context.Context, wm indexer.WebhookMessager) *Service {
	return &Service{
		name:       name,                                   // Set the name
		queue:      make(chan indexer.Message, bufferSize), // Initialize the buffered queue channel
		quit:       make(chan bool),                        // Initialize the quit channel
		maxRetries: maxRetries,                             // Set the maximum retries
		bufferSize: bufferSize,                             // Set the buffer size
		ctx:        ctx,                                    // Set the context
		wm:         wm,                                     // Set the webhook messager
	}
}

// Enqueue method enqueues a message to the queue channel.
func (s *Service) Enqueue(message indexer.Message) {
	// if the queue channel is almost full, notify the webhook messager with a warning notification
	bufferWarning := s.bufferSize - (s.bufferSize / 10)
	if len(s.queue) > bufferWarning {
		s.wm.NotifyWarning(s.ctx, errors.New(fmt.Sprintf("%s queue is almost full", s.name)))
	}

	// if the queue channel is full, notify the webhook messager with an error notification
	if len(s.queue) == s.bufferSize {
		s.wm.NotifyError(s.ctx, errors.New(fmt.Sprintf("%s queue is full", s.name)))
	}

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
	log.Default().Println(fmt.Sprintf("starting queue service '%s'", s.name))
	for {
		select {
		case message := <-s.queue:
			// Create a batch
			batch := make([]indexer.Message, 0, batchSize)

			batch = append(batch, message)

			time.Sleep(250 * time.Millisecond)

			// Fill the batch
		batchLoop:
			for len(batch) < batchSize {
				select {
				case item, ok := <-s.queue:
					if !ok {
						return fmt.Errorf("channel is closed") // Channel is closed
					}
					batch = append(batch, item)
				default:
					break batchLoop // Channel is empty
				}
			}

			msgs, errs := p.Process(batch)
			for i, msg := range msgs {
				err := errs[i]
				if err != nil {
					if msg.RetryCount < s.maxRetries {
						msg.RetryCount++

						if len(s.queue) < 1 && len(msgs) == 1 {
							extraWait := time.Duration(msg.RetryCount) * time.Second
							time.Sleep(extraWait)
						}

						s.Enqueue(msg)
						continue
					}

					if s.wm != nil {
						s.wm.NotifyError(s.ctx, err)
					}
				}
			}
		case <-s.quit:
			log.Default().Println(fmt.Sprintf("stopping queue service '%s'", s.name))
			return nil
		}
	}
}
