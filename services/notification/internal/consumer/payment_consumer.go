package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

type PaymentCompleted struct {
	OrderID string `json:"order_id"`
}

type PaymentConsumer struct {
	log *slog.Logger
}

func New(log *slog.Logger) *PaymentConsumer {
	return &PaymentConsumer{log: log}
}

func (c *PaymentConsumer) Handle(ctx context.Context, body []byte) error {
	var evt PaymentCompleted
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	// Simulated email send — in a real system this would call an email provider.
	c.log.Info("sending email",
		"order_id", evt.OrderID,
		"to", "customer@example.com",
		"subject", "Your order is confirmed",
	)
	return nil
}
