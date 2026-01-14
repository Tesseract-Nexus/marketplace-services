.PHONY: help build run test clean migrate seed

help:
	@echo "Available commands:"
	@echo "  make build    - Build the service"
	@echo "  make run      - Run the service"
	@echo "  make test     - Run tests"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make migrate  - Run database migrations"
	@echo "  make seed     - Seed database with sample data"

build:
	@echo "Building customers-service..."
	@go build -o bin/customers-service ./cmd/main.go

run:
	@echo "Running customers-service..."
	@go run ./cmd/main.go

test:
	@echo "Running tests..."
	@go test -v ./...

clean:
	@echo "Cleaning..."
	@rm -rf bin/

migrate:
	@echo "Running migrations..."
	@PGPASSWORD=devpass psql -h localhost -U dev -d tesseract_hub -p 5433 -f migrations/001_create_customers_tables.sql

seed:
	@echo "Seeding database..."
	@PGPASSWORD=devpass psql -h localhost -U dev -d tesseract_hub -p 5433 -f migrations/002_seed_customers.sql
