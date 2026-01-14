# Orders Service

A comprehensive order management service built with Go, Gin, and GORM following clean architecture principles.

## Features

- **Complete Order Management**: Create, read, update, and delete orders
- **Order Status Tracking**: Track order lifecycle from pending to delivered
- **Order Cancellation & Refunds**: Support for order cancellations and partial/full refunds
- **Real-time Tracking**: Order timeline and shipping tracking
- **Comprehensive API**: RESTful API with full CRUD operations
- **Database Migrations**: Automatic schema migrations
- **Docker Support**: Containerized deployment
- **API Documentation**: Swagger/OpenAPI documentation

## Architecture

The service follows clean architecture principles with clear separation of concerns:

```
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration management
│   ├── handlers/               # HTTP handlers/controllers
│   ├── middleware/             # HTTP middleware
│   ├── models/                 # Domain models/entities
│   ├── repository/             # Data access layer
│   └── services/               # Business logic layer
├── migrations/                 # Database migrations
├── Dockerfile                  # Docker configuration
└── README.md                   # This file
```

## API Endpoints

All API endpoints are prefixed with `/api/v1`.

### Orders
- `POST /api/v1/orders` - Create a new order
- `GET /api/v1/orders` - List orders with filtering and pagination
- `GET /api/v1/orders/:id` - Get order by ID
- `GET /api/v1/orders/number/:orderNumber` - Get order by order number
- `PUT /api/v1/orders/:id` - Update order
- `PATCH /api/v1/orders/:id/status` - Update order status
- `POST /api/v1/orders/:id/cancel` - Cancel order
- `POST /api/v1/orders/:id/refund` - Process refund
- `GET /api/v1/orders/:id/tracking` - Get order tracking
- `POST /api/v1/orders/:id/tracking` - Add shipping tracking

### Returns & RMA
Complete return management with RMA (Return Merchandise Authorization) workflow.

#### Create & List Returns
- `POST /api/v1/returns` - Create new return request
- `GET /api/v1/returns` - List returns with filtering
- `GET /api/v1/returns/:id` - Get return by ID
- `GET /api/v1/returns/rma/:rma` - Get return by RMA number

#### Return Workflow Actions
- `POST /api/v1/returns/:id/approve` - Approve return request
- `POST /api/v1/returns/:id/reject` - Reject return request
- `POST /api/v1/returns/:id/in-transit` - Mark return as in transit
- `POST /api/v1/returns/:id/received` - Mark return as received
- `POST /api/v1/returns/:id/inspect` - Complete inspection
- `POST /api/v1/returns/:id/complete` - Complete return (issue refund)
- `POST /api/v1/returns/:id/cancel` - Cancel return

#### Return Policy & Stats
- `GET /api/v1/returns/policy` - Get return policy settings
- `PUT /api/v1/returns/policy` - Update return policy
- `GET /api/v1/returns/stats` - Get return statistics

### Health & Monitoring
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /metrics` - Prometheus metrics

## Environment Variables

```bash
# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=orders_db
DB_SSL_MODE=disable

# Application Configuration
APP_ENV=development
LOG_LEVEL=info
JWT_SECRET=your-secret-key
```

## Quick Start

### Local Development

1. **Clone and navigate to the service:**
   ```bash
   cd domains/ecommerce/services/orders-service
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Set up environment variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run the service:**
   ```bash
   go run cmd/main.go
   ```

5. **Access the API:**
   - Service: http://localhost:8080
   - Health: http://localhost:8080/health
   - Swagger: http://localhost:8080/swagger/index.html

### Docker

1. **Build the image:**
   ```bash
   docker build -t orders-service .
   ```

2. **Run with Docker Compose:**
   ```bash
   docker-compose up -d
   ```

## Database Schema

The service manages the following entities:

- **Orders**: Main order entity with customer, payment, shipping info
- **Order Items**: Individual items within an order
- **Order Customer**: Customer information for each order
- **Order Shipping**: Shipping address and tracking details
- **Order Payment**: Payment method and transaction details
- **Order Timeline**: Audit trail of order events
- **Order Discounts**: Applied coupons and discounts
- **Order Refunds**: Refund transactions and history
- **Returns**: Return/RMA records with status tracking
- **Return Items**: Individual items being returned
- **Return Policy**: Configurable return policy settings

## Order Lifecycle

```
PENDING → CONFIRMED → PROCESSING → SHIPPED → DELIVERED
    ↓         ↓           ↓
CANCELLED ← ─ ─ ─ ─ ─ ─ ─ ┘
    ↓
REFUNDED
```

## Return Lifecycle

```
PENDING → APPROVED → IN_TRANSIT → RECEIVED → INSPECTED → COMPLETED
    ↓                                 ↓           ↓
REJECTED                        INSPECTION_FAILED
    ↓                                             ↓
CANCELLED ← ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘
```

### Return States
| State | Description |
|-------|-------------|
| `pending` | Return request submitted, awaiting approval |
| `approved` | Return approved, awaiting item shipment |
| `rejected` | Return request rejected |
| `in_transit` | Item shipped back by customer |
| `received` | Item received at warehouse |
| `inspected` | Item inspected and verified |
| `inspection_failed` | Item failed inspection |
| `completed` | Refund processed, return complete |
| `cancelled` | Return cancelled by customer or system |

## Integration

This service integrates with:

- **Frontend**: Orders Hub MFE at `http://localhost:3004`
- **Admin Panel**: Ecommerce Admin at `http://localhost:3010`
- **API Contracts**: Shared TypeScript types via `@workspace/api-contracts`

## Development

### Adding New Features

1. **Models**: Add new entities in `internal/models/`
2. **Repository**: Extend data access in `internal/repository/`
3. **Services**: Add business logic in `internal/services/`
4. **Handlers**: Create HTTP endpoints in `internal/handlers/`
5. **Routes**: Register routes in `cmd/main.go`

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

## Deployment

The service is designed to be deployed in Kubernetes alongside other microservices in the tesseract-hub ecosystem.

### Production Checklist

- [ ] Configure production database
- [ ] Set up proper JWT secrets
- [ ] Configure CORS for production domains
- [ ] Set up monitoring and logging
- [ ] Configure resource limits
- [ ] Set up health checks

## Monitoring

The service exposes:

- Health check endpoint for liveness/readiness probes
- Request logging with correlation IDs
- Error tracking and recovery middleware
- Database connection monitoring

## Security

- CORS protection for cross-origin requests
- Request ID tracking for audit trails
- Input validation on all endpoints
- SQL injection protection via GORM
- Panic recovery middleware
