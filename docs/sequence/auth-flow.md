# Auth flow

```mermaid
sequenceDiagram
    autonumber
    Client->>Gateway: POST /login {username}
    Gateway-->>Client: {token} (HS256, 24h)
    Client->>Gateway: GET /orders/{id} Authorization: Bearer <token>
    Gateway->>Gateway: verify signature + expiry
    Gateway->>Order: GET /orders/{id} (X-User-Id: alice)
    Order-->>Gateway: 200 {order}
    Gateway-->>Client: 200 {order}