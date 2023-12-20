package queue

import (
	"context"
	"time"

	"github.com/citizenwallet/indexer/pkg/indexer"
)

type Service struct {
	queue      chan indexer.Message
	quit       chan bool
	maxRetries int

	ctx context.Context
	wm  indexer.WebhookMessager
}

type Processor interface {
	Process(indexer.Message) error
}

func NewService(maxRetries int, ctx context.Context, wm indexer.WebhookMessager) *Service {
	return &Service{
		queue:      make(chan indexer.Message),
		quit:       make(chan bool),
		maxRetries: maxRetries,
		ctx:        ctx,
		wm:         wm,
	}
}

func (s *Service) Enqueue(message indexer.Message) {
	s.queue <- message
}

func (s *Service) Close() {
	s.quit <- true
}

func (s *Service) Start(p Processor) error {
	for {
		select {
		case message := <-s.queue:
			// process an item in the queue
			// it is up to the processor to handle the data type
			err := p.Process(message)
			if err != nil {
				// if there is an error, requeue the message
				if message.RetryCount < s.maxRetries {
					message.RetryCount++
					s.queue <- message
					if len(s.queue) == 1 {
						// if the queue was empty, we need to wait a bit
						// to avoid a busy loop
						extraWait := time.Duration(message.RetryCount) * time.Second
						time.Sleep(extraWait)
					}
					continue
				}

				s.wm.NotifyError(s.ctx, err)
			}
		case <-s.quit:
			// quit the service
			return nil
		}
	}
}
