package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusShipped   Status = "shipped"
	StatusCancelled Status = "cancelled"
)

var (
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrEmptyItems        = errors.New("order must contain at least one item")
	ErrInvalidQuantity   = errors.New("quantity must be greater than zero")
)

type OrderItem struct {
	ID         uuid.UUID `json:"id"`
	ProductID  string    `json:"product_id"`
	Quantity   int       `json:"quantity"`
	PriceCents int64     `json:"price_cents"`
}

type Order struct {
	ID             uuid.UUID   `json:"id"`
	CustomerID     string      `json:"customer_id"`
	IdempotencyKey string      `json:"-"`
	Status         Status      `json:"status"`
	TotalCents     int64       `json:"total_cents"`
	Items          []OrderItem `json:"items"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// CanTransitionTo reports whether the order may move to the given status.
func (o *Order) CanTransitionTo(next Status) bool {
	switch o.Status {
	case StatusPending:
		return next == StatusPaid || next == StatusCancelled
	case StatusPaid:
		return next == StatusShipped || next == StatusCancelled
	default:
		return false
	}
}

// TransitionTo applies the status change if allowed.
func (o *Order) TransitionTo(next Status) error {
	if !o.CanTransitionTo(next) {
		return ErrInvalidTransition
	}
	o.Status = next
	o.UpdatedAt = time.Now().UTC()
	return nil
}

// Validate checks that the order has well-formed items.
func (o *Order) Validate() error {
	if len(o.Items) == 0 {
		return ErrEmptyItems
	}
	for _, item := range o.Items {
		if item.Quantity <= 0 {
			return ErrInvalidQuantity
		}
	}
	return nil
}
