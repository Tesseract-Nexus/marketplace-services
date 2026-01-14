package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"staff-service/internal/models"
	"staff-service/internal/repository"
	"github.com/xuri/excelize/v2"
)

type ImportHandler struct {
	repo repository.StaffRepository
}

func NewImportHandler(repo repository.StaffRepository) *ImportHandler {
	return &ImportHandler{
		repo: repo,
	}
}

// GetImportTemplate returns the import template definition or file
// GET /api/v1/staff/import/template
func (h *ImportHandler) GetImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	template := models.StaffImportTemplate()

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
	c.Header("Content-Disposition", "attachment; filename=staff_import_template.csv")

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

	sheetName := "Staff"
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
	f.SetCellValue("Instructions", "A1", "Staff Import Instructions")

	// Smart Import Feature
	f.SetCellValue("Instructions", "A3", "SMART IMPORT FEATURE:")
	f.SetCellValue("Instructions", "A4", "You can use EITHER UUIDs (departmentId, teamId, managerId) OR names (departmentName, teamName, managerEmail):")
	f.SetCellValue("Instructions", "A5", "- departmentName: System will look up department by name. If not found, it will be AUTO-CREATED.")
	f.SetCellValue("Instructions", "A6", "- teamName: System will look up team by name. If not found, it will be AUTO-CREATED.")
	f.SetCellValue("Instructions", "A7", "- managerEmail: System will look up manager by email. Manager MUST exist.")
	f.SetCellValue("Instructions", "A8", "- You can mix and match: use departmentName for one staff and departmentId for another.")

	f.SetCellValue("Instructions", "A10", "EMPLOYEE ID:")
	f.SetCellValue("Instructions", "A11", "Employee IDs are AUTO-GENERATED in format: {BUSINESS_CODE}-{7_DIGIT_SEQUENCE}")
	f.SetCellValue("Instructions", "A12", "Example: DEMST-0000001, DEMST-0000002, etc.")
	f.SetCellValue("Instructions", "A13", "Business code is configured in your tenant settings.")

	f.SetCellValue("Instructions", "A15", "PHONE NUMBER FORMAT:")
	f.SetCellValue("Instructions", "A16", "Use format: +{COUNTRY_CODE}-{LOCAL_NUMBER}")
	f.SetCellValue("Instructions", "A17", "Example: +61-412345678 (Australia), +1-5551234567 (US)")

	f.SetCellValue("Instructions", "A19", "Column Definitions:")
	f.SetCellValue("Instructions", "A20", "Column")
	f.SetCellValue("Instructions", "B20", "Description")
	f.SetCellValue("Instructions", "C20", "Required")
	f.SetCellValue("Instructions", "D20", "Type")
	f.SetCellValue("Instructions", "E20", "Example")

	for i, col := range template.Columns {
		row := i + 21
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

	// Set active sheet to Staff
	sheetIdx, _ := f.GetSheetIndex(sheetName)
	f.SetActiveSheet(sheetIdx)

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=staff_import_template.xlsx")

	f.Write(c.Writer)
}

