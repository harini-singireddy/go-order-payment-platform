package domain

import "errors"

var ErrInsufficientStock = errors.New("insufficient stock")

type Item struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}
