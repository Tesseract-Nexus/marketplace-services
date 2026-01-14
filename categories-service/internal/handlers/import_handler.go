package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"categories-service/internal/models"
	"categories-service/internal/repository"

	"github.com/gin-gonic/gin"
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
	repo *repository.CategoryRepository
}

func NewImportHandler(repo *repository.CategoryRepository) *ImportHandler {
	return &ImportHandler{repo: repo}
}

// CategoryImportTemplate returns the template definition for categories
func CategoryImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "categories",
		Version: "1.0",
		Columns: []ImportTemplateColumn{
			{Name: "name", Description: "Category name", Required: true, Type: "string", Example: "Electronics"},
			{Name: "slug", Description: "URL-friendly slug (auto-generated if empty)", Required: false, Type: "string", Example: "electronics"},
			{Name: "description", Description: "Category description", Required: false, Type: "string", Example: "Electronic devices and accessories"},
			{Name: "parentId", Description: "Parent category UUID (for subcategories)", Required: false, Type: "uuid", Example: "550e8400-e29b-41d4-a716-446655440000"},
			{Name: "position", Description: "Display order position", Required: false, Type: "number", Example: "1"},
			{Name: "isActive", Description: "Whether category is active (true/false)", Required: false, Type: "boolean", Example: "true"},
			{Name: "tier", Description: "Category tier (BASIC, PREMIUM, ENTERPRISE)", Required: false, Type: "string", Example: "BASIC"},
			{Name: "tags", Description: "Comma-separated tags", Required: false, Type: "string", Example: "tech,gadgets"},
			{Name: "seoTitle", Description: "SEO title", Required: false, Type: "string", Example: "Buy Electronics Online"},
			{Name: "seoDescription", Description: "SEO meta description", Required: false, Type: "string", Example: "Shop for the best electronics"},
			{Name: "seoKeywords", Description: "Comma-separated SEO keywords", Required: false, Type: "string", Example: "electronics,gadgets,tech"},
			{Name: "imageUrl", Description: "Category image URL", Required: false, Type: "string", Example: "https://example.com/image.jpg"},
			{Name: "bannerUrl", Description: "Category banner URL", Required: false, Type: "string", Example: "https://example.com/banner.jpg"},
		},
		SampleData: []map[string]string{
			{
				"name":           "Electronics",
				"slug":           "electronics",
				"description":    "Electronic devices and accessories",
				"parentId":       "",
				"position":       "1",
				"isActive":       "true",
				"tier":           "BASIC",
				"tags":           "tech,gadgets",
				"seoTitle":       "Buy Electronics Online",
				"seoDescription": "Shop for the best electronics",
				"seoKeywords":    "electronics,gadgets,tech",
				"imageUrl":       "",
				"bannerUrl":      "",
			},
			{
				"name":           "Smartphones",
				"slug":           "smartphones",
				"description":    "Latest smartphones and accessories",
				"parentId":       "PARENT-CATEGORY-UUID",
				"position":       "1",
				"isActive":       "true",
				"tier":           "PREMIUM",
				"tags":           "phones,mobile",
				"seoTitle":       "Buy Smartphones Online",
				"seoDescription": "Shop for latest smartphones",
				"seoKeywords":    "smartphones,phones,mobile",
				"imageUrl":       "",
				"bannerUrl":      "",
			},
		},
	}
}

// GetImportTemplate returns the import template definition or file
// GET /api/v1/categories/import/template
func (h *ImportHandler) GetImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	template := CategoryImportTemplate()

	switch format {
	case "csv":
		h.generateCSVTemplate(c, template)
	case "xlsx":
		h.generateXLSXTemplate(c, template)
	default:
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"template": template,
		})
	}
}

// generateCSVTemplate generates and downloads a CSV template
func (h *ImportHandler) generateCSVTemplate(c *gin.Context, template ImportTemplate) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=categories_import_template.csv")

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

// generateXLSXTemplate generates and downloads an Excel template
func (h *ImportHandler) generateXLSXTemplate(c *gin.Context, template ImportTemplate) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Categories"
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
		f.SetColWidth(sheetName, colName, colName, 20)
	}

	for rowIdx, sample := range template.SampleData {
		for colIdx, col := range template.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheetName, cell, sample[col.Name])
		}
	}

	f.NewSheet("Instructions")
	f.SetCellValue("Instructions", "A1", "Category Import Instructions")
	f.SetCellValue("Instructions", "A3", "Column Definitions:")

	for i, col := range template.Columns {
		row := i + 4
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

	f.SetColWidth("Instructions", "A", "A", 20)
	f.SetColWidth("Instructions", "B", "B", 40)
	f.SetColWidth("Instructions", "C", "C", 15)
	f.SetColWidth("Instructions", "D", "D", 15)
	f.SetColWidth("Instructions", "E", "E", 40)

	sheetIdx, _ := f.GetSheetIndex(sheetName)
	f.SetActiveSheet(sheetIdx)

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=categories_import_template.xlsx")

	f.Write(c.Writer)
}

