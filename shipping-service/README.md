# Shipping Service

Multi-carrier shipping management microservice for the Tesseract Hub platform. Handles shipment creation, tracking, rate calculations with automatic carrier failover.

## Features

- **Multi-Carrier Support**: Shiprocket (India), Delhivery, Shippo, ShipEngine
- **Automatic Failover**: Primary carrier with fallback mechanism
- **Region-Based Routing**: Separate carriers for India vs. global shipments
- **Rate Calculation**: Compare rates across carriers
- **Shipment Tracking**: Real-time tracking with event history
- **Shipment Cancellation**: Cancel shipments with status management

## Tech Stack

- **Language**: Go 1.25
- **Framework**: Gin v1.10.0
- **Database**: PostgreSQL with GORM v1.25.11
- **Port**: 8088

## API Endpoints

### Shipments
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/shipments` | Create shipment |
| GET | `/api/shipments` | List shipments |
| GET | `/api/shipments/:id` | Get shipment |
| GET | `/api/shipments/order/:orderId` | Get shipments by order |
| PUT | `/api/shipments/:id/cancel` | Cancel shipment |
| PUT | `/api/shipments/:id/status` | Update status |

### Rates & Tracking
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/rates` | Get shipping rates |
| GET | `/api/track/:trackingNumber` | Track shipment |

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |

## Environment Variables

```env
# Server
PORT=8088
NODE_ENV=development

# Database
DATABASE_URL=postgresql://dev:devpass@localhost:5432/shipping

# Shiprocket (India)
SHIPROCKET_API_KEY=your-api-key
SHIPROCKET_API_SECRET=your-api-secret
SHIPROCKET_BASE_URL=https://apiv2.shiprocket.in
SHIPROCKET_ENABLED=true
SHIPROCKET_IS_PRODUCTION=false

# Delhivery (India Fallback)
DELHIVERY_API_KEY=your-api-key
DELHIVERY_BASE_URL=https://track.delhivery.com/api
DELHIVERY_ENABLED=false
DELHIVERY_IS_PRODUCTION=false

# Shippo (Global)
SHIPPO_API_KEY=your-api-key
SHIPPO_BASE_URL=https://api.goshippo.com
SHIPPO_ENABLED=false
SHIPPO_IS_PRODUCTION=false

# ShipEngine (Global Fallback)
SHIPENGINE_API_KEY=your-api-key
SHIPENGINE_BASE_URL=https://api.shipengine.com/v1
SHIPENGINE_ENABLED=false
SHIPENGINE_IS_PRODUCTION=false
```

## Data Models

### Shipment
- UUID primary key with tenant isolation
- Carrier and tracking information
- From/To address (embedded)
- Weight and dimensions
- Cost and currency
- Estimated and actual delivery dates

### Shipment Status
- PENDING, CREATED, PICKED_UP, IN_TRANSIT
- OUT_FOR_DELIVERY, DELIVERED, FAILED
- CANCELLED, RETURNED

### Supported Carriers
**India**: SHIPROCKET, DELHIVERY, BLUEDART, DTDC
**Global**: SHIPPO, SHIPENGINE, FEDEX, UPS, DHL

## Carrier Selection Logic

1. Detect destination region (India vs. Global)
2. Select primary carrier for region
3. Attempt shipment creation
4. On failure, automatically fallback to secondary carrier
5. Return result with carrier used

## Running Locally

```bash
# Set environment variables
cp .env.example .env

# Run with Docker
docker-compose up

# Or run directly
go run cmd/main.go
```

## License

MIT
