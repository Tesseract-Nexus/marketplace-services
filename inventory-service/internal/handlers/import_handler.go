package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"inventory-service/internal/models"
	"inventory-service/internal/repository"
	"github.com/xuri/excelize/v2"
)

// ImportFormat represents the file format for import
type ImportFormat string

const (
	ImportFormatCSV  ImportFormat = "csv"
	ImportFormatXLSX ImportFormat = "xlsx"
)

// ImportTemplateColumn defines a column in the import template
type ImportTemplateColumn struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Example     string `json:"example"`
}

// ImportTemplate defines the structure of an import template
type ImportTemplate struct {
	Entity     string                 `json:"entity"`
	Version    string                 `json:"version"`
	Columns    []ImportTemplateColumn `json:"columns"`
	SampleData []map[string]string    `json:"sampleData,omitempty"`
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
	FailedCount  int              `json:"failedCount"`
	SkippedCount int              `json:"skippedCount"`
	Errors       []ImportRowError `json:"errors,omitempty"`
	CreatedIDs   []string         `json:"createdIds,omitempty"`
}

type ImportHandler struct {
	repo *repository.InventoryRepository
}

func NewImportHandler(repo *repository.InventoryRepository) *ImportHandler {
	return &ImportHandler{repo: repo}
}

// WarehouseImportTemplate returns the template for warehouses
func WarehouseImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "warehouses",
		Version: "1.0",
		Columns: []ImportTemplateColumn{
			{Name: "code", Description: "Unique warehouse code", Required: true, Type: "string", Example: "WH-001"},
			{Name: "name", Description: "Warehouse name", Required: true, Type: "string", Example: "Main Warehouse"},
			{Name: "address1", Description: "Street address", Required: true, Type: "string", Example: "123 Main Street"},
			{Name: "city", Description: "City", Required: true, Type: "string", Example: "New York"},
			{Name: "state", Description: "State/Province", Required: true, Type: "string", Example: "NY"},
			{Name: "postalCode", Description: "Postal/ZIP code", Required: true, Type: "string", Example: "10001"},
			{Name: "country", Description: "Country code", Required: false, Type: "string", Example: "US"},
			{Name: "address2", Description: "Address line 2", Required: false, Type: "string", Example: "Suite 100"},
			{Name: "phone", Description: "Phone number", Required: false, Type: "string", Example: "+1-555-123-4567"},
			{Name: "email", Description: "Email address", Required: false, Type: "string", Example: "warehouse@example.com"},
			{Name: "managerName", Description: "Manager name", Required: false, Type: "string", Example: "John Smith"},
			{Name: "status", Description: "Status (ACTIVE, INACTIVE, CLOSED)", Required: false, Type: "string", Example: "ACTIVE"},
			{Name: "isDefault", Description: "Is default warehouse (true/false)", Required: false, Type: "boolean", Example: "false"},
			{Name: "priority", Description: "Priority order", Required: false, Type: "number", Example: "1"},
		},
		SampleData: []map[string]string{
			{
				"code":        "WH-MAIN",
				"name":        "Main Distribution Center",
				"address1":    "123 Warehouse Drive",
				"city":        "Los Angeles",
				"state":       "CA",
				"postalCode":  "90001",
				"country":     "US",
				"address2":    "",
				"phone":       "+1-555-123-4567",
				"email":       "main@warehouse.com",
				"managerName": "John Smith",
				"status":      "ACTIVE",
				"isDefault":   "true",
				"priority":    "1",
			},
			{
				"code":        "WH-EAST",
				"name":        "East Coast Warehouse",
				"address1":    "456 Industrial Ave",
				"city":        "Newark",
				"state":       "NJ",
				"postalCode":  "07101",
				"country":     "US",
				"address2":    "Building B",
				"phone":       "+1-555-987-6543",
				"email":       "east@warehouse.com",
				"managerName": "Jane Doe",
				"status":      "ACTIVE",
				"isDefault":   "false",
				"priority":    "2",
			},
		},
	}
}

