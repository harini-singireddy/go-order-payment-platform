# ADR-001: Use RabbitMQ over Kafka

## Status
Accepted

## Context
We need async communication between Order, Payment, Inventory, and Notification.
Business events: OrderCreated, PaymentCompleted, InventoryFailed.

## Decision
Use RabbitMQ.

## Rationale
- Order volume is thousands, not millions — Kafka's throughput advantage is unused.
- Events require flexible routing: OrderCreated goes to both Inventory and Payment.
  RabbitMQ's Exchange/Queue binding models this naturally; Kafka requires separate topics.
- Lower operational complexity for local development with Docker Compose.
- No event replay requirement for this project scope.

## Consequences
- If throughput reaches 10k+/sec, re-evaluate Kafka.
- No event replay capability (acceptable here).
