package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"

	"opp/pkg/broker"
	"opp/pkg/database"

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/consumer"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/handler"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/service"
)

func TestOrderToPaidFlow(t *testing.T) {
	ctx := context.Background()

	// --- 1. Start Postgres ---
	pgC, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("orders_db"),
		postgres.WithUsername("opp"),
		postgres.WithPassword("opp_dev_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := database.Connect(ctx, database.Config{DSN: dsn})
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";
		CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			customer_id TEXT NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'pending',
			total_cents BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS order_items (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			product_id TEXT NOT NULL,
			quantity INT NOT NULL CHECK (quantity > 0),
			price_cents BIGINT NOT NULL CHECK (price_cents >= 0)
		);
	`)
	require.NoError(t, err)

	// --- 2. Start RabbitMQ ---
	rmqC, err := rabbitmq.Run(ctx, "rabbitmq:3.13-management-alpine",
		rabbitmq.WithAdminUsername("opp"),
		rabbitmq.WithAdminPassword("opp_dev_password"),
	)
	require.NoError(t, err)
	defer func() { _ = rmqC.Terminate(ctx) }()

	amqpURL, err := rmqC.AmqpURL(ctx)
	require.NoError(t, err)

	// --- 3. Wire broker + repo + service + handler ---
	logg := testLogger(t)

	rb, err := broker.NewRabbit(amqpURL, logg, "orders", "payments")
	require.NoError(t, err)
	defer rb.Close()

	repo := repository.NewOrderRepo(pool)
	svc := service.NewOrderService(repo, rb, logg)
	h := handler.NewOrderHandler(svc)

	// --- 4. Start Order's payment.completed consumer ---
	pc := consumer.NewPaymentConsumer(repo)
	require.NoError(t, rb.Consume(ctx, "payments", "test.order.payment.completed", "payment.completed", pc.Handle))

	// --- 5. Start Order HTTP server ---
	httpSrv := httptest.NewServer(h.Routes())
	defer httpSrv.Close()

	// --- 6. POST an order (publishes order.created) ---
	body := `{"customer_id":"test-customer","items":[{"product_id":"prod-1","quantity":2,"price_cents":1000}]}`
	req, _ := http.NewRequest(http.MethodPost, httpSrv.URL+"/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "integration-test-key-1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	orderID, err := uuid.Parse(created.ID)
	require.NoError(t, err)

	// --- 7. Simulate Payment: publish payment.completed ---
	require.NoError(t, rb.Publish(ctx, "payments", "payment.completed", map[string]string{
		"order_id": orderID.String(),
	}))

	// --- 8. Poll until status = paid ---
	require.Eventually(t, func() bool {
		var s string
		err := pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&s)
		return err == nil && s == "paid"
	}, 10*time.Second, 200*time.Millisecond, "order did not reach paid status")

	// --- 9. Idempotency: repeat POST with same key → 200, same order ---
	req2, _ := http.NewRequest(http.MethodPost, httpSrv.URL+"/orders", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "integration-test-key-1")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var repeat struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&repeat))
	require.Equal(t, created.ID, repeat.ID, "idempotency broken: different order returned")

	// --- 10. Only one order row for the idempotency key ---
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM orders WHERE idempotency_key = $1", "integration-test-key-1",
	).Scan(&count))
	require.Equal(t, 1, count)
}
