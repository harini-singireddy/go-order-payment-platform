package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"opp/pkg/broker"

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/domain"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
)

type OrderService struct {
	repo *repository.OrderRepo
	pub  broker.Publisher
	log  *slog.Logger
}

var ordersCreated = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "opp",
	Subsystem: "order",
	Name:      "orders_created_total",
	Help:      "Total orders created.",
})

func NewOrderService(repo *repository.OrderRepo, pub broker.Publisher, log *slog.Logger) *OrderService {
	return &OrderService{repo: repo, pub: pub, log: log}
}

type OrderCreatedEvent struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Items      []struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

func (s *OrderService) CreateOrder(ctx context.Context, idemKey string, o *domain.Order) (*domain.Order, bool, error) {
	if idemKey == "" {
		return nil, false, errors.New("idempotency key required")
	}
	if err := o.Validate(); err != nil {
		return nil, false, err
	}

	var total int64
	for _, it := range o.Items {
		total += int64(it.Quantity) * it.PriceCents
	}
	o.TotalCents = total
	o.IdempotencyKey = idemKey
	o.Status = domain.StatusPending

	err := s.repo.Insert(ctx, o)
	if err == nil {
		ordersCreated.Inc()
		s.publishCreated(ctx, o)
		return o, true, nil
	}
	if !errors.Is(err, repository.ErrDuplicateKey) {
		return nil, false, fmt.Errorf("insert order: %w", err)
	}

	existing, getErr := s.repo.GetByIdempotencyKey(ctx, idemKey)
	if getErr != nil {
		return nil, false, fmt.Errorf("fetch existing order: %w", getErr)
	}
	return existing, false, nil
}

func (s *OrderService) publishCreated(ctx context.Context, o *domain.Order) {
	if s.pub == nil {
		return
	}
	evt := OrderCreatedEvent{OrderID: o.ID.String(), CustomerID: o.CustomerID}
	for _, it := range o.Items {
		evt.Items = append(evt.Items, struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}{ProductID: it.ProductID, Quantity: it.Quantity})
	}
	if err := s.pub.Publish(ctx, "orders", "order.created", evt); err != nil {
		s.log.Error("publish order.created failed", "order_id", o.ID, "err", err)
	}
}

func (s *OrderService) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id)
}
