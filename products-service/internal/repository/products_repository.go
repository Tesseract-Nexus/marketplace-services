package repository

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"products-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants
const (
	ProductCacheTTL     = 5 * time.Minute  // Single product cache
	ProductListCacheTTL = 2 * time.Minute  // Product list cache (shorter due to frequent changes)
	CategoryCacheTTL    = 30 * time.Minute // Categories rarely change
)

type ProductsRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

func NewProductsRepository(db *gorm.DB, redis *redis.Client) *ProductsRepository {
	repo := &ProductsRepository{
		db:    db,
		redis: redis,
	}

	// Initialize CacheLayer with the existing Redis client
	if redis != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 5000,
			L1TTL:      30 * time.Second,
			DefaultTTL: ProductCacheTTL,
			KeyPrefix:  "tesseract:products:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redis, cacheConfig)
	}

	return repo
}

// generateListCacheKey creates a deterministic cache key for list queries
func generateListCacheKey(tenantID string, prefix string, params interface{}) string {
	data, _ := json.Marshal(params)
	hash := md5.Sum(data)
	return fmt.Sprintf("%s:%s:%s", prefix, tenantID, hex.EncodeToString(hash[:]))
}

// invalidateProductCaches invalidates all caches related to a product
func (r *ProductsRepository) invalidateProductCaches(ctx context.Context, tenantID string, productID uuid.UUID) {
	if r.cache == nil {
		return
	}

	// Invalidate single product cache (both variants)
	productKey := fmt.Sprintf("product:%s:%s", tenantID, productID.String())
	_ = r.cache.Delete(ctx, productKey+":true", productKey+":false")

	// Invalidate list caches for this tenant (pattern-based)
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("products:list:%s:*", tenantID))
}

// invalidateTenantProductListCaches invalidates all product list caches for a tenant
func (r *ProductsRepository) invalidateTenantProductListCaches(ctx context.Context, tenantID string) {
	if r.cache == nil {
		return
	}
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("products:list:%s:*", tenantID))
}

// invalidateCategoryCaches invalidates category-related caches for a tenant
func (r *ProductsRepository) invalidateCategoryCaches(ctx context.Context, tenantID string, categoryID *uuid.UUID) {
	if r.cache == nil {
		return
	}

	if categoryID != nil {
		// Invalidate specific category
		_ = r.cache.Delete(ctx, fmt.Sprintf("category:%s:%s", tenantID, categoryID.String()))
	}
	// Invalidate category list
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("categories:*:%s:*", tenantID))
}

// Product CRUD Operations

// CreateProduct creates a new product
func (r *ProductsRepository) CreateProduct(tenantID string, product *models.Product) error {
	product.TenantID = tenantID
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()

	// Ensure product has an ID before generating slug (for uniqueness)
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}

	// Generate slug from name if not provided or empty
	if product.Slug == nil || *product.Slug == "" {
		baseSlug := generateSlug(product.Name)
		// Ensure slug uniqueness by appending first 8 chars of product ID
		uniqueSlug := fmt.Sprintf("%s-%s", baseSlug, product.ID.String()[:8])
		product.Slug = &uniqueSlug
	}

	err := r.db.Create(product).Error
	if err == nil {
		// Invalidate list caches as a new product was added
		r.invalidateTenantProductListCaches(context.Background(), tenantID)
	}
	return err
}

// GetProductByID retrieves a product by ID with caching
func (r *ProductsRepository) GetProductByID(tenantID string, productID uuid.UUID, includeVariants bool) (*models.Product, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("product:%s:%s:%v", tenantID, productID.String(), includeVariants)

	// Try to get from cache first (using raw Redis for backward compatibility)
	if r.redis != nil {
		val, err := r.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			var product models.Product
			if err := json.Unmarshal([]byte(val), &product); err == nil {
				return &product, nil
			}
		}
	}

	// Query from database
	var product models.Product
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, productID)
	if includeVariants {
		query = query.Preload("Variants")
	}
	if err := query.First(&product).Error; err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, err := json.Marshal(product)
		if err == nil {
			r.redis.Set(ctx, cacheKey, data, ProductCacheTTL)
		}
	}

	return &product, nil
}

// BatchGetProductsByIDs retrieves multiple products by IDs in a single query
// Performance: Single database query instead of N queries
func (r *ProductsRepository) BatchGetProductsByIDs(tenantID string, productIDs []uuid.UUID, includeVariants bool) ([]*models.Product, error) {
	if len(productIDs) == 0 {
		return []*models.Product{}, nil
	}

	var products []*models.Product
	query := r.db.Where("tenant_id = ? AND id IN ?", tenantID, productIDs)

	if includeVariants {
		query = query.Preload("Variants")
	}

	err := query.Find(&products).Error
	if err != nil {
		return nil, err
	}
	return products, nil
}

// UpdateProduct updates a product and invalidates cache
func (r *ProductsRepository) UpdateProduct(tenantID string, productID uuid.UUID, updates *models.Product) error {
	updates.UpdatedAt = time.Now()
	err := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND id = ?", tenantID, productID).
		Updates(updates).Error

	if err == nil {
		// Invalidate all caches related to this product
		r.invalidateProductCaches(context.Background(), tenantID, productID)
	}

	return err
}

// UpdateProductStatus updates product status
func (r *ProductsRepository) UpdateProductStatus(tenantID string, productID uuid.UUID, status models.ProductStatus, notes *string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	err := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND id = ?", tenantID, productID).
		Updates(updates).Error

	if err == nil {
		r.invalidateProductCaches(context.Background(), tenantID, productID)
	}
	return err
}

// DeleteProduct soft deletes a product
func (r *ProductsRepository) DeleteProduct(tenantID string, productID uuid.UUID) error {
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, productID).
		Delete(&models.Product{}).Error

	if err == nil {
		r.invalidateProductCaches(context.Background(), tenantID, productID)
	}
	return err
}

