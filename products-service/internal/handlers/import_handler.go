package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"products-service/internal/clients"
	"products-service/internal/models"
	"products-service/internal/repository"
	"github.com/xuri/excelize/v2"
)

const (
	DefaultBatchSize = 100 // Default rows per batch
	MaxBatchSize     = 500 // Maximum rows per batch
	DefaultRetries   = 2   // Default retry attempts for failed batches
	MaxRetries       = 5   // Maximum retry attempts
)

type ImportHandler struct {
	repo             *repository.ProductsRepository
	inventoryClient  *clients.InventoryClient
	categoriesClient *clients.CategoriesClient
	vendorClient     *clients.VendorClient
}

func NewImportHandler(repo *repository.ProductsRepository, inventoryClient *clients.InventoryClient, categoriesClient *clients.CategoriesClient, vendorClient *clients.VendorClient) *ImportHandler {
	return &ImportHandler{
		repo:             repo,
		inventoryClient:  inventoryClient,
		categoriesClient: categoriesClient,
		vendorClient:     vendorClient,
	}
}

// GetImportTemplate returns the import template definition or file
// GET /api/v1/products/import/template
func (h *ImportHandler) GetImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	template := models.ProductImportTemplate()

	switch format {
	case "csv":
		h.generateCSVTemplate(c, template)
	case "xlsx":
		h.generateXLSXTemplate(c, template)
	default:
		// Return JSON template definition
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"template": template,
		})
	}
}

// generateCSVTemplate generates and downloads a CSV template (headers only)
func (h *ImportHandler) generateCSVTemplate(c *gin.Context, template models.ImportTemplate) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=products_import_template.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write header row only
	headers := make([]string, len(template.Columns))
	for i, col := range template.Columns {
		headers[i] = col.Name
	}
	writer.Write(headers)
}

// generateXLSXTemplate generates and downloads an Excel template
func (h *ImportHandler) generateXLSXTemplate(c *gin.Context, template models.ImportTemplate) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Products"
	f.SetSheetName("Sheet1", sheetName)

	// Style for header row
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	// Style for required columns
	requiredStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"C65911"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	// Write header row only (no sample data)
	for i, col := range template.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		headerText := col.Name
		if col.Required {
			headerText = col.Name + " *"
		}
		f.SetCellValue(sheetName, cell, headerText)

		if col.Required {
			f.SetCellStyle(sheetName, cell, cell, requiredStyle)
		} else {
			f.SetCellStyle(sheetName, cell, cell, headerStyle)
		}

		// Set column width
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, colName, colName, 20)
	}

	// Add Instructions sheet
	f.NewSheet("Instructions")
	f.SetCellValue("Instructions", "A1", "Product Import Instructions")

	// Smart Import Feature
	f.SetCellValue("Instructions", "A3", "SMART IMPORT FEATURE:")
	f.SetCellValue("Instructions", "A4", "You can use EITHER UUIDs (categoryId, vendorId) OR names (categoryName, vendorName):")
	f.SetCellValue("Instructions", "A5", "- categoryName: If provided, the system will look up the category by name. If not found, it will auto-create it.")
	f.SetCellValue("Instructions", "A6", "- vendorName: If provided, the system will look up the vendor by name. Vendor MUST exist (create vendors first).")
	f.SetCellValue("Instructions", "A7", "- You can mix and match: use categoryName for one product and categoryId for another.")

	f.SetCellValue("Instructions", "A9", "IMPORT ORDER (if using names):")
	f.SetCellValue("Instructions", "A10", "1. Create Vendors first (they must exist before importing products with vendorName)")
	f.SetCellValue("Instructions", "A11", "2. Import Products (categories will be auto-created if using categoryName)")

	f.SetCellValue("Instructions", "A13", "Column Definitions:")
	f.SetCellValue("Instructions", "A14", "Column")
	f.SetCellValue("Instructions", "B14", "Description")
	f.SetCellValue("Instructions", "C14", "Required")
	f.SetCellValue("Instructions", "D14", "Type")
	f.SetCellValue("Instructions", "E14", "Example")

	for i, col := range template.Columns {
		row := i + 15
		f.SetCellValue("Instructions", fmt.Sprintf("A%d", row), col.Name)
		f.SetCellValue("Instructions", fmt.Sprintf("B%d", row), col.Description)
		required := "Optional"
		if col.Required {
			required = "Required"
		}
		f.SetCellValue("Instructions", fmt.Sprintf("C%d", row), required)
		f.SetCellValue("Instructions", fmt.Sprintf("D%d", row), col.Type)
		f.SetCellValue("Instructions", fmt.Sprintf("E%d", row), col.Example)
	}

	f.SetColWidth("Instructions", "A", "A", 25)
	f.SetColWidth("Instructions", "B", "B", 60)
	f.SetColWidth("Instructions", "C", "C", 15)
	f.SetColWidth("Instructions", "D", "D", 15)
	f.SetColWidth("Instructions", "E", "E", 40)

	// Set active sheet to Products
	sheetIdx, _ := f.GetSheetIndex(sheetName)
	f.SetActiveSheet(sheetIdx)

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=products_import_template.xlsx")

	f.Write(c.Writer)
}