// SupplierImportTemplate returns the template for suppliers
func SupplierImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "suppliers",
		Version: "1.0",
		Columns: []ImportTemplateColumn{
			{Name: "code", Description: "Unique supplier code", Required: true, Type: "string", Example: "SUP-001"},
			{Name: "name", Description: "Supplier name", Required: true, Type: "string", Example: "ABC Supplies Inc."},
			{Name: "contactName", Description: "Primary contact name", Required: false, Type: "string", Example: "John Smith"},
			{Name: "email", Description: "Email address", Required: false, Type: "string", Example: "contact@supplier.com"},
			{Name: "phone", Description: "Phone number", Required: false, Type: "string", Example: "+1-555-123-4567"},
			{Name: "website", Description: "Website URL", Required: false, Type: "string", Example: "https://supplier.com"},
			{Name: "address1", Description: "Street address", Required: false, Type: "string", Example: "123 Supplier Lane"},
			{Name: "address2", Description: "Address line 2", Required: false, Type: "string", Example: "Suite 200"},
			{Name: "city", Description: "City", Required: false, Type: "string", Example: "Chicago"},
			{Name: "state", Description: "State/Province", Required: false, Type: "string", Example: "IL"},
			{Name: "postalCode", Description: "Postal/ZIP code", Required: false, Type: "string", Example: "60601"},
			{Name: "country", Description: "Country code", Required: false, Type: "string", Example: "US"},
			{Name: "taxId", Description: "Tax ID number", Required: false, Type: "string", Example: "12-3456789"},
			{Name: "paymentTerms", Description: "Payment terms", Required: false, Type: "string", Example: "Net 30"},
			{Name: "leadTimeDays", Description: "Lead time in days", Required: false, Type: "number", Example: "7"},
			{Name: "minOrderValue", Description: "Minimum order value", Required: false, Type: "number", Example: "100.00"},
			{Name: "currencyCode", Description: "Currency code", Required: false, Type: "string", Example: "USD"},
			{Name: "status", Description: "Status (ACTIVE, INACTIVE, BLACKLISTED)", Required: false, Type: "string", Example: "ACTIVE"},
			{Name: "notes", Description: "Additional notes", Required: false, Type: "string", Example: "Preferred supplier"},
		},
		SampleData: []map[string]string{
			{
				"code":          "SUP-ABC",
				"name":          "ABC Manufacturing Co.",
				"contactName":   "Alice Johnson",
				"email":         "alice@abcmfg.com",
				"phone":         "+1-555-111-2222",
				"website":       "https://abcmfg.com",
				"address1":      "100 Factory Road",
				"city":          "Detroit",
				"state":         "MI",
				"postalCode":    "48201",
				"country":       "US",
				"taxId":         "12-3456789",
				"paymentTerms":  "Net 30",
				"leadTimeDays":  "14",
				"minOrderValue": "500.00",
				"currencyCode":  "USD",
				"status":        "ACTIVE",
				"notes":         "Primary electronics supplier",
			},
		},
	}
}

// GetWarehouseImportTemplate returns the warehouse import template
// GET /api/v1/warehouses/import/template
func (h *ImportHandler) GetWarehouseImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	template := WarehouseImportTemplate()

	switch format {
	case "csv":
		h.generateCSVTemplate(c, template, "warehouses")
	case "xlsx":
		h.generateXLSXTemplate(c, template, "Warehouses")
	default:
		c.JSON(http.StatusOK, gin.H{"success": true, "template": template})
	}
}

// GetSupplierImportTemplate returns the supplier import template
// GET /api/v1/suppliers/import/template
func (h *ImportHandler) GetSupplierImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	template := SupplierImportTemplate()

	switch format {
	case "csv":
		h.generateCSVTemplate(c, template, "suppliers")
	case "xlsx":
		h.generateXLSXTemplate(c, template, "Suppliers")
	default:
		c.JSON(http.StatusOK, gin.H{"success": true, "template": template})
	}
}

