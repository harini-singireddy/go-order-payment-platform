package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryRepo struct{ pool *pgxpool.Pool }

func NewInventoryRepo(pool *pgxpool.Pool) *InventoryRepo { return &InventoryRepo{pool: pool} }

type Stock struct {
	ProductID string `json:"product_id"`
	Stock     int    `json:"stock"`
	Reserved  int    `json:"reserved"`
}

// Reserve atomically reserves quantity for a product. Returns false if insufficient.
func (r *InventoryRepo) Reserve(ctx context.Context, productID string, qty int) (bool, error) {
	const q = `
		UPDATE inventory
		SET reserved = reserved + $1, updated_at = now()
		WHERE product_id = $2 AND stock - reserved >= $1
	`
	tag, err := r.pool.Exec(ctx, q, qty, productID)
	if err != nil {
		return false, fmt.Errorf("reserve: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Release removes a reservation (compensation).
func (r *InventoryRepo) Release(ctx context.Context, productID string, qty int) error {
	const q = `UPDATE inventory SET reserved = GREATEST(reserved - $1, 0), updated_at = now() WHERE product_id = $2`
	_, err := r.pool.Exec(ctx, q, qty, productID)
	return err
}

func (r *InventoryRepo) Get(ctx context.Context, productID string) (*Stock, error) {
	const q = `SELECT product_id, stock, reserved FROM inventory WHERE product_id = $1`
	var s Stock
	err := r.pool.QueryRow(ctx, q, productID).Scan(&s.ProductID, &s.Stock, &s.Reserved)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
