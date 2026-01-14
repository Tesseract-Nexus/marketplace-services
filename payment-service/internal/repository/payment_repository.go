package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"payment-service/internal/models"
	"gorm.io/gorm"
)

// PaymentRepository handles payment data operations
type PaymentRepository struct {
	db *gorm.DB
}

// NewPaymentRepository creates a new payment repository
func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// GetGatewayConfig gets a gateway configuration by ID
func (r *PaymentRepository) GetGatewayConfig(ctx context.Context, configID uuid.UUID) (*models.PaymentGatewayConfig, error) {
	var config models.PaymentGatewayConfig
	err := r.db.WithContext(ctx).First(&config, "id = ?", configID).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetGatewayConfigByType gets a gateway configuration by tenant and type
func (r *PaymentRepository) GetGatewayConfigByType(ctx context.Context, tenantID string, gatewayType models.GatewayType) (*models.PaymentGatewayConfig, error) {
	var config models.PaymentGatewayConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND gateway_type = ? AND is_enabled = true", tenantID, gatewayType).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ListGatewayConfigs lists all gateway configurations for a tenant
func (r *PaymentRepository) ListGatewayConfigs(ctx context.Context, tenantID string) ([]models.PaymentGatewayConfig, error) {
	var configs []models.PaymentGatewayConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("priority ASC").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// CreateGatewayConfig creates a new gateway configuration
func (r *PaymentRepository) CreateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// UpdateGatewayConfig updates a gateway configuration
func (r *PaymentRepository) UpdateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error {
	config.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(config).Error
}

// DeleteGatewayConfig deletes a gateway configuration
func (r *PaymentRepository) DeleteGatewayConfig(ctx context.Context, configID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.PaymentGatewayConfig{}, "id = ?", configID).Error
}

// CreatePaymentTransaction creates a new payment transaction
func (r *PaymentRepository) CreatePaymentTransaction(ctx context.Context, tx *models.PaymentTransaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

// GetPaymentTransaction gets a payment transaction by ID
func (r *PaymentRepository) GetPaymentTransaction(ctx context.Context, paymentID uuid.UUID) (*models.PaymentTransaction, error) {
	var payment models.PaymentTransaction
	err := r.db.WithContext(ctx).Preload("GatewayConfig").Preload("Refunds").First(&payment, "id = ?", paymentID).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// GetPaymentTransactionByGatewayID gets a payment by gateway transaction ID
func (r *PaymentRepository) GetPaymentTransactionByGatewayID(ctx context.Context, gatewayTxID string) (*models.PaymentTransaction, error) {
	var payment models.PaymentTransaction
	err := r.db.WithContext(ctx).Where("gateway_transaction_id = ?", gatewayTxID).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// GetPaymentTransactionByOrderID gets the most recent payment by order ID (string format)
func (r *PaymentRepository) GetPaymentTransactionByOrderID(ctx context.Context, orderID string) (*models.PaymentTransaction, error) {
	var payment models.PaymentTransaction
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at DESC").First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// UpdatePaymentTransaction updates a payment transaction
func (r *PaymentRepository) UpdatePaymentTransaction(ctx context.Context, tx *models.PaymentTransaction) error {
	tx.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(tx).Error
}

// ListPaymentTransactionsByOrder lists all payments for an order
func (r *PaymentRepository) ListPaymentTransactionsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.PaymentTransaction, error) {
	var payments []models.PaymentTransaction
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at DESC").Find(&payments).Error
	if err != nil {
		return nil, err
	}
	return payments, nil
}

// ListPaymentTransactionsByCustomer lists all payments for a customer
func (r *PaymentRepository) ListPaymentTransactionsByCustomer(ctx context.Context, customerID uuid.UUID, tenantID string) ([]models.PaymentTransaction, error) {
	var payments []models.PaymentTransaction
	err := r.db.WithContext(ctx).Where("customer_id = ? AND tenant_id = ?", customerID, tenantID).Order("created_at DESC").Find(&payments).Error
	if err != nil {
		return nil, err
	}
	return payments, nil
}

// CreateRefundTransaction creates a new refund transaction
func (r *PaymentRepository) CreateRefundTransaction(ctx context.Context, refund *models.RefundTransaction) error {
	return r.db.WithContext(ctx).Create(refund).Error
}

// GetRefundTransaction gets a refund transaction by ID
func (r *PaymentRepository) GetRefundTransaction(ctx context.Context, refundID uuid.UUID) (*models.RefundTransaction, error) {
	var refund models.RefundTransaction
	err := r.db.WithContext(ctx).Preload("PaymentTransaction").First(&refund, "id = ?", refundID).Error
	if err != nil {
		return nil, err
	}
	return &refund, nil
}

// UpdateRefundTransaction updates a refund transaction
func (r *PaymentRepository) UpdateRefundTransaction(ctx context.Context, refund *models.RefundTransaction) error {
	refund.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(refund).Error
}

// ListRefundTransactionsByPayment lists all refunds for a payment
func (r *PaymentRepository) ListRefundTransactionsByPayment(ctx context.Context, paymentID uuid.UUID) ([]models.RefundTransaction, error) {
	var refunds []models.RefundTransaction
	err := r.db.WithContext(ctx).Where("payment_transaction_id = ?", paymentID).Order("created_at DESC").Find(&refunds).Error
	if err != nil {
		return nil, err
	}
	return refunds, nil
}

// CreateWebhookEvent creates a new webhook event
func (r *PaymentRepository) CreateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetWebhookEvent gets a webhook event by gateway event ID
func (r *PaymentRepository) GetWebhookEvent(ctx context.Context, gatewayType models.GatewayType, eventID string) (*models.WebhookEvent, error) {
	var event models.WebhookEvent
	err := r.db.WithContext(ctx).Where("gateway_type = ? AND event_id = ?", gatewayType, eventID).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// UpdateWebhookEvent updates a webhook event
func (r *PaymentRepository) UpdateWebhookEvent(ctx context.Context, event *models.WebhookEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// ListUnprocessedWebhookEvents lists unprocessed webhook events
func (r *PaymentRepository) ListUnprocessedWebhookEvents(ctx context.Context, limit int) ([]models.WebhookEvent, error) {
	var events []models.WebhookEvent
	err := r.db.WithContext(ctx).Where("processed = false").Order("created_at ASC").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

// GetPaymentSettings gets payment settings for a tenant
func (r *PaymentRepository) GetPaymentSettings(ctx context.Context, tenantID string) (*models.PaymentSettings, error) {
	var settings models.PaymentSettings
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// CreatePaymentSettings creates payment settings
func (r *PaymentRepository) CreatePaymentSettings(ctx context.Context, settings *models.PaymentSettings) error {
	return r.db.WithContext(ctx).Create(settings).Error
}

// UpdatePaymentSettings updates payment settings
func (r *PaymentRepository) UpdatePaymentSettings(ctx context.Context, settings *models.PaymentSettings) error {
	settings.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(settings).Error
}

// SavePaymentMethod saves a payment method
func (r *PaymentRepository) SavePaymentMethod(ctx context.Context, method *models.SavedPaymentMethod) error {
	return r.db.WithContext(ctx).Create(method).Error
}

// GetPaymentMethod gets a payment method by ID
func (r *PaymentRepository) GetPaymentMethod(ctx context.Context, methodID uuid.UUID) (*models.SavedPaymentMethod, error) {
	var method models.SavedPaymentMethod
	err := r.db.WithContext(ctx).First(&method, "id = ?", methodID).Error
	if err != nil {
		return nil, err
	}
	return &method, nil
}

// ListPaymentMethodsByCustomer lists all payment methods for a customer
func (r *PaymentRepository) ListPaymentMethodsByCustomer(ctx context.Context, customerID uuid.UUID, tenantID string) ([]models.SavedPaymentMethod, error) {
	var methods []models.SavedPaymentMethod
	err := r.db.WithContext(ctx).Where("customer_id = ? AND tenant_id = ? AND is_active = true", customerID, tenantID).Order("is_default DESC, created_at DESC").Find(&methods).Error
	if err != nil {
		return nil, err
	}
	return methods, nil
}

// UpdatePaymentMethod updates a payment method
func (r *PaymentRepository) UpdatePaymentMethod(ctx context.Context, method *models.SavedPaymentMethod) error {
	method.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(method).Error
}

// DeletePaymentMethod deletes a payment method
func (r *PaymentRepository) DeletePaymentMethod(ctx context.Context, methodID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.SavedPaymentMethod{}, "id = ?", methodID).Error
}

// ==================== Gateway Customer Methods ====================

// GetGatewayCustomer gets a gateway customer mapping by customer ID and gateway type
func (r *PaymentRepository) GetGatewayCustomer(ctx context.Context, tenantID string, customerID uuid.UUID, gatewayType models.GatewayType) (*models.GatewayCustomer, error) {
	var customer models.GatewayCustomer
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND customer_id = ? AND gateway_type = ?", tenantID, customerID, gatewayType).First(&customer).Error
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

// CreateGatewayCustomer creates a gateway customer mapping
func (r *PaymentRepository) CreateGatewayCustomer(ctx context.Context, customer *models.GatewayCustomer) error {
	return r.db.WithContext(ctx).Create(customer).Error
}

// UpdateGatewayCustomer updates a gateway customer mapping
func (r *PaymentRepository) UpdateGatewayCustomer(ctx context.Context, customer *models.GatewayCustomer) error {
	customer.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(customer).Error
}

// GetOrCreateGatewayCustomer gets or creates a gateway customer mapping
func (r *PaymentRepository) GetOrCreateGatewayCustomer(ctx context.Context, customer *models.GatewayCustomer) (*models.GatewayCustomer, bool, error) {
	existing, err := r.GetGatewayCustomer(ctx, customer.TenantID, customer.CustomerID, customer.GatewayType)
	if err == nil {
		return existing, false, nil // Found existing
	}
	// Create new
	if err := r.CreateGatewayCustomer(ctx, customer); err != nil {
		return nil, false, err
	}
	return customer, true, nil // Created new
}