func (h *ImportHandler) generateCSVTemplate(c *gin.Context, template ImportTemplate, entity string) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_import_template.csv", entity))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	headers := make([]string, len(template.Columns))
	for i, col := range template.Columns {
		headers[i] = col.Name
	}
	writer.Write(headers)

	for _, sample := range template.SampleData {
		row := make([]string, len(template.Columns))
		for i, col := range template.Columns {
			row[i] = sample[col.Name]
		}
		writer.Write(row)
	}
}

func (h *ImportHandler) generateXLSXTemplate(c *gin.Context, template ImportTemplate, sheetName string) {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", sheetName)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
	})

	requiredStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"C65911"}, Pattern: 1},
	})

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

		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, colName, colName, 18)
	}

	for rowIdx, sample := range template.SampleData {
		for colIdx, col := range template.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheetName, cell, sample[col.Name])
		}
	}

	sheetIdx, _ := f.GetSheetIndex(sheetName)
	f.SetActiveSheet(sheetIdx)

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_import_template.xlsx", strings.ToLower(sheetName)))

	f.Write(c.Writer)
}

// ImportWarehouses imports warehouses from CSV or Excel file
// POST /api/v1/warehouses/import
func (h *ImportHandler) ImportWarehouses(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "FILE_REQUIRED", Message: "Please upload a CSV or Excel file"},
		})
		return
	}
	defer file.Close()

	skipDuplicates := c.DefaultPostForm("skipDuplicates", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	rows, parseErr := h.parseFile(file, header.Filename)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "PARSE_ERROR", Message: parseErr.Error()},
		})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "EMPTY_FILE", Message: "The file contains no data rows"},
		})
		return
	}

	result := h.processWarehouseRows(tenantID.(string), userID.(string), rows, skipDuplicates, validateOnly)
	c.JSON(http.StatusOK, result)
}

// ImportSuppliers imports suppliers from CSV or Excel file
// POST /api/v1/suppliers/import
func (h *ImportHandler) ImportSuppliers(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "FILE_REQUIRED", Message: "Please upload a CSV or Excel file"},
		})
		return
	}
	defer file.Close()

	skipDuplicates := c.DefaultPostForm("skipDuplicates", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	rows, parseErr := h.parseFile(file, header.Filename)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "PARSE_ERROR", Message: parseErr.Error()},
		})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "EMPTY_FILE", Message: "The file contains no data rows"},
		})
		return
	}

	result := h.processSupplierRows(tenantID.(string), userID.(string), rows, skipDuplicates, validateOnly)
	c.JSON(http.StatusOK, result)
}

func (h *ImportHandler) parseFile(file io.Reader, filename string) ([]map[string]string, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		return h.parseCSV(file)
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		return h.parseXLSX(file)
	}
	return nil, fmt.Errorf("only CSV and XLSX files are supported")
}

func (h *ImportHandler) parseCSV(file io.Reader) ([]map[string]string, error) {
	reader := csv.NewReader(file)

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	for i := range headers {
		headers[i] = strings.TrimSpace(strings.ToLower(headers[i]))
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
		row["_row"] = strconv.Itoa(lineNum + 1)
		rows = append(rows, row)
		lineNum++
	}

	return rows, nil
}

func (h *ImportHandler) parseXLSX(file io.Reader) ([]map[string]string, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	sheetName := sheets[0]
	excelRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(excelRows) < 2 {
		return nil, fmt.Errorf("file must have a header row and at least one data row")
	}

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
		row["_row"] = strconv.Itoa(rowIdx + 2)
		rows = append(rows, row)
	}

	return rows, nil
}

