# Reviews Service

Enterprise reviews management service with multi-tenant support, ML-based spam detection, and comprehensive analytics.

## Features

- **Multi-tenant architecture** with proper data isolation
- **Comprehensive review management** - Create, read, update, delete reviews
- **Advanced moderation** - Status management, bulk operations, spam detection
- **Media support** - Images, videos, and files attached to reviews
- **Reactions and comments** - User engagement features
- **Analytics and reporting** - Detailed insights and trends
- **ML integration** - Spam detection and sentiment analysis
- **Search and filtering** - Advanced search capabilities
- **Export functionality** - Data export in multiple formats

## API Endpoints

### Core CRUD Operations
- `POST /api/v1/reviews` - Create a new review
- `GET /api/v1/reviews` - List reviews with filtering and pagination
- `GET /api/v1/reviews/{id}` - Get a specific review
- `PUT /api/v1/reviews/{id}` - Update a review
- `DELETE /api/v1/reviews/{id}` - Delete a review

### Moderation Operations
- `PUT /api/v1/reviews/{id}/status` - Update review status
- `POST /api/v1/reviews/bulk/status` - Bulk status updates
- `POST /api/v1/reviews/bulk/moderate` - Bulk moderation operations

### Media Operations
- `POST /api/v1/reviews/{id}/media` - Add media to review
- `DELETE /api/v1/reviews/{id}/media/{mediaId}` - Remove media

### Reactions and Comments
- `POST /api/v1/reviews/{id}/reactions` - Add reaction
- `DELETE /api/v1/reviews/{id}/reactions/{reactionId}` - Remove reaction
- `POST /api/v1/reviews/{id}/comments` - Add comment
- `PUT /api/v1/reviews/{id}/comments/{commentId}` - Update comment
- `DELETE /api/v1/reviews/{id}/comments/{commentId}` - Delete comment

### Analytics and Reporting
- `GET /api/v1/reviews/analytics` - Get analytics data
- `GET /api/v1/reviews/stats` - Get statistics
- `POST /api/v1/reviews/export` - Export reviews data

### ML and Advanced Features
- `POST /api/v1/reviews/{id}/report` - Report a review
- `POST /api/v1/reviews/{id}/moderate/ai` - AI-powered moderation
- `GET /api/v1/reviews/similar/{id}` - Find similar reviews
- `POST /api/v1/reviews/search` - Advanced search
- `GET /api/v1/reviews/trending` - Get trending reviews

## Database Schema

The service uses PostgreSQL with JSONB columns for flexible data storage:

- **reviews** - Main reviews table with full-text search capabilities
- Indexes optimized for multi-tenant queries and common filtering patterns
- JSONB columns for ratings, comments, reactions, media, tags, and metadata

## Configuration

Environment variables:

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=reviews_db
DB_SSL_MODE=disable

# Server
PORT=8084
ENVIRONMENT=development

# Authentication
JWT_SECRET=your-secret-key

# External Services
ML_SERVICE_URL=http://localhost:8090
MEDIA_SERVICE_URL=http://localhost:8091

# Review Settings
MAX_REVIEW_LENGTH=5000
MAX_MEDIA_PER_REVIEW=10
SPAM_DETECTION_THRESHOLD=0.8
AUTO_MODERATION_ENABLED=true

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Development

1. **Start the database:**
   ```bash
   docker-compose up -d reviews-db
   ```

2. **Run migrations:**
   ```bash
   # Migrations will be applied automatically on startup
   ```

3. **Start the service:**
   ```bash
   go run cmd/main.go
   ```

4. **Access Swagger documentation:**
   ```
   http://localhost:8084/swagger/index.html
   ```

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
go test ./... -tags=integration

# Test coverage
go test -cover ./...
```

## Deployment

### Docker
```bash
# Build image
docker build -t reviews-service .

# Run with docker-compose
docker-compose up
```

### Kubernetes
Deploy using the provided manifests in the `k8s/` directory.

## Architecture

The service follows clean architecture principles:

- **cmd/** - Application entry points
- **internal/config/** - Configuration management
- **internal/models/** - Domain models and DTOs
- **internal/repository/** - Data access layer
- **internal/handlers/** - HTTP handlers and API logic
- **internal/middleware/** - HTTP middleware (auth, CORS, etc.)
- **migrations/** - Database migrations

## Security

- Multi-tenant data isolation
- JWT-based authentication
- Azure AD integration support
- Rate limiting and spam protection
- Input validation and sanitization
- SQL injection prevention with GORM

## Monitoring

- Health check endpoints (`/health`, `/ready`)
- Structured logging with contextual information
- Metrics collection ready for Prometheus
- Error tracking and alerting integration points

## Performance

- Database indexes optimized for common query patterns
- JSONB indexes for complex filtering
- Connection pooling and query optimization
- Caching integration points for high-traffic scenarios