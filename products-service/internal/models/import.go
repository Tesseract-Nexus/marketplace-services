package models

// ImportFormat represents the file format for import
type ImportFormat string

const (
	ImportFormatCSV  ImportFormat = "csv"
	ImportFormatXLSX ImportFormat = "xlsx"
)

// ImportStatus represents the status of an import job
type ImportStatus string

const (
	ImportStatusPending    ImportStatus = "PENDING"
	ImportStatusProcessing ImportStatus = "PROCESSING"
	ImportStatusCompleted  ImportStatus = "COMPLETED"
	ImportStatusFailed     ImportStatus = "FAILED"
)

// ImportTemplateColumn defines a column in the import template
type ImportTemplateColumn struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type"` // string, number, boolean, uuid
	Example     string `json:"example"`
}

// ImportTemplate defines the structure of an import template
type ImportTemplate struct {
	Entity      string                 `json:"entity"`
	Version     string                 `json:"version"`
	Columns     []ImportTemplateColumn `json:"columns"`
	SampleData  []map[string]string    `json:"sampleData,omitempty"`
}

// ImportRowError represents an error for a specific row
type ImportRowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	Success      bool             `json:"success"`
	TotalRows    int              `json:"totalRows"`
	SuccessCount int              `json:"successCount"`
	CreatedCount int              `json:"createdCount"`
	UpdatedCount int              `json:"updatedCount"`
	FailedCount  int              `json:"failedCount"`
	SkippedCount int              `json:"skippedCount"`
	Errors       []ImportRowError `json:"errors,omitempty"`
	CreatedIDs   []string         `json:"createdIds,omitempty"`
	UpdatedIDs   []string         `json:"updatedIds,omitempty"`
}

// ImportRequest represents import configuration
type ImportRequest struct {
	SkipDuplicates bool `json:"skipDuplicates"`
	UpdateExisting bool `json:"updateExisting"` // if true, update products with matching SKU instead of skipping
	SkipHeader     bool `json:"skipHeader"`     // defaults to true
	ValidateOnly   bool `json:"validateOnly"`   // dry run mode
	BatchSize      int  `json:"batchSize"`      // number of rows per batch (default: 100, max: 500)
	MaxRetries     int  `json:"maxRetries"`     // max retries for failed batches (default: 2)
}

// BatchProgress represents the progress of a batch import operation
type BatchProgress struct {
	TotalBatches     int    `json:"totalBatches"`
	CompletedBatches int    `json:"completedBatches"`
	CurrentBatch     int    `json:"currentBatch"`
	PercentComplete  int    `json:"percentComplete"`
	Status           string `json:"status"` // "processing", "completed", "failed"
}

// BatchResult represents the result of processing a single batch
type BatchResult struct {
	BatchNumber  int              `json:"batchNumber"`
	StartRow     int              `json:"startRow"`
	EndRow       int              `json:"endRow"`
	Success      bool             `json:"success"`
	CreatedCount int              `json:"createdCount"`
	UpdatedCount int              `json:"updatedCount"`
	FailedCount  int              `json:"failedCount"`
	SkippedCount int              `json:"skippedCount"`
	Errors       []ImportRowError `json:"errors,omitempty"`
	CreatedIDs   []string         `json:"createdIds,omitempty"`
	UpdatedIDs   []string         `json:"updatedIds,omitempty"`
	RetryCount   int              `json:"retryCount"`
}

// EnhancedImportResult represents the result of a batch import operation
type EnhancedImportResult struct {
	Success       bool             `json:"success"`
	TotalRows     int              `json:"totalRows"`
	TotalBatches  int              `json:"totalBatches"`
	SuccessCount  int              `json:"successCount"`
	CreatedCount  int              `json:"createdCount"`
	UpdatedCount  int              `json:"updatedCount"`
	FailedCount   int              `json:"failedCount"`
	SkippedCount  int              `json:"skippedCount"`
	BatchResults  []BatchResult    `json:"batchResults,omitempty"`
	Errors        []ImportRowError `json:"errors,omitempty"`
	CreatedIDs    []string         `json:"createdIds,omitempty"`
	UpdatedIDs    []string         `json:"updatedIds,omitempty"`
	ProcessingMs  int64            `json:"processingMs"`
	AvgBatchMs    int64            `json:"avgBatchMs"`
}

// ProductImportColumns returns the column definitions for product import
func ProductImportColumns() []ImportTemplateColumn {
	return []ImportTemplateColumn{
		{Name: "name", Description: "Product name", Required: true, Type: "string", Example: "Blue Cotton T-Shirt"},
		{Name: "sku", Description: "Unique product SKU", Required: true, Type: "string", Example: "TSH-BLU-001"},
		{Name: "price", Description: "Product price", Required: true, Type: "number", Example: "29.99"},
		{Name: "categoryId", Description: "Category UUID (use this OR categoryName)", Required: false, Type: "uuid", Example: ""},
		{Name: "categoryName", Description: "Category name - auto-creates if not exists", Required: false, Type: "string", Example: "Electronics"},
		{Name: "vendorId", Description: "Vendor UUID (use this OR vendorName)", Required: false, Type: "uuid", Example: ""},
		{Name: "vendorName", Description: "Vendor name - must exist", Required: false, Type: "string", Example: "Demo Store"},
		{Name: "warehouseName", Description: "Warehouse name - auto-creates if not exists (optional)", Required: false, Type: "string", Example: "Main Warehouse"},
		{Name: "supplierName", Description: "Supplier name - auto-creates if not exists (optional)", Required: false, Type: "string", Example: "Acme Corp"},
		{Name: "description", Description: "Product description", Required: false, Type: "string", Example: ""},
		{Name: "comparePrice", Description: "Original/compare price", Required: false, Type: "number", Example: ""},
		{Name: "costPrice", Description: "Cost price", Required: false, Type: "number", Example: ""},
		{Name: "quantity", Description: "Initial stock quantity", Required: false, Type: "number", Example: ""},
		{Name: "minOrderQty", Description: "Minimum order quantity", Required: false, Type: "number", Example: ""},
		{Name: "maxOrderQty", Description: "Maximum order quantity", Required: false, Type: "number", Example: ""},
		{Name: "lowStockThreshold", Description: "Low stock alert threshold", Required: false, Type: "number", Example: ""},
		{Name: "weight", Description: "Product weight (kg)", Required: false, Type: "number", Example: ""},
		{Name: "brand", Description: "Brand name", Required: false, Type: "string", Example: ""},
		{Name: "tags", Description: "Comma-separated tags", Required: false, Type: "string", Example: ""},
		{Name: "searchKeywords", Description: "Search keywords", Required: false, Type: "string", Example: ""},
	}
}

// ProductImportTemplate returns the template definition for products
func ProductImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "products",
		Version: "1.1",
		Columns: ProductImportColumns(),
	}
}