func (h *ImportHandler) processWarehouseRows(tenantID, userID string, rows []map[string]string, skipDuplicates, validateOnly bool) *ImportResult {
	result := &ImportResult{
		TotalRows:  len(rows),
		Errors:     make([]ImportRowError, 0),
		CreatedIDs: make([]string, 0),
	}

	warehouses := make([]*models.Warehouse, 0, len(rows))
	template := WarehouseImportTemplate()
	requiredCols := make(map[string]bool)
	for _, col := range template.Columns {
		if col.Required {
			requiredCols[col.Name] = true
		}
	}

	for _, row := range rows {
		rowNum, _ := strconv.Atoi(row["_row"])

		for colName := range requiredCols {
			if row[colName] == "" {
				result.Errors = append(result.Errors, ImportRowError{
					Row:     rowNum,
					Column:  colName,
					Code:    "REQUIRED_FIELD",
					Message: fmt.Sprintf("Required field '%s' is empty", colName),
				})
			}
		}

		hasErrors := false
		for _, e := range result.Errors {
			if e.Row == rowNum {
				hasErrors = true
				break
			}
		}
		if hasErrors {
			continue
		}

		warehouse := &models.Warehouse{
			TenantID:   tenantID,
			Code:       row["code"],
			Name:       row["name"],
			Address1:   row["address1"],
			City:       row["city"],
			State:      row["state"],
			PostalCode: row["postalcode"],
			Country:    "US",
			Status:     models.WarehouseStatusActive,
			CreatedBy:  strPtr(userID),
			UpdatedBy:  strPtr(userID),
		}

		if row["country"] != "" {
			warehouse.Country = row["country"]
		}
		if row["address2"] != "" {
			warehouse.Address2 = strPtr(row["address2"])
		}
		if row["phone"] != "" {
			warehouse.Phone = strPtr(row["phone"])
		}
		if row["email"] != "" {
			warehouse.Email = strPtr(row["email"])
		}
		if row["managername"] != "" {
			warehouse.ManagerName = strPtr(row["managername"])
		}
		if row["status"] != "" {
			warehouse.Status = models.WarehouseStatus(strings.ToUpper(row["status"]))
		}
		if row["isdefault"] != "" {
			warehouse.IsDefault = strings.ToLower(row["isdefault"]) == "true"
		}
		if row["priority"] != "" {
			if pri, err := strconv.Atoi(row["priority"]); err == nil {
				warehouse.Priority = pri
			}
		}

		warehouses = append(warehouses, warehouse)
	}

	if validateOnly {
		result.Success = len(result.Errors) == 0
		result.SuccessCount = len(warehouses)
		result.FailedCount = result.TotalRows - len(warehouses)
		return result
	}

	if len(warehouses) == 0 {
		result.Success = false
		result.FailedCount = result.TotalRows
		return result
	}

	bulkResult, err := h.repo.BulkCreateWarehouses(tenantID, warehouses, skipDuplicates)
	if err != nil && bulkResult.Success == 0 {
		result.Success = false
		result.Errors = append(result.Errors, ImportRowError{
			Row:     0,
			Code:    "BULK_CREATE_FAILED",
			Message: err.Error(),
		})
		return result
	}

	for _, wh := range bulkResult.Created {
		result.CreatedIDs = append(result.CreatedIDs, wh.ID.String())
	}

	for _, bulkErr := range bulkResult.Errors {
		rowNum := 0
		if bulkErr.Index < len(rows) {
			rowNum, _ = strconv.Atoi(rows[bulkErr.Index]["_row"])
		}
		result.Errors = append(result.Errors, ImportRowError{
			Row:     rowNum,
			Code:    bulkErr.Code,
			Message: bulkErr.Message,
		})
	}

	result.Success = bulkResult.Success > 0
	result.SuccessCount = bulkResult.Success
	result.FailedCount = bulkResult.Failed + (result.TotalRows - len(warehouses))

	return result
}

