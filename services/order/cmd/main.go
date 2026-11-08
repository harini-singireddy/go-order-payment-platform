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

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/handler"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/service"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logg := logger.New("order")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://opp:opp_dev_password@localhost:5432/orders_db?sslmode=disable"
	}
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		amqpURL = "amqp://opp:opp_dev_password@localhost:5672/"
	}

	pool, err := database.Connect(ctx, database.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	rb, err := broker.NewRabbit(amqpURL, logg, "orders")
	if err != nil {
		log.Fatalf("connect rabbit: %v", err)
	}
	defer rb.Close()

	repo := repository.NewOrderRepo(pool)
	svc := service.NewOrderService(repo, rb, logg)
	h := handler.NewOrderHandler(svc)

	var root http.Handler = h.Routes()
	root = middleware.RequestID(root)
	root = middleware.Metrics("order")(root)

	srv := &http.Server{Addr: ":8081", Handler: root, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	go func() {
		logg.Info("order service listening", "addr", srv.Addr)
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