// ImportProducts imports products from CSV or Excel file with enterprise-grade batch processing
// POST /api/v1/products/import
// Supports large file imports with configurable batch sizes, retry logic, and partial commits
func (h *ImportHandler) ImportProducts(c *gin.Context) {
	// Use IstioAuth context keys
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	userEmail, _ := c.Get("user_email")
	startTime := time.Now()

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FILE_REQUIRED",
				Message: "Please upload a CSV or Excel file",
			},
		})
		return
	}
	defer file.Close()

	// Get import options
	skipDuplicates := c.DefaultPostForm("skipDuplicates", "false") == "true"
	updateExisting := c.DefaultPostForm("updateExisting", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	// Get batch processing options
	batchSize := DefaultBatchSize
	if bs := c.DefaultPostForm("batchSize", ""); bs != "" {
		if parsed, err := strconv.Atoi(bs); err == nil && parsed > 0 {
			batchSize = parsed
			if batchSize > MaxBatchSize {
				batchSize = MaxBatchSize
			}
		}
	}

	maxRetries := DefaultRetries
	if mr := c.DefaultPostForm("maxRetries", ""); mr != "" {
		if parsed, err := strconv.Atoi(mr); err == nil && parsed >= 0 {
			maxRetries = parsed
			if maxRetries > MaxRetries {
				maxRetries = MaxRetries
			}
		}
	}

	// Determine file format
	filename := header.Filename
	var format models.ImportFormat
	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		format = models.ImportFormatCSV
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		format = models.ImportFormatXLSX
	} else {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_FORMAT",
				Message: "Only CSV and XLSX files are supported",
			},
		})
		return
	}

	// Parse file
	var rows []map[string]string
	var parseErr error

	if format == models.ImportFormatCSV {
		rows, parseErr = h.parseCSV(file)
	} else {
		rows, parseErr = h.parseXLSX(file)
	}

	if parseErr != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "PARSE_ERROR",
				Message: parseErr.Error(),
			},
		})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "EMPTY_FILE",
				Message: "The file contains no data rows",
			},
		})
		return
	}

	// Process import with batch processing for enterprise-grade performance
	userEmailStr := ""
	if userEmail != nil {
		userEmailStr = userEmail.(string)
	}
	result := h.processImportWithBatching(
		tenantID.(string),
		userID.(string),
		userEmailStr,
		rows,
		skipDuplicates,
		updateExisting,
		validateOnly,
		batchSize,
		maxRetries,
	)

	// Add processing time metrics
	result.ProcessingMs = time.Since(startTime).Milliseconds()
	if result.TotalBatches > 0 {
		result.AvgBatchMs = result.ProcessingMs / int64(result.TotalBatches)
	}

	c.JSON(http.StatusOK, result)
}