func (h *ImportHandler) processSupplierRows(tenantID, userID string, rows []map[string]string, skipDuplicates, validateOnly bool) *ImportResult {
	result := &ImportResult{
		TotalRows:  len(rows),
		Errors:     make([]ImportRowError, 0),
		CreatedIDs: make([]string, 0),
	}

	suppliers := make([]*models.Supplier, 0, len(rows))
	template := SupplierImportTemplate()
	requiredCols := make(map[string]bool)
	for _, col := range template.Columns {
		if col.Required {
			requiredCols[col.Name] = true
		}
	}

	for _, row := range rows {
		rowNum, _ := strconv.Atoi(row["_row"])

		for colName := range requiredCols {
			if row[colName] == "" {
				result.Errors = append(result.Errors, ImportRowError{
					Row:     rowNum,
					Column:  colName,
					Code:    "REQUIRED_FIELD",
					Message: fmt.Sprintf("Required field '%s' is empty", colName),
				})
			}
		}

		hasErrors := false
		for _, e := range result.Errors {
			if e.Row == rowNum {
				hasErrors = true
				break
			}
		}
		if hasErrors {
			continue
		}

		supplier := &models.Supplier{
			TenantID:  tenantID,
			Code:      row["code"],
			Name:      row["name"],
			Status:    models.SupplierStatusActive,
			CreatedBy: strPtr(userID),
			UpdatedBy: strPtr(userID),
		}

		if row["contactname"] != "" {
			supplier.ContactName = strPtr(row["contactname"])
		}
		if row["email"] != "" {
			supplier.Email = strPtr(row["email"])
		}
		if row["phone"] != "" {
			supplier.Phone = strPtr(row["phone"])
		}
		if row["website"] != "" {
			supplier.Website = strPtr(row["website"])
		}
		if row["address1"] != "" {
			supplier.Address1 = strPtr(row["address1"])
		}
		if row["address2"] != "" {
			supplier.Address2 = strPtr(row["address2"])
		}
		if row["city"] != "" {
			supplier.City = strPtr(row["city"])
		}
		if row["state"] != "" {
			supplier.State = strPtr(row["state"])
		}
		if row["postalcode"] != "" {
			supplier.PostalCode = strPtr(row["postalcode"])
		}
		if row["country"] != "" {
			supplier.Country = strPtr(row["country"])
		}
		if row["taxid"] != "" {
			supplier.TaxID = strPtr(row["taxid"])
		}
		if row["paymentterms"] != "" {
			supplier.PaymentTerms = strPtr(row["paymentterms"])
		}
		if row["leadtimedays"] != "" {
			if days, err := strconv.Atoi(row["leadtimedays"]); err == nil {
				supplier.LeadTimeDays = &days
			}
		}
		if row["minordervalue"] != "" {
			if val, err := strconv.ParseFloat(row["minordervalue"], 64); err == nil {
				supplier.MinOrderValue = &val
			}
		}
		if row["currencycode"] != "" {
			supplier.CurrencyCode = strPtr(row["currencycode"])
		}
		if row["status"] != "" {
			supplier.Status = models.SupplierStatus(strings.ToUpper(row["status"]))
		}
		if row["notes"] != "" {
			supplier.Notes = strPtr(row["notes"])
		}

		suppliers = append(suppliers, supplier)
	}

	if validateOnly {
		result.Success = len(result.Errors) == 0
		result.SuccessCount = len(suppliers)
		result.FailedCount = result.TotalRows - len(suppliers)
		return result
	}

	if len(suppliers) == 0 {
		result.Success = false
		result.FailedCount = result.TotalRows
		return result
	}

	bulkResult, err := h.repo.BulkCreateSuppliers(tenantID, suppliers, skipDuplicates)
	if err != nil && bulkResult.Success == 0 {
		result.Success = false
		result.Errors = append(result.Errors, ImportRowError{
			Row:     0,
			Code:    "BULK_CREATE_FAILED",
			Message: err.Error(),
		})
		return result
	}

	for _, sup := range bulkResult.Created {
		result.CreatedIDs = append(result.CreatedIDs, sup.ID.String())
	}

	for _, bulkErr := range bulkResult.Errors {
		rowNum := 0
		if bulkErr.Index < len(rows) {
			rowNum, _ = strconv.Atoi(rows[bulkErr.Index]["_row"])
		}
		result.Errors = append(result.Errors, ImportRowError{
			Row:     rowNum,
			Code:    bulkErr.Code,
			Message: bulkErr.Message,
		})
	}

	result.Success = bulkResult.Success > 0
	result.SuccessCount = bulkResult.Success
	result.FailedCount = bulkResult.Failed + (result.TotalRows - len(suppliers))

	return result
}

func strPtr(s string) *string {
	return &s
}
