# Interview Prep — one page

## What this project is

A microservices order & payment platform in Go. Five services, event-driven
with RabbitMQ, gRPC for the read path, Postgres for per-service data,
JWT auth and per-IP rate limiting at the gateway, Prometheus + Grafana for
observability, one `docker compose up` to run everything locally.

## Services in one line each

- **Gateway** — JWT auth, rate limiting, reverse proxy to Order HTTP.
- **Order** — creates orders (idempotent), owns the status state machine, exposes gRPC `GetOrder`.
- **Payment** — consumes `order.created`, calls Order gRPC to verify status/total, inserts payment, publishes `payment.completed`.
- **Inventory** — consumes `order.created`, reserves stock with optimistic locking, publishes `inventory.reserved` or `inventory.failed`.
- **Notification** — consumes `payment.completed`, logs a simulated email.

## The flow in words

Client → Gateway (JWT) → Order (insert pending) → RabbitMQ `order.created` →
Inventory reserves stock / Payment charges (via gRPC read of the order) →
`payment.completed` → Order updates status to `paid` → Notification logs.

## Five questions and answers

**1. How do you prevent duplicate orders?**
Unique constraint on `orders.idempotency_key`. On duplicate insert, Postgres
returns error 23505, the service catches it and returns the existing row.
ADR-002.

**2. How do you prevent overselling?**
Conditional UPDATE: `SET reserved = reserved + $1 WHERE stock - reserved >= $1`.
`RowsAffected() == 0` means insufficient stock. No distributed lock needed.
ADR-003.

**3. Why RabbitMQ and not Kafka?**
Order volume is thousands, not millions. Events need flexible routing
(`order.created` → both Inventory and Payment). RabbitMQ's Exchange model
fits; Kafka's Topic model would need separate topics. ADR-001.

**4. Why HTTP and gRPC on separate ports?**
Distinct listeners are simpler to debug (`curl :8081`, `grpcurl :9091`),
each has its own timeouts and shutdown. cmux on one port is valid but
adds complexity not justified here. ADR-004.

**5. How do you test it?**
End-to-end flow verified manually with `docs/testing.md`, plus an
integration test with Testcontainers (real Postgres + RabbitMQ) that asserts
the order reaches `paid` and the payment row exists. CI runs `go vet`,
`go build`, and the integration test on every push.

## What I would do next

- **Redis idempotency fast-path** — check `GET idem:<key>` before the DB
  insert; DB constraint stays as the correctness guarantee.
- **OpenTelemetry tracing** — propagate trace IDs through gRPC and RabbitMQ
  so a single request is visible end-to-end in Jaeger.
- **`inventory.failed` → cancel the order** — currently Payment proceeds even
  if inventory fails; a compensation step would mark the order cancelled.
- **Multi-replica gateway** — move the rate limiter to Redis so counters are
  shared across instances.
- **Load test with `hey`** — record requests/sec and p99 in the README.

## Known limitations (be honest about these)

- Rate limiter is in-memory, per-IP, single-instance. `/healthz` shares
  the limit.
- Tokens are not revocable; valid until expiry (24h).
- No event replay (RabbitMQ default). Kafka would be the choice if replay
  were required.
- Notification is a log line, not a real email.
- `inventory.failed` is published but Order does not yet cancel the order.
