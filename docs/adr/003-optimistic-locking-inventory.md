# ADR-003: Optimistic locking for inventory reservation

## Status
Accepted

## Context
Two orders may race for the last unit of a product. We must never oversell.

## Decision
Reserve stock with a conditional UPDATE:

    UPDATE inventory
    SET reserved = reserved + $1, updated_at = now()
    WHERE product_id = $2 AND stock - reserved >= $1

If RowsAffected() == 0, the reservation failed (insufficient stock).

## Rationale
- No row-level lock is held across a network call (unlike SELECT FOR UPDATE
  followed by application logic), so the transaction is short.
- The invariant (stock - reserved >= 0) is encoded in the WHERE clause.
  PostgreSQL's atomicity guarantees that only one of two concurrent UPDATEs
  matching the same row can succeed.
- No distributed lock needed — one DB, one query.

## Consequences
- Requires the CHECK (stock >= 0) constraint to hold even under drift;
  it is included in the migration.
- On failure in a multi-item order, partial reservations must be released.
  This is handled by re-issuing Release for each already-reserved item in
  the failure path.
