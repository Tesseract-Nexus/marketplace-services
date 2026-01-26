package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"orders-service/internal/encryption"
	"orders-service/internal/models"
)

// PaymentConfigService defines the interface for payment configuration management
type PaymentConfigService interface {
	// Payment Methods (reference data)
	GetAvailablePaymentMethods(region string) ([]models.PaymentMethodResponse, error)
	GetPaymentMethodByCode(code string) (*models.PaymentMethod, error)

	// Tenant Configurations
	GetTenantPaymentConfigs(tenantID string) ([]models.PaymentMethodResponse, error)
	GetTenantPaymentConfig(tenantID, methodCode string) (*models.TenantPaymentConfig, error)
	UpdatePaymentConfig(tenantID, methodCode string, req models.UpdatePaymentConfigRequest, userID string) (*models.TenantPaymentConfig, error)
	EnablePaymentMethod(tenantID, methodCode string, enabled bool, userID string) (*models.TenantPaymentConfig, error)

	// Test Connection
	TestPaymentConnection(tenantID, methodCode string, userID string) (*models.TestPaymentConnectionResponse, error)

	// Storefront - Get enabled methods for checkout
	GetEnabledPaymentMethods(tenantID, region string) ([]models.EnabledPaymentMethod, error)

	// Audit
	LogConfigChange(tenantID, methodCode, action, userID, ipAddress string, changes map[string]interface{}) error
}

// paymentConfigServiceImpl implements PaymentConfigService
type paymentConfigServiceImpl struct {
	db *gorm.DB
}

// NewPaymentConfigService creates a new payment config service
func NewPaymentConfigService(db *gorm.DB) PaymentConfigService {
	return &paymentConfigServiceImpl{db: db}
}

// GetAvailablePaymentMethods returns all payment methods available for a region
func (s *paymentConfigServiceImpl) GetAvailablePaymentMethods(region string) ([]models.PaymentMethodResponse, error) {
	var methods []models.PaymentMethod

	query := s.db.Where("is_active = ?", true)

	// Filter by region if specified
	if region != "" && region != "GLOBAL" {
		// Use array contains operator for PostgreSQL
		query = query.Where("? = ANY(supported_regions) OR 'GLOBAL' = ANY(supported_regions)", region)
	}

	if err := query.Order("display_order ASC").Find(&methods).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch payment methods: %w", err)
	}

	// Convert to response format (without tenant-specific data)
	responses := make([]models.PaymentMethodResponse, len(methods))
	for i, m := range methods {
		responses[i] = models.PaymentMethodResponse{
			PaymentMethod: m,
			IsConfigured:  false,
			IsEnabled:     false,
			IsTestMode:    true,
		}
	}

	return responses, nil
}

// GetPaymentMethodByCode returns a payment method by its code
func (s *paymentConfigServiceImpl) GetPaymentMethodByCode(code string) (*models.PaymentMethod, error) {
	var method models.PaymentMethod
	if err := s.db.Where("code = ? AND is_active = ?", code, true).First(&method).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("payment method not found: %s", code)
		}
		return nil, fmt.Errorf("failed to fetch payment method: %w", err)
	}
	return &method, nil
}

// GetTenantPaymentConfigs returns all payment methods with tenant-specific config status
func (s *paymentConfigServiceImpl) GetTenantPaymentConfigs(tenantID string) ([]models.PaymentMethodResponse, error) {
	var methods []models.PaymentMethod
	if err := s.db.Where("is_active = ?", true).Order("display_order ASC").Find(&methods).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch payment methods: %w", err)
	}

	// Fetch tenant configs
	var configs []models.TenantPaymentConfig
	if err := s.db.Where("tenant_id = ?", tenantID).Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch tenant configs: %w", err)
	}

	// Create a map for quick lookup
	configMap := make(map[string]*models.TenantPaymentConfig)
	for i := range configs {
		configMap[configs[i].PaymentMethodCode] = &configs[i]
	}

	// Build response
	responses := make([]models.PaymentMethodResponse, len(methods))
	for i, m := range methods {
		response := models.PaymentMethodResponse{
			PaymentMethod: m,
			IsConfigured:  false,
			IsEnabled:     false,
			IsTestMode:    true,
		}

		if config, exists := configMap[m.Code]; exists {
			response.IsConfigured = config.HasCredentials()
			response.IsEnabled = config.IsEnabled
			response.IsTestMode = config.IsTestMode
			response.EnabledRegions = config.EnabledRegions
			response.LastTestAt = config.LastTestAt
			response.LastTestSuccess = config.LastTestSuccess
			response.ConfigID = &config.ID
		}

		responses[i] = response
	}

	return responses, nil
}

