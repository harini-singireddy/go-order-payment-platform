# ADR-004: Separate HTTP and gRPC ports per service

## Status
Accepted

## Context
Order exposes REST (for the Gateway) and gRPC (for Payment). Both run in
the same process. Inventory exposes only HTTP. Payment exposes only HTTP
but acts as a gRPC client.

## Decision
- HTTP on 80xx (Order: 8081, Payment: 8082, Inventory: 8083, Gateway: 8080).
- gRPC on 90xx (Order: 9091).
- Same last two digits for the same service.

## Rationale
- Distinct listeners are simpler to debug: `curl :8081`, `grpcurl :9091`.
- Each protocol has its own timeouts and shutdown semantics; one can be
  stopped without affecting the other.
- cmux (multiplexing on one port) is a valid alternative but adds
  complexity that is not justified at this scale.

## Consequences
- docker-compose and Kubernetes manifests must expose two ports per
  service that serves gRPC.
- Prometheus scrape config targets the HTTP port only; gRPC metrics would
  be added via grpc-prometheus interceptor if needed.