// ImportStaff imports staff from CSV or Excel file
// POST /api/v1/staff/import
func (h *ImportHandler) ImportStaff(c *gin.Context) {
	// Use GetString for consistency with other handlers (returns empty string if not found)
	tenantIDStr := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	vendorIDStr := c.GetString("vendor_id")
	businessCodeStr := c.GetString("business_code")

	// Validate required context values
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TENANT_REQUIRED",
				Message: "Tenant ID is required",
			},
		})
		return
	}

	// Default business code if not configured
	if businessCodeStr == "" {
		businessCodeStr = "EMP"
	}

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

	// Override business code if provided in form
	if formBusinessCode := c.PostForm("businessCode"); formBusinessCode != "" {
		businessCodeStr = formBusinessCode
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

	// Limit to 100 rows per import
	if len(rows) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TOO_MANY_ROWS",
				Message: "Maximum 100 staff members can be imported at once",
			},
		})
		return
	}

	// Validate and convert rows to staff
	result := h.processImportRows(tenantIDStr, userIDStr, vendorIDStr, businessCodeStr, rows, skipDuplicates, updateExisting, validateOnly)

	c.JSON(http.StatusOK, result)
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

	// Get first sheet (should be "Staff")
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	sheetName := sheets[0]
	// Prefer "Staff" sheet if it exists
	for _, name := range sheets {
		if strings.EqualFold(name, "Staff") {
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

// processImportRows validates and imports staff rows with smart name resolution
func (h *ImportHandler) processImportRows(tenantID, userID, vendorID, businessCode string, rows []map[string]string, skipDuplicates, updateExisting, validateOnly bool) *models.ImportResult {
	result := &models.ImportResult{
		TotalRows:  len(rows),
		Errors:     make([]models.ImportRowError, 0),
		CreatedIDs: make([]string, 0),
		UpdatedIDs: make([]string, 0),
	}

	staffList := make([]*models.Staff, 0, len(rows))

	// Cache for resolved names to avoid repeated lookups
	departmentCache := make(map[string]string)  // name -> id
	teamCache := make(map[string]string)        // "deptId:name" -> id
	managerCache := make(map[string]*uuid.UUID) // email -> id

	for _, row := range rows {
		rowNum, _ := strconv.Atoi(row["_row"])

		// Validate required fields
		h.validateRequiredFields(row, rowNum, result)

		// Skip row if it has validation errors
		if h.hasRowErrors(result, rowNum) {
			continue
		}

		// Resolve department (by name or use provided ID) - auto-creates if not found
		departmentID := h.resolveDepartment(tenantID, vendorID, userID, row, rowNum, result, departmentCache)

		// Resolve team (by name or use provided ID) - auto-creates if not found
		teamID := h.resolveTeam(tenantID, vendorID, departmentID, userID, row, rowNum, result, teamCache)

		// Resolve manager (by email or use provided ID)
		managerID := h.resolveManager(tenantID, row, rowNum, result, managerCache)

		// Parse role
		role := models.StaffRole(strings.ToLower(row["role"]))
		if !isValidRole(role) {
			h.addError(result, rowNum, "role", "INVALID", fmt.Sprintf("Invalid role: %s", row["role"]))
			continue
		}

		// Parse employment type
		employmentType := models.EmploymentType(strings.ToLower(row["employmenttype"]))
		if !isValidEmploymentType(employmentType) {
			h.addError(result, rowNum, "employmentType", "INVALID", fmt.Sprintf("Invalid employment type: %s", row["employmenttype"]))
			continue
		}

		// Build staff member
		staff := &models.Staff{
			FirstName:      row["firstname"],
			LastName:       row["lastname"],
			MiddleName:     optionalString(row["middlename"]),
			DisplayName:    optionalString(row["displayname"]),
			Email:          strings.ToLower(row["email"]),
			AlternateEmail: optionalString(row["alternateemail"]),
			PhoneNumber:    optionalString(row["phonenumber"]),
			MobileNumber:   optionalString(row["mobilenumber"]),
			Role:           role,
			EmploymentType: employmentType,
			DepartmentID:   optionalString(departmentID),
			TeamID:         optionalString(teamID),
			ManagerID:      managerID,
			JobTitle:       optionalString(row["jobtitle"]),
			StartDate:      parseOptionalDate(row["startdate"]),
			EndDate:        parseOptionalDate(row["enddate"]),
			LocationID:     optionalString(row["locationid"]),
			CostCenter:     optionalString(row["costcenter"]),
			// Address fields (aligned with onboarding)
			StreetAddress:  optionalString(row["streetaddress"]),
			StreetAddress2: optionalString(row["streetaddress2"]),
			City:           optionalString(row["city"]),
			State:          optionalString(row["state"]),
			StateCode:      optionalString(row["statecode"]),
			PostalCode:     optionalString(row["postalcode"]),
			Country:        optionalString(row["country"]),
			CountryCode:    optionalString(row["countrycode"]),
			Timezone:       optionalString(row["timezone"]),
			Locale:         optionalString(row["locale"]),
			Notes:          optionalString(row["notes"]),
			Tags:           parseTags(row["tags"]),
			IsActive:       true,
			CreatedBy:      &userID,
			UpdatedBy:      &userID,
		}

		staffList = append(staffList, staff)
	}

	// If validate only, return validation results
	if validateOnly {
		result.Success = len(result.Errors) == 0
		result.SuccessCount = len(staffList)
		result.FailedCount = result.TotalRows - len(staffList)
		return result
	}

	// If there are validation errors for some rows, we still process valid rows
	if len(staffList) == 0 {
		result.Success = false
		result.FailedCount = result.TotalRows
		return result
	}

	// Bulk create with auto-generated employee IDs
	bulkResult, err := h.repo.BulkCreateWithEmployeeIDs(tenantID, vendorID, businessCode, staffList, skipDuplicates)
	if err != nil && bulkResult == nil {
		result.Success = false
		result.Errors = append(result.Errors, models.ImportRowError{
			Row:     0,
			Code:    "BULK_CREATE_FAILED",
			Message: err.Error(),
		})
		return result
	}

	// Add created staff IDs
	for _, staff := range bulkResult.Created {
		result.CreatedIDs = append(result.CreatedIDs, staff.ID.String())
	}

	// Add bulk create errors
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
	result.FailedCount = bulkResult.Failed + (result.TotalRows - len(staffList))
	result.SkippedCount = result.TotalRows - len(staffList) - bulkResult.Failed + bulkResult.Skipped

	return result
}

// validateRequiredFields checks that all required fields are present
func (h *ImportHandler) validateRequiredFields(row map[string]string, rowNum int, result *models.ImportResult) {
	if row["firstname"] == "" {
		h.addError(result, rowNum, "firstName", "REQUIRED", "First name is required")
	}
	if row["lastname"] == "" {
		h.addError(result, rowNum, "lastName", "REQUIRED", "Last name is required")
	}
	if row["email"] == "" {
		h.addError(result, rowNum, "email", "REQUIRED", "Email is required")
	} else if !isValidEmail(row["email"]) {
		h.addError(result, rowNum, "email", "INVALID", "Invalid email format")
	}
	if row["role"] == "" {
		h.addError(result, rowNum, "role", "REQUIRED", "Role is required")
	}
	if row["employmenttype"] == "" {
		h.addError(result, rowNum, "employmentType", "REQUIRED", "Employment type is required")
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

// resolveDepartment resolves department by name or ID (auto-creates if not found)
func (h *ImportHandler) resolveDepartment(tenantID, vendorID, userID string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]string) string {
	departmentID := row["departmentid"]
	departmentName := row["departmentname"]

	// If ID is provided, use it directly
	if departmentID != "" {
		return departmentID
	}

	// If no name provided, return empty
	if departmentName == "" {
		return ""
	}

	// Check cache first
	cacheKey := strings.ToLower(departmentName)
	if cachedID, ok := cache[cacheKey]; ok {
		return cachedID
	}

	// Look up or create department (like products do with warehouses/suppliers)
	department, _, err := h.repo.GetOrCreateDepartment(tenantID, vendorID, departmentName, userID)
	if err != nil {
		h.addError(result, rowNum, "departmentName", "DEPARTMENT_ERROR", fmt.Sprintf("Failed to resolve department '%s': %s", departmentName, err.Error()))
		return ""
	}

	departmentID = department.ID.String()
	cache[cacheKey] = departmentID
	return departmentID
}

// resolveTeam resolves team by name or ID (auto-creates if not found)
func (h *ImportHandler) resolveTeam(tenantID, vendorID, departmentID, userID string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]string) string {
	teamID := row["teamid"]
	teamName := row["teamname"]

	// If ID is provided, use it directly
	if teamID != "" {
		return teamID
	}

	// If no name provided, return empty
	if teamName == "" {
		return ""
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", departmentID, strings.ToLower(teamName))
	if cachedID, ok := cache[cacheKey]; ok {
		return cachedID
	}

	// Look up or create team (like products do with warehouses/suppliers)
	team, _, err := h.repo.GetOrCreateTeam(tenantID, vendorID, departmentID, teamName, userID)
	if err != nil {
		h.addError(result, rowNum, "teamName", "TEAM_ERROR", fmt.Sprintf("Failed to resolve team '%s': %s", teamName, err.Error()))
		return ""
	}

	teamID = team.ID.String()
	cache[cacheKey] = teamID
	return teamID
}

// resolveManager resolves manager by email or ID
func (h *ImportHandler) resolveManager(tenantID string, row map[string]string, rowNum int, result *models.ImportResult, cache map[string]*uuid.UUID) *uuid.UUID {
	managerIDStr := row["managerid"]
	managerEmail := row["manageremail"]

	// If ID is provided, use it directly
	if managerIDStr != "" {
		managerID, err := uuid.Parse(managerIDStr)
		if err != nil {
			h.addError(result, rowNum, "managerId", "INVALID", "Invalid manager UUID format")
			return nil
		}
		return &managerID
	}

	// If no email provided, return nil
	if managerEmail == "" {
		return nil
	}

	// Check cache first
	cacheKey := strings.ToLower(managerEmail)
	if cachedID, ok := cache[cacheKey]; ok {
		return cachedID
	}

	// Look up manager by email
	manager, err := h.repo.GetByEmail(tenantID, managerEmail)
	if err != nil {
		h.addError(result, rowNum, "managerEmail", "MANAGER_NOT_FOUND", fmt.Sprintf("Manager with email '%s' not found. Create the manager first.", managerEmail))
		return nil
	}

	cache[cacheKey] = &manager.ID
	return &manager.ID
}

// Helper functions

func isValidEmail(email string) bool {
	// Simple validation - contains @ and .
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func isValidRole(role models.StaffRole) bool {
	validRoles := []models.StaffRole{
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleManager,
		models.RoleSeniorEmployee,
		models.RoleEmployee,
		models.RoleIntern,
		models.RoleContractor,
		models.RoleGuest,
		models.RoleReadonly,
	}
	for _, r := range validRoles {
		if role == r {
			return true
		}
	}
	return false
}

func isValidEmploymentType(et models.EmploymentType) bool {
	validTypes := []models.EmploymentType{
		models.EmploymentFullTime,
		models.EmploymentPartTime,
		models.EmploymentContract,
		models.EmploymentTemporary,
		models.EmploymentIntern,
		models.EmploymentConsultant,
		models.EmploymentVolunteer,
	}
	for _, t := range validTypes {
		if et == t {
			return true
		}
	}
	return false
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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

func parseOptionalDate(value string) *time.Time {
	if value == "" {
		return nil
	}

	// Try common date formats
	formats := []string{
		"2006-01-02",
		"01/02/2006",
		"02/01/2006",
		"2006/01/02",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return &t
		}
	}

	return nil
}

func parseTags(value string) *models.JSONArray {
	if value == "" {
		return nil
	}
	tags := strings.Split(value, ",")
	result := make(models.JSONArray, len(tags))
	for i, tag := range tags {
		result[i] = strings.TrimSpace(tag)
	}
	return &result
}

// ==================== Department Import ====================

// GetDepartmentImportTemplate returns the import template for departments
// GET /api/v1/departments/import/template
func (h *ImportHandler) GetDepartmentImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	template := models.DepartmentImportTemplate()

	switch format {
	case "csv":
		h.generateDepartmentCSVTemplate(c, template)
	case "xlsx":
		h.generateDepartmentXLSXTemplate(c, template)
	default:
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"template": template,
		})
	}
}

func (h *ImportHandler) generateDepartmentCSVTemplate(c *gin.Context, template models.ImportTemplate) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=department_import_template.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	headers := make([]string, len(template.Columns))
	for i, col := range template.Columns {
		headers[i] = col.Name
	}
	writer.Write(headers)
}

