package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/domain"
)

var (
	ErrNotFound      = errors.New("order not found")
	ErrDuplicateKey  = errors.New("duplicate idempotency key")
)

type OrderRepo struct {
	pool *pgxpool.Pool
}

func NewOrderRepo(pool *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{pool: pool}
}

// Insert creates an order and its items in a single transaction.
func (r *OrderRepo) Insert(ctx context.Context, o *domain.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertOrder = `
		INSERT INTO orders (customer_id, idempotency_key, status, total_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, insertOrder,
		o.CustomerID, o.IdempotencyKey, string(o.Status), o.TotalCents,
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateKey
		}
		return fmt.Errorf("insert order: %w", err)
	}

	const insertItem = `
		INSERT INTO order_items (order_id, product_id, quantity, price_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	for i := range o.Items {
		err = tx.QueryRow(ctx, insertItem,
			o.ID, o.Items[i].ProductID, o.Items[i].Quantity, o.Items[i].PriceCents,
		).Scan(&o.Items[i].ID)
		if err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetByID loads an order and its items.
func (r *OrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	const q = `
		SELECT id, customer_id, idempotency_key, status, total_cents, created_at, updated_at
		FROM orders WHERE id = $1
	`
	var o domain.Order
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&o.ID, &o.CustomerID, &o.IdempotencyKey, &o.Status,
		&o.TotalCents, &o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query order: %w", err)
	}

	items, err := r.loadItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return &o, nil
}

// GetByIdempotencyKey is used by the service for idempotent retries.
func (r *OrderRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	const q = `
		SELECT id FROM orders WHERE idempotency_key = $1
	`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query idempotency key: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *OrderRepo) loadItems(ctx context.Context, orderID uuid.UUID) ([]domain.OrderItem, error) {
	const q = `
		SELECT id, product_id, quantity, price_cents
		FROM order_items WHERE order_id = $1 ORDER BY id
	`
	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(&it.ID, &it.ProductID, &it.Quantity, &it.PriceCents); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
