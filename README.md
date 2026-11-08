cd ~/Desktop/go-order-payment-platform
cat > README.md <<'EOF'
# go-order-payment-platform

Production-grade order & payment microservices in Go — gRPC, RabbitMQ, PostgreSQL, Redis, idempotency, optimistic locking, Prometheus.

## Tech Stack

- **Go 1.22+** with Go Workspaces (monorepo, one module per service)
- **gRPC** for internal service calls, **REST** at the gateway (grpc-gateway)
- **PostgreSQL** with per-service schema isolation
- **Redis** for idempotency keys and inventory caching
- **RabbitMQ** for async event-driven communication
- **Docker Compose** for local development
- **Prometheus + Grafana** for observability

## Services

| Service | Responsibility |
|---------|----------------|
| Gateway | JWT auth, routing, rate limiting |
| Order | Order lifecycle, idempotency, state machine |
| Payment | Payment processing, idempotency |
| Inventory | Stock management, optimistic locking |
| Notification | Async event consumer (email simulation) |

## Quick Start

```bash
# 1. Start infrastructure (Postgres, Redis, RabbitMQ, Prometheus, Grafana)
make up

# 2. Apply migrations (creates tables in orders_db, payments_db, inventory_db)
make migrate

# 3. Seed inventory
make seed

# 4. Run the three services — one per terminal
make run-order       # HTTP :8081, gRPC :9091
make run-inventory   # HTTP :8083, consumes order.created
make run-payment     # consumes order.created, gRPC client to Order

```bash
docker compose -f deployments/docker-compose.yml up -d
make migrate
make run-all