func (h *ImportHandler) generateDepartmentXLSXTemplate(c *gin.Context, template models.ImportTemplate) {
	f := excelize.NewFile()
	sheetName := "Departments"
	f.SetSheetName("Sheet1", sheetName)

	// Write headers
	for i, col := range template.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, col.Name)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=department_import_template.xlsx")
	f.Write(c.Writer)
}

// ImportDepartments imports departments from CSV or Excel file
// POST /api/v1/departments/import
func (h *ImportHandler) ImportDepartments(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	vendorIDStr := c.GetString("vendor_id")

	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "TENANT_REQUIRED", Message: "Tenant ID is required"},
		})
		return
	}

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
	updateExisting := c.DefaultPostForm("updateExisting", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	filename := header.Filename
	var rows []map[string]string

	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		rows, err = h.parseCSV(file)
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		rows, err = h.parseXLSX(file)
	} else {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_FORMAT", Message: "Only CSV and XLSX files are supported"},
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "PARSE_ERROR", Message: err.Error()},
		})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "EMPTY_FILE", Message: "File contains no data rows"},
		})
		return
	}

	if len(rows) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "TOO_MANY_ROWS", Message: "Maximum 100 departments can be imported at once"},
		})
		return
	}

	result := h.processDepartmentImportRows(tenantIDStr, userIDStr, vendorIDStr, rows, skipDuplicates, updateExisting, validateOnly)
	c.JSON(http.StatusOK, result)
}

