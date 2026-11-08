package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"opp/pkg/broker"
	"opp/pkg/database"
	"opp/pkg/logger"

	"github.com/harini-singireddy/go-order-payment-platform/services/payment/internal/consumer"
	"github.com/harini-singireddy/go-order-payment-platform/services/payment/internal/repository"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logg := logger.New("payment")
	dsn := env("DATABASE_URL", "postgres://opp:opp_dev_password@localhost:5432/payments_db?sslmode=disable")
	amqpURL := env("AMQP_URL", "amqp://opp:opp_dev_password@localhost:5672/")
	orderGRPC := env("ORDER_GRPC_ADDR", "localhost:9091")

	pool, err := database.Connect(ctx, database.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	rb, err := broker.NewRabbit(amqpURL, logg, "orders", "payments")
	if err != nil {
		log.Fatalf("connect rabbit: %v", err)
	}
	defer rb.Close()

	repo := repository.NewPaymentRepo(pool)
	oc, err := consumer.New(repo, rb, logg, orderGRPC)
	if err != nil {
		log.Fatalf("init consumer: %v", err)
	}
	defer oc.Close()

	if err := rb.Consume(ctx, "orders", "payment.order.created", "order.created", oc.Handle); err != nil {
		log.Fatalf("consume: %v", err)
	}

	logg.Info("payment service running")
	<-ctx.Done()
	logg.Info("shutting down")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
