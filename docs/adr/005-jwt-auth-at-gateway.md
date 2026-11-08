# ADR-005: JWT auth at the gateway, not per-service

## Status
Accepted

## Context
External clients call the platform over REST. Internal services call each
other over gRPC and RabbitMQ. We need to authenticate users without
replicating auth logic in every service.

## Decision
- The Gateway validates a Bearer JWT (HS256) on every protected route.
- On success it injects `user_id` into the request context and forwards
  `X-User-Id` downstream.
- Internal services trust the gateway; they do not validate JWTs.
- `/login` issues a token for any username (simulated — a real system
  would verify credentials against a user store).

## Rationale
- One auth implementation, not five. A bug in one service's auth middleware
  becomes a bug in all of them.
- The gateway is the only component exposed to untrusted clients, so it is
  the natural place for the trust boundary.
- Stateless: no session store, no per-request DB lookup. The token carries
  the user identity.

## Consequences
- Internal services must not be reachable from outside the network, or
  they can be called without auth. Enforced by network policy in prod.
- Token revocation is not supported (tokens are valid until expiry, 24h).
  A real system would use short-lived tokens + a refresh endpoint, or a
  revocation list.
- `alg: none` attacks are blocked by the signing-method check in the
  middleware (`t.Method.(*jwt.SigningMethodHMAC)`).