func (h *ImportHandler) processDepartmentImportRows(tenantID, userID, vendorID string, rows []map[string]string, skipDuplicates, updateExisting, validateOnly bool) *models.ImportResult {
	result := &models.ImportResult{
		Success:   true,
		TotalRows: len(rows),
		Errors:    []models.ImportRowError{},
	}

	parentCache := make(map[string]string)

	for rowNum, row := range rows {
		actualRow := rowNum + 2 // Account for header row

		name := strings.TrimSpace(row["name"])
		if name == "" {
			h.addError(result, actualRow, "name", "REQUIRED", "Department name is required")
			continue
		}

		// Check if department already exists
		existing, _ := h.repo.GetDepartmentByName(tenantID, vendorID, name)
		if existing != nil {
			if skipDuplicates {
				result.SkippedCount++
				continue
			}
			if !updateExisting {
				h.addError(result, actualRow, "name", "DUPLICATE", fmt.Sprintf("Department '%s' already exists", name))
				continue
			}
		}

		// Resolve parent department if specified
		var parentID *string
		parentName := strings.TrimSpace(row["parentdepartmentname"])
		if parentName != "" {
			if cachedID, ok := parentCache[strings.ToLower(parentName)]; ok {
				parentID = &cachedID
			} else {
				parent, err := h.repo.GetDepartmentByName(tenantID, vendorID, parentName)
				if err != nil || parent == nil {
					h.addError(result, actualRow, "parentDepartmentName", "NOT_FOUND", fmt.Sprintf("Parent department '%s' not found", parentName))
					continue
				}
				parentIDStr := parent.ID.String()
				parentCache[strings.ToLower(parentName)] = parentIDStr
				parentID = &parentIDStr
			}
		}

		if validateOnly {
			result.SuccessCount++
			continue
		}

		// Generate code if not provided
		code := strings.TrimSpace(row["code"])
		if code == "" {
			code = generateSlug(name)
		}

		description := optionalString(strings.TrimSpace(row["description"]))

		if existing != nil && updateExisting {
			// Update existing department
			existing.Name = name
			existing.Code = optionalString(code)
			existing.Description = description
			if parentID != nil {
				parentUUID, _ := uuid.Parse(*parentID)
				existing.ParentDepartmentID = &parentUUID
			}
			if err := h.repo.UpdateDepartment(existing); err != nil {
				h.addError(result, actualRow, "", "UPDATE_ERROR", err.Error())
				continue
			}
			result.UpdatedCount++
			result.UpdatedIDs = append(result.UpdatedIDs, existing.ID.String())
		} else {
			// Create new department
			dept := &models.Department{
				TenantID:    tenantID,
				VendorID:    optionalString(vendorID),
				Name:        name,
				Code:        optionalString(code),
				Description: description,
				IsActive:    true,
				CreatedBy:   optionalString(userID),
			}
			if parentID != nil {
				parentUUID, _ := uuid.Parse(*parentID)
				dept.ParentDepartmentID = &parentUUID
			}
			if err := h.repo.CreateDepartment(dept); err != nil {
				h.addError(result, actualRow, "", "CREATE_ERROR", err.Error())
				continue
			}
			result.CreatedCount++
			result.CreatedIDs = append(result.CreatedIDs, dept.ID.String())
			parentCache[strings.ToLower(name)] = dept.ID.String()
		}
	}

	result.SuccessCount += result.CreatedCount + result.UpdatedCount
	result.FailedCount = result.TotalRows - result.SuccessCount - result.SkippedCount
	result.Success = result.FailedCount == 0
	return result
}

