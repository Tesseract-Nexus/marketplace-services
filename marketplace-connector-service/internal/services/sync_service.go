package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/clients/amazon"
	"marketplace-connector-service/internal/clients/dukaan"
	"marketplace-connector-service/internal/clients/shopify"
	"marketplace-connector-service/internal/config"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
	"marketplace-connector-service/internal/secrets"
)

// SyncService handles marketplace synchronization operations
type SyncService struct {
	syncRepo       *repository.SyncRepository
	connectionRepo *repository.ConnectionRepository
	mappingRepo    *repository.MappingRepository
	secretManager  *secrets.GCPSecretManager
	config         *config.Config
	activeJobs     map[uuid.UUID]context.CancelFunc
	mu             sync.RWMutex
	concurrency    *TenantSemaphore
	retrier        *clients.Retrier
}

// NewSyncService creates a new sync service
func NewSyncService(
	syncRepo *repository.SyncRepository,
	connectionRepo *repository.ConnectionRepository,
	mappingRepo *repository.MappingRepository,
	secretManager *secrets.GCPSecretManager,
	cfg *config.Config,
) *SyncService {
	return &SyncService{
		syncRepo:       syncRepo,
		connectionRepo: connectionRepo,
		mappingRepo:    mappingRepo,
		secretManager:  secretManager,
		config:         cfg,
		activeJobs:     make(map[uuid.UUID]context.CancelFunc),
		concurrency:    NewTenantSemaphore(DefaultConcurrencyConfig()),
		retrier:        clients.NewRetrier(clients.DefaultRetryConfig()),
	}
}

// SetConcurrencyLimiter sets the concurrency limiter
func (s *SyncService) SetConcurrencyLimiter(concurrency *TenantSemaphore) {
	s.concurrency = concurrency
}

// CreateJobRequest contains the data for creating a new sync job
type CreateJobRequest struct {
	ConnectionID   uuid.UUID           `json:"connectionId"`
	SyncType       models.SyncType     `json:"syncType"`
	TriggeredBy    models.TriggerType  `json:"triggeredBy"`
	CreatedBy      string              `json:"createdBy,omitempty"`
	JobType        models.JobType      `json:"jobType,omitempty"`
	IdempotencyKey string              `json:"idempotencyKey,omitempty"`
	Priority       int                 `json:"priority,omitempty"`
}

// CreateJob creates and starts a new sync job
func (s *SyncService) CreateJob(ctx context.Context, tenantID string, req *CreateJobRequest) (*models.MarketplaceSyncJob, error) {
	// Verify connection exists and belongs to tenant
	connection, err := s.connectionRepo.GetByID(ctx, req.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}
	if connection.TenantID != tenantID {
		return nil, fmt.Errorf("connection does not belong to tenant")
	}
	if connection.Status != models.ConnectionConnected {
		return nil, fmt.Errorf("connection is not active")
	}

	// Check idempotency key if provided
	if req.IdempotencyKey != "" {
		existingJob, err := s.syncRepo.GetJobByIdempotencyKey(ctx, req.IdempotencyKey)
		if err == nil && existingJob != nil {
			// Return existing job if idempotency key matches
			return existingJob, nil
		}
	}

	// Check for running jobs
	runningJobs, err := s.syncRepo.GetRunningJobs(ctx, req.ConnectionID)
	if err != nil {
		return nil, err
	}
	if len(runningJobs) > 0 {
		return nil, fmt.Errorf("a sync job is already running for this connection")
	}

	// Check concurrency limits
	if s.concurrency != nil {
		if !s.concurrency.CanAcceptJob(tenantID, req.ConnectionID.String()) {
			return nil, fmt.Errorf("concurrency limit reached for tenant or connection")
		}
	}

	// Set defaults for job type and priority
	jobType := req.JobType
	if jobType == "" {
		jobType = models.JobTypeFullImport
	}
	priority := req.Priority
	if priority == 0 {
		priority = 5 // Default priority
	}

	// Generate idempotency key if not provided
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("%s-%s-%s-%d", tenantID, req.ConnectionID, req.SyncType, time.Now().Unix())
	}

	// Create job record
	now := time.Now()
	job := &models.MarketplaceSyncJob{
		ID:             uuid.New(),
		ConnectionID:   req.ConnectionID,
		TenantID:       tenantID,
		SyncType:       req.SyncType,
		Direction:      models.SyncDirectionInbound,
		Status:         models.SyncStatusRunning,
		TriggeredBy:    req.TriggeredBy,
		CreatedBy:      req.CreatedBy,
		StartedAt:      &now,
		JobType:        jobType,
		IdempotencyKey: idempotencyKey,
		Priority:       priority,
	}
	job.SetProgress(&models.SyncProgress{})

	if err := s.syncRepo.CreateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Start sync in background
	jobCtx, cancel := context.WithTimeout(context.Background(), s.config.SyncTimeout)
	s.mu.Lock()
	s.activeJobs[job.ID] = cancel
	s.mu.Unlock()

	go s.runSync(jobCtx, job, connection)

	return job, nil
}

