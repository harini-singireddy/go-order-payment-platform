package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"opp/pkg/broker"
	"opp/pkg/logger"

	"github.com/harini-singireddy/go-order-payment-platform/services/notification/internal/consumer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logg := logger.New("notification")
	amqpURL := env("AMQP_URL", "amqp://opp:opp_dev_password@localhost:5672/")

	rb, err := broker.NewRabbit(amqpURL, logg, "payments")
	if err != nil {
		log.Fatalf("connect rabbit: %v", err)
	}
	defer rb.Close()

	pc := consumer.New(logg)
	if err := rb.Consume(ctx, "payments", "notification.payment.completed", "payment.completed", pc.Handle); err != nil {
		log.Fatalf("consume: %v", err)
	}

	logg.Info("notification service running")
	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: ":8084", Handler: mux}
	go func() {
		logg.Info("notification HTTP listening", "addr", srv.Addr)
		_ = srv.ListenAndServe()
	}()
	<-ctx.Done()
	logg.Info("shutting down")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
