package validation

import (
	"fmt"
	"regexp"
	"strings"

	"customers-service/internal/models"
)

// AddressValidationError represents a validation error with field details
type AddressValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e AddressValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// AddressValidationErrors is a collection of validation errors
type AddressValidationErrors []AddressValidationError

func (e AddressValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are validation errors
func (e AddressValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Common regex patterns for validation
var (
	// ISO 3166-1 alpha-2 country code pattern
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)

	// Phone number pattern (international format, flexible)
	phonePattern = regexp.MustCompile(`^[\+]?[(]?[0-9]{1,4}[)]?[-\s\.]?[(]?[0-9]{1,3}[)]?[-\s\.]?[0-9]{1,4}[-\s\.]?[0-9]{1,4}[-\s\.]?[0-9]{1,9}$`)

	// Postal code patterns by country (common ones)
	postalCodePatterns = map[string]*regexp.Regexp{
		"US": regexp.MustCompile(`^\d{5}(-\d{4})?$`),                    // 12345 or 12345-6789
		"CA": regexp.MustCompile(`^[A-Za-z]\d[A-Za-z][ -]?\d[A-Za-z]\d$`), // A1A 1A1
		"GB": regexp.MustCompile(`^[A-Z]{1,2}[0-9][A-Z0-9]? ?[0-9][A-Z]{2}$`), // SW1A 1AA
		"AU": regexp.MustCompile(`^\d{4}$`),                             // 2000
		"IN": regexp.MustCompile(`^\d{6}$`),                             // 110001
		"DE": regexp.MustCompile(`^\d{5}$`),                             // 10115
		"FR": regexp.MustCompile(`^\d{5}$`),                             // 75001
	}

	// Label validation (alphanumeric with spaces, hyphens, and underscores)
	labelPattern = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)

	// Valid address types
	validAddressTypes = map[models.AddressType]bool{
		models.AddressTypeShipping: true,
		models.AddressTypeBilling:  true,
		models.AddressTypeBoth:     true,
	}

	// Characters that could indicate SQL injection or XSS attempts
	dangerousCharsPattern = regexp.MustCompile(`[<>\"';\x00-\x1f]`)
)

// Field length limits
const (
	MaxFirstNameLength    = 100
	MaxLastNameLength     = 100
	MaxCompanyLength      = 255
	MaxAddressLine1Length = 255
	MaxAddressLine2Length = 255
	MaxCityLength         = 100
	MaxStateLength        = 100
	MaxPostalCodeLength   = 20
	MaxPhoneLength        = 50
	MaxLabelLength        = 50
	MaxAddressesPerCustomer = 20
)

