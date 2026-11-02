build:
	@for dir in services/*/; do echo "Building $$dir..."; (cd $$dir \&\& go build -o /dev/null ./...); done

test:
	go test ./...

lint:
	golangci-lint run ./...

run-all:
	@echo "Run services in separate terminals (placeholder)"

migrate:
	@echo "Run migrations (placeholder)"

.PHONY: build test lint run-all migrate
