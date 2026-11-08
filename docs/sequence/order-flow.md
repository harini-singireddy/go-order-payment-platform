# Order flow (happy path + inventory failure)

## Happy path: order reaches `paid`

```mermaid
sequenceDiagram
    autonumber
    Client->>Order: POST /orders (Idempotency-Key)
    Order->>DB: INSERT order (pending) [UNIQUE idempotency_key]
    Order-->>RabbitMQ: publish order.created
    RabbitMQ-->>Inventory: deliver order.created
    Inventory->>DB: UPDATE inventory SET reserved [optimistic lock]
    Inventory-->>RabbitMQ: publish inventory.reserved
    RabbitMQ-->>Payment: deliver order.created
    Payment->>Order: gRPC GetOrder(id)
    Order-->>Payment: {status=pending, total_cents}
    Payment->>DB: INSERT payment (succeeded) [UNIQUE order_id]
    Payment-->>RabbitMQ: publish payment.completed
    RabbitMQ-->>Order: deliver payment.completed
    Order->>DB: UPDATE orders SET status='paid' WHERE status='pending'