// GetJob retrieves a sync job by ID
func (s *SyncService) GetJob(ctx context.Context, id uuid.UUID) (*models.MarketplaceSyncJob, error) {
	return s.syncRepo.GetJobByID(ctx, id)
}

// ListJobs lists sync jobs
func (s *SyncService) ListJobs(ctx context.Context, tenantID string, opts *repository.SyncListOptions) ([]models.MarketplaceSyncJob, int64, error) {
	if opts == nil {
		opts = &repository.SyncListOptions{}
	}
	opts.TenantID = tenantID
	return s.syncRepo.ListJobs(ctx, *opts)
}

// CancelJob cancels a running sync job
func (s *SyncService) CancelJob(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	cancel, exists := s.activeJobs[id]
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("job not found or not running")
	}

	cancel()
	return s.syncRepo.UpdateJobStatus(ctx, id, models.SyncStatusCancelled, "Cancelled by user")
}

// GetJobLogs retrieves logs for a sync job
func (s *SyncService) GetJobLogs(ctx context.Context, jobID uuid.UUID, opts *repository.LogListOptions) ([]models.MarketplaceSyncLog, error) {
	if opts == nil {
		opts = &repository.LogListOptions{Limit: 100}
	}
	return s.syncRepo.GetJobLogs(ctx, jobID, *opts)
}

// GetStats retrieves sync statistics
func (s *SyncService) GetStats(ctx context.Context, tenantID string, connectionID *uuid.UUID) (*repository.SyncStats, error) {
	return s.syncRepo.GetSyncStats(ctx, tenantID, connectionID)
}

// runSync executes the sync operation
func (s *SyncService) runSync(ctx context.Context, job *models.MarketplaceSyncJob, connection *models.MarketplaceConnection) {
	defer func() {
		s.mu.Lock()
		delete(s.activeJobs, job.ID)
		s.mu.Unlock()
	}()

	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Sync started", nil)

	// Get credentials and initialize client
	client, err := s.initializeClient(ctx, connection)
	if err != nil {
		s.failJob(ctx, job.ID, fmt.Sprintf("Failed to initialize client: %v", err))
		return
	}

	// Execute sync based on type
	var syncErr error
	switch job.SyncType {
	case models.SyncTypeProducts:
		syncErr = s.syncProducts(ctx, job, connection, client)
	case models.SyncTypeOrders:
		syncErr = s.syncOrders(ctx, job, connection, client)
	case models.SyncTypeInventory:
		syncErr = s.syncInventory(ctx, job, connection, client)
	case models.SyncTypeFull:
		if syncErr = s.syncProducts(ctx, job, connection, client); syncErr == nil {
			if syncErr = s.syncOrders(ctx, job, connection, client); syncErr == nil {
				syncErr = s.syncInventory(ctx, job, connection, client)
			}
		}
	default:
		syncErr = fmt.Errorf("unsupported sync type: %s", job.SyncType)
	}

	if syncErr != nil {
		if ctx.Err() != nil {
			_ = s.syncRepo.UpdateJobStatus(ctx, job.ID, models.SyncStatusCancelled, "Cancelled")
		} else {
			s.failJob(ctx, job.ID, syncErr.Error())
		}
		return
	}

	// Mark as completed
	_ = s.syncRepo.UpdateJobStatus(context.Background(), job.ID, models.SyncStatusCompleted, "")
	s.logEvent(context.Background(), job.ID, models.LogLevelInfo, "Sync completed successfully", nil)

	// Update connection last sync time
	connection.LastSyncAt = timePtr(time.Now())
	_ = s.connectionRepo.Update(context.Background(), connection)
}

