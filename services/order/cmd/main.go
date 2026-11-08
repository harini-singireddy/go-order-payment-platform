package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"opp/pkg/broker"
	"opp/pkg/database"
	"opp/pkg/logger"
	"opp/pkg/middleware"

	orderv1 "github.com/harini-singireddy/go-order-payment-platform/proto/order/v1"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/consumer"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/grpcserver"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/handler"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/service"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logg := logger.New("order")
	dsn := env("DATABASE_URL", "postgres://opp:opp_dev_password@localhost:5432/orders_db?sslmode=disable")
	amqpURL := env("AMQP_URL", "amqp://opp:opp_dev_password@localhost:5672/")

	pool, err := database.Connect(ctx, database.Config{DSN: dsn})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	rb, err := broker.NewRabbit(amqpURL, logg, "orders", "payments", "inventory")
	if err != nil {
		log.Fatalf("connect rabbit: %v", err)
	}
	defer rb.Close()

	repo := repository.NewOrderRepo(pool)
	svc := service.NewOrderService(repo, rb, logg)

	// HTTP server on :8081
	h := handler.NewOrderHandler(svc)
	var root http.Handler = h.Routes()
	root = middleware.RequestID(root)
	root = middleware.Metrics("order")(root)
	srvHTTP := &http.Server{Addr: ":8081", Handler: root, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	go func() {
		logg.Info("order HTTP listening", "addr", srvHTTP.Addr)
		if err := srvHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Error("http listen failed", "err", err)
			cancel()
		}
	}()

	// gRPC server on :9091
	lis, err := net.Listen("tcp", ":9091")
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcSrv, grpcserver.New(repo))
	go func() {
		logg.Info("order gRPC listening", "addr", ":9091")
		if err := grpcSrv.Serve(lis); err != nil {
			logg.Error("grpc serve failed", "err", err)
			cancel()
		}
	}()

	// Consume payment.completed
	pc := consumer.NewPaymentConsumer(repo)
	if err := rb.Consume(ctx, "payments", "order.payment.completed", "payment.completed", pc.Handle); err != nil {
		log.Fatalf("consume payment.completed: %v", err)
	}

	<-ctx.Done()
	logg.Info("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = srvHTTP.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
