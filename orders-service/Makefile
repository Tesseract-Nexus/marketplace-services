.PHONY: build run clean test

# Build the service
build:
	@echo "Building orders-service..."
	@go build -o bin/orders-service cmd/main.go

# Run the service
run:
	@echo "Starting orders-service on port 8080..."
	@go run cmd/main.go

# Run with custom port
run-port:
	@echo "Starting orders-service on port $(PORT)..."
	@SERVER_PORT=$(PORT) go run cmd/main.go

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	@go test ./... -v

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@go mod download

# Database migrations
migrate-up:
	@echo "Running database migrations..."
	@echo "Migrations run automatically on service start"

# Run with docker-compose
docker-up:
	@echo "Starting orders-service with docker-compose..."
	@docker-compose up -d

docker-down:
	@echo "Stopping orders-service docker containers..."
	@docker-compose down

docker-logs:
	@echo "Showing orders-service logs..."
	@docker-compose logs -f

# Development
dev: deps run

# Help
help:
	@echo "Orders Service - Available commands:"
	@echo "  make build       - Build the service binary"
	@echo "  make run         - Run the service (default port 8080)"
	@echo "  make run-port    - Run with custom port (make run-port PORT=8081)"
	@echo "  make test        - Run tests"
	@echo "  make deps        - Install dependencies"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start with docker-compose"
	@echo "  make docker-down - Stop docker containers"
	@echo "  make docker-logs - Show docker logs"
	@echo "  make dev         - Install deps and run"