// ==================== Team Import ====================

// GetTeamImportTemplate returns the import template for teams
// GET /api/v1/teams/import/template
func (h *ImportHandler) GetTeamImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	template := models.TeamImportTemplate()

	switch format {
	case "csv":
		h.generateTeamCSVTemplate(c, template)
	case "xlsx":
		h.generateTeamXLSXTemplate(c, template)
	default:
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"template": template,
		})
	}
}

func (h *ImportHandler) generateTeamCSVTemplate(c *gin.Context, template models.ImportTemplate) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=team_import_template.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	headers := make([]string, len(template.Columns))
	for i, col := range template.Columns {
		headers[i] = col.Name
	}
	writer.Write(headers)
}

func (h *ImportHandler) generateTeamXLSXTemplate(c *gin.Context, template models.ImportTemplate) {
	f := excelize.NewFile()
	sheetName := "Teams"
	f.SetSheetName("Sheet1", sheetName)

	for i, col := range template.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, col.Name)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=team_import_template.xlsx")
	f.Write(c.Writer)
}

// ImportTeams imports teams from CSV or Excel file
// POST /api/v1/teams/import
func (h *ImportHandler) ImportTeams(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	vendorIDStr := c.GetString("vendor_id")

	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "TENANT_REQUIRED", Message: "Tenant ID is required"},
		})
		return
	}

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
	updateExisting := c.DefaultPostForm("updateExisting", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	filename := header.Filename
	var rows []map[string]string

	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		rows, err = h.parseCSV(file)
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		rows, err = h.parseXLSX(file)
	} else {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_FORMAT", Message: "Only CSV and XLSX files are supported"},
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "PARSE_ERROR", Message: err.Error()},
		})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "EMPTY_FILE", Message: "File contains no data rows"},
		})
		return
	}

	if len(rows) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "TOO_MANY_ROWS", Message: "Maximum 100 teams can be imported at once"},
		})
		return
	}

	result := h.processTeamImportRows(tenantIDStr, userIDStr, vendorIDStr, rows, skipDuplicates, updateExisting, validateOnly)
	c.JSON(http.StatusOK, result)
}

