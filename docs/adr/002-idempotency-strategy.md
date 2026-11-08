# ADR-002: Idempotency via DB unique constraint, not Redis

## Status
Accepted

## Context
POST /orders must be idempotent: the same Idempotency-Key must not create
two orders. Similarly, payment.completed events may be redelivered by
RabbitMQ on ack failure, and must not create two payments for one order.

## Decision
- `orders.idempotency_key` has a UNIQUE constraint.
- `payments.order_id` has a UNIQUE constraint.
- On duplicate insert, the service catches PostgreSQL error 23505
  (unique_violation) and returns the existing row instead of erroring.

## Rationale
- The DB is the source of truth. A Redis lock can be bypassed if the
  process crashes between the Redis write and the DB insert; the DB
  constraint cannot be bypassed.
- No extra infrastructure in the critical path — Postgres already
  enforces the invariant.
- Redis will be added later as a cache of "recently seen keys" to avoid
  the round-trip, but the DB constraint remains the correctness guarantee.

## Consequences
- A duplicate request incurs a DB round-trip before returning the existing
  row. Acceptable at this scale.
- If we ever partition the DB by customer, the idempotency key must
  include the partition key.