// processImportWithBatching handles large imports with batch processing, retry logic, and partial commits
func (h *ImportHandler) processImportWithBatching(
	tenantID, userID, userEmail string,
	rows []map[string]string,
	skipDuplicates, updateExisting, validateOnly bool,
	batchSize, maxRetries int,
) *models.EnhancedImportResult {
	totalRows := len(rows)
	totalBatches := (totalRows + batchSize - 1) / batchSize

	result := &models.EnhancedImportResult{
		TotalRows:    totalRows,
		TotalBatches: totalBatches,
		BatchResults: make([]models.BatchResult, 0, totalBatches),
		Errors:       make([]models.ImportRowError, 0),
		CreatedIDs:   make([]string, 0),
		UpdatedIDs:   make([]string, 0),
	}

	// Pre-warm caches for better performance across batches
	categoryCache := make(map[string]string)
	vendorCache := make(map[string]string)
	warehouseCache := make(map[string]*struct{ ID, Name string })
	supplierCache := make(map[string]*struct{ ID, Name string })
	var cacheMutex sync.RWMutex

	// Process batches
	for batchNum := 0; batchNum < totalBatches; batchNum++ {
		startIdx := batchNum * batchSize
		endIdx := startIdx + batchSize
		if endIdx > totalRows {
			endIdx = totalRows
		}

		batchRows := rows[startIdx:endIdx]
		startRow, _ := strconv.Atoi(batchRows[0]["_row"])
		endRow, _ := strconv.Atoi(batchRows[len(batchRows)-1]["_row"])

		// Process batch with retry logic
		batchResult := h.processBatchWithRetry(
			tenantID, userID, userEmail,
			batchRows, batchNum+1, startRow, endRow,
			skipDuplicates, updateExisting, validateOnly,
			maxRetries,
			categoryCache, vendorCache, warehouseCache, supplierCache,
			&cacheMutex,
		)

		result.BatchResults = append(result.BatchResults, batchResult)

		// Aggregate results
		result.CreatedCount += batchResult.CreatedCount
		result.UpdatedCount += batchResult.UpdatedCount
		result.FailedCount += batchResult.FailedCount
		result.SkippedCount += batchResult.SkippedCount
		result.Errors = append(result.Errors, batchResult.Errors...)
		result.CreatedIDs = append(result.CreatedIDs, batchResult.CreatedIDs...)
		result.UpdatedIDs = append(result.UpdatedIDs, batchResult.UpdatedIDs...)
	}

	// For validation mode (validateOnly=true), SuccessCount = valid rows (not created/updated)
	// For actual import, SuccessCount = created + updated
	if validateOnly {
		// In validation mode, success count is total rows minus failed rows
		result.SuccessCount = totalRows - result.FailedCount
		result.Success = result.SuccessCount > 0
	} else {
		result.SuccessCount = result.CreatedCount + result.UpdatedCount
		result.Success = result.SuccessCount > 0 || result.SkippedCount > 0
	}

	return result
}

// processBatchWithRetry processes a single batch with retry logic for transient failures
func (h *ImportHandler) processBatchWithRetry(
	tenantID, userID, userEmail string,
	rows []map[string]string,
	batchNum, startRow, endRow int,
	skipDuplicates, updateExisting, validateOnly bool,
	maxRetries int,
	categoryCache map[string]string,
	vendorCache map[string]string,
	warehouseCache map[string]*struct{ ID, Name string },
	supplierCache map[string]*struct{ ID, Name string },
	cacheMutex *sync.RWMutex,
) models.BatchResult {
	var batchResult models.BatchResult
	batchResult.BatchNumber = batchNum
	batchResult.StartRow = startRow
	batchResult.EndRow = endRow

	for retry := 0; retry <= maxRetries; retry++ {
		batchResult.RetryCount = retry

		// Process the batch
		innerResult := h.processSingleBatch(
			tenantID, userID, userEmail,
			rows, skipDuplicates, updateExisting, validateOnly,
			categoryCache, vendorCache, warehouseCache, supplierCache,
			cacheMutex,
		)

		batchResult.CreatedCount = innerResult.CreatedCount
		batchResult.UpdatedCount = innerResult.UpdatedCount
		batchResult.FailedCount = innerResult.FailedCount
		batchResult.SkippedCount = innerResult.SkippedCount
		batchResult.Errors = innerResult.Errors
		batchResult.CreatedIDs = innerResult.CreatedIDs
		batchResult.UpdatedIDs = innerResult.UpdatedIDs
		batchResult.Success = innerResult.Success

		// If successful or if errors are validation errors (not transient), don't retry
		if batchResult.Success || !h.hasTransientErrors(batchResult.Errors) {
			break
		}

		// Wait before retry (exponential backoff)
		if retry < maxRetries {
			time.Sleep(time.Duration(100*(1<<retry)) * time.Millisecond)
		}
	}

	return batchResult
}

