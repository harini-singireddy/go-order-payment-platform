package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"opp/pkg/broker"

	orderv1 "github.com/harini-singireddy/go-order-payment-platform/proto/order/v1"
	"github.com/harini-singireddy/go-order-payment-platform/services/payment/internal/repository"
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
	repo    *repository.PaymentRepo
	pub     broker.Publisher
	log     *slog.Logger
	ordConn *grpc.ClientConn
	ordCli  orderv1.OrderServiceClient
}

func New(repo *repository.PaymentRepo, pub broker.Publisher, log *slog.Logger, orderGRPCAddr string) (*OrderConsumer, error) {
	conn, err := grpc.NewClient(orderGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial order grpc: %w", err)
	}
	return &OrderConsumer{
		repo: repo, pub: pub, log: log,
		ordConn: conn,
		ordCli:  orderv1.NewOrderServiceClient(conn),
	}, nil
}

func (c *OrderConsumer) Close() { _ = c.ordConn.Close() }

func (c *OrderConsumer) Handle(ctx context.Context, body []byte) error {
	var evt OrderCreated
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	oid, err := uuid.Parse(evt.OrderID)
	if err != nil {
		return fmt.Errorf("parse order id: %w", err)
	}

	// Verify order exists and is pending via gRPC
	resp, err := c.ordCli.GetOrder(ctx, &orderv1.GetOrderRequest{Id: evt.OrderID})
	if err != nil {
		return fmt.Errorf("grpc get order: %w", err)
	}
	if resp.GetStatus() != "pending" {
		c.log.Warn("order not pending, skipping", "order_id", evt.OrderID, "status", resp.GetStatus())
		return nil
	}

	// Simulate charge (always succeeds)
	p := &repository.Payment{
		OrderID:     oid,
		AmountCents: resp.GetTotalCents(),
		Status:      "succeeded",
	}
	if err := c.repo.Insert(ctx, p); err != nil {
		if err == repository.ErrDuplicate {
			c.log.Info("payment already exists, skipping", "order_id", evt.OrderID)
			return nil
		}
		return err
	}

	c.log.Info("payment succeeded", "order_id", evt.OrderID, "amount_cents", p.AmountCents)
	return c.pub.Publish(ctx, "payments", "payment.completed", map[string]string{
		"order_id": evt.OrderID,
	})
}