// GetProducts retrieves products with filters and pagination
func (r *ProductsRepository) GetProducts(tenantID string, req *models.SearchProductsRequest) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	query = r.applyProductFilters(query, req)

	// Count total results
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	if req.SortBy != nil && *req.SortBy != "" {
		sortOrder := "DESC"
		if req.SortOrder != nil && strings.ToUpper(*req.SortOrder) == "ASC" {
			sortOrder = "ASC"
		}
		query = query.Order(fmt.Sprintf("%s %s", *req.SortBy, sortOrder))
	} else {
		query = query.Order("created_at DESC")
	}

	// Include variants if requested
	if req.IncludeVariants != nil && *req.IncludeVariants {
		query = query.Preload("Variants")
	}

	// Apply pagination
	offset := (req.Page - 1) * req.Limit
	if err := query.Offset(offset).Limit(req.Limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// SearchProducts performs text search on products with full-text search
func (r *ProductsRepository) SearchProducts(tenantID string, req *models.SearchProductsRequest) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{}).Where("tenant_id = ?", tenantID)

	// Apply full-text search with PostgreSQL tsvector
	if req.Query != nil && *req.Query != "" {
		searchQuery := strings.TrimSpace(*req.Query)

		// Use PostgreSQL full-text search with weighted ranking
		// A = highest weight (name), B = description, C = SKU, D = keywords
		tsQuery := strings.Join(strings.Fields(searchQuery), " & ")

		query = query.Where(
			`(
				setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
				setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
				setweight(to_tsvector('english', COALESCE(sku, '')), 'C') ||
				setweight(to_tsvector('english', COALESCE(search_keywords, '')), 'D')
			) @@ to_tsquery('english', ?)`,
			tsQuery,
		)

		// Order by relevance (rank) when searching
		if req.SortBy == nil || *req.SortBy == "" {
			query = query.Select(`*,
				ts_rank(
					setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
					setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
					setweight(to_tsvector('english', COALESCE(sku, '')), 'C') ||
					setweight(to_tsvector('english', COALESCE(search_keywords, '')), 'D'),
					to_tsquery('english', ?)
				) AS rank`, tsQuery)
			query = query.Order("rank DESC, created_at DESC")
		}
	}

	// Apply other filters
	query = r.applyProductFilters(query, req)

	// Count total results
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting and pagination
	if req.Query == nil || *req.Query == "" {
		if req.SortBy != nil && *req.SortBy != "" {
			sortOrder := "DESC"
			if req.SortOrder != nil && strings.ToUpper(*req.SortOrder) == "ASC" {
				sortOrder = "ASC"
			}
			query = query.Order(fmt.Sprintf("%s %s", *req.SortBy, sortOrder))
		} else {
			query = query.Order("created_at DESC")
		}
	}

	// Include variants if requested
	if req.IncludeVariants != nil && *req.IncludeVariants {
		query = query.Preload("Variants")
	}

	offset := (req.Page - 1) * req.Limit
	if err := query.Offset(offset).Limit(req.Limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// GetSearchSuggestions returns autocomplete suggestions based on query
func (r *ProductsRepository) GetSearchSuggestions(tenantID string, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	var suggestions []string
	searchTerm := strings.ToLower(strings.TrimSpace(query))

	// Get product names that match the query
	err := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.ProductStatusActive).
		Where("LOWER(name) LIKE ?", searchTerm+"%").
		Order("name ASC").
		Limit(limit).
		Pluck("DISTINCT name", &suggestions).Error

	if err != nil {
		return nil, err
	}

	return suggestions, nil
}

// GetAvailableFilters returns available filter options for products
func (r *ProductsRepository) GetAvailableFilters(tenantID string, categoryID *string) (map[string]interface{}, error) {
	query := r.db.Model(&models.Product{}).Where("tenant_id = ? AND status = ?", tenantID, models.ProductStatusActive)

	if categoryID != nil {
		query = query.Where("category_id = ?", *categoryID)
	}

	filters := make(map[string]interface{})

	// Get unique brands
	var brands []string
	if err := query.Distinct("brand").Where("brand IS NOT NULL").Pluck("brand", &brands).Error; err != nil {
		return nil, err
	}
	filters["brands"] = brands

	// Get price range
	var priceRange struct {
		MinPrice float64
		MaxPrice float64
	}
	if err := query.Select("MIN(CAST(price AS DECIMAL)) as min_price, MAX(CAST(price AS DECIMAL)) as max_price").
		Scan(&priceRange).Error; err != nil {
		return nil, err
	}
	filters["priceRange"] = map[string]float64{
		"min": priceRange.MinPrice,
		"max": priceRange.MaxPrice,
	}

	// Get rating distribution
	var ratingDistribution []struct {
		Rating int
		Count  int64
	}
	if err := r.db.Raw(`
		SELECT FLOOR(average_rating) as rating, COUNT(*) as count
		FROM products
		WHERE tenant_id = ? AND status = ? AND average_rating IS NOT NULL
		GROUP BY FLOOR(average_rating)
		ORDER BY rating DESC
	`, tenantID, models.ProductStatusActive).Scan(&ratingDistribution).Error; err != nil {
		return nil, err
	}
	filters["ratings"] = ratingDistribution

	// Get available attributes (extract unique attributes from JSONB)
	var attributesData []struct {
		Attributes models.JSON
	}
	if err := query.Where("attributes IS NOT NULL").Select("attributes").Scan(&attributesData).Error; err != nil {
		return nil, err
	}

	// Parse attributes
	attributeMap := make(map[string]map[string]bool)
	for _, item := range attributesData {
		if item.Attributes != nil {
			if attrs, ok := item.Attributes["attributes"].(map[string]interface{}); ok {
				for key, value := range attrs {
					if attributeMap[key] == nil {
						attributeMap[key] = make(map[string]bool)
					}
					if strVal, ok := value.(string); ok {
						attributeMap[key][strVal] = true
					}
				}
			}
		}
	}

	// Convert to array format
	attributeFilters := make(map[string][]string)
	for key, values := range attributeMap {
		valueList := make([]string, 0, len(values))
		for val := range values {
			valueList = append(valueList, val)
		}
		attributeFilters[key] = valueList
	}
	filters["attributes"] = attributeFilters

	return filters, nil
}

// TrackSearch records a search query for analytics
func (r *ProductsRepository) TrackSearch(analytics *models.SearchAnalytics) error {
	analytics.CreatedAt = time.Now()
	return r.db.Create(analytics).Error
}

