# Architecture

## Service Responsibilities

| Service | Does | Does NOT do |
|---------|------|-------------|
| Gateway | JWT validation, routing, rate limiting | Business logic |
| Order | Create/query orders, state machine, idempotency | Process payments |
| Payment | Simulate payment, idempotency, verify order via gRPC | Manage order state |
| Inventory | Deduct/release stock with optimistic locking | Know about payments |
| Notification | Send email on payment.completed event | Block any flow |

## Communication

| From | To | Pattern | Protocol |
|------|-----|---------|----------|
| Client | Gateway | Sync | REST |
| Gateway | Order | Sync | gRPC |
| Order | RabbitMQ | Async | AMQP (publishes `order.created`) |
| Inventory | RabbitMQ | Async | AMQP (consumes `order.created`, publishes `inventory.reserved` / `inventory.failed`) |
| Payment | RabbitMQ | Async | AMQP (consumes `order.created`, publishes `payment.completed`) |
| Payment | Order | Sync | gRPC (`GetOrder` — read authoritative status and total) |
| Order | RabbitMQ | Async | AMQP (consumes `payment.completed` → status `pending` → `paid`) |

## Trust boundary

The Gateway is the only component reachable from untrusted clients. It
terminates JWT auth and rate limits. All internal traffic — gRPC between
Payment and Order, RabbitMQ events, Order's HTTP — stays inside the trust
boundary and does NOT pass through the Gateway.

In production this is enforced by network policy: Order's :9091 is only
reachable from Payment's pod, Order's :8081 only from the Gateway. On
localhost, everything is reachable, which is why the boundary is implicit.

## Ports

| Service | HTTP | gRPC |
|---------|------|------|
| Gateway | 8080 | — |
| Order | 8081 | 9091 |
| Payment | 8082 | — |
| Inventory | 8083 | — |
| Notification | 8084 | — |

## Order → Payment flow

```mermaid
sequenceDiagram
    Client->>Order: POST /orders
    Order->>DB: INSERT order (status=pending)
    Order-->>RabbitMQ: order.created
    RabbitMQ-->>Inventory: order.created
    Inventory->>DB: UPDATE inventory SET reserved (optimistic lock)
    alt insufficient stock
        Inventory-->>RabbitMQ: inventory.failed
    else reserved
        Inventory-->>RabbitMQ: inventory.reserved
    end
    RabbitMQ-->>Payment: order.created
    Payment->>Order: gRPC GetOrder(id)
    Order-->>Payment: {status=pending, total_cents}
    Payment->>DB: INSERT payment (status=succeeded)
    Payment-->>RabbitMQ: payment.completed
    RabbitMQ-->>Order: payment.completed
    Order->>DB: UPDATE orders SET status='paid'