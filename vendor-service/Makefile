.PHONY: help build run test clean docker-build docker-run docker-stop migrate-up migrate-down swagger

# Variables
SERVICE_NAME := vendor-service
DOCKER_IMAGE := tesseract-hub/$(SERVICE_NAME)
DOCKER_TAG := latest
POSTGRES_CONTAINER := tesseract-vendor-postgres
API_CONTAINER := tesseract-vendor-api
MIGRATE_CONTAINER := tesseract-vendor-migrate

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Show this help message
	@echo '$(GREEN)Vendor Service Makefile$(NC)'
	@echo ''
	@echo 'Usage:'
	@echo '  make $(YELLOW)<target>$(NC)'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the vendor service binary
	@echo "$(GREEN)Building vendor service...$(NC)"
	go build -o bin/$(SERVICE_NAME) ./cmd/main.go

run: ## Run the vendor service locally
	@echo "$(GREEN)Running vendor service...$(NC)"
	go run ./cmd/main.go

test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f Dockerfile ../../../..

docker-run: ## Run service with docker-compose
	@echo "$(GREEN)Starting vendor service with docker-compose...$(NC)"
	docker-compose up -d

docker-stop: ## Stop docker-compose services
	@echo "$(GREEN)Stopping docker-compose services...$(NC)"
	docker-compose down

docker-logs: ## View docker-compose logs
	@echo "$(GREEN)Viewing logs...$(NC)"
	docker-compose logs -f

docker-clean: docker-stop ## Stop and remove all containers and volumes
	@echo "$(RED)Removing all containers and volumes...$(NC)"
	docker-compose down -v

migrate-up: ## Run database migrations up
	@echo "$(GREEN)Running migrations up...$(NC)"
	docker-compose run --rm migrate

migrate-down: ## Run database migrations down
	@echo "$(YELLOW)Running migrations down...$(NC)"
	docker-compose run --rm migrate -path /migrations -database "postgres://vendor_user:vendor_password_dev@postgres:5432/vendor_db?sslmode=disable" down

migrate-create: ## Create a new migration (usage: make migrate-create name=create_new_table)
	@echo "$(GREEN)Creating new migration: $(name)...$(NC)"
	migrate create -ext sql -dir ./migrations -seq $(name)

swagger: ## Generate swagger documentation
	@echo "$(GREEN)Generating swagger documentation...$(NC)"
	swag init -g ./cmd/main.go -o ./docs

lint: ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	golangci-lint run

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...

deps: ## Download dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	go mod download
	go mod tidy

dev: ## Run in development mode with hot reload
	@echo "$(GREEN)Running in development mode...$(NC)"
	air

db-shell: ## Connect to database shell
	@echo "$(GREEN)Connecting to database...$(NC)"
	docker exec -it $(POSTGRES_CONTAINER) psql -U vendor_user -d vendor_db

api-shell: ## Connect to API container shell
	@echo "$(GREEN)Connecting to API container...$(NC)"
	docker exec -it $(API_CONTAINER) /bin/sh

health-check: ## Check service health
	@echo "$(GREEN)Checking service health...$(NC)"
	curl -f http://localhost:8081/health || echo "$(RED)Service is not healthy$(NC)"