// hasTransientErrors checks if any errors are transient (DB connection, timeout, etc.)
func (h *ImportHandler) hasTransientErrors(errors []models.ImportRowError) bool {
	for _, err := range errors {
		if err.Code == "DB_ERROR" || err.Code == "BULK_CREATE_FAILED" || err.Code == "BULK_UPSERT_FAILED" {
			return true
		}
	}
	return false
}

// processSingleBatch processes a single batch of rows
func (h *ImportHandler) processSingleBatch(
	tenantID, userID, userEmail string,
	rows []map[string]string,
	skipDuplicates, updateExisting, validateOnly bool,
	categoryCache map[string]string,
	vendorCache map[string]string,
	warehouseCache map[string]*struct{ ID, Name string },
	supplierCache map[string]*struct{ ID, Name string },
	cacheMutex *sync.RWMutex,
) *models.ImportResult {
	result := &models.ImportResult{
		TotalRows:  len(rows),
		Errors:     make([]models.ImportRowError, 0),
		CreatedIDs: make([]string, 0),
		UpdatedIDs: make([]string, 0),
	}

	products := make([]*models.Product, 0, len(rows))

	for _, row := range rows {
		rowNum, _ := strconv.Atoi(row["_row"])

		// Validate required fields
		h.validateRequiredFields(row, rowNum, result)

		// Check category/vendor reference fields
		if row["categoryid"] == "" && row["categoryname"] == "" {
			h.addError(result, rowNum, "category", "REQUIRED", "Either 'categoryId' or 'categoryName' is required")
		}
		if row["vendorid"] == "" && row["vendorname"] == "" {
			h.addError(result, rowNum, "vendor", "REQUIRED", "Either 'vendorId' or 'vendorName' is required")
		}

		// Skip row if it has validation errors
		if h.hasRowErrors(result, rowNum) {
			continue
		}

		// Resolve category with thread-safe cache
		categoryID := h.resolveCategoryWithCache(tenantID, userID, userEmail, row, rowNum, result, categoryCache, cacheMutex)
		if categoryID == "" {
			continue
		}

		// Resolve vendor with thread-safe cache
		vendorID := h.resolveVendorWithCache(tenantID, userID, userEmail, row, rowNum, result, vendorCache, cacheMutex)
		if vendorID == "" {
			continue
		}

		// Resolve warehouse and supplier
		warehouseID, warehouseName := h.resolveWarehouseWithCache(tenantID, row, rowNum, result, warehouseCache, cacheMutex)
		supplierID, supplierName := h.resolveSupplierWithCache(tenantID, row, rowNum, result, supplierCache, cacheMutex)

		// Build product
		slug := generateSlug(row["name"])
		product := &models.Product{
			TenantID:          tenantID,
			Name:              row["name"],
			SKU:               row["sku"],
			Slug:              &slug,
			Price:             row["price"],
			CategoryID:        categoryID,
			VendorID:          vendorID,
			WarehouseID:       warehouseID,
			WarehouseName:     warehouseName,
			SupplierID:        supplierID,
			SupplierName:      supplierName,
			Description:       optionalString(row["description"]),
			ComparePrice:      optionalString(row["compareprice"]),
			CostPrice:         optionalString(row["costprice"]),
			Brand:             optionalString(row["brand"]),
			SearchKeywords:    optionalString(row["searchkeywords"]),
			Weight:            optionalString(row["weight"]),
			Quantity:          parseOptionalInt(row["quantity"]),
			MinOrderQty:       parseOptionalInt(row["minorderqty"]),
			MaxOrderQty:       parseOptionalInt(row["maxorderqty"]),
			LowStockThreshold: parseOptionalInt(row["lowstockthreshold"]),
			Tags:              parseTags(row["tags"]),
			CreatedBy:         stringPtr(userID),
			UpdatedBy:         stringPtr(userID),
			Status:            models.ProductStatusDraft,
		}

		products = append(products, product)
	}

	// If validate only, return validation results
	if validateOnly {
		result.Success = len(result.Errors) == 0
		result.SuccessCount = len(products)
		result.FailedCount = result.TotalRows - len(products)
		return result
	}

	// If there are validation errors for some rows, we still process valid rows
	if len(products) == 0 {
		result.Success = false
		result.FailedCount = result.TotalRows
		return result
	}

	// Execute bulk operation
	if updateExisting {
		h.executeBulkUpsert(tenantID, products, rows, result)
	} else {
		h.executeBulkCreate(tenantID, products, rows, skipDuplicates, result)
	}

	return result
}

