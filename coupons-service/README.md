# Coupons Service

A comprehensive microservice for managing promotional coupons with multi-tenant support.

## Features

- **Complete Coupon Management**: Create, update, delete, and manage promotional coupons
- **Advanced Targeting**: Support for category, product, user group, and geographic targeting
- **Usage Tracking**: Real-time usage monitoring and analytics
- **Flexible Discounts**: Percentage, fixed amount, buy-x-get-y, and free shipping discounts
- **Multi-tenant Architecture**: Isolated data per tenant
- **Authentication**: JWT and Azure AD support
- **Validation Engine**: Comprehensive coupon validation logic
- **Analytics**: Usage statistics and reporting

## API Documentation

The service provides RESTful APIs with Swagger documentation available at `/swagger/index.html` when running.

### Key Endpoints

- `POST /api/v1/coupons` - Create coupon
- `GET /api/v1/coupons` - List coupons with filters
- `GET /api/v1/coupons/:id` - Get specific coupon
- `PUT /api/v1/coupons/:id` - Update coupon
- `DELETE /api/v1/coupons/:id` - Delete coupon
- `POST /api/v1/coupons/validate` - Validate coupon
- `POST /api/v1/coupons/:id/apply` - Apply coupon
- `GET /api/v1/coupons/analytics` - Get analytics

## Docker Setup

### Using Docker Compose (Recommended)

```bash
# Start all services (PostgreSQL + API + Migrations)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Manual Docker Build

```bash
# Build the image
docker build -t coupons-service .

# Run with external PostgreSQL
docker run -p 8082:8082 \
  -e DB_HOST=localhost \
  -e DB_NAME=coupons_db \
  -e DB_USER=postgres \
  -e DB_PASSWORD=password \
  coupons-service
```

## Local Development

### Prerequisites

- Go 1.23+
- PostgreSQL 16+
- Git

### Setup

```bash
# Clone the repository
git clone <repository-url>
cd coupons-service

# Install dependencies
go mod download

# Set up environment variables
cp .env.example .env

# Run database migrations
migrate -path ./migrations -database "postgres://postgres:password@localhost:5432/coupons_db?sslmode=disable" up

# Run the service
go run cmd/main.go
```

### Environment Variables

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=coupons_db
DB_SSL_MODE=disable

# Server
PORT=8082
ENVIRONMENT=development

# Authentication
JWT_SECRET=your-jwt-secret-key
AZURE_TENANT_ID=your-azure-tenant-id
AZURE_APPLICATION_ID=your-azure-app-id

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Database Schema

The service uses PostgreSQL with the following main tables:

- `coupons` - Main coupon data with JSONB fields for flexible targeting
- `coupon_usage` - Usage tracking and audit trail

Key features:
- JSONB columns for flexible data storage
- GIN indexes for efficient querying
- Triggers for automatic timestamp updates
- Foreign key constraints for data integrity

## Architecture

```
coupons-service/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── handlers/            # HTTP handlers
│   ├── middleware/          # Authentication & CORS
│   ├── models/              # Data models
│   └── repository/          # Database operations
├── migrations/              # Database migrations
├── Dockerfile               # Container definition
├── docker-compose.yml       # Local development setup
└── go.mod                   # Go dependencies
```

## Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Test endpoints (after starting service)
curl http://localhost:8082/health
```

## Production Deployment

1. **Environment**: Set `ENVIRONMENT=production`
2. **Database**: Use managed PostgreSQL service
3. **Authentication**: Configure Azure AD properly
4. **Monitoring**: Monitor `/health` endpoint
5. **Logging**: Configure structured logging
6. **SSL**: Use HTTPS in production

## Contributing

1. Follow Go coding standards
2. Add tests for new features
3. Update documentation
4. Use conventional commit messages