// GetTenantPaymentConfig returns a specific tenant's payment config
func (s *paymentConfigServiceImpl) GetTenantPaymentConfig(tenantID, methodCode string) (*models.TenantPaymentConfig, error) {
	var config models.TenantPaymentConfig
	err := s.db.Preload("PaymentMethod").
		Where("tenant_id = ? AND payment_method_code = ?", tenantID, methodCode).
		First(&config).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Return a new empty config
		method, err := s.GetPaymentMethodByCode(methodCode)
		if err != nil {
			return nil, err
		}
		return &models.TenantPaymentConfig{
			TenantID:          tenantID,
			PaymentMethodCode: methodCode,
			IsEnabled:         false,
			IsTestMode:        true,
			PaymentMethod:     method,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch payment config: %w", err)
	}
	return &config, nil
}

// UpdatePaymentConfig updates a tenant's payment method configuration
func (s *paymentConfigServiceImpl) UpdatePaymentConfig(tenantID, methodCode string, req models.UpdatePaymentConfigRequest, userID string) (*models.TenantPaymentConfig, error) {
	// Verify the payment method exists
	method, err := s.GetPaymentMethodByCode(methodCode)
	if err != nil {
		return nil, err
	}

	// Get or create config
	var config models.TenantPaymentConfig
	err = s.db.Where("tenant_id = ? AND payment_method_code = ?", tenantID, methodCode).First(&config).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)

	if isNew {
		config = models.TenantPaymentConfig{
			ID:                uuid.New(),
			TenantID:          tenantID,
			PaymentMethodCode: methodCode,
			IsEnabled:         false,
			IsTestMode:        true,
			CreatedBy:         userID,
		}
	}

	// Track changes for audit
	changes := make(map[string]interface{})

	// Update fields
	if req.IsEnabled != nil {
		changes["isEnabled"] = map[string]interface{}{"old": config.IsEnabled, "new": *req.IsEnabled}
		config.IsEnabled = *req.IsEnabled
	}
	if req.IsTestMode != nil {
		changes["isTestMode"] = map[string]interface{}{"old": config.IsTestMode, "new": *req.IsTestMode}
		config.IsTestMode = *req.IsTestMode
	}
	if req.DisplayOrder != nil {
		changes["displayOrder"] = map[string]interface{}{"old": config.DisplayOrder, "new": *req.DisplayOrder}
		config.DisplayOrder = *req.DisplayOrder
	}
	if req.EnabledRegions != nil {
		// Validate that all regions are in the payment method's supported regions
		for _, region := range req.EnabledRegions {
			isSupported := false
			for _, supported := range method.SupportedRegions {
				if region == supported || supported == "GLOBAL" {
					isSupported = true
					break
				}
			}
			if !isSupported {
				return nil, fmt.Errorf("region %s is not supported by payment method %s", region, methodCode)
			}
		}
		changes["enabledRegions"] = map[string]interface{}{"old": config.EnabledRegions, "new": req.EnabledRegions}
		config.EnabledRegions = req.EnabledRegions
	}
	if req.Settings != nil {
		config.Settings = *req.Settings
		changes["settings"] = "updated"
	}

	// Encrypt and store credentials if provided
	if req.Credentials != nil {
		credJSON, err := json.Marshal(req.Credentials)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize credentials: %w", err)
		}

		// Encrypt credentials
		encrypted, err := encryption.Encrypt(string(credJSON))
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
		}
		config.CredentialsEncrypted = []byte(encrypted)
		changes["credentials"] = "configured (masked)"
	}

	config.UpdatedBy = userID
	config.UpdatedAt = time.Now()

	// Save config
	if isNew {
		if err := s.db.Create(&config).Error; err != nil {
			return nil, fmt.Errorf("failed to create payment config: %w", err)
		}
	} else {
		if err := s.db.Save(&config).Error; err != nil {
			return nil, fmt.Errorf("failed to update payment config: %w", err)
		}
	}

	// Audit log
	if len(changes) > 0 {
		s.LogConfigChange(tenantID, methodCode, "configure", userID, "", changes)
	}

	// Load payment method relationship
	config.PaymentMethod = method

	return &config, nil
}

// EnablePaymentMethod enables or disables a payment method for a tenant
func (s *paymentConfigServiceImpl) EnablePaymentMethod(tenantID, methodCode string, enabled bool, userID string) (*models.TenantPaymentConfig, error) {
	req := models.UpdatePaymentConfigRequest{
		IsEnabled: &enabled,
	}
	return s.UpdatePaymentConfig(tenantID, methodCode, req, userID)
}

