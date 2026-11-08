package grpcserver

import (
	"context"

	"github.com/google/uuid"

	orderv1 "github.com/harini-singireddy/go-order-payment-platform/proto/order/v1"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
)

type OrderGRPCServer struct {
	orderv1.UnimplementedOrderServiceServer
	repo *repository.OrderRepo
}

func New(repo *repository.OrderRepo) *OrderGRPCServer {
	return &OrderGRPCServer{repo: repo}
}

func (s *OrderGRPCServer) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, err
	}
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &orderv1.GetOrderResponse{
		Id:         o.ID.String(),
		CustomerId: o.CustomerID,
		Status:     string(o.Status),
		TotalCents: o.TotalCents,
	}, nil
}
