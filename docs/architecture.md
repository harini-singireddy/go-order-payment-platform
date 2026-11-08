# Architecture

## Service Responsibilities

| Service | Does | Does NOT do |
|---------|------|-------------|
| Gateway | JWT validation, rate limiting, reverse proxy to Order HTTP | Business logic |
| Order | Create/query orders, state machine, idempotency, gRPC GetOrder | Process payments |
| Payment | Simulate charge, idempotency, verify order via gRPC | Manage order state |
| Inventory | Deduct/release stock with optimistic locking | Know about payments |
| Notification | Log "sending email" on payment.completed | Block any flow |

## Communication

| From | To | Pattern | Protocol |
|------|-----|---------|----------|
| Client | Gateway | Sync | REST + JWT |
| Gateway | Order | Sync | HTTP reverse proxy |
| Order | RabbitMQ | Async | AMQP (publishes `order.created`) |
| Inventory | RabbitMQ | Async | AMQP (consumes `order.created`, publishes `inventory.reserved` / `inventory.failed`) |
| Payment | RabbitMQ | Async | AMQP (consumes `order.created`, publishes `payment.completed`) |
| Payment | Order | Sync | gRPC (`GetOrder` — read authoritative status and total) |
| Order | RabbitMQ | Async | AMQP (consumes `payment.completed` → status `pending` → `paid`) |
| Notification | RabbitMQ | Async | AMQP (consumes `payment.completed`, logs simulated email) |

## Trust boundary

The Gateway is the only component reachable from untrusted clients. It
terminates JWT auth and rate limits. All internal traffic — gRPC between
Payment and Order, RabbitMQ events, Order's HTTP — stays inside the trust
boundary and does NOT pass through the Gateway.

In production this is enforced by network policy: Order's `:9091` is only
reachable from Payment's pod, Order's `:8081` only from the Gateway. On
localhost, everything is reachable, which is why the boundary is implicit.

## Ports

| Service | HTTP | gRPC |
|---------|------|------|
| Gateway | 8080 | — |
| Order | 8081 | 9091 |
| Payment | 8082 | — |
| Inventory | 8083 | — |
| Notification | 8084 | — |

## Containers (local stack)

```mermaid
flowchart LR
    Client([curl / browser]) -->|REST :8080| GW[gateway]
    GW -->|HTTP :8081| ORD[order]
    ORD <-->|AMQP| RMQ[(rabbitmq)]
    INV[inventory] <-->|AMQP| RMQ
    PAY[payment] <-->|AMQP| RMQ
    NOT[notification] <-->|AMQP| RMQ
    PAY -->|gRPC :9091| ORD
    ORD --> PG[(postgres)]
    PAY --> PG
    INV --> PG
    PROM[prometheus] -.->|scrape /metrics| GW & ORD & PAY & INV & NOT
    GRAF[grafana] --> PROM