// Thread-safe cache resolution functions
func (h *ImportHandler) resolveCategoryWithCache(tenantID, userID, userEmail string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]string, mutex *sync.RWMutex) string {
	categoryID := row["categoryid"]
	categoryName := row["categoryname"]

	if categoryID != "" {
		return categoryID
	}

	if categoryName == "" {
		return ""
	}

	cacheKey := strings.ToLower(categoryName)

	// Check cache with read lock
	mutex.RLock()
	if cachedID, ok := cache[cacheKey]; ok {
		mutex.RUnlock()
		return cachedID
	}
	mutex.RUnlock()

	// Look up or create category via HTTP client
	if h.categoriesClient == nil {
		h.addError(result, rowNum, "categoryName", "CATEGORY_ERROR", "Categories client not configured")
		return ""
	}

	// Create user context for RBAC
	userCtx := &clients.UserContext{
		UserID:    userID,
		UserEmail: userEmail,
	}

	category, wasCreated, err := h.categoriesClient.GetOrCreateCategoryWithContext(tenantID, categoryName, userID, userCtx)
	if err != nil {
		h.addError(result, rowNum, "categoryName", "CATEGORY_ERROR", fmt.Sprintf("Failed to resolve category '%s': %s", categoryName, err.Error()))
		return ""
	}

	// Log skip or creation
	if wasCreated {
		// Category was newly created - already logged by client
	} else {
		// Category existed - already logged by client
	}

	categoryID = category.ID

	// Cache with write lock
	mutex.Lock()
	cache[cacheKey] = categoryID
	mutex.Unlock()

	return categoryID
}

func (h *ImportHandler) resolveVendorWithCache(tenantID, userID, userEmail string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]string, mutex *sync.RWMutex) string {
	vendorID := row["vendorid"]
	vendorName := row["vendorname"]

	if vendorID != "" {
		return vendorID
	}

	if vendorName == "" {
		return ""
	}

	cacheKey := strings.ToLower(vendorName)

	mutex.RLock()
	if cachedID, ok := cache[cacheKey]; ok {
		mutex.RUnlock()
		return cachedID
	}
	mutex.RUnlock()

	// Look up vendor via HTTP client
	if h.vendorClient == nil {
		h.addError(result, rowNum, "vendorName", "VENDOR_NOT_FOUND", "Vendor client not configured")
		return ""
	}

	// Create user context for RBAC
	vendorUserCtx := &clients.VendorUserContext{
		UserID:    userID,
		UserEmail: userEmail,
	}

	vendor, err := h.vendorClient.GetVendorByNameWithContext(tenantID, vendorName, vendorUserCtx)
	if err != nil {
		h.addError(result, rowNum, "vendorName", "VENDOR_NOT_FOUND", fmt.Sprintf("Vendor '%s' not found. Create the vendor first.", vendorName))
		return ""
	}

	vendorID = vendor.ID

	mutex.Lock()
	cache[cacheKey] = vendorID
	mutex.Unlock()

	return vendorID
}

func (h *ImportHandler) resolveWarehouseWithCache(tenantID string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]*struct{ ID, Name string }, mutex *sync.RWMutex) (*string, *string) {
	warehouseName := row["warehousename"]
	if warehouseName == "" {
		return nil, nil
	}

	cacheKey := strings.ToLower(warehouseName)

	mutex.RLock()
	if cached, ok := cache[cacheKey]; ok {
		mutex.RUnlock()
		return &cached.ID, &cached.Name
	}
	mutex.RUnlock()

	if h.inventoryClient == nil {
		mutex.Lock()
		cache[cacheKey] = &struct{ ID, Name string }{"", warehouseName}
		mutex.Unlock()
		return nil, &warehouseName
	}

	warehouse, _, err := h.inventoryClient.GetOrCreateWarehouse(tenantID, warehouseName)
	if err != nil {
		mutex.Lock()
		cache[cacheKey] = &struct{ ID, Name string }{"", warehouseName}
		mutex.Unlock()
		return nil, &warehouseName
	}

	mutex.Lock()
	cache[cacheKey] = &struct{ ID, Name string }{warehouse.ID, warehouse.Name}
	mutex.Unlock()
	return &warehouse.ID, &warehouse.Name
}

