package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDuplicate = errors.New("payment already exists")

type PaymentRepo struct{ pool *pgxpool.Pool }

func NewPaymentRepo(pool *pgxpool.Pool) *PaymentRepo { return &PaymentRepo{pool: pool} }

type Payment struct {
	ID          uuid.UUID `json:"id"`
	OrderID     uuid.UUID `json:"order_id"`
	AmountCents int64     `json:"amount_cents"`
	Status      string    `json:"status"`
}

func (r *PaymentRepo) Insert(ctx context.Context, p *Payment) error {
	const q = `
		INSERT INTO payments (order_id, amount_cents, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err := r.pool.QueryRow(ctx, q, p.OrderID, p.AmountCents, p.Status).Scan(&p.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}
