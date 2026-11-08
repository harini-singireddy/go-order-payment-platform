package broker

import "context"

// Publisher publishes a JSON payload with a routing key on an exchange.
type Publisher interface {
	Publish(ctx context.Context, exchange, routingKey string, payload any) error
	Close() error
}

// Handler processes a raw message body. Returning an error causes a nack.
type Handler func(ctx context.Context, body []byte) error

// Consumer subscribes to a queue bound to an exchange with a routing key.
type Consumer interface {
	Consume(ctx context.Context, exchange, queue, routingKey string, handler Handler) error
	Close() error
}