func (h *ImportHandler) resolveSupplierWithCache(tenantID string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]*struct{ ID, Name string }, mutex *sync.RWMutex) (*string, *string) {
	supplierName := row["suppliername"]
	if supplierName == "" {
		return nil, nil
	}

	cacheKey := strings.ToLower(supplierName)

	mutex.RLock()
	if cached, ok := cache[cacheKey]; ok {
		mutex.RUnlock()
		return &cached.ID, &cached.Name
	}
	mutex.RUnlock()

	if h.inventoryClient == nil {
		mutex.Lock()
		cache[cacheKey] = &struct{ ID, Name string }{"", supplierName}
		mutex.Unlock()
		return nil, &supplierName
	}

	supplier, _, err := h.inventoryClient.GetOrCreateSupplier(tenantID, supplierName)
	if err != nil {
		mutex.Lock()
		cache[cacheKey] = &struct{ ID, Name string }{"", supplierName}
		mutex.Unlock()
		return nil, &supplierName
	}

	mutex.Lock()
	cache[cacheKey] = &struct{ ID, Name string }{supplier.ID, supplier.Name}
	mutex.Unlock()
	return &supplier.ID, &supplier.Name
}

// executeBulkCreate handles the bulk create operation
func (h *ImportHandler) executeBulkCreate(tenantID string, products []*models.Product, rows []map[string]string, skipDuplicates bool, result *models.ImportResult) {
	bulkResult, err := h.repo.BulkCreate(tenantID, products, skipDuplicates)
	if err != nil && bulkResult.Success == 0 && bulkResult.Skipped == 0 {
		result.Success = false
		result.Errors = append(result.Errors, models.ImportRowError{
			Row:     0,
			Code:    "BULK_CREATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	for _, product := range bulkResult.Created {
		result.CreatedIDs = append(result.CreatedIDs, product.ID.String())
	}

	for _, bulkErr := range bulkResult.Errors {
		rowNum := 0
		if bulkErr.Index < len(rows) {
			rowNum, _ = strconv.Atoi(rows[bulkErr.Index]["_row"])
		}
		result.Errors = append(result.Errors, models.ImportRowError{
			Row:     rowNum,
			Code:    bulkErr.Code,
			Message: bulkErr.Message,
		})
	}

	result.Success = bulkResult.Success > 0 || bulkResult.Skipped > 0
	result.SuccessCount = bulkResult.Success
	result.CreatedCount = len(bulkResult.Created)
	result.FailedCount = bulkResult.Failed + (result.TotalRows - len(products))
	result.SkippedCount = result.TotalRows - len(products) - bulkResult.Failed + bulkResult.Skipped
}

// executeBulkUpsert handles the bulk upsert operation
func (h *ImportHandler) executeBulkUpsert(tenantID string, products []*models.Product, rows []map[string]string, result *models.ImportResult) {
	upsertResult, err := h.repo.BulkUpsert(tenantID, products)
	if err != nil && upsertResult.Success == 0 {
		result.Success = false
		result.Errors = append(result.Errors, models.ImportRowError{
			Row:     0,
			Code:    "BULK_UPSERT_FAILED",
			Message: err.Error(),
		})
		return
	}

	for _, product := range upsertResult.Created {
		result.CreatedIDs = append(result.CreatedIDs, product.ID.String())
	}

	for _, product := range upsertResult.Updated {
		result.UpdatedIDs = append(result.UpdatedIDs, product.ID.String())
	}

	for _, bulkErr := range upsertResult.Errors {
		rowNum := 0
		if bulkErr.Index < len(rows) {
			rowNum, _ = strconv.Atoi(rows[bulkErr.Index]["_row"])
		}
		result.Errors = append(result.Errors, models.ImportRowError{
			Row:     rowNum,
			Code:    bulkErr.Code,
			Message: bulkErr.Message,
		})
	}

	result.Success = upsertResult.Success > 0
	result.SuccessCount = upsertResult.Success
	result.CreatedCount = len(upsertResult.Created)
	result.UpdatedCount = len(upsertResult.Updated)
	result.FailedCount = upsertResult.Failed + (result.TotalRows - len(products))
	result.SkippedCount = result.TotalRows - len(products) - upsertResult.Failed
}

// parseCSV parses a CSV file into rows
func (h *ImportHandler) parseCSV(file io.Reader) ([]map[string]string, error) {
	reader := csv.NewReader(file)

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Normalize headers
	for i := range headers {
		headers[i] = strings.TrimSpace(strings.ToLower(headers[i]))
		// Remove required marker if present
		headers[i] = strings.TrimSuffix(headers[i], " *")
	}

	var rows []map[string]string
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading line %d: %w", lineNum+1, err)
		}

		row := make(map[string]string)
		for i, value := range record {
			if i < len(headers) {
				row[headers[i]] = strings.TrimSpace(value)
			}
		}
		row["_row"] = strconv.Itoa(lineNum + 1) // Track row number for error reporting
		rows = append(rows, row)
		lineNum++
	}

	return rows, nil
}

