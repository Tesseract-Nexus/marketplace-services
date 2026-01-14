.PHONY: help build run clean test migrate-up migrate-down

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the payment service binary
	@echo "Building payment service..."
	@go build -o bin/payment-service ./cmd/main.go
	@echo "✓ Build complete: bin/payment-service"

run: ## Run the payment service
	@echo "Starting payment service..."
	@if [ -f .env ]; then \
		export $$(cat .env | xargs) && go run cmd/main.go; \
	else \
		go run cmd/main.go; \
	fi

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "✓ Clean complete"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies downloaded"

migrate-up: ## Run database migrations
	@echo "Running migrations..."
	@psql $(DATABASE_URL) -f migrations/001_create_payment_tables.sql
	@psql $(DATABASE_URL) -f migrations/002_seed_payment_data.sql
	@echo "✓ Migrations complete"

migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS payment_settings CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS payment_disputes CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS saved_payment_methods CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS payment_webhook_events CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS refund_transactions CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS payment_transactions CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS payment_gateway_configs CASCADE;"
	@echo "✓ Rollback complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t payment-service:latest .
	@echo "✓ Docker image built"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8092:8092 --env-file .env payment-service:latest

dev: ## Run in development mode with hot reload
	@echo "Starting development server..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found. Install it with: go install github.com/cosmtrek/air@latest"; \
		make run; \
	fi
