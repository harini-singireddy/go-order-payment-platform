# Architecture

## Service Responsibilities

| Service | Does | Does NOT do |
|---------|------|-------------|
| Gateway | JWT validation, routing, rate limiting | Business logic |
| Order | Create/query orders, state machine | Process payments |
| Payment | Simulate payment, idempotency | Manage order state |
| Inventory | Deduct/release stock | Know about payments |
| Notification | Send email on PaymentCompleted event | Block any flow |

## Communication

| From | To | Pattern | Protocol |
|------|-----|---------|----------|
| Client | Gateway | Sync | REST |
| Gateway | Order | Sync | gRPC |
| Order | RabbitMQ | Async | AMQP (publishes OrderCreated) |
| Inventory | RabbitMQ | Async | AMQP (consumes OrderCreated, publishes InventoryFailed) |
| Payment | RabbitMQ | Async | AMQP (consumes OrderCreated, publishes PaymentCompleted) |
| Notification | RabbitMQ | Async | AMQP (consumes PaymentCompleted) |

## Data Ownership

Each service has its own PostgreSQL schema. No cross-schema joins.
Cross-service data flows through gRPC calls or RabbitMQ events.

- `order` schema: `orders`, `order_items`
- `payment` schema: `payments`
- `inventory` schema: `inventory`