func (h *ImportHandler) processTeamImportRows(tenantID, userID, vendorID string, rows []map[string]string, skipDuplicates, updateExisting, validateOnly bool) *models.ImportResult {
	result := &models.ImportResult{
		Success:   true,
		TotalRows: len(rows),
		Errors:    []models.ImportRowError{},
	}

	deptCache := make(map[string]string)

	for rowNum, row := range rows {
		actualRow := rowNum + 2

		name := strings.TrimSpace(row["name"])
		if name == "" {
			h.addError(result, actualRow, "name", "REQUIRED", "Team name is required")
			continue
		}

		departmentName := strings.TrimSpace(row["departmentname"])
		if departmentName == "" {
			h.addError(result, actualRow, "departmentName", "REQUIRED", "Department name is required")
			continue
		}

		// Resolve department (auto-create if not exists)
		var departmentID string
		if cachedID, ok := deptCache[strings.ToLower(departmentName)]; ok {
			departmentID = cachedID
		} else {
			dept, _, err := h.repo.GetOrCreateDepartment(tenantID, vendorID, departmentName, userID)
			if err != nil {
				h.addError(result, actualRow, "departmentName", "DEPT_ERROR", fmt.Sprintf("Failed to resolve department: %s", err.Error()))
				continue
			}
			departmentID = dept.ID.String()
			deptCache[strings.ToLower(departmentName)] = departmentID
		}

		// Check if team already exists in this department
		existing, _ := h.repo.GetTeamByName(tenantID, vendorID, departmentID, name)
		if existing != nil {
			if skipDuplicates {
				result.SkippedCount++
				continue
			}
			if !updateExisting {
				h.addError(result, actualRow, "name", "DUPLICATE", fmt.Sprintf("Team '%s' already exists in department", name))
				continue
			}
		}

		if validateOnly {
			result.SuccessCount++
			continue
		}

		code := strings.TrimSpace(row["code"])
		if code == "" {
			code = generateSlug(name)
		}

		var maxCapacity *int
		if maxCapStr := strings.TrimSpace(row["maxcapacity"]); maxCapStr != "" {
			if cap, err := strconv.Atoi(maxCapStr); err == nil {
				maxCapacity = &cap
			}
		}

		deptUUID, _ := uuid.Parse(departmentID)

		if existing != nil && updateExisting {
			existing.Name = name
			existing.Code = optionalString(code)
			existing.MaxCapacity = maxCapacity
			if err := h.repo.UpdateTeam(existing); err != nil {
				h.addError(result, actualRow, "", "UPDATE_ERROR", err.Error())
				continue
			}
			result.UpdatedCount++
			result.UpdatedIDs = append(result.UpdatedIDs, existing.ID.String())
		} else {
			team := &models.Team{
				TenantID:     tenantID,
				VendorID:     optionalString(vendorID),
				DepartmentID: deptUUID,
				Name:         name,
				Code:         optionalString(code),
				MaxCapacity:  maxCapacity,
				IsActive:     true,
				CreatedBy:    optionalString(userID),
			}
			if err := h.repo.CreateTeam(team); err != nil {
				h.addError(result, actualRow, "", "CREATE_ERROR", err.Error())
				continue
			}
			result.CreatedCount++
			result.CreatedIDs = append(result.CreatedIDs, team.ID.String())
		}
	}

	result.SuccessCount += result.CreatedCount + result.UpdatedCount
	result.FailedCount = result.TotalRows - result.SuccessCount - result.SkippedCount
	result.Success = result.FailedCount == 0
	return result
}