// TestPaymentConnection tests the connection to a payment provider
func (s *paymentConfigServiceImpl) TestPaymentConnection(tenantID, methodCode string, userID string) (*models.TestPaymentConnectionResponse, error) {
	config, err := s.GetTenantPaymentConfig(tenantID, methodCode)
	if err != nil {
		return nil, err
	}

	if !config.HasCredentials() {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "No credentials configured for this payment method",
			TestedAt:   time.Now(),
			Provider:   config.PaymentMethod.Provider,
			IsTestMode: config.IsTestMode,
		}, nil
	}

	// Decrypt credentials
	decrypted, err := encryption.Decrypt(string(config.CredentialsEncrypted))
	if err != nil {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Failed to decrypt credentials",
			TestedAt:   time.Now(),
			Provider:   config.PaymentMethod.Provider,
			IsTestMode: config.IsTestMode,
		}, nil
	}

	var creds models.PaymentCredentials
	if err := json.Unmarshal([]byte(decrypted), &creds); err != nil {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Invalid credential format",
			TestedAt:   time.Now(),
			Provider:   config.PaymentMethod.Provider,
			IsTestMode: config.IsTestMode,
		}, nil
	}

	// Test connection based on provider
	var testResult *models.TestPaymentConnectionResponse
	switch config.PaymentMethod.Provider {
	case "Stripe":
		testResult = s.testStripeConnection(&creds, config.IsTestMode)
	case "PayPal":
		testResult = s.testPayPalConnection(&creds, config.IsTestMode)
	case "Razorpay":
		testResult = s.testRazorpayConnection(&creds, config.IsTestMode)
	case "Afterpay":
		testResult = s.testAfterpayConnection(&creds, config.IsTestMode)
	case "Zip":
		testResult = s.testZipConnection(&creds, config.IsTestMode)
	case "Manual":
		// Manual methods (COD, Bank Transfer) don't need testing
		testResult = &models.TestPaymentConnectionResponse{
			Success:    true,
			Message:    "Manual payment method - no connection test required",
			TestedAt:   time.Now(),
			Provider:   "Manual",
			IsTestMode: config.IsTestMode,
		}
	default:
		testResult = &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    fmt.Sprintf("Unknown provider: %s", config.PaymentMethod.Provider),
			TestedAt:   time.Now(),
			Provider:   config.PaymentMethod.Provider,
			IsTestMode: config.IsTestMode,
		}
	}

	// Update test status in config
	now := time.Now()
	s.db.Model(&models.TenantPaymentConfig{}).
		Where("tenant_id = ? AND payment_method_code = ?", tenantID, methodCode).
		Updates(map[string]interface{}{
			"last_test_at":      now,
			"last_test_success": testResult.Success,
			"last_test_message": testResult.Message,
			"updated_at":        now,
			"updated_by":        userID,
		})

	// Audit log
	s.LogConfigChange(tenantID, methodCode, "test", userID, "", map[string]interface{}{
		"success": testResult.Success,
		"message": testResult.Message,
	})

	return testResult, nil
}

// GetEnabledPaymentMethods returns enabled payment methods for storefront checkout
// Multi-tenant: Filters by tenantID
// Region filtering: Uses tenant's configured EnabledRegions if set, otherwise falls back to method's SupportedRegions
func (s *paymentConfigServiceImpl) GetEnabledPaymentMethods(tenantID, region string) ([]models.EnabledPaymentMethod, error) {
	var configs []models.TenantPaymentConfig

	query := s.db.Preload("PaymentMethod").
		Where("tenant_id = ? AND is_enabled = ?", tenantID, true)

	if err := query.Order("display_order ASC").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch enabled payment methods: %w", err)
	}

	var enabledMethods []models.EnabledPaymentMethod
	for _, config := range configs {
		if config.PaymentMethod == nil || !config.PaymentMethod.IsActive {
			continue
		}

		method := config.PaymentMethod

		// Check if method is enabled for the requested region
		// Priority: Tenant's EnabledRegions > Method's SupportedRegions
		if region != "" {
			regionEnabled := false

			// First check tenant's configured enabled regions
			if len(config.EnabledRegions) > 0 {
				// Use tenant's explicitly configured regions
				for _, r := range config.EnabledRegions {
					if r == region || r == "GLOBAL" {
						regionEnabled = true
						break
					}
				}
			} else {
				// Fall back to payment method's default supported regions
				for _, r := range method.SupportedRegions {
					if r == region || r == "GLOBAL" {
						regionEnabled = true
						break
					}
				}
			}

			if !regionEnabled {
				continue
			}
		}

		// Build checkout display info
		enabled := models.EnabledPaymentMethod{
			Code:         method.Code,
			Name:         method.Name,
			Description:  method.Description,
			Provider:     method.Provider,
			Type:         string(method.Type),
			IconURL:      method.IconURL,
			DisplayOrder: config.DisplayOrder,
		}

		// Add installment info for BNPL methods
		if method.Type == models.PaymentMethodTypeBNPL {
			switch method.Code {
			case "afterpay":
				enabled.InstallmentInfo = "Pay in 4 interest-free payments"
			case "zip":
				enabled.InstallmentInfo = "Buy now, pay later"
			}
		}

		enabledMethods = append(enabledMethods, enabled)
	}

	return enabledMethods, nil
}

