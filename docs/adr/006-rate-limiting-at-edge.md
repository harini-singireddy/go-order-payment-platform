# ADR-006: Rate limiting at the edge

## Status
Accepted

## Context
Public endpoints are vulnerable to abuse (scraping, brute force, accidental
loops). We need a limit that protects the system without adding a dependency.

## Decision
- Per-IP token-bucket rate limiter in the Gateway.
- 10 requests/sec sustained, burst 20.
- Implemented with `golang.org/x/time/rate`, in-memory map keyed by IP.

## Rationale
- Rate limiting belongs at the edge — that is where untrusted traffic enters.
  Limiting inside a service means the traffic has already crossed the
  network, so you are limiting your own resources, not the attacker's.
- In-memory is sufficient for a single-instance gateway. A multi-instance
  gateway would need Redis-backed counters (or a service mesh's rate-limit
  filter) to share state.
- `rate.Allow()` is non-blocking — correct for HTTP middleware. Blocking
  would tie up goroutines under load.

## Consequences
- Per-IP limits are defeated by distributed attacks (many IPs). A real
  system would add per-user and per-token limits on top.
- The limiter map grows with the number of distinct IPs. For a demo this
  is fine; production would need TTL-based eviction.
- `/healthz` currently shares the limit. In production, exempt health
  checks so monitoring is not affected by user traffic.