// ==================== Role Import ====================

// GetRoleImportTemplate returns the import template for roles
// GET /api/v1/roles/import/template
func (h *ImportHandler) GetRoleImportTemplate(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	template := models.ImportTemplate{
		Entity:  "role",
		Version: "1.0",
		Columns: models.RoleImportColumns(),
	}

	switch format {
	case "csv":
		h.generateRoleCSVTemplate(c, template)
	case "xlsx":
		h.generateRoleXLSXTemplate(c, template)
	default:
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"template": template,
		})
	}
}

func (h *ImportHandler) generateRoleCSVTemplate(c *gin.Context, template models.ImportTemplate) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=role_import_template.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	headers := make([]string, len(template.Columns))
	for i, col := range template.Columns {
		headers[i] = col.Name
	}
	writer.Write(headers)
}

func (h *ImportHandler) generateRoleXLSXTemplate(c *gin.Context, template models.ImportTemplate) {
	f := excelize.NewFile()
	sheetName := "Roles"
	f.SetSheetName("Sheet1", sheetName)

	for i, col := range template.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, col.Name)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=role_import_template.xlsx")
	f.Write(c.Writer)
}

// ImportRoles imports roles from CSV or Excel file
// POST /api/v1/roles/import
func (h *ImportHandler) ImportRoles(c *gin.Context) {
	tenantIDStr := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	vendorIDStr := c.GetString("vendor_id")

	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "TENANT_REQUIRED", Message: "Tenant ID is required"},
		})
		return
	}

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
	updateExisting := c.DefaultPostForm("updateExisting", "false") == "true"
	validateOnly := c.DefaultPostForm("validateOnly", "false") == "true"

	filename := header.Filename
	var rows []map[string]string

	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		rows, err = h.parseCSV(file)
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		rows, err = h.parseXLSX(file)
	} else {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_FORMAT", Message: "Only CSV and XLSX files are supported"},
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "PARSE_ERROR", Message: err.Error()},
		})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "EMPTY_FILE", Message: "File contains no data rows"},
		})
		return
	}

	if len(rows) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "TOO_MANY_ROWS", Message: "Maximum 100 roles can be imported at once"},
		})
		return
	}

	result := h.processRoleImportRows(tenantIDStr, userIDStr, vendorIDStr, rows, skipDuplicates, updateExisting, validateOnly)
	c.JSON(http.StatusOK, result)
}