// ValidateAddress validates a CustomerAddress for creation or update
func ValidateAddress(address *models.CustomerAddress, isUpdate bool) AddressValidationErrors {
	var errors AddressValidationErrors

	// Required field validation (skip some for updates if not provided)
	if !isUpdate || address.AddressLine1 != "" {
		if strings.TrimSpace(address.AddressLine1) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "addressLine1",
				Message: "Address line 1 is required",
				Code:    "REQUIRED",
			})
		}
	}

	if !isUpdate || address.City != "" {
		if strings.TrimSpace(address.City) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "city",
				Message: "City is required",
				Code:    "REQUIRED",
			})
		}
	}

	if !isUpdate || address.PostalCode != "" {
		if strings.TrimSpace(address.PostalCode) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "postalCode",
				Message: "Postal code is required",
				Code:    "REQUIRED",
			})
		}
	}

	if !isUpdate || address.Country != "" {
		if strings.TrimSpace(address.Country) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "country",
				Message: "Country is required",
				Code:    "REQUIRED",
			})
		}
	}

	// Length validation
	if len(address.FirstName) > MaxFirstNameLength {
		errors = append(errors, AddressValidationError{
			Field:   "firstName",
			Message: fmt.Sprintf("First name must not exceed %d characters", MaxFirstNameLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.LastName) > MaxLastNameLength {
		errors = append(errors, AddressValidationError{
			Field:   "lastName",
			Message: fmt.Sprintf("Last name must not exceed %d characters", MaxLastNameLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.Company) > MaxCompanyLength {
		errors = append(errors, AddressValidationError{
			Field:   "company",
			Message: fmt.Sprintf("Company must not exceed %d characters", MaxCompanyLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.AddressLine1) > MaxAddressLine1Length {
		errors = append(errors, AddressValidationError{
			Field:   "addressLine1",
			Message: fmt.Sprintf("Address line 1 must not exceed %d characters", MaxAddressLine1Length),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.AddressLine2) > MaxAddressLine2Length {
		errors = append(errors, AddressValidationError{
			Field:   "addressLine2",
			Message: fmt.Sprintf("Address line 2 must not exceed %d characters", MaxAddressLine2Length),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.City) > MaxCityLength {
		errors = append(errors, AddressValidationError{
			Field:   "city",
			Message: fmt.Sprintf("City must not exceed %d characters", MaxCityLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.State) > MaxStateLength {
		errors = append(errors, AddressValidationError{
			Field:   "state",
			Message: fmt.Sprintf("State must not exceed %d characters", MaxStateLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.PostalCode) > MaxPostalCodeLength {
		errors = append(errors, AddressValidationError{
			Field:   "postalCode",
			Message: fmt.Sprintf("Postal code must not exceed %d characters", MaxPostalCodeLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.Phone) > MaxPhoneLength {
		errors = append(errors, AddressValidationError{
			Field:   "phone",
			Message: fmt.Sprintf("Phone must not exceed %d characters", MaxPhoneLength),
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.Label) > MaxLabelLength {
		errors = append(errors, AddressValidationError{
			Field:   "label",
			Message: fmt.Sprintf("Label must not exceed %d characters", MaxLabelLength),
			Code:    "MAX_LENGTH",
		})
	}

	// Format validation
	if address.Country != "" && !countryCodePattern.MatchString(strings.ToUpper(address.Country)) {
		errors = append(errors, AddressValidationError{
			Field:   "country",
			Message: "Country must be a valid 2-letter ISO country code (e.g., US, GB, AU)",
			Code:    "INVALID_FORMAT",
		})
	}

	if address.Phone != "" && !phonePattern.MatchString(address.Phone) {
		errors = append(errors, AddressValidationError{
			Field:   "phone",
			Message: "Phone number format is invalid",
			Code:    "INVALID_FORMAT",
		})
	}

	// Validate postal code format for known countries
	if address.Country != "" && address.PostalCode != "" {
		countryUpper := strings.ToUpper(address.Country)
		if pattern, ok := postalCodePatterns[countryUpper]; ok {
			if !pattern.MatchString(strings.ToUpper(address.PostalCode)) {
				errors = append(errors, AddressValidationError{
					Field:   "postalCode",
					Message: fmt.Sprintf("Invalid postal code format for %s", countryUpper),
					Code:    "INVALID_FORMAT",
				})
			}
		}
	}

	// Validate address type
	if address.AddressType != "" && !validAddressTypes[address.AddressType] {
		errors = append(errors, AddressValidationError{
			Field:   "addressType",
			Message: "Address type must be SHIPPING, BILLING, or BOTH",
			Code:    "INVALID_VALUE",
		})
	}

	// Validate label format if provided
	if address.Label != "" && !labelPattern.MatchString(address.Label) {
		errors = append(errors, AddressValidationError{
			Field:   "label",
			Message: "Label can only contain letters, numbers, spaces, hyphens, and underscores",
			Code:    "INVALID_FORMAT",
		})
	}

	// Security: Check for potentially dangerous characters
	fieldsToCheck := map[string]string{
		"firstName":    address.FirstName,
		"lastName":     address.LastName,
		"company":      address.Company,
		"addressLine1": address.AddressLine1,
		"addressLine2": address.AddressLine2,
		"city":         address.City,
		"state":        address.State,
	}

	for field, value := range fieldsToCheck {
		if dangerousCharsPattern.MatchString(value) {
			errors = append(errors, AddressValidationError{
				Field:   field,
				Message: "Contains invalid characters",
				Code:    "INVALID_CHARS",
			})
		}
	}

	return errors
}

// SanitizeAddress normalizes and sanitizes address fields
func SanitizeAddress(address *models.CustomerAddress) {
	address.FirstName = strings.TrimSpace(address.FirstName)
	address.LastName = strings.TrimSpace(address.LastName)
	address.Company = strings.TrimSpace(address.Company)
	address.AddressLine1 = strings.TrimSpace(address.AddressLine1)
	address.AddressLine2 = strings.TrimSpace(address.AddressLine2)
	address.City = strings.TrimSpace(address.City)
	address.State = strings.TrimSpace(address.State)
	address.PostalCode = strings.TrimSpace(strings.ToUpper(address.PostalCode))
	address.Country = strings.TrimSpace(strings.ToUpper(address.Country))
	address.Phone = strings.TrimSpace(address.Phone)
	address.Label = strings.TrimSpace(address.Label)

	// Set default address type if not provided
	if address.AddressType == "" {
		address.AddressType = models.AddressTypeShipping
	}
}

// MaskAddress creates a masked version of an address for logging
// This prevents PII from being logged
func MaskAddress(address *models.CustomerAddress) map[string]interface{} {
	return map[string]interface{}{
		"id":          address.ID.String(),
		"customerId":  address.CustomerID.String(),
		"tenantId":    address.TenantID,
		"addressType": address.AddressType,
		"isDefault":   address.IsDefault,
		"label":       address.Label,
		"city":        maskString(address.City, 2),
		"state":       address.State,
		"country":     address.Country,
		"postalCode":  maskPostalCode(address.PostalCode),
	}
}

// maskString masks a string, keeping only the first n characters visible
func maskString(s string, keepChars int) string {
	if len(s) <= keepChars {
		return strings.Repeat("*", len(s))
	}
	return s[:keepChars] + strings.Repeat("*", len(s)-keepChars)
}

// maskPostalCode masks a postal code, keeping only the first 2-3 chars
func maskPostalCode(postalCode string) string {
	if len(postalCode) <= 3 {
		return strings.Repeat("*", len(postalCode))
	}
	return postalCode[:3] + strings.Repeat("*", len(postalCode)-3)
}
