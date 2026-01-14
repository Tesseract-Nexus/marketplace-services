# Staff Service

Enterprise staff management service with multi-tenant support, built with Go, PostgreSQL, and comprehensive API endpoints.

## Features

- ✅ **Complete CRUD Operations** - Create, Read, Update, Delete staff members
- ✅ **Multi-tenant Architecture** - Isolated data per tenant
- ✅ **Advanced Filtering** - Filter by role, department, employment type, etc.
- ✅ **Search Functionality** - Full-text search across staff data
- ✅ **Bulk Operations** - Bulk create and update staff members
- ✅ **Analytics & Reporting** - Staff analytics and export functionality
- ✅ **Organizational Hierarchy** - Manager-employee relationships
- ✅ **Soft Deletes** - Data retention with soft delete capability
- ✅ **PostgreSQL Integration** - Optimized database schema with indexes
- ✅ **Docker Support** - Container-ready with docker-compose
- ✅ **API Documentation** - Swagger/OpenAPI documentation
- ✅ **Comprehensive Testing** - End-to-end endpoint testing

## API Endpoints

### Core Staff Management
- `POST /api/v1/staff` - Create staff member
- `GET /api/v1/staff/{id}` - Get staff by ID
- `PUT /api/v1/staff/{id}` - Update staff member
- `DELETE /api/v1/staff/{id}` - Delete staff member (soft delete)
- `GET /api/v1/staff` - List staff with filtering and pagination

### Advanced Features
- `POST /api/v1/staff/bulk` - Bulk create staff members
- `PUT /api/v1/staff/bulk` - Bulk update staff members
- `POST /api/v1/staff/export` - Export staff data
- `GET /api/v1/staff/analytics` - Get staff analytics
- `GET /api/v1/staff/hierarchy` - Get organizational hierarchy

### Health & Monitoring
- `GET /api/v1/health` - Health check

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.21+ (for local development)
- jq (for testing script)

### Using Docker (Recommended)

1. **Start the services:**
```bash
docker-compose up -d
```

This will start:
- PostgreSQL database on port 5432
- Staff service API on port 8080
- Automatic database migration

2. **Test the API:**
```bash
./test_endpoints.sh
```

3. **View API documentation:**
```
http://localhost:8080/swagger/index.html
```

### Local Development

1. **Start PostgreSQL:**
```bash
docker-compose up postgres -d
```

2. **Run migrations:**
```bash
docker-compose run --rm migrate
```

3. **Start the service:**
```bash
go run cmd/main.go
```

## Configuration

The service can be configured using environment variables:

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=staff_db
DB_SSL_MODE=disable

# Server
PORT=8080
ENVIRONMENT=development

# JWT
JWT_SECRET=your-jwt-secret-key

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Database Schema

The service uses PostgreSQL with the following key features:

- **UUID primary keys** for global uniqueness
- **Tenant isolation** with composite indexes
- **JSONB fields** for flexible data storage (skills, certifications, custom fields)
- **Enum types** for roles and employment types
- **Soft deletes** with timestamp tracking
- **Optimized indexes** for performance
- **Automatic timestamp updates** via triggers

## API Usage Examples

### Create a Staff Member
```bash
curl -X POST http://localhost:8080/api/v1/staff \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: your-tenant" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@company.com",
    "role": "employee",
    "employment_type": "full_time",
    "department_id": "IT"
  }'
```

### Get Staff List with Filtering
```bash
curl "http://localhost:8080/api/v1/staff?role=manager&is_active=true&page=1&limit=20" \
  -H "X-Tenant-ID: your-tenant"
```

### Search Staff
```bash
curl "http://localhost:8080/api/v1/staff?search=john" \
  -H "X-Tenant-ID: your-tenant"
```

### Get Analytics
```bash
curl http://localhost:8080/api/v1/staff/analytics \
  -H "X-Tenant-ID: your-tenant"
```

## Testing

### Automated Testing
Run the comprehensive test script:
```bash
./test_endpoints.sh
```

This tests all endpoints including:
- ✅ Health check
- ✅ CRUD operations
- ✅ Filtering and pagination
- ✅ Search functionality
- ✅ Bulk operations
- ✅ Analytics
- ✅ Error handling
- ✅ Data validation

### Manual Testing
Use the Swagger UI at `http://localhost:8080/swagger/index.html` for interactive API testing.

## Data Model

### Staff Entity
```go
type Staff struct {
    ID              uuid.UUID       // Primary key
    TenantID        string          // Multi-tenant isolation
    FirstName       string          // Required
    LastName        string          // Required
    Email           string          // Required, unique per tenant
    EmployeeID      *string         // Optional, unique per tenant
    Role            StaffRole       // Enum: admin, manager, employee, etc.
    EmploymentType  EmploymentType  // Enum: full_time, part_time, etc.
    DepartmentID    *string         // Department reference
    ManagerID       *uuid.UUID      // Self-referential for hierarchy
    Skills          *JSON           // JSONB array of skills
    Certifications  *JSON           // JSONB array of certifications
    IsActive        bool            // Soft enable/disable
    // ... additional fields for comprehensive staff management
}
```

## Multi-Tenant Support

The service supports multi-tenancy through:
- **Tenant ID header** (`X-Tenant-ID`)
- **Data isolation** at the database level
- **Tenant-scoped queries** for all operations
- **Unique constraints** scoped to tenant

## Performance Optimizations

- **Database indexes** on frequently queried fields
- **Composite indexes** for tenant + field combinations
- **JSONB GIN indexes** for flexible field searching
- **Pagination** to limit result sets
- **Bulk operations** for efficient batch processing

## Development

### Project Structure
```
staff-service/
├── cmd/                    # Application entry points
├── internal/
│   ├── config/            # Configuration management
│   ├── handlers/          # HTTP handlers
│   ├── middleware/        # HTTP middleware
│   ├── models/           # Data models
│   └── repository/       # Data access layer
├── migrations/           # Database migrations
├── docs/                # API documentation
├── docker-compose.yml   # Local development setup
├── Dockerfile          # Container definition
└── test_endpoints.sh   # E2E testing script
```

### Adding New Features

1. **Update the model** in `internal/models/staff.go`
2. **Add repository methods** in `internal/repository/staff_repository.go`
3. **Create handlers** in `internal/handlers/staff_handlers.go`
4. **Add routes** in `cmd/main.go`
5. **Update tests** in `test_endpoints.sh`

## Deployment

The service is containerized and can be deployed using:

- **Docker Compose** (development)
- **Kubernetes** (production)
- **AWS ECS/Fargate**
- **Google Cloud Run**
- **Azure Container Instances**

## Monitoring

The service provides:
- **Health check endpoint** for load balancer health checks
- **Structured logging** with request tracing
- **Metrics endpoints** (can be extended with Prometheus)
- **Database connection monitoring**

## Security

- **Input validation** on all endpoints
- **SQL injection protection** via GORM
- **Tenant isolation** at data level
- **CORS configuration** for web clients
- **JWT token support** (extendable)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Run the test suite
5. Submit a pull request

## License

MIT License - see LICENSE file for details.