func (h *ImportHandler) processRoleImportRows(tenantID, userID, vendorID string, rows []map[string]string, skipDuplicates, updateExisting, validateOnly bool) *models.ImportResult {
	result := &models.ImportResult{
		Success:   true,
		TotalRows: len(rows),
		Errors:    []models.ImportRowError{},
	}

	for rowNum, row := range rows {
		actualRow := rowNum + 2

		name := strings.TrimSpace(row["name"])
		if name == "" {
			h.addError(result, actualRow, "name", "REQUIRED", "Role name is required")
			continue
		}

		// Check if role already exists
		existing, _ := h.repo.GetRoleByName(tenantID, vendorID, name)
		if existing != nil {
			if skipDuplicates {
				result.SkippedCount++
				continue
			}
			if !updateExisting {
				h.addError(result, actualRow, "name", "DUPLICATE", fmt.Sprintf("Role '%s' already exists", name))
				continue
			}
		}

		if validateOnly {
			result.SuccessCount++
			continue
		}

		displayName := strings.TrimSpace(row["displayname"])
		if displayName == "" {
			displayName = name
		}

		description := optionalString(strings.TrimSpace(row["description"]))

		priority := 100 // Default priority
		if priorityStr := strings.TrimSpace(row["priority"]); priorityStr != "" {
			if p, err := strconv.Atoi(priorityStr); err == nil {
				priority = p
			}
		}

		// Parse permissions
		var permissions []string
		if permStr := strings.TrimSpace(row["permissions"]); permStr != "" {
			for _, perm := range strings.Split(permStr, ",") {
				perm = strings.TrimSpace(perm)
				if perm != "" {
					permissions = append(permissions, perm)
				}
			}
		}

		if existing != nil && updateExisting {
			existing.DisplayName = displayName
			existing.Description = description
			existing.PriorityLevel = priority
			if err := h.repo.UpdateRole(existing); err != nil {
				h.addError(result, actualRow, "", "UPDATE_ERROR", err.Error())
				continue
			}
			// Update permissions if provided
			if len(permissions) > 0 {
				h.repo.SetRolePermissions(existing.ID.String(), permissions)
			}
			result.UpdatedCount++
			result.UpdatedIDs = append(result.UpdatedIDs, existing.ID.String())
		} else {
			role := &models.Role{
				TenantID:      tenantID,
				VendorID:      optionalString(vendorID),
				Name:          name,
				DisplayName:   displayName,
				Description:   description,
				PriorityLevel: priority,
				IsSystem:      false,
				IsActive:      true,
				CreatedBy:     optionalString(userID),
			}
			if err := h.repo.CreateRole(role); err != nil {
				h.addError(result, actualRow, "", "CREATE_ERROR", err.Error())
				continue
			}
			// Set permissions if provided
			if len(permissions) > 0 {
				h.repo.SetRolePermissions(role.ID.String(), permissions)
			}
			result.CreatedCount++
			result.CreatedIDs = append(result.CreatedIDs, role.ID.String())
		}
	}

	result.SuccessCount += result.CreatedCount + result.UpdatedCount
	result.FailedCount = result.TotalRows - result.SuccessCount - result.SkippedCount
	result.Success = result.FailedCount == 0
	return result
}