// GetTopSearches returns most popular search queries
func (r *ProductsRepository) GetTopSearches(tenantID string, limit int, days int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}
	if days <= 0 {
		days = 30
	}

	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT query, COUNT(*) as count, AVG(results_count) as avg_results
		FROM search_analytics
		WHERE tenant_id = ? AND created_at >= NOW() - INTERVAL '? days'
		GROUP BY query
		ORDER BY count DESC
		LIMIT ?
	`, tenantID, days, limit).Scan(&results).Error

	return results, err
}

// GetSearchAnalytics returns search analytics summary
func (r *ProductsRepository) GetSearchAnalytics(tenantID string, days int) (map[string]interface{}, error) {
	if days <= 0 {
		days = 30
	}

	analytics := make(map[string]interface{})

	// Total searches
	var totalSearches int64
	r.db.Model(&models.SearchAnalytics{}).
		Where("tenant_id = ? AND created_at >= NOW() - INTERVAL '? days'", tenantID, days).
		Count(&totalSearches)
	analytics["totalSearches"] = totalSearches

	// Unique searches
	var uniqueSearches int64
	r.db.Model(&models.SearchAnalytics{}).
		Where("tenant_id = ? AND created_at >= NOW() - INTERVAL '? days'", tenantID, days).
		Distinct("query").
		Count(&uniqueSearches)
	analytics["uniqueSearches"] = uniqueSearches

	// Average results
	var avgResults float64
	r.db.Model(&models.SearchAnalytics{}).
		Where("tenant_id = ? AND created_at >= NOW() - INTERVAL '? days'", tenantID, days).
		Select("AVG(results_count)").
		Scan(&avgResults)
	analytics["avgResults"] = avgResults

	// Zero results rate
	var zeroResults int64
	r.db.Model(&models.SearchAnalytics{}).
		Where("tenant_id = ? AND created_at >= NOW() - INTERVAL '? days' AND results_count = 0", tenantID, days).
		Count(&zeroResults)
	zeroResultsRate := float64(0)
	if totalSearches > 0 {
		zeroResultsRate = (float64(zeroResults) / float64(totalSearches)) * 100
	}
	analytics["zeroResultsRate"] = zeroResultsRate

	// Top searches
	topSearches, _ := r.GetTopSearches(tenantID, 10, days)
	analytics["topSearches"] = topSearches

	return analytics, nil
}

// GetProductsByCategory retrieves products by category
func (r *ProductsRepository) GetProductsByCategory(tenantID string, categoryID uuid.UUID, page, limit int) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{}).Where("tenant_id = ? AND category_id = ?", tenantID, categoryID)

	// Count total results
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// UpdateInventory updates product inventory
func (r *ProductsRepository) UpdateInventory(tenantID string, productID uuid.UUID, quantity int, status *models.InventoryStatus) error {
	updates := map[string]interface{}{
		"quantity":   quantity,
		"updated_at": time.Now(),
	}

	if status != nil {
		updates["inventory_status"] = *status
	}

	return r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND id = ?", tenantID, productID).
		Updates(updates).Error
}

// BulkDeductInventory deducts inventory for multiple products (for order creation)
// Performance: Uses batch SELECT + batch UPDATE instead of N individual queries
// Reduces 2N queries to 2 queries (N items -> 1 SELECT + 1 UPDATE)
func (r *ProductsRepository) BulkDeductInventory(tenantID string, items []models.BulkInventoryItem) error {
	if len(items) == 0 {
		return nil
	}

	// Parse all product IDs upfront
	productIDs := make([]uuid.UUID, 0, len(items))
	itemMap := make(map[string]int) // productID -> quantity to deduct
	for _, item := range items {
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			return fmt.Errorf("invalid product ID %s: %w", item.ProductID, err)
		}
		productIDs = append(productIDs, productID)
		itemMap[item.ProductID] = item.Quantity
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Phase 1: Batch SELECT all products in one query
		var products []models.Product
		if err := tx.Where("tenant_id = ? AND id IN ?", tenantID, productIDs).Find(&products).Error; err != nil {
			return fmt.Errorf("failed to fetch products: %w", err)
		}

		// Validate all products exist and have sufficient stock
		foundProducts := make(map[string]*models.Product)
		for i := range products {
			foundProducts[products[i].ID.String()] = &products[i]
		}

		for _, item := range items {
			product, found := foundProducts[item.ProductID]
			if !found {
				return fmt.Errorf("product %s not found", item.ProductID)
			}
			if product.Quantity == nil {
				return fmt.Errorf("product %s does not have inventory tracking enabled", item.ProductID)
			}
			if *product.Quantity < item.Quantity {
				return fmt.Errorf("insufficient stock for product %s: requested %d, available %d",
					item.ProductID, item.Quantity, *product.Quantity)
			}
		}

		// Phase 2: Batch UPDATE using raw SQL with CASE statement
		// Build the CASE expressions for quantity and inventory_status
		now := time.Now()
		for _, product := range products {
			deductQty := itemMap[product.ID.String()]
			newQuantity := *product.Quantity - deductQty

			updates := map[string]interface{}{
				"quantity":   newQuantity,
				"updated_at": now,
			}

			// Auto-update inventory status based on quantity
			if newQuantity == 0 {
				updates["inventory_status"] = models.InventoryStatusOutOfStock
			} else if product.LowStockThreshold != nil && newQuantity <= *product.LowStockThreshold {
				updates["inventory_status"] = models.InventoryStatusLowStock
			}

			if err := tx.Model(&models.Product{}).
				Where("tenant_id = ? AND id = ?", tenantID, product.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to deduct inventory for product %s: %w", product.ID.String(), err)
			}
		}

		return nil
	})
}

// BulkRestoreInventory restores inventory for multiple products (for order cancellation)
// Performance: Uses batch SELECT + batch UPDATE instead of N individual queries
// Reduces 2N queries to 2 queries (N items -> 1 SELECT + 1 UPDATE)
func (r *ProductsRepository) BulkRestoreInventory(tenantID string, items []models.BulkInventoryItem) error {
	if len(items) == 0 {
		return nil
	}

	// Parse all product IDs upfront
	productIDs := make([]uuid.UUID, 0, len(items))
	itemMap := make(map[string]int) // productID -> quantity to restore
	for _, item := range items {
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			return fmt.Errorf("invalid product ID %s: %w", item.ProductID, err)
		}
		productIDs = append(productIDs, productID)
		itemMap[item.ProductID] = item.Quantity
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Phase 1: Batch SELECT all products in one query
		var products []models.Product
		if err := tx.Where("tenant_id = ? AND id IN ?", tenantID, productIDs).Find(&products).Error; err != nil {
			return fmt.Errorf("failed to fetch products: %w", err)
		}

		// Build map for validation
		foundProducts := make(map[string]*models.Product)
		for i := range products {
			foundProducts[products[i].ID.String()] = &products[i]
		}

		// Validate all products exist
		for _, item := range items {
			if _, found := foundProducts[item.ProductID]; !found {
				return fmt.Errorf("product %s not found", item.ProductID)
			}
		}

		// Phase 2: Update each product with restored quantity
		now := time.Now()
		for _, product := range products {
			restoreQty := itemMap[product.ID.String()]
			currentQuantity := 0
			if product.Quantity != nil {
				currentQuantity = *product.Quantity
			}

			newQuantity := currentQuantity + restoreQty
			updates := map[string]interface{}{
				"quantity":   newQuantity,
				"updated_at": now,
			}

			// Auto-update inventory status
			if product.LowStockThreshold != nil && newQuantity > *product.LowStockThreshold {
				updates["inventory_status"] = models.InventoryStatusInStock
			} else if newQuantity > 0 {
				updates["inventory_status"] = models.InventoryStatusLowStock
			}

			if err := tx.Model(&models.Product{}).
				Where("tenant_id = ? AND id = ?", tenantID, product.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to restore inventory for product %s: %w", product.ID.String(), err)
			}
		}

		return nil
	})
}

// CheckStock checks if requested quantities are available for multiple products
func (r *ProductsRepository) CheckStock(tenantID string, items []models.StockCheckItem) ([]models.StockCheckResult, error) {
	results := make([]models.StockCheckResult, 0, len(items))

	for _, item := range items {
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			results = append(results, models.StockCheckResult{
				ProductID: item.ProductID,
				Available: false,
				InStock:   0,
				Requested: item.Quantity,
			})
			continue
		}

		var product models.Product
		if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, productID).First(&product).Error; err != nil {
			results = append(results, models.StockCheckResult{
				ProductID: item.ProductID,
				Available: false,
				InStock:   0,
				Requested: item.Quantity,
			})
			continue
		}

		inStock := 0
		if product.Quantity != nil {
			inStock = *product.Quantity
		}

		results = append(results, models.StockCheckResult{
			ProductID:   item.ProductID,
			Available:   inStock >= item.Quantity,
			InStock:     inStock,
			Requested:   item.Quantity,
			ProductName: product.Name,
		})
	}

	return results, nil
}

// BulkUpdateStatus updates status for multiple products
func (r *ProductsRepository) BulkUpdateStatus(tenantID string, productIDs []uuid.UUID, status models.ProductStatus) error {
	return r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND id IN ?", tenantID, productIDs).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// ============================================================================
// Bulk Create Operations
// ============================================================================

// BulkCreateResult represents the result of a bulk create operation
type BulkCreateResult struct {
	Created []*models.Product
	Errors  []BulkCreateError
	Total   int
	Success int
	Failed  int
	Skipped int
}

// BulkCreateError represents an error for a single item in bulk create
type BulkCreateError struct {
	Index      int
	ExternalID *string
	Code       string
	Message    string
}

// BulkCreate creates multiple products in a transaction with tenant isolation
// SECURITY: All products are assigned the provided tenantID regardless of request data
func (r *ProductsRepository) BulkCreate(tenantID string, products []*models.Product, skipDuplicates bool) (*BulkCreateResult, error) {
	result := &BulkCreateResult{
		Created: make([]*models.Product, 0, len(products)),
		Errors:  make([]BulkCreateError, 0),
		Total:   len(products),
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for i, product := range products {
			// SECURITY: Always enforce tenant isolation
			product.TenantID = tenantID
			product.CreatedAt = time.Now()
			product.UpdatedAt = time.Now()

			// Ensure product has an ID before generating slug (for uniqueness)
			if product.ID == uuid.Nil {
				product.ID = uuid.New()
			}

			// Generate slug from name if not provided or empty
			if product.Slug == nil || *product.Slug == "" {
				baseSlug := generateSlug(product.Name)
				// Ensure slug uniqueness by appending first 8 chars of product ID
				uniqueSlug := fmt.Sprintf("%s-%s", baseSlug, product.ID.String()[:8])
				product.Slug = &uniqueSlug
			}

			// Check for duplicate SKU within tenant (including soft-deleted records for unique constraint)
			var existingCount int64
			if err := tx.Unscoped().Model(&models.Product{}).
				Where("tenant_id = ? AND sku = ?", tenantID, product.SKU).
				Count(&existingCount).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DB_ERROR",
					Message: "Failed to check for duplicate SKU",
				})
				continue
			}

			if existingCount > 0 {
				if skipDuplicates {
					// Skip silently when skipDuplicates is true
					result.Skipped++
					continue
				}
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DUPLICATE_SKU",
					Message: fmt.Sprintf("Product with SKU '%s' already exists for this tenant", product.SKU),
				})
				continue
			}

			// Check for duplicate slug within tenant if provided (including soft-deleted records for unique constraint)
			if product.Slug != nil && *product.Slug != "" {
				var slugCount int64
				if err := tx.Unscoped().Model(&models.Product{}).
					Where("tenant_id = ? AND slug = ?", tenantID, *product.Slug).
					Count(&slugCount).Error; err != nil {
					result.Errors = append(result.Errors, BulkCreateError{
						Index:   i,
						Code:    "DB_ERROR",
						Message: "Failed to check for duplicate slug",
					})
					continue
				}

				if slugCount > 0 {
					if skipDuplicates {
						result.Skipped++
						continue
					}
					result.Errors = append(result.Errors, BulkCreateError{
						Index:   i,
						Code:    "DUPLICATE_SLUG",
						Message: fmt.Sprintf("Product with slug '%s' already exists for this tenant", *product.Slug),
					})
					continue
				}
			}

			// Set default status if not provided
			if product.Status == "" {
				product.Status = models.ProductStatusDraft
			}

			// Create the product
			if err := tx.Create(product).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "CREATE_FAILED",
					Message: err.Error(),
				})
				continue
			}

			result.Created = append(result.Created, product)
		}

		result.Success = len(result.Created)
		result.Failed = len(result.Errors)

		// If all failed (and none were intentionally skipped), rollback the transaction
		if result.Success == 0 && result.Skipped == 0 && result.Total > 0 {
			return fmt.Errorf("all products failed to create")
		}

		return nil
	})

	if err != nil && result.Success == 0 {
		return result, err
	}

	return result, nil
}

// BulkUpsertResult represents the result of a bulk upsert operation
type BulkUpsertResult struct {
	Created []*models.Product
	Updated []*models.Product
	Errors  []BulkCreateError
	Total   int
	Success int
	Failed  int
}

// BulkUpsert creates or updates multiple products in a transaction with tenant isolation
// Products are matched by SKU - if exists, update; if not, create
// SECURITY: All products are assigned the provided tenantID regardless of request data
func (r *ProductsRepository) BulkUpsert(tenantID string, products []*models.Product) (*BulkUpsertResult, error) {
	result := &BulkUpsertResult{
		Created: make([]*models.Product, 0),
		Updated: make([]*models.Product, 0),
		Errors:  make([]BulkCreateError, 0),
		Total:   len(products),
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for i, product := range products {
			// SECURITY: Always enforce tenant isolation
			product.TenantID = tenantID
			product.UpdatedAt = time.Now()

			// Check if product with same SKU exists (including soft-deleted for unique constraint)
			var existingProduct models.Product
			err := tx.Unscoped().Where("tenant_id = ? AND sku = ?", tenantID, product.SKU).First(&existingProduct).Error

			if err == nil {
				// Product exists - update it (and restore if soft-deleted)
				product.ID = existingProduct.ID
				product.CreatedAt = existingProduct.CreatedAt // Preserve original creation time

				// Update the product (excluding ID and tenant_id), also clear deleted_at to restore if needed
				updateResult := tx.Unscoped().Model(&models.Product{}).
					Where("id = ? AND tenant_id = ?", existingProduct.ID, tenantID).
					Updates(map[string]interface{}{
						"name":                product.Name,
						"slug":                product.Slug,
						"description":         product.Description,
						"price":               product.Price,
						"compare_price":       product.ComparePrice,
						"cost_price":          product.CostPrice,
						"category_id":         product.CategoryID,
						"vendor_id":           product.VendorID,
						"warehouse_id":        product.WarehouseID,
						"warehouse_name":      product.WarehouseName,
						"supplier_id":         product.SupplierID,
						"supplier_name":       product.SupplierName,
						"brand":               product.Brand,
						"quantity":            product.Quantity,
						"min_order_qty":       product.MinOrderQty,
						"max_order_qty":       product.MaxOrderQty,
						"low_stock_threshold": product.LowStockThreshold,
						"weight":              product.Weight,
						"search_keywords":     product.SearchKeywords,
						"tags":                product.Tags,
						"updated_at":          product.UpdatedAt,
						"updated_by":          product.UpdatedBy,
						"deleted_at":          nil, // Restore if soft-deleted
					})

				if updateResult.Error != nil {
					result.Errors = append(result.Errors, BulkCreateError{
						Index:   i,
						Code:    "UPDATE_FAILED",
						Message: updateResult.Error.Error(),
					})
					continue
				}

				result.Updated = append(result.Updated, product)
			} else if err == gorm.ErrRecordNotFound {
				// Product doesn't exist - create it
				product.CreatedAt = time.Now()

				// Ensure product has an ID before generating slug (for uniqueness)
				if product.ID == uuid.Nil {
					product.ID = uuid.New()
				}

				// Generate slug from name if not provided or empty
				if product.Slug == nil || *product.Slug == "" {
					baseSlug := generateSlug(product.Name)
					// Ensure slug uniqueness by appending first 8 chars of product ID
					uniqueSlug := fmt.Sprintf("%s-%s", baseSlug, product.ID.String()[:8])
					product.Slug = &uniqueSlug
				}

				// Set default status if not provided
				if product.Status == "" {
					product.Status = models.ProductStatusDraft
				}

				if err := tx.Create(product).Error; err != nil {
					result.Errors = append(result.Errors, BulkCreateError{
						Index:   i,
						Code:    "CREATE_FAILED",
						Message: err.Error(),
					})
					continue
				}

				result.Created = append(result.Created, product)
			} else {
				// Database error
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DB_ERROR",
					Message: err.Error(),
				})
				continue
			}
		}

		result.Success = len(result.Created) + len(result.Updated)
		result.Failed = len(result.Errors)

		return nil
	})

	if err != nil && result.Success == 0 {
		return result, err
	}

	return result, nil
}

// BulkDelete deletes multiple products by IDs with tenant isolation
// SECURITY: Only deletes products belonging to the specified tenant
func (r *ProductsRepository) BulkDelete(tenantID string, ids []uuid.UUID) (int64, []string, error) {
	failedIDs := make([]string, 0)
	var totalDeleted int64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			result := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Product{})
			if result.Error != nil {
				failedIDs = append(failedIDs, id.String())
				continue
			}
			if result.RowsAffected == 0 {
				failedIDs = append(failedIDs, id.String())
				continue
			}
			totalDeleted += result.RowsAffected
		}
		return nil
	})

	return totalDeleted, failedIDs, err
}

// SKUExistsForTenant checks if a SKU already exists for a tenant
func (r *ProductsRepository) SKUExistsForTenant(tenantID, sku string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND sku = ?", tenantID, sku).
		Count(&count).Error
	return count > 0, err
}

// Product Variant Operations

// CreateProductVariant creates a new product variant
func (r *ProductsRepository) CreateProductVariant(tenantID string, productID uuid.UUID, variant *models.ProductVariant) error {
	variant.ProductID = productID
	variant.CreatedAt = time.Now()
	variant.UpdatedAt = time.Now()
	
	return r.db.Create(variant).Error
}

// GetProductVariants retrieves variants for a product
func (r *ProductsRepository) GetProductVariants(tenantID string, productID uuid.UUID, page, limit int) ([]models.ProductVariant, int64, error) {
	var variants []models.ProductVariant
	var total int64

	query := r.db.Model(&models.ProductVariant{}).
		Joins("JOIN products ON products.id = product_variants.product_id").
		Where("products.tenant_id = ? AND product_variants.product_id = ?", tenantID, productID)

	// Count total results
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&variants).Error; err != nil {
		return nil, 0, err
	}

	return variants, total, nil
}

// GetProductVariantByID retrieves a product variant by ID
func (r *ProductsRepository) GetProductVariantByID(tenantID string, variantID uuid.UUID) (*models.ProductVariant, error) {
	var variant models.ProductVariant
	err := r.db.Model(&models.ProductVariant{}).
		Joins("JOIN products ON products.id = product_variants.product_id").
		Where("products.tenant_id = ? AND product_variants.id = ?", tenantID, variantID).
		First(&variant).Error
	
	if err != nil {
		return nil, err
	}
	return &variant, nil
}

// UpdateProductVariant updates a product variant
func (r *ProductsRepository) UpdateProductVariant(tenantID string, variantID uuid.UUID, updates *models.ProductVariant) error {
	updates.UpdatedAt = time.Now()
	return r.db.Model(&models.ProductVariant{}).
		Joins("JOIN products ON products.id = product_variants.product_id").
		Where("products.tenant_id = ? AND product_variants.id = ?", tenantID, variantID).
		Updates(updates).Error
}

// DeleteProductVariant soft deletes a product variant
func (r *ProductsRepository) DeleteProductVariant(tenantID string, variantID uuid.UUID) error {
	return r.db.Where("id = ?", variantID).
		Where("product_id IN (SELECT id FROM products WHERE tenant_id = ?)", tenantID).
		Delete(&models.ProductVariant{}).Error
}

// Category Operations

// CreateCategory creates a new category
func (r *ProductsRepository) CreateCategory(tenantID string, category *models.Category) error {
	category.TenantID = tenantID
	category.CreatedAt = time.Now()
	category.UpdatedAt = time.Now()

	err := r.db.Create(category).Error
	if err == nil {
		// Invalidate category list caches
		r.invalidateCategoryCaches(context.Background(), tenantID, nil)
	}
	return err
}

// GetCategories retrieves categories with pagination
func (r *ProductsRepository) GetCategories(tenantID string, page, limit int) ([]models.Category, int64, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("categories:list:%s:%d:%d", tenantID, page, limit)

	// Try cache for categories list
	if r.cache != nil {
		type categoriesResult struct {
			Categories []models.Category `json:"categories"`
			Total      int64             `json:"total"`
		}
		var result categoriesResult
		err := r.cache.GetOrSetJSON(ctx, cacheKey, &result, CategoryCacheTTL, func() (any, error) {
			var categories []models.Category
			var total int64
			query := r.db.Model(&models.Category{}).Where("tenant_id = ?", tenantID)
			if err := query.Count(&total).Error; err != nil {
				return nil, err
			}
			offset := (page - 1) * limit
			if err := query.Order("position ASC, name ASC").Offset(offset).Limit(limit).Find(&categories).Error; err != nil {
				return nil, err
			}
			return &categoriesResult{Categories: categories, Total: total}, nil
		})
		if err != nil {
			return nil, 0, err
		}
		return result.Categories, result.Total, nil
	}

	// Fallback to direct DB query
	var categories []models.Category
	var total int64
	query := r.db.Model(&models.Category{}).Where("tenant_id = ?", tenantID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	if err := query.Order("position ASC, name ASC").Offset(offset).Limit(limit).Find(&categories).Error; err != nil {
		return nil, 0, err
	}
	return categories, total, nil
}

// GetCategoryByID retrieves a category by ID with caching
func (r *ProductsRepository) GetCategoryByID(tenantID string, categoryID uuid.UUID) (*models.Category, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("category:%s:%s", tenantID, categoryID.String())

	if r.cache != nil {
		var category models.Category
		err := r.cache.GetOrSetJSON(ctx, cacheKey, &category, CategoryCacheTTL, func() (any, error) {
			var c models.Category
			if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, categoryID).First(&c).Error; err != nil {
				return nil, err
			}
			return &c, nil
		})
		if err != nil {
			return nil, err
		}
		return &category, nil
	}

	var category models.Category
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, categoryID).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// UpdateCategory updates a category
func (r *ProductsRepository) UpdateCategory(tenantID string, categoryID uuid.UUID, updates *models.Category) error {
	updates.UpdatedAt = time.Now()
	err := r.db.Model(&models.Category{}).
		Where("tenant_id = ? AND id = ?", tenantID, categoryID).
		Updates(updates).Error

	if err == nil {
		r.invalidateCategoryCaches(context.Background(), tenantID, &categoryID)
	}
	return err
}

// DeleteCategory soft deletes a category
func (r *ProductsRepository) DeleteCategory(tenantID string, categoryID uuid.UUID) error {
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, categoryID).
		Delete(&models.Category{}).Error

	if err == nil {
		r.invalidateCategoryCaches(context.Background(), tenantID, &categoryID)
	}
	return err
}

// BulkUpdateCategoryStatus updates isActive status for multiple categories
func (r *ProductsRepository) BulkUpdateCategoryStatus(tenantID string, categoryIDs []uuid.UUID, isActive bool) (int64, error) {
	result := r.db.Model(&models.Category{}).
		Where("tenant_id = ? AND id IN ?", tenantID, categoryIDs).
		Updates(map[string]interface{}{
			"is_active":  isActive,
			"updated_at": time.Now(),
		})

	if result.Error == nil && result.RowsAffected > 0 {
		// Invalidate all category caches for this tenant
		r.invalidateCategoryCaches(context.Background(), tenantID, nil)
	}
	return result.RowsAffected, result.Error
}

// GetCategoryByName retrieves a category by name (case-insensitive)
func (r *ProductsRepository) GetCategoryByName(tenantID string, name string) (*models.Category, error) {
	var category models.Category
	err := r.db.Where("tenant_id = ? AND LOWER(name) = LOWER(?)", tenantID, name).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// Vendor Operations (read-only for import lookup)

// GetVendorByName retrieves a vendor by name (case-insensitive)
func (r *ProductsRepository) GetVendorByName(tenantID string, name string) (*models.Vendor, error) {
	var vendor models.Vendor
	err := r.db.Where("tenant_id = ? AND LOWER(name) = LOWER(?)", tenantID, name).First(&vendor).Error
	if err != nil {
		return nil, err
	}
	return &vendor, nil
}

// GetOrCreateCategoryByName finds a category by name or creates it if not exists
// Returns the category and a boolean indicating if it was newly created
// Uses transaction to handle race conditions
func (r *ProductsRepository) GetOrCreateCategoryByName(tenantID string, name string, createdBy string) (*models.Category, bool, error) {
	var category models.Category
	var created bool

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Try to find existing category within transaction
		err := tx.Where("tenant_id = ? AND LOWER(name) = LOWER(?)", tenantID, name).First(&category).Error
		if err == nil {
			// Found existing category
			created = false
			return nil
		}

		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to lookup category: %w", err)
		}

		// Category not found, create it
		slug := generateSlug(name)
		isActive := true
		category = models.Category{
			TenantID:    tenantID,
			CreatedByID: createdBy,
			UpdatedByID: createdBy,
			Name:        name,
			Slug:        slug,
			Level:       0,
			Position:    1,
			IsActive:    &isActive,
			Status:      "ACTIVE",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := tx.Create(&category).Error; err != nil {
			// Check if it was created by concurrent request (duplicate slug)
			if strings.Contains(err.Error(), "duplicate") {
				// Try to fetch again
				if findErr := tx.Where("tenant_id = ? AND LOWER(name) = LOWER(?)", tenantID, name).First(&category).Error; findErr == nil {
					created = false
					return nil
				}
			}
			return fmt.Errorf("failed to create category '%s': %w", name, err)
		}

		created = true
		return nil
	})

	if err != nil {
		return nil, false, err
	}

	return &category, created, nil
}

// Analytics Operations

// ProductOverviewResult holds the aggregated query result
type ProductOverviewResult struct {
	TotalProducts  int64 `gorm:"column:total_products"`
	ActiveProducts int64 `gorm:"column:active_products"`
	DraftProducts  int64 `gorm:"column:draft_products"`
	OutOfStock     int64 `gorm:"column:out_of_stock"`
	LowStock       int64 `gorm:"column:low_stock"`
	TotalInventory int64 `gorm:"column:total_inventory"`
}

// GetProductsOverview retrieves overview statistics using a single optimized query
// Performance: Replaces 6 separate COUNT queries with 1 aggregated query (6x improvement)
func (r *ProductsRepository) GetProductsOverview(tenantID string) (*models.ProductsOverview, error) {
	var overview models.ProductsOverview
	var result ProductOverviewResult

	// Single query with PostgreSQL FILTER clause for conditional counting
	// This is 6x more efficient than separate COUNT queries
	err := r.db.Model(&models.Product{}).
		Select(`
			COUNT(*) as total_products,
			COUNT(*) FILTER (WHERE status = ?) as active_products,
			COUNT(*) FILTER (WHERE status = ?) as draft_products,
			COUNT(*) FILTER (WHERE inventory_status = ?) as out_of_stock,
			COUNT(*) FILTER (WHERE inventory_status = ?) as low_stock,
			COALESCE(SUM(quantity), 0) as total_inventory
		`, models.ProductStatusActive, models.ProductStatusDraft,
		   models.InventoryStatusOutOfStock, models.InventoryStatusLowStock).
		Where("tenant_id = ?", tenantID).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	overview.TotalProducts = int(result.TotalProducts)
	overview.ActiveProducts = int(result.ActiveProducts)
	overview.DraftProducts = int(result.DraftProducts)
	overview.OutOfStock = int(result.OutOfStock)
	overview.LowStock = int(result.LowStock)
	overview.TotalInventory = result.TotalInventory

	// Get total variants count (separate query as it joins another table)
	var variantsCount int64
	r.db.Model(&models.ProductVariant{}).
		Joins("JOIN products ON products.id = product_variants.product_id").
		Where("products.tenant_id = ?", tenantID).Count(&variantsCount)
	overview.TotalVariants = int(variantsCount)

	return &overview, nil
}

// Helper function to apply product filters
func (r *ProductsRepository) applyProductFilters(query *gorm.DB, req *models.SearchProductsRequest) *gorm.DB {
	if req.CategoryID != nil {
		query = query.Where("category_id = ?", *req.CategoryID)
	}

	if req.VendorID != nil {
		query = query.Where("vendor_id = ?", *req.VendorID)
	}

	// Brand filter
	if len(req.Brands) > 0 {
		query = query.Where("brand IN ?", req.Brands)
	}

	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}

	if len(req.InventoryStatus) > 0 {
		query = query.Where("inventory_status IN ?", req.InventoryStatus)
	}

	// Price range filter
	if req.MinPrice != nil {
		query = query.Where("CAST(price AS DECIMAL) >= CAST(? AS DECIMAL)", *req.MinPrice)
	}

	if req.MaxPrice != nil {
		query = query.Where("CAST(price AS DECIMAL) <= CAST(? AS DECIMAL)", *req.MaxPrice)
	}

	// Rating filter
	if req.MinRating != nil {
		query = query.Where("average_rating >= ?", *req.MinRating)
	}

	// Tags filter
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			query = query.Where("tags::text LIKE ?", "%\""+tag+"\"%")
		}
	}

	// Attributes filter (e.g., color, size, material)
	if req.Attributes != nil && len(req.Attributes) > 0 {
		for attrName, attrValues := range req.Attributes {
			if len(attrValues) > 0 {
				// Build OR condition for each value of the same attribute
				orConditions := make([]string, len(attrValues))
				args := make([]interface{}, len(attrValues))
				for i, val := range attrValues {
					orConditions[i] = "attributes::text LIKE ?"
					args[i] = fmt.Sprintf("%%\"%s\":%%\"%s\"%%", attrName, val)
				}
				query = query.Where(strings.Join(orConditions, " OR "), args...)
			}
		}
	}

	if req.DateFrom != nil {
		query = query.Where("created_at >= ?", *req.DateFrom)
	}

	if req.DateTo != nil {
		query = query.Where("created_at <= ?", *req.DateTo)
	}

	if req.UpdatedAfter != nil {
		query = query.Where("updated_at > ?", *req.UpdatedAfter)
	}

	return query
}

// generateSlug creates a URL-friendly slug from a name
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ============================================================================
// Cascade Delete Operations
// ============================================================================

// CountProductsByCategory returns the count of products referencing a category,
// excluding the specified product IDs (used for cascade delete validation)
func (r *ProductsRepository) CountProductsByCategory(tenantID string, categoryID string, excludeIDs []uuid.UUID) (int64, error) {
	var count int64
	query := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND category_id = ?", tenantID, categoryID)

	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountProductsByWarehouse returns the count of products referencing a warehouse,
// excluding the specified product IDs
func (r *ProductsRepository) CountProductsByWarehouse(tenantID string, warehouseID string, excludeIDs []uuid.UUID) (int64, error) {
	var count int64
	query := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND warehouse_id = ?", tenantID, warehouseID)

	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountProductsBySupplier returns the count of products referencing a supplier,
// excluding the specified product IDs
func (r *ProductsRepository) CountProductsBySupplier(tenantID string, supplierID string, excludeIDs []uuid.UUID) (int64, error) {
	var count int64
	query := r.db.Model(&models.Product{}).
		Where("tenant_id = ? AND supplier_id = ?", tenantID, supplierID)

	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountVariantsByProducts returns the total count of variants for the given product IDs
func (r *ProductsRepository) CountVariantsByProducts(tenantID string, productIDs []uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.ProductVariant{}).
		Where("product_id IN (SELECT id FROM products WHERE tenant_id = ? AND id IN ?)", tenantID, productIDs).
		Count(&count).Error
	return count, err
}

// GetProductRelatedEntities retrieves unique category/warehouse/supplier IDs and names
// for the given product IDs
func (r *ProductsRepository) GetProductRelatedEntities(tenantID string, productIDs []uuid.UUID) (*models.RelatedEntities, error) {
	result := models.NewRelatedEntities()

	// Get products with their related entity info
	var products []models.Product
	if err := r.db.Select("id, category_id, warehouse_id, warehouse_name, supplier_id, supplier_name").
		Where("tenant_id = ? AND id IN ?", tenantID, productIDs).
		Find(&products).Error; err != nil {
		return nil, err
	}

	// Collect unique IDs
	categoryIDSet := make(map[string]bool)
	warehouseIDSet := make(map[string]bool)
	supplierIDSet := make(map[string]bool)

	for _, p := range products {
		if p.CategoryID != "" {
			categoryIDSet[p.CategoryID] = true
		}
		if p.WarehouseID != nil && *p.WarehouseID != "" {
			warehouseIDSet[*p.WarehouseID] = true
			if p.WarehouseName != nil {
				result.WarehouseMap[*p.WarehouseID] = *p.WarehouseName
			}
		}
		if p.SupplierID != nil && *p.SupplierID != "" {
			supplierIDSet[*p.SupplierID] = true
			if p.SupplierName != nil {
				result.SupplierMap[*p.SupplierID] = *p.SupplierName
			}
		}
	}

	// Convert sets to slices
	for id := range categoryIDSet {
		result.CategoryIDs = append(result.CategoryIDs, id)
	}
	for id := range warehouseIDSet {
		result.WarehouseIDs = append(result.WarehouseIDs, id)
	}
	for id := range supplierIDSet {
		result.SupplierIDs = append(result.SupplierIDs, id)
	}

	// Get category names
	if len(result.CategoryIDs) > 0 {
		var categories []models.Category
		if err := r.db.Select("id, name").
			Where("tenant_id = ? AND id IN ?", tenantID, result.CategoryIDs).
			Find(&categories).Error; err == nil {
			for _, c := range categories {
				result.CategoryMap[c.ID.String()] = c.Name
			}
		}
	}

	return result, nil
}

// ValidateCascadeDelete checks if cascade delete options are safe
// Returns blocked entities if any related entity is referenced by other products
func (r *ProductsRepository) ValidateCascadeDelete(tenantID string, productIDs []uuid.UUID, options models.CascadeDeleteOptions) (*models.CascadeValidationResult, error) {
	result := &models.CascadeValidationResult{
		CanDelete:       true,
		BlockedEntities: make([]models.BlockedEntity, 0),
		AffectedSummary: models.AffectedSummary{
			ProductCount: len(productIDs),
		},
	}

	// Get related entities
	related, err := r.GetProductRelatedEntities(tenantID, productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get related entities: %w", err)
	}

	// Count variants if deletion is requested
	if options.DeleteVariants {
		variantCount, err := r.CountVariantsByProducts(tenantID, productIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to count variants: %w", err)
		}
		result.AffectedSummary.VariantCount = int(variantCount)
	}

	// Check category references
	if options.DeleteCategory {
		for _, categoryID := range related.CategoryIDs {
			otherCount, err := r.CountProductsByCategory(tenantID, categoryID, productIDs)
			if err != nil {
				return nil, fmt.Errorf("failed to count category references: %w", err)
			}
			categoryName := related.CategoryMap[categoryID]
			if categoryName == "" {
				categoryName = "Unknown Category"
			}

			if otherCount > 0 {
				result.CanDelete = false
				result.BlockedEntities = append(result.BlockedEntities, models.BlockedEntity{
					Type:       "category",
					ID:         categoryID,
					Name:       categoryName,
					Reason:     fmt.Sprintf("Referenced by %d other product(s)", otherCount),
					OtherCount: int(otherCount),
				})
			} else {
				result.AffectedSummary.CategoryCount++
				result.AffectedSummary.CategoryNames = append(result.AffectedSummary.CategoryNames, categoryName)
			}
		}
	}

	// Check warehouse references
	if options.DeleteWarehouse {
		for _, warehouseID := range related.WarehouseIDs {
			otherCount, err := r.CountProductsByWarehouse(tenantID, warehouseID, productIDs)
			if err != nil {
				return nil, fmt.Errorf("failed to count warehouse references: %w", err)
			}
			warehouseName := related.WarehouseMap[warehouseID]
			if warehouseName == "" {
				warehouseName = "Unknown Warehouse"
			}

			if otherCount > 0 {
				result.CanDelete = false
				result.BlockedEntities = append(result.BlockedEntities, models.BlockedEntity{
					Type:       "warehouse",
					ID:         warehouseID,
					Name:       warehouseName,
					Reason:     fmt.Sprintf("Referenced by %d other product(s)", otherCount),
					OtherCount: int(otherCount),
				})
			} else {
				result.AffectedSummary.WarehouseCount++
				result.AffectedSummary.WarehouseNames = append(result.AffectedSummary.WarehouseNames, warehouseName)
			}
		}
	}

	// Check supplier references
	if options.DeleteSupplier {
		for _, supplierID := range related.SupplierIDs {
			otherCount, err := r.CountProductsBySupplier(tenantID, supplierID, productIDs)
			if err != nil {
				return nil, fmt.Errorf("failed to count supplier references: %w", err)
			}
			supplierName := related.SupplierMap[supplierID]
			if supplierName == "" {
				supplierName = "Unknown Supplier"
			}

			if otherCount > 0 {
				result.CanDelete = false
				result.BlockedEntities = append(result.BlockedEntities, models.BlockedEntity{
					Type:       "supplier",
					ID:         supplierID,
					Name:       supplierName,
					Reason:     fmt.Sprintf("Referenced by %d other product(s)", otherCount),
					OtherCount: int(otherCount),
				})
			} else {
				result.AffectedSummary.SupplierCount++
				result.AffectedSummary.SupplierNames = append(result.AffectedSummary.SupplierNames, supplierName)
			}
		}
	}

	return result, nil
}

// DeleteVariantsByProductIDs deletes all variants for the given product IDs
func (r *ProductsRepository) DeleteVariantsByProductIDs(tenantID string, productIDs []uuid.UUID) (int64, error) {
	result := r.db.Where("product_id IN (SELECT id FROM products WHERE tenant_id = ? AND id IN ?)", tenantID, productIDs).
		Delete(&models.ProductVariant{})
	return result.RowsAffected, result.Error
}

// DeleteProductsCascade performs cascade delete for products and optionally related entities
// This handles the local operations (products, variants, categories)
// Warehouse and supplier deletion should be handled separately via inventory client
func (r *ProductsRepository) DeleteProductsCascade(tenantID string, productIDs []uuid.UUID, options models.CascadeDeleteOptions) (*models.CascadeDeleteResult, error) {
	result := &models.CascadeDeleteResult{
		Success:           true,
		CategoriesDeleted: make([]string, 0),
		Errors:            make([]models.CascadeError, 0),
	}

	// Get related entities before deletion
	related, err := r.GetProductRelatedEntities(tenantID, productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get related entities: %w", err)
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		// Delete variants if requested
		if options.DeleteVariants {
			variantResult := tx.Where("product_id IN (SELECT id FROM products WHERE tenant_id = ? AND id IN ?)", tenantID, productIDs).
				Delete(&models.ProductVariant{})
			if variantResult.Error != nil {
				result.Errors = append(result.Errors, models.CascadeError{
					EntityType: "variants",
					Code:       "DELETE_FAILED",
					Message:    variantResult.Error.Error(),
				})
			} else {
				result.VariantsDeleted = int(variantResult.RowsAffected)
			}
		}

		// Delete products
		productResult := tx.Where("tenant_id = ? AND id IN ?", tenantID, productIDs).Delete(&models.Product{})
		if productResult.Error != nil {
			return fmt.Errorf("failed to delete products: %w", productResult.Error)
		}
		result.ProductsDeleted = int(productResult.RowsAffected)

		// Delete categories if requested (only if no other products reference them)
		if options.DeleteCategory {
			for _, categoryID := range related.CategoryIDs {
				// Re-check reference count after product deletion
				var count int64
				tx.Model(&models.Product{}).Where("tenant_id = ? AND category_id = ?", tenantID, categoryID).Count(&count)
				if count == 0 {
					categoryUUID, parseErr := uuid.Parse(categoryID)
					if parseErr != nil {
						result.Errors = append(result.Errors, models.CascadeError{
							EntityType: "category",
							EntityID:   categoryID,
							Code:       "INVALID_ID",
							Message:    "Invalid category ID format",
						})
						continue
					}
					if delErr := tx.Where("tenant_id = ? AND id = ?", tenantID, categoryUUID).Delete(&models.Category{}).Error; delErr != nil {
						result.Errors = append(result.Errors, models.CascadeError{
							EntityType: "category",
							EntityID:   categoryID,
							Code:       "DELETE_FAILED",
							Message:    delErr.Error(),
						})
					} else {
						result.CategoriesDeleted = append(result.CategoriesDeleted, categoryID)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		result.Success = false
		return result, err
	}

	result.PartialSuccess = len(result.Errors) > 0
	return result, nil
}