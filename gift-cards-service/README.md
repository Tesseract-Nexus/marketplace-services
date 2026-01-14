# Gift Cards Service

Digital gift card management microservice for the Tesseract Hub platform. Provides gift card creation, redemption, refunds, and transaction tracking.

## Features

- **Gift Card Creation**: Customizable balance, currency, recipient info
- **Balance Management**: Real-time balance checking and updates
- **Redemption**: Partial redemption with concurrent safety
- **Refunds**: Refund capability with balance capping
- **Transaction History**: Complete audit trail
- **Analytics**: Statistics and usage metrics

## Tech Stack

- **Language**: Go
- **Framework**: Gin
- **Database**: PostgreSQL with GORM
- **Port**: 8083

## API Endpoints

### Gift Card Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/gift-cards` | Create gift card |
| GET | `/api/v1/gift-cards` | List gift cards |
| GET | `/api/v1/gift-cards/:id` | Get by ID |
| PUT | `/api/v1/gift-cards/:id` | Update gift card |
| DELETE | `/api/v1/gift-cards/:id` | Delete (soft) |
| PATCH | `/api/v1/gift-cards/:id/status` | Update status |

### Operations
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/gift-cards/check-balance` | Check balance |
| POST | `/api/v1/gift-cards/redeem` | Redeem amount |
| POST | `/api/v1/gift-cards/apply` | Validate for order |
| POST | `/api/v1/gift-cards/refund` | Refund amount |

### Analytics
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/gift-cards/stats` | Get statistics |
| GET | `/api/v1/gift-cards/:id/transactions` | Transaction history |

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |

## Query Parameters

### Filtering
- `query` - Search code, email, name
- `status` - Filter by status
- `purchasedBy` - Filter by purchaser
- `recipientEmail` - Filter by email
- `minBalance`, `maxBalance` - Balance range
- `expiringBefore` - Expiration threshold
- `createdFrom`, `createdTo` - Date range

### Pagination
- `page` - Page number (default: 1)
- `limit` - Items per page (default: 20, max: 100)
- `sortBy` - Sort field
- `sortOrder` - ASC or DESC

## Gift Card Status

- **ACTIVE**: Can be used
- **REDEEMED**: Fully consumed (balance = 0)
- **EXPIRED**: Past expiration date
- **CANCELLED**: Manually cancelled
- **SUSPENDED**: Temporarily unavailable

## Transaction Types

- **ISSUE**: Initial creation
- **REDEMPTION**: Amount used in order
- **REFUND**: Amount refunded back
- **ADJUSTMENT**: Manual admin adjustment
- **EXPIRY**: Automatic expiration

## Code Format

- Format: `XXXX-XXXX-XXXX-XXXX`
- 16 hexadecimal characters
- Cryptographically secure generation
- Uniqueness guaranteed

## Data Model

### GiftCard
- UUID primary key with tenant isolation
- Unique code
- Initial and current balance
- Currency code (default: USD)
- Recipient: email, name
- Sender name and message
- Purchase info: user, order, date
- Expiration date
- Usage tracking: count, last used
- Metadata (JSONB)

### GiftCardTransaction
- Transaction type and amount
- Balance before/after
- Order and user references
- Description
- Expiration tracking

## Environment Variables

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=gift_cards_db
DB_SSL_MODE=disable

# Server
PORT=8083
ENVIRONMENT=development
LOG_LEVEL=info

# Authentication
JWT_SECRET=your-secret-key
```

## Concurrency Safety

- Pessimistic locking for redemption
- Transactional operations with rollback
- Atomic balance updates

## License

MIT
