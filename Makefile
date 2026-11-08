.PHONY: build test lint migrate run-order run-inventory run-payment up down

build:
	@for dir in pkg proto services/*/; do \
		echo "==> $$dir"; \
		(cd $$dir && go build ./...) || exit 1; \
	done
	@echo "✅ all modules built"

test:
	go test ./...

lint:
	golangci-lint run ./...

up:
	docker compose -f deployments/docker-compose.yml up -d

down:
	docker compose -f deployments/docker-compose.yml down

migrate:
	docker exec -i opp-postgres psql -U opp -d orders_db    < services/order/migrations/0001_create_orders.up.sql
	docker exec -i opp-postgres psql -U opp -d payments_db  < services/payment/migrations/0001_create_payments.up.sql
	docker exec -i opp-postgres psql -U opp -d inventory_db < services/inventory/migrations/0001_create_inventory.up.sql
	@echo "✅ migrations applied"

seed:
	docker exec -it opp-postgres psql -U opp -d inventory_db -c \
		"INSERT INTO inventory (product_id, stock) VALUES ('prod-1', 10), ('prod-2', 5) ON CONFLICT DO NOTHING;"

run-order:
	cd services/order && go run ./cmd

run-inventory:
	cd services/inventory && go run ./cmd

run-payment:
	cd services/payment && go run ./cmd