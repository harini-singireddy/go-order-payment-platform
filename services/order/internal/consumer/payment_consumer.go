package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
)

type PaymentCompleted struct {
	OrderID string `json:"order_id"`
}

type PaymentConsumer struct {
	repo *repository.OrderRepo
}

func NewPaymentConsumer(repo *repository.OrderRepo) *PaymentConsumer {
	return &PaymentConsumer{repo: repo}
}

func (c *PaymentConsumer) Handle(ctx context.Context, body []byte) error {
	var evt PaymentCompleted
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	oid, err := uuid.Parse(evt.OrderID)
	if err != nil {
		return fmt.Errorf("parse order id: %w", err)
	}
	return c.repo.MarkPaid(ctx, oid)
}
