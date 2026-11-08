package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/domain"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
)

type OrderService struct {
	repo *repository.OrderRepo
}

func NewOrderService(repo *repository.OrderRepo) *OrderService {
	return &OrderService{repo: repo}
}

// CreateOrder is idempotent: the same idempotency key returns the same order.
func (s *OrderService) CreateOrder(ctx context.Context, idemKey string, o *domain.Order) (*domain.Order, bool, error) {
	if idemKey == "" {
		return nil, false, errors.New("idempotency key required")
	}
	if err := o.Validate(); err != nil {
		return nil, false, err
	}

	// Compute total from items to avoid trusting client input.
	var total int64
	for _, it := range o.Items {
		total += int64(it.Quantity) * it.PriceCents
	}
	o.TotalCents = total
	o.IdempotencyKey = idemKey
	o.Status = domain.StatusPending

	err := s.repo.Insert(ctx, o)
	if err == nil {
		return o, true, nil // created
	}
	if !errors.Is(err, repository.ErrDuplicateKey) {
		return nil, false, fmt.Errorf("insert order: %w", err)
	}

	// Duplicate -> fetch the existing order and return it.
	existing, getErr := s.repo.GetByIdempotencyKey(ctx, idemKey)
	if getErr != nil {
		return nil, false, fmt.Errorf("fetch existing order: %w", getErr)
	}
	return existing, false, nil // not created
}

func (s *OrderService) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id)
}
