package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"opp/pkg/broker"
	"opp/pkg/database"
	"opp/pkg/logger"
	"opp/pkg/middleware"

	"github.com/harini-singireddy/go-order-payment-platform/services/inventory/internal/consumer"
	"github.com/harini-singireddy/go-order-payment-platform/services/inventory/internal/handler"
	"github.com/harini-singireddy/go-order-payment-platform/services/inventory/internal/repository"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logg := logger.New("inventory")
	dsn := env("DATABASE_URL", "postgres://opp:opp_dev_password@localhost:5432/inventory_db?sslmode=disable")
	amqpURL := env("AMQP_URL", "amqp://opp:opp_dev_password@localhost:5672/")

	pool, err := database.Connect(ctx, database.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	rb, err := broker.NewRabbit(amqpURL, logg, "orders", "inventory")
	if err != nil {
		log.Fatalf("connect rabbit: %v", err)
	}
	defer rb.Close()

	repo := repository.NewInventoryRepo(pool)
	oc := consumer.New(repo, rb, logg)

	if err := rb.Consume(ctx, "orders", "inventory.order.created", "order.created", oc.Handle); err != nil {
		log.Fatalf("consume: %v", err)
	}

	h := handler.New(repo)
	var root http.Handler = h.Routes()
	root = middleware.RequestID(root)
	root = middleware.Metrics("inventory")(root)

	srv := &http.Server{Addr: ":8083", Handler: root}
	go func() {
		logg.Info("inventory service listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Error("listen failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logg.Info("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
