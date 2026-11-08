# go-order-payment-platform

[![CI](https://github.com/harini-singireddy/go-order-payment-platform/actions/workflows/ci.yml/badge.svg)](https://github.com/harini-singireddy/go-order-payment-platform/actions/workflows/ci.yml)

Production-grade order & payment microservices in Go — gRPC, RabbitMQ,
PostgreSQL, idempotency, optimistic locking, Prometheus + Grafana.

## Tech Stack

- **Go 1.27** with Go Workspaces (monorepo, one module per service)
- **gRPC** for internal service calls, **REST** at the gateway
- **PostgreSQL** with per-service database isolation
- **Redis** (provisioned for future caching, not on the critical path)
- **RabbitMQ** for async event-driven communication
- **Docker Compose** for local development
- **Prometheus + Grafana** for observability
- **Testcontainers** for integration tests, **GitHub Actions** for CI

## Architecture

See [`docs/architecture.md`](./docs/architecture.md) for the full diagram.
The short version: Gateway → Order → RabbitMQ → Inventory + Payment → back
to Order → Notification. Payment reads Order via gRPC before charging.

## Quick Start

```bash
# 1. Start the entire stack (infra + 5 services)
docker compose -f deployments/docker-compose.yml up -d

# 2. Apply migrations (first run only)
docker exec -i opp-postgres psql -U opp -d orders_db    < services/order/migrations/0001_create_orders.up.sql
docker exec -i opp-postgres psql -U opp -d payments_db  < services/payment/migrations/0001_create_payments.up.sql
docker exec -i opp-postgres psql -U opp -d inventory_db < services/inventory/migrations/0001_create_inventory.up.sql
docker exec -it opp-postgres psql -U opp -d inventory_db -c "INSERT INTO inventory (product_id, stock) VALUES ('prod-1', 10), ('prod-2', 5) ON CONFLICT DO NOTHING;"

# 3. Demo
TOKEN=$(curl -s -X POST localhost:8080/login -H 'Content-Type: application/json' \
  -d '{"username":"alice"}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')

curl -X POST localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-1' \
  -d '{"customer_id":"c1","items":[{"product_id":"prod-1","quantity":1,"price_cents":1000}]}'

# Order moves to "paid" within ~1 second.
# Grafana: http://localhost:3000 (admin/admin) → "OPP Overview"
# RabbitMQ UI: http://localhost:15672 (opp/opp_dev_password)

# 4. Stop
docker compose -f deployments/docker-compose.yml down