# Testing Guide

How to verify the system works, from individual containers to the full event chain.

## 1. All containers healthy

    docker compose -f deployments/docker-compose.yml ps

Expected: nine containers `Up`. `postgres`, `redis`, `rabbitmq` show `(healthy)`.

If any app container is `Exited`, check its logs:

    docker logs opp-<service> 2>&1 | tail -30

## 2. Each component individually

### Postgres

    docker exec -it opp-postgres psql -U opp -d orders_db -c "\dt"
    docker exec -it opp-postgres psql -U opp -d payments_db -c "\dt"
    docker exec -it opp-postgres psql -U opp -d inventory_db -c "\dt"

Expected: `orders`, `order_items` / `payments` / `inventory`.

### Redis

    docker exec -it opp-redis redis-cli ping

Expected: `PONG`.

### RabbitMQ

Open <http://localhost:15672> (opp / opp_dev_password).

Expected exchanges: `orders`, `payments`, `inventory`.

Expected queues and bindings:

| Queue | Exchange | Routing key |
|-------|----------|-------------|
| inventory.order.created | orders | order.created |
| payment.order.created | orders | order.created |
| order.payment.completed | payments | payment.completed |
| notification.payment.completed | payments | payment.completed |

### Prometheus

    curl -s localhost:9090/api/v1/targets | python3 -c '
    import sys,json
    for t in json.load(sys.stdin)["data"]["activeTargets"]:
        print(t["labels"]["job"], t["health"])'

Expected: five jobs (`gateway`, `order`, `payment`, `inventory`, `notification`), all `up`.

### Grafana

Open <http://localhost:3000> (admin / admin). Dashboard **OPP Overview** should show four panels with data.

### /healthz on each service

    curl -s localhost:8080/healthz   # gateway
    curl -s localhost:8081/healthz   # order
    curl -s localhost:8082/healthz   # payment
    curl -s localhost:8083/healthz   # inventory
    curl -s localhost:8084/healthz   # notification

Expected: `{"status":"ok"}` from each.

### /metrics on each service

    curl -s localhost:8081/metrics | grep -c '^opp_'

Expected: a non-zero count (dozens of lines).

## 3. End-to-end happy path

    TOKEN=$(curl -s -X POST localhost:8080/login \
      -H 'Content-Type: application/json' \
      -d '{"username":"alice"}' \
      | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')

    curl -s -X POST localhost:8080/orders \
      -H "Authorization: Bearer $TOKEN" \
      -H 'Content-Type: application/json' \
      -H 'Idempotency-Key: test-happy-1' \
      -d '{"customer_id":"c1","items":[{"product_id":"prod-1","quantity":1,"price_cents":1000}]}'

    sleep 2

    docker exec -it opp-postgres psql -U opp -d orders_db \
      -c "SELECT status FROM orders WHERE idempotency_key='test-happy-1';"

Expected: `paid`.

    docker exec -it opp-postgres psql -U opp -d payments_db \
      -c "SELECT amount_cents, status FROM payments WHERE order_id = (SELECT id FROM orders WHERE idempotency_key='test-happy-1');"

Expected: `1000 | succeeded`.

    docker logs opp-notification 2>&1 | tail -1

Expected: a line containing `"sending email"` and the order ID.

## 4. Failure and edge cases

### 4.1 Auth: no token → 401

    curl -i localhost:8080/orders/00000000-0000-0000-0000-000000000000

Expected: `HTTP/1.1 401 Unauthorized`.

### 4.2 Idempotency: same key twice → same order, 200 the second time

Repeat the POST from section 3 with the same `Idempotency-Key: test-happy-1`.

Expected: `HTTP/1.1 200 OK` (not 201), same `id` in the body.

    docker exec -it opp-postgres psql -U opp -d orders_db \
      -c "SELECT count(*) FROM orders WHERE idempotency_key='test-happy-1';"

Expected: `1`.

### 4.3 Validation: empty items → 400

    curl -i -X POST localhost:8080/orders \
      -H "Authorization: Bearer $TOKEN" \
      -H 'Content-Type: application/json' \
      -H 'Idempotency-Key: test-empty-1' \
      -d '{"customer_id":"c1","items":[]}'

Expected: `400 Bad Request`, `{"error":"order must contain at least one item"}`.

### 4.4 Oversell: reserve more than stock → inventory.failed

`prod-2` is seeded with stock 5. Order 5, then 1 more:

    curl -s -X POST localhost:8080/orders \
      -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
      -H 'Idempotency-Key: test-oversell-1' \
      -d '{"customer_id":"c1","items":[{"product_id":"prod-2","quantity":5,"price_cents":100}]}'

    sleep 1

    curl -s -X POST localhost:8080/orders \
      -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
      -H 'Idempotency-Key: test-oversell-2' \
      -d '{"customer_id":"c1","items":[{"product_id":"prod-2","quantity":1,"price_cents":100}]}'

    sleep 1

    docker exec -it opp-postgres psql -U opp -d inventory_db \
      -c "SELECT stock, reserved FROM inventory WHERE product_id='prod-2';"

Expected: `stock = 5`, `reserved = 5` (the second attempt could not reserve).

    docker logs opp-inventory 2>&1 | grep -i 'inventory failed' | tail -1

Expected: a log line with the second order ID and reason `insufficient stock`.

### 4.5 Rate limit: rapid requests → some 429

    for i in $(seq 1 30); do
      curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/healthz
    done | sort | uniq -c

Expected: a mixture of `200` and `429` (roughly 20/10 with burst 20, 10 rps).

### 4.6 Dependency down: Order stopped → gateway returns 502

    docker stop opp-order
    curl -i localhost:8080/orders/x -H "Authorization: Bearer $TOKEN"
    docker start opp-order

Expected: `502 Bad Gateway` while Order is down.

## 5. Cleanup

    docker compose -f deployments/docker-compose.yml down

To also remove data volumes (fresh start on next `up`):

    docker compose -f deployments/docker-compose.yml down -v