// syncProducts syncs products from the marketplace
func (s *SyncService) syncProducts(ctx context.Context, job *models.MarketplaceSyncJob, connection *models.MarketplaceConnection, client clients.MarketplaceClient) error {
	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Starting product sync", nil)

	progress := &models.SyncProgress{}
	var cursor string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		result, err := client.GetProducts(ctx, &clients.ListOptions{
			Limit:  s.config.SyncBatchSize,
			Cursor: cursor,
		})
		if err != nil {
			return fmt.Errorf("failed to fetch products: %w", err)
		}

		progress.TotalItems += len(result.Products)

		for _, extProduct := range result.Products {
			if err := s.processProduct(ctx, job, connection, extProduct); err != nil {
				progress.FailedItems++
				s.logEvent(ctx, job.ID, models.LogLevelError, "Failed to process product", models.JSONB{
					"externalId": extProduct.ID,
					"error":      err.Error(),
				})
			} else {
				progress.SuccessfulItems++
			}
			progress.ProcessedItems++
			progress.Percentage = float64(progress.ProcessedItems) / float64(progress.TotalItems) * 100

			// Update progress periodically
			if progress.ProcessedItems%10 == 0 {
				_ = s.syncRepo.UpdateJobProgress(ctx, job.ID, progress)
			}
		}

		if !result.HasMore {
			break
		}
		cursor = result.NextCursor
	}

	_ = s.syncRepo.UpdateJobProgress(ctx, job.ID, progress)
	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Product sync completed", models.JSONB{
		"total":      progress.TotalItems,
		"successful": progress.SuccessfulItems,
		"failed":     progress.FailedItems,
	})

	return nil
}

// processProduct processes a single product from the marketplace
func (s *SyncService) processProduct(ctx context.Context, job *models.MarketplaceSyncJob, connection *models.MarketplaceConnection, extProduct clients.ExternalProduct) error {
	// Create schema mapper for this tenant/vendor
	mapper := NewSchemaMapper(connection.TenantID, connection.VendorID)

	// Transform external product to internal format
	internalProduct, err := mapper.MapExternalProductToInternal(&extProduct, connection.MarketplaceType)
	if err != nil {
		return fmt.Errorf("failed to map product: %w", err)
	}

	// Log the transformation for debugging
	s.logEvent(ctx, job.ID, models.LogLevelDebug, "Mapped external product to internal schema", models.JSONB{
		"externalId":  extProduct.ID,
		"internalId":  internalProduct.ID.String(),
		"name":        internalProduct.Name,
		"sku":         internalProduct.SKU,
		"price":       internalProduct.Price,
		"status":      internalProduct.Status,
		"variantCount": len(internalProduct.Variants),
	})

	// Check if mapping exists
	mapping, err := s.mappingRepo.GetProductMappingByExternal(ctx, connection.ID, extProduct.ID, nil)

	now := time.Now()
	if err != nil {
		// Create new mapping - use the internal product ID from our mapping
		mapping = &models.MarketplaceProductMapping{
			ID:                uuid.New(),
			ConnectionID:      connection.ID,
			TenantID:          connection.TenantID,
			InternalProductID: internalProduct.ID,
			ExternalProductID: extProduct.ID,
			SyncStatus:        models.MappingSynced,
			LastSyncedAt:      &now,
			LastSyncDirection: string(models.SyncDirectionInbound),
		}

		// Store SKU for inventory mapping
		if internalProduct.SKU != "" {
			mapping.ExternalSKU = &internalProduct.SKU
		}

		// TODO: Send transformed product to products-service API
		// productPayload, _ := ToJSON(internalProduct)
		// resp, err := http.Post(s.config.ProductsServiceURL + "/api/v1/products", "application/json", bytes.NewBuffer(productPayload))

		return s.mappingRepo.CreateProductMapping(ctx, mapping)
	}

	// Update existing mapping
	mapping.LastSyncedAt = &now
	mapping.LastSyncDirection = string(models.SyncDirectionInbound)
	mapping.SyncStatus = models.MappingSynced

	// TODO: Update existing product in products-service
	// productPayload, _ := ToJSON(internalProduct)
	// resp, err := http.Put(s.config.ProductsServiceURL + "/api/v1/products/" + mapping.InternalProductID.String(), "application/json", bytes.NewBuffer(productPayload))

	return s.mappingRepo.UpsertProductMapping(ctx, mapping)
}