// ImportCategories imports categories from CSV or Excel file
// POST /api/v1/categories/import
func (h *ImportHandler) ImportCategories(c *gin.Context) {
	tenantID, _ := c.Get("tenantId")
	userID, _ := c.Get("userId")

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

	skipDuplicates := c.DefaultPostForm("skipDuplicates", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	filename := header.Filename
	var format ImportFormat
	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		format = ImportFormatCSV
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		format = ImportFormatXLSX
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

	var rows []map[string]string
	var parseErr error

	if format == ImportFormatCSV {
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

	result := h.processImportRows(tenantID.(string), userID.(string), rows, skipDuplicates, validateOnly)

	c.JSON(http.StatusOK, result)
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
	for _, name := range sheets {
		if strings.EqualFold(name, "Categories") {
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

func (h *ImportHandler) processImportRows(tenantID, userID string, rows []map[string]string, skipDuplicates, validateOnly bool) *ImportResult {
	result := &ImportResult{
		TotalRows:  len(rows),
		Errors:     make([]ImportRowError, 0),
		CreatedIDs: make([]string, 0),
	}

	categories := make([]*models.Category, 0, len(rows))
	template := CategoryImportTemplate()
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

		category := &models.Category{
			TenantID:    tenantID,
			Name:        row["name"],
			CreatedByID: userID,
			UpdatedByID: userID,
			Status:      models.StatusDraft,
			IsActive:    true,
		}

		// Slug
		if row["slug"] != "" {
			category.Slug = row["slug"]
		} else {
			category.Slug = generateSlugFromName(row["name"])
		}

		// Optional fields
		if row["description"] != "" {
			category.Description = stringPtr(row["description"])
		}
		if row["imageurl"] != "" {
			category.ImageURL = stringPtr(row["imageurl"])
		}
		if row["bannerurl"] != "" {
			category.BannerURL = stringPtr(row["bannerurl"])
		}
		if row["seotitle"] != "" {
			category.SeoTitle = stringPtr(row["seotitle"])
		}
		if row["seodescription"] != "" {
			category.SeoDescription = stringPtr(row["seodescription"])
		}

		// Position
		if row["position"] != "" {
			if pos, err := strconv.Atoi(row["position"]); err == nil {
				category.Position = pos
			}
		}

		// IsActive
		if row["isactive"] != "" {
			category.IsActive = strings.ToLower(row["isactive"]) == "true"
		}

		// Tier
		if row["tier"] != "" {
			tier := models.CategoryTier(strings.ToUpper(row["tier"]))
			category.Tier = &tier
		}

		// Tags
		if row["tags"] != "" {
			tags := strings.Split(row["tags"], ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			tagsJSON := make(models.JSON)
			tagsJSON["tags"] = tags
			category.Tags = &tagsJSON
		}

		// SEO Keywords
		if row["seokeywords"] != "" {
			keywords := strings.Split(row["seokeywords"], ",")
			for i := range keywords {
				keywords[i] = strings.TrimSpace(keywords[i])
			}
			keywordsJSON := make(models.JSON)
			keywordsJSON["keywords"] = keywords
			category.SeoKeywords = &keywordsJSON
		}

		categories = append(categories, category)
	}

	if validateOnly {
		result.Success = len(result.Errors) == 0
		result.SuccessCount = len(categories)
		result.FailedCount = result.TotalRows - len(categories)
		return result
	}

	if len(categories) == 0 {
		result.Success = false
		result.FailedCount = result.TotalRows
		return result
	}

	bulkResult, err := h.repo.BulkCreate(tenantID, categories)
	if err != nil && bulkResult.Success == 0 {
		result.Success = false
		result.Errors = append(result.Errors, ImportRowError{
			Row:     0,
			Code:    "BULK_CREATE_FAILED",
			Message: err.Error(),
		})
		return result
	}

	for _, cat := range bulkResult.Created {
		result.CreatedIDs = append(result.CreatedIDs, cat.ID.String())
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
	result.FailedCount = bulkResult.Failed + (result.TotalRows - len(categories))
	result.SkippedCount = result.TotalRows - len(categories) - bulkResult.Failed

	return result
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

func generateSlugFromName(name string) string {
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
