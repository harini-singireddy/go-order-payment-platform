package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitConfig struct {
	URL string
}

// Rabbit is both a Publisher and a Consumer backed by amqp091-go.
type Rabbit struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	log  *slog.Logger
}

// NewRabbit dials RabbitMQ, opens a channel, and declares the given exchanges (topic).
func NewRabbit(url string, log *slog.Logger, exchanges ...string) (*Rabbit, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbit: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}
	for _, ex := range exchanges {
		if err := ch.ExchangeDeclare(ex, "topic", true, false, false, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("declare exchange %s: %w", ex, err)
		}
	}
	return &Rabbit{conn: conn, ch: ch, log: log}, nil
}

func (r *Rabbit) Publish(ctx context.Context, exchange, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
		Timestamp:   time.Now(),
	})
}

func (r *Rabbit) Consume(ctx context.Context, exchange, queue, routingKey string, handler Handler) error {
	if _, err := r.ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", queue, err)
	}
	if err := r.ch.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %s: %w", queue, err)
	}
	if err := r.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("qos: %w", err)
	}
	msgs, err := r.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-msgs:
				if !ok {
					return
				}
				if err := handler(ctx, m.Body); err != nil {
					r.log.Error("handler failed", "queue", queue, "err", err)
					_ = m.Nack(false, true) // requeue
					continue
				}
				_ = m.Ack(false)
			}
		}
	}()
	return nil
}

func (r *Rabbit) Close() error {
	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
