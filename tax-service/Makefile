.PHONY: help build run clean test migrate-up migrate-down

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the tax service binary
	@echo "Building tax service..."
	@go build -o bin/tax-service ./cmd/main.go
	@echo "✓ Build complete: bin/tax-service"

run: ## Run the tax service
	@echo "Starting tax service..."
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
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub < migrations/001_create_tax_tables.sql
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub < migrations/002_seed_tax_data.sql
	@echo "✓ Migrations complete"

migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_reports CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_nexus CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_calculation_cache CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_exemption_certificates CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_rate_category_overrides CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS product_tax_categories CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_rates CASCADE;"
	@docker exec -i tesseract-postgres-local psql -U dev -d tesseract_hub -c "DROP TABLE IF EXISTS tax_jurisdictions CASCADE;"
	@echo "✓ Rollback complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t tax-service:latest .
	@echo "✓ Docker image built"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8091:8091 --env-file .env tax-service:latest

dev: ## Run in development mode with hot reload
	@echo "Starting development server..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found. Install it with: go install github.com/cosmtrek/air@latest"; \
		make run; \
	fi