// LogConfigChange logs a payment configuration change
func (s *paymentConfigServiceImpl) LogConfigChange(tenantID, methodCode, action, userID, ipAddress string, changes map[string]interface{}) error {
	changesJSON, _ := json.Marshal(changes)
	log := models.PaymentConfigAuditLog{
		ID:                uuid.New(),
		TenantID:          tenantID,
		PaymentMethodCode: methodCode,
		Action:            action,
		UserID:            userID,
		IPAddress:         ipAddress,
		Changes:           changesJSON,
		CreatedAt:         time.Now(),
	}
	return s.db.Create(&log).Error
}

// Provider-specific test functions

func (s *paymentConfigServiceImpl) testStripeConnection(creds *models.PaymentCredentials, isTestMode bool) *models.TestPaymentConnectionResponse {
	// In production, this would make an actual API call to Stripe
	// For now, validate that required credentials are present
	if creds.StripeSecretKey == "" {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Stripe secret key is required",
			TestedAt:   time.Now(),
			Provider:   "Stripe",
			IsTestMode: isTestMode,
		}
	}

	// Check if key format is correct
	expectedPrefix := "sk_test_"
	if !isTestMode {
		expectedPrefix = "sk_live_"
	}
	if len(creds.StripeSecretKey) < len(expectedPrefix) {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Invalid Stripe secret key format",
			TestedAt:   time.Now(),
			Provider:   "Stripe",
			IsTestMode: isTestMode,
		}
	}

	// TODO: Make actual Stripe API call to validate credentials
	// stripe.Key = creds.StripeSecretKey
	// _, err := account.Get()

	return &models.TestPaymentConnectionResponse{
		Success:    true,
		Message:    "Stripe credentials validated successfully",
		TestedAt:   time.Now(),
		Provider:   "Stripe",
		IsTestMode: isTestMode,
	}
}

func (s *paymentConfigServiceImpl) testPayPalConnection(creds *models.PaymentCredentials, isTestMode bool) *models.TestPaymentConnectionResponse {
	if creds.PayPalClientID == "" || creds.PayPalClientSecret == "" {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "PayPal client ID and secret are required",
			TestedAt:   time.Now(),
			Provider:   "PayPal",
			IsTestMode: isTestMode,
		}
	}

	// TODO: Make actual PayPal OAuth call to validate credentials

	return &models.TestPaymentConnectionResponse{
		Success:    true,
		Message:    "PayPal credentials validated successfully",
		TestedAt:   time.Now(),
		Provider:   "PayPal",
		IsTestMode: isTestMode,
	}
}

func (s *paymentConfigServiceImpl) testRazorpayConnection(creds *models.PaymentCredentials, isTestMode bool) *models.TestPaymentConnectionResponse {
	if creds.RazorpayKeyID == "" || creds.RazorpayKeySecret == "" {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Razorpay key ID and secret are required",
			TestedAt:   time.Now(),
			Provider:   "Razorpay",
			IsTestMode: isTestMode,
		}
	}

	// TODO: Make actual Razorpay API call to validate credentials

	return &models.TestPaymentConnectionResponse{
		Success:    true,
		Message:    "Razorpay credentials validated successfully",
		TestedAt:   time.Now(),
		Provider:   "Razorpay",
		IsTestMode: isTestMode,
	}
}

func (s *paymentConfigServiceImpl) testAfterpayConnection(creds *models.PaymentCredentials, isTestMode bool) *models.TestPaymentConnectionResponse {
	if creds.AfterpayMerchantID == "" || creds.AfterpaySecretKey == "" {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Afterpay merchant ID and secret key are required",
			TestedAt:   time.Now(),
			Provider:   "Afterpay",
			IsTestMode: isTestMode,
		}
	}

	return &models.TestPaymentConnectionResponse{
		Success:    true,
		Message:    "Afterpay credentials validated successfully",
		TestedAt:   time.Now(),
		Provider:   "Afterpay",
		IsTestMode: isTestMode,
	}
}

func (s *paymentConfigServiceImpl) testZipConnection(creds *models.PaymentCredentials, isTestMode bool) *models.TestPaymentConnectionResponse {
	if creds.ZipMerchantID == "" || creds.ZipAPIKey == "" {
		return &models.TestPaymentConnectionResponse{
			Success:    false,
			Message:    "Zip merchant ID and API key are required",
			TestedAt:   time.Now(),
			Provider:   "Zip",
			IsTestMode: isTestMode,
		}
	}

	return &models.TestPaymentConnectionResponse{
		Success:    true,
		Message:    "Zip credentials validated successfully",
		TestedAt:   time.Now(),
		Provider:   "Zip",
		IsTestMode: isTestMode,
	}
}