// parseXLSX parses an Excel file into rows
func (h *ImportHandler) parseXLSX(file io.Reader) ([]map[string]string, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get first sheet (should be "Products")
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	sheetName := sheets[0]
	// Prefer "Products" sheet if it exists
	for _, name := range sheets {
		if strings.EqualFold(name, "Products") {
			sheetName = name
			break
		}
	}

	excelRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(excelRows) < 2 {
		return nil, fmt.Errorf("file must have a header row and at least one data row")
	}

	// First row is header
	headers := excelRows[0]
	for i := range headers {
		headers[i] = strings.TrimSpace(strings.ToLower(headers[i]))
		headers[i] = strings.TrimSuffix(headers[i], " *")
	}

	var rows []map[string]string
	for rowIdx, excelRow := range excelRows[1:] {
		row := make(map[string]string)
		for i, value := range excelRow {
			if i < len(headers) {
				row[headers[i]] = strings.TrimSpace(value)
			}
		}
		row["_row"] = strconv.Itoa(rowIdx + 2) // Track row number (1-indexed, +1 for header)
		rows = append(rows, row)
	}

	return rows, nil
}

// validateRequiredFields checks that all required fields are present
func (h *ImportHandler) validateRequiredFields(row map[string]string, rowNum int, result *models.ImportResult) {
	if row["name"] == "" {
		h.addError(result, rowNum, "name", "REQUIRED", "Product name is required")
	}
	if row["sku"] == "" {
		h.addError(result, rowNum, "sku", "REQUIRED", "SKU is required")
	}
	if row["price"] == "" {
		h.addError(result, rowNum, "price", "REQUIRED", "Price is required")
	} else if _, err := strconv.ParseFloat(row["price"], 64); err != nil {
		h.addError(result, rowNum, "price", "INVALID", "Price must be a valid number")
	}
}

// addError is a helper to add an error to the result
func (h *ImportHandler) addError(result *models.ImportResult, rowNum int, column, code, message string) {
	result.Errors = append(result.Errors, models.ImportRowError{
		Row:     rowNum,
		Column:  column,
		Code:    code,
		Message: message,
	})
}

// hasRowErrors checks if the given row already has errors
func (h *ImportHandler) hasRowErrors(result *models.ImportResult, rowNum int) bool {
	for _, e := range result.Errors {
		if e.Row == rowNum {
			return true
		}
	}
	return false
}

// optionalString returns nil for empty strings, pointer otherwise
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// parseOptionalInt parses an optional integer from a row field
func parseOptionalInt(value string) *int {
	if value == "" {
		return nil
	}
	if num, err := strconv.Atoi(value); err == nil {
		return &num
	}
	return nil
}

// parseTags parses comma-separated tags into JSON format
func parseTags(value string) *models.JSON {
	if value == "" {
		return nil
	}
	tags := strings.Split(value, ",")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}
	tagsJSON := make(models.JSON)
	tagsJSON["tags"] = tags
	return &tagsJSON
}
