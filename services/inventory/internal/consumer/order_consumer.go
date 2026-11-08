package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opp/pkg/broker"

	"github.com/harini-singireddy/go-order-payment-platform/services/inventory/internal/repository"
)

type OrderCreated struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Items      []struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

type OrderConsumer struct {
	repo *repository.InventoryRepo
	pub  broker.Publisher
	log  *slog.Logger
}

func New(repo *repository.InventoryRepo, pub broker.Publisher, log *slog.Logger) *OrderConsumer {
	return &OrderConsumer{repo: repo, pub: pub, log: log}
}

func (c *OrderConsumer) Handle(ctx context.Context, body []byte) error {
	var evt OrderCreated
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	for _, it := range evt.Items {
		ok, err := c.repo.Reserve(ctx, it.ProductID, it.Quantity)
		if err != nil {
			return err
		}
		if !ok {
			c.fail(ctx, evt, it.ProductID, "insufficient stock")
			return nil
		}
	}
	c.log.Info("inventory reserved", "order_id", evt.OrderID)
	_ = c.pub.Publish(ctx, "inventory", "inventory.reserved", map[string]string{"order_id": evt.OrderID})
	return nil
}

func (c *OrderConsumer) fail(ctx context.Context, evt OrderCreated, productID, reason string) {
	// release any partial reservations
	for _, it := range evt.Items {
		_ = c.repo.Release(ctx, it.ProductID, it.Quantity)
	}
	c.log.Warn("inventory failed", "order_id", evt.OrderID, "product_id", productID, "reason", reason)
	_ = c.pub.Publish(ctx, "inventory", "inventory.failed", map[string]string{
		"order_id": evt.OrderID, "product_id": productID, "reason": reason,
	})
}