// syncOrders syncs orders from the marketplace
func (s *SyncService) syncOrders(ctx context.Context, job *models.MarketplaceSyncJob, connection *models.MarketplaceConnection, client clients.MarketplaceClient) error {
	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Starting order sync", nil)

	progress := job.GetProgress()
	var cursor string

	// Get orders from the last 30 days by default
	createdAfter := time.Now().AddDate(0, 0, -30)
	if connection.LastSyncAt != nil {
		createdAfter = *connection.LastSyncAt
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		result, err := client.GetOrders(ctx, &clients.OrderListOptions{
			ListOptions: clients.ListOptions{
				Limit:  s.config.SyncBatchSize,
				Cursor: cursor,
			},
			CreatedAfter: createdAfter,
		})
		if err != nil {
			return fmt.Errorf("failed to fetch orders: %w", err)
		}

		progress.TotalItems += len(result.Orders)

		for _, extOrder := range result.Orders {
			if err := s.processOrder(ctx, job, connection, extOrder); err != nil {
				progress.FailedItems++
				s.logEvent(ctx, job.ID, models.LogLevelError, "Failed to process order", models.JSONB{
					"externalId": extOrder.ID,
					"error":      err.Error(),
				})
			} else {
				progress.SuccessfulItems++
			}
			progress.ProcessedItems++
		}

		_ = s.syncRepo.UpdateJobProgress(ctx, job.ID, progress)

		if !result.HasMore {
			break
		}
		cursor = result.NextCursor
	}

	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Order sync completed", models.JSONB{
		"total":      progress.TotalItems,
		"successful": progress.SuccessfulItems,
		"failed":     progress.FailedItems,
	})

	return nil
}

// processOrder processes a single order from the marketplace
func (s *SyncService) processOrder(ctx context.Context, job *models.MarketplaceSyncJob, connection *models.MarketplaceConnection, extOrder clients.ExternalOrder) error {
	// Create schema mapper for this tenant/vendor
	mapper := NewSchemaMapper(connection.TenantID, connection.VendorID)

	// Get existing product mappings to map line item SKUs to internal product IDs
	productMappings := make(map[string]uuid.UUID)
	for _, lineItem := range extOrder.LineItems {
		if lineItem.SKU != "" {
			mapping, err := s.mappingRepo.GetProductMappingBySKU(ctx, connection.ID, lineItem.SKU)
			if err == nil && mapping != nil {
				productMappings[lineItem.SKU] = mapping.InternalProductID
			}
		}
	}

	// Transform external order to internal format
	internalOrder, err := mapper.MapExternalOrderToInternal(&extOrder, connection.MarketplaceType, productMappings)
	if err != nil {
		return fmt.Errorf("failed to map order: %w", err)
	}

	// Log the transformation for debugging
	s.logEvent(ctx, job.ID, models.LogLevelDebug, "Mapped external order to internal schema", models.JSONB{
		"externalId":        extOrder.ID,
		"internalId":        internalOrder.ID.String(),
		"orderNumber":       internalOrder.OrderNumber,
		"status":            internalOrder.Status,
		"paymentStatus":     internalOrder.PaymentStatus,
		"fulfillmentStatus": internalOrder.FulfillmentStatus,
		"total":             internalOrder.Total,
		"currency":          internalOrder.Currency,
		"itemCount":         len(internalOrder.Items),
	})

	// Check if mapping exists
	mapping, err := s.mappingRepo.GetOrderMappingByExternal(ctx, connection.ID, extOrder.ID)

	now := time.Now()
	if err != nil {
		// Create new mapping - use the internal order ID from our mapping
		mapping = &models.MarketplaceOrderMapping{
			ID:                   uuid.New(),
			ConnectionID:         connection.ID,
			TenantID:             connection.TenantID,
			InternalOrderID:      internalOrder.ID,
			ExternalOrderID:      extOrder.ID,
			ExternalOrderNumber:  &internalOrder.OrderNumber,
			SyncStatus:           models.MappingSynced,
			LastSyncedAt:         &now,
			MarketplaceStatus:    &extOrder.Status,
			MarketplaceCreatedAt: &extOrder.CreatedAt,
		}

		// TODO: Send transformed order to orders-service API
		// orderPayload, _ := ToJSON(internalOrder)
		// resp, err := http.Post(s.config.OrdersServiceURL + "/api/v1/orders", "application/json", bytes.NewBuffer(orderPayload))

		return s.mappingRepo.CreateOrderMapping(ctx, mapping)
	}

	// Update existing mapping
	mapping.LastSyncedAt = &now
	mapping.MarketplaceStatus = &extOrder.Status
	mapping.SyncStatus = models.MappingSynced

	// TODO: Update existing order status in orders-service
	// statusPayload := map[string]string{"status": internalOrder.Status, "fulfillment_status": internalOrder.FulfillmentStatus}
	// resp, err := http.Patch(s.config.OrdersServiceURL + "/api/v1/orders/" + mapping.InternalOrderID.String() + "/status", "application/json", bytes.NewBuffer(statusPayload))

	return s.mappingRepo.UpsertOrderMapping(ctx, mapping)
}

