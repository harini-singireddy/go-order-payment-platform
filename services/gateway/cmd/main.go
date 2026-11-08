package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"opp/pkg/logger"
	"opp/pkg/middleware"

	gatewayauth "github.com/harini-singireddy/go-order-payment-platform/services/gateway/internal/auth"
	gatewaymw "github.com/harini-singireddy/go-order-payment-platform/services/gateway/internal/middleware"
	"github.com/harini-singireddy/go-order-payment-platform/services/gateway/internal/proxy"
)

func main() {
	logg := logger.New("gateway")
	secret := []byte(env("JWT_SECRET", "dev-secret-change-me"))
	orderTarget := env("ORDER_HTTP_URL", "http://localhost:8081")

	orderProxy, err := proxy.NewOrderProxy(orderTarget)
	if err != nil {
		log.Fatalf("init proxy: %v", err)
	}

	rl := gatewaymw.NewIPRateLimiter(10, 20)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Metrics("gateway"))
	r.Use(rl.Middleware)

	// Public endpoints (no auth)
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Username == "" {
			http.Error(w, `{"error":"username required"}`, http.StatusBadRequest)
			return
		}
		token, err := gatewayauth.Generate(secret, body.Username)
		if err != nil {
			http.Error(w, `{"error":"token generation failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Protected: all /orders/* traffic goes through JWT then proxy
	r.Group(func(r chi.Router) {
		r.Use(gatewayauth.Middleware(secret))
		r.Handle("/orders", orderProxy)
		r.Handle("/orders/*", orderProxy)
	})

	srv := &http.Server{Addr: ":8080", Handler: r}

	go func() {
		logg.Info("gateway listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Error("listen failed", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logg.Info("shutting down")
	_ = srv.Close()
	time.Sleep(100 * time.Millisecond)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
