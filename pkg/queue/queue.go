package queue

import "github.com/citizenwallet/indexer/pkg/indexer"

type Service struct {
	queue      chan indexer.Message
	quit       chan bool
	maxRetries int
}

type Processor interface {
	Process(indexer.Message) error
}

func NewService(maxRetries int) *Service {
	return &Service{
		queue:      make(chan indexer.Message),
		quit:       make(chan bool),
		maxRetries: maxRetries,
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
					continue
				}
				return err
			}
		case <-s.quit:
			// quit the service
			return nil
		}
	}
}