// syncInventory syncs inventory from the marketplace
func (s *SyncService) syncInventory(ctx context.Context, job *models.MarketplaceSyncJob, connection *models.MarketplaceConnection, client clients.MarketplaceClient) error {
	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Starting inventory sync", nil)

	// Get all product mappings to get SKUs
	mappings, _, err := s.mappingRepo.ListProductMappings(ctx, repository.MappingListOptions{
		ConnectionID: connection.ID,
		Limit:        1000,
	})
	if err != nil {
		return err
	}

	skus := make([]string, 0, len(mappings))
	for _, m := range mappings {
		if m.ExternalSKU != nil && *m.ExternalSKU != "" {
			skus = append(skus, *m.ExternalSKU)
		}
	}

	if len(skus) == 0 {
		s.logEvent(ctx, job.ID, models.LogLevelInfo, "No SKUs to sync inventory for", nil)
		return nil
	}

	// Fetch inventory in batches
	batchSize := 50
	for i := 0; i < len(skus); i += batchSize {
		end := i + batchSize
		if end > len(skus) {
			end = len(skus)
		}
		batch := skus[i:end]

		inventory, err := client.GetInventory(ctx, batch)
		if err != nil {
			s.logEvent(ctx, job.ID, models.LogLevelError, "Failed to fetch inventory batch", models.JSONB{
				"error": err.Error(),
			})
			continue
		}

		for sku, level := range inventory {
			s.logEvent(ctx, job.ID, models.LogLevelDebug, "Inventory level", models.JSONB{
				"sku":      sku,
				"quantity": level.Quantity,
			})
			// Would update inventory-service here
		}
	}

	s.logEvent(ctx, job.ID, models.LogLevelInfo, "Inventory sync completed", nil)
	return nil
}

// initializeClient creates and initializes a marketplace client
func (s *SyncService) initializeClient(ctx context.Context, connection *models.MarketplaceConnection) (clients.MarketplaceClient, error) {
	if s.secretManager == nil {
		return nil, fmt.Errorf("secret manager not configured")
	}

	secret, err := s.secretManager.GetSecret(ctx, connection.SecretReference)
	if err != nil {
		return nil, err
	}

	client, err := s.createClient(connection.MarketplaceType)
	if err != nil {
		return nil, err
	}

	if err := client.Initialize(ctx, secret.Credentials); err != nil {
		return nil, err
	}

	return client, nil
}

// createClient creates a marketplace client based on the type
func (s *SyncService) createClient(marketplaceType models.MarketplaceType) (clients.MarketplaceClient, error) {
	switch marketplaceType {
	case models.MarketplaceAmazon:
		return amazon.NewAmazonClient(), nil
	case models.MarketplaceShopify:
		return shopify.NewShopifyClient(), nil
	case models.MarketplaceDukaan:
		return dukaan.NewDukaanClient(), nil
	default:
		return nil, &clients.UnsupportedMarketplaceError{MarketplaceType: string(marketplaceType)}
	}
}

// failJob marks a job as failed
func (s *SyncService) failJob(ctx context.Context, jobID uuid.UUID, message string) {
	_ = s.syncRepo.UpdateJobStatus(context.Background(), jobID, models.SyncStatusFailed, message)
	s.logEvent(context.Background(), jobID, models.LogLevelError, message, nil)
}

// logEvent creates a sync log entry
func (s *SyncService) logEvent(ctx context.Context, jobID uuid.UUID, level models.LogLevel, message string, data models.JSONB) {
	log := &models.MarketplaceSyncLog{
		ID:        uuid.New(),
		SyncJobID: jobID,
		Level:     level,
		Message:   message,
		Data:      data,
	}
	_ = s.syncRepo.CreateLog(ctx, log)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
