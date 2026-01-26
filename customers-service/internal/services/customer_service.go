package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/clients"
	"customers-service/internal/models"
	"customers-service/internal/repository"
)

// CustomerService handles customer business logic
type CustomerService struct {
	repo               *repository.CustomerRepository
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
	segmentEvaluator   *SegmentEvaluator
}

// NewCustomerService creates a new customer service
func NewCustomerService(repo *repository.CustomerRepository, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient) *CustomerService {
	return &CustomerService{
		repo:               repo,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
	}
}

// SetSegmentEvaluator sets the segment evaluator (optional, for dynamic segment evaluation)
func (s *CustomerService) SetSegmentEvaluator(evaluator *SegmentEvaluator) {
	s.segmentEvaluator = evaluator
}

// CreateCustomerRequest represents request to create a customer
// Note: TenantID is NOT binding:required because it comes from the X-Tenant-ID header,
// not from the request body. The handler must extract it from context.
type CreateCustomerRequest struct {
	TenantID       string               `json:"tenantId"`
	UserID         *uuid.UUID           `json:"userId"`
	Email          string               `json:"email" binding:"required,email"`
	FirstName      string               `json:"firstName" binding:"required"`
	LastName       string               `json:"lastName" binding:"required"`
	Phone          string               `json:"phone"`
	CustomerType   models.CustomerType  `json:"customerType"`
	MarketingOptIn bool                 `json:"marketingOptIn"`
	Tags           []string             `json:"tags"`
	Notes          string               `json:"notes"`
}

// CreateCustomer creates a new customer or returns existing one
func (s *CustomerService) CreateCustomer(ctx context.Context, req CreateCustomerRequest) (*models.Customer, error) {
	// Check if customer already exists by email
	existing, err := s.repo.GetByEmail(ctx, req.TenantID, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing customer: %w", err)
	}

	if existing != nil {
		// Update existing customer if needed
		if req.UserID != nil && existing.UserID == nil {
			existing.UserID = req.UserID
			if err := s.repo.Update(ctx, existing); err != nil {
				return nil, fmt.Errorf("failed to update customer: %w", err)
			}
		}
		return existing, nil
	}

	// Create new customer
	customer := &models.Customer{
		TenantID:       req.TenantID,
		UserID:         req.UserID,
		Email:          req.Email,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Phone:          req.Phone,
		Status:         models.CustomerStatusActive,
		CustomerType:   req.CustomerType,
		MarketingOptIn: req.MarketingOptIn,
		Tags:           req.Tags,
		Notes:          req.Notes,
	}

	if customer.CustomerType == "" {
		customer.CustomerType = models.CustomerTypeRetail
	}

	if err := s.repo.Create(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	// Send welcome email notification ONLY after email is verified
	// Welcome email is triggered by verification-service after successful OTP verification
	// This ensures customers receive welcome email only when their email is confirmed
	if s.notificationClient != nil && customer.EmailVerified {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromCustomer(customer)
			notification.StorefrontURL = s.tenantClient.BuildStorefrontURL(notifyCtx, customer.TenantID)

			if err := s.notificationClient.SendCustomerWelcomeNotification(notifyCtx, notification); err != nil {
				log.Printf("[CustomerService] Failed to send welcome notification: %v", err)
			}
		}()
	}

	// Evaluate dynamic segments for the new customer (non-blocking)
	if s.segmentEvaluator != nil {
		go func() {
			evalCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.segmentEvaluator.EvaluateCustomerSegments(evalCtx, customer); err != nil {
				log.Printf("[CustomerService] Failed to evaluate segments for new customer: %v", err)
			}
		}()
	}

	return customer, nil
}

// CreateFromEvent creates a customer record from an event without sending notifications.
// This is used by the customer registration event subscriber to sync customers from tenant-service.
// Since tenant-service already handles welcome/verification emails, we skip notifications here.
func (s *CustomerService) CreateFromEvent(ctx context.Context, customer *models.Customer) error {
	// Check if customer already exists by ID or email
	existing, err := s.repo.GetByID(ctx, customer.TenantID, customer.ID)
	if err == nil && existing != nil {
		log.Printf("[CustomerService] Customer %s already exists, skipping create from event", customer.Email)
		return nil
	}

	// Also check by email in case ID differs
	existingByEmail, err := s.repo.GetByEmail(ctx, customer.TenantID, customer.Email)
	if err == nil && existingByEmail != nil {
		log.Printf("[CustomerService] Customer with email %s already exists, skipping create from event", customer.Email)
		return nil
	}

	// Set defaults
	if customer.Status == "" {
		customer.Status = models.CustomerStatusActive
	}
	if customer.CustomerType == "" {
		customer.CustomerType = models.CustomerTypeRetail
	}

	if err := s.repo.Create(ctx, customer); err != nil {
		return fmt.Errorf("failed to create customer from event: %w", err)
	}

	log.Printf("[CustomerService] Created customer from event: %s (%s)", customer.Email, customer.ID)

	// Evaluate dynamic segments for the new customer (non-blocking)
	if s.segmentEvaluator != nil {
		go func() {
			evalCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.segmentEvaluator.EvaluateCustomerSegments(evalCtx, customer); err != nil {
				log.Printf("[CustomerService] Failed to evaluate segments for customer from event: %v", err)
			}
		}()
	}

	return nil
}

// GetByID retrieves a customer by ID (alias for GetCustomer)
func (s *CustomerService) GetByID(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.Customer, error) {
	return s.repo.GetByID(ctx, tenantID, customerID)
}

// GetCustomer retrieves a customer by ID
func (s *CustomerService) GetCustomer(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.Customer, error) {
	return s.repo.GetByID(ctx, tenantID, customerID)
}

// BatchGetCustomers retrieves multiple customers by IDs in a single query
// Performance: Single database query instead of N queries
func (s *CustomerService) BatchGetCustomers(ctx context.Context, tenantID string, customerIDs []uuid.UUID) ([]*models.Customer, error) {
	return s.repo.BatchGetByIDs(ctx, tenantID, customerIDs)
}

// UpdateCustomerRequest represents request to update a customer
type UpdateCustomerRequest struct {
	FirstName      *string               `json:"firstName"`
	LastName       *string               `json:"lastName"`
	Phone          *string               `json:"phone"`
	Status         *models.CustomerStatus `json:"status"`
	CustomerType   *models.CustomerType   `json:"customerType"`
	MarketingOptIn *bool                  `json:"marketingOptIn"`
	Tags           []string               `json:"tags"`
	Notes          *string                `json:"notes"`
}

// UpdateCustomer updates a customer
func (s *CustomerService) UpdateCustomer(ctx context.Context, tenantID string, customerID uuid.UUID, req UpdateCustomerRequest) (*models.Customer, error) {
	customer, err := s.repo.GetByID(ctx, tenantID, customerID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.FirstName != nil {
		customer.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		customer.LastName = *req.LastName
	}
	if req.Phone != nil {
		customer.Phone = *req.Phone
	}
	if req.Status != nil {
		customer.Status = *req.Status
	}
	if req.CustomerType != nil {
		customer.CustomerType = *req.CustomerType
	}
	if req.MarketingOptIn != nil {
		customer.MarketingOptIn = *req.MarketingOptIn
	}
	if req.Tags != nil {
		customer.Tags = req.Tags
	}
	if req.Notes != nil {
		customer.Notes = *req.Notes
	}

	if err := s.repo.Update(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	// Re-evaluate dynamic segments after customer update (non-blocking)
	if s.segmentEvaluator != nil {
		go func() {
			evalCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.segmentEvaluator.EvaluateCustomerSegments(evalCtx, customer); err != nil {
				log.Printf("[CustomerService] Failed to evaluate segments for updated customer: %v", err)
			}
		}()
	}

	return customer, nil
}

// ListCustomersRequest represents request to list customers
type ListCustomersRequest struct {
	TenantID     string                 `json:"tenantId" binding:"required"`
	Status       *models.CustomerStatus `json:"status"`
	CustomerType *models.CustomerType   `json:"customerType"`
	Search       string                 `json:"search"`
	Tags         []string               `json:"tags"`
	Page         int                    `json:"page"`
	PageSize     int                    `json:"pageSize"`
	SortBy       string                 `json:"sortBy"`
	SortOrder    string                 `json:"sortOrder"`
}

// ListCustomersResponse represents response for listing customers
type ListCustomersResponse struct {
	Customers  []models.Customer `json:"customers"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

// ListCustomers lists customers with filters and pagination
func (s *CustomerService) ListCustomers(ctx context.Context, req ListCustomersRequest) (*ListCustomersResponse, error) {
	// Set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	offset := (req.Page - 1) * req.PageSize

	filter := repository.ListFilter{
		TenantID:     req.TenantID,
		Status:       req.Status,
		CustomerType: req.CustomerType,
		Search:       req.Search,
		Tags:         req.Tags,
		Limit:        req.PageSize,
		Offset:       offset,
		SortBy:       req.SortBy,
		SortOrder:    req.SortOrder,
	}

	customers, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list customers: %w", err)
	}

	totalPages := int(total) / req.PageSize
	if int(total)%req.PageSize > 0 {
		totalPages++
	}

	return &ListCustomersResponse{
		Customers:  customers,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// DeleteCustomer soft deletes a customer
func (s *CustomerService) DeleteCustomer(ctx context.Context, tenantID string, customerID uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, customerID)
}

// RecordOrder records order information for a customer and returns updated customer
func (s *CustomerService) RecordOrder(ctx context.Context, tenantID string, customerID uuid.UUID, orderTotal float64) (*models.Customer, error) {
	customer, err := s.repo.GetByID(ctx, tenantID, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	orderDate := time.Now()

	// Update stats
	stats := map[string]interface{}{
		"total_orders":    customer.TotalOrders + 1,
		"total_spent":     customer.TotalSpent + orderTotal,
		"last_order_date": orderDate,
	}

	if customer.FirstOrderDate == nil {
		stats["first_order_date"] = orderDate
	}

	// Calculate new average order value
	newTotalOrders := customer.TotalOrders + 1
	newTotalSpent := customer.TotalSpent + orderTotal
	stats["average_order_value"] = newTotalSpent / float64(newTotalOrders)
	stats["lifetime_value"] = newTotalSpent // Can be more sophisticated

	if err := s.repo.UpdateStats(ctx, customerID, stats); err != nil {
		return nil, fmt.Errorf("failed to update customer stats: %w", err)
	}

	// Return updated customer
	return s.repo.GetByID(ctx, tenantID, customerID)
}

// AddAddress adds an address to a customer
func (s *CustomerService) AddAddress(ctx context.Context, address *models.CustomerAddress) error {
	return s.repo.AddAddress(ctx, address)
}

// DeleteAddress deletes an address
func (s *CustomerService) DeleteAddress(ctx context.Context, tenantID string, addressID uuid.UUID) error {
	return s.repo.DeleteAddress(ctx, tenantID, addressID)
}

// UpdateAddress updates an existing address
func (s *CustomerService) UpdateAddress(ctx context.Context, tenantID string, addressID uuid.UUID, address *models.CustomerAddress) (*models.CustomerAddress, error) {
	// Get existing address to verify ownership
	existing, err := s.repo.GetAddressByID(ctx, tenantID, addressID)
	if err != nil {
		return nil, fmt.Errorf("address not found: %w", err)
	}

	// Update fields but preserve ID, CustomerID, and TenantID
	address.ID = existing.ID
	address.CustomerID = existing.CustomerID
	address.TenantID = existing.TenantID

	if err := s.repo.UpdateAddress(ctx, address); err != nil {
		return nil, fmt.Errorf("failed to update address: %w", err)
	}

	return address, nil
}

// GetAddresses retrieves customer addresses
func (s *CustomerService) GetAddresses(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerAddress, error) {
	return s.repo.GetAddresses(ctx, tenantID, customerID)
}

// AddNote adds a note to a customer
func (s *CustomerService) AddNote(ctx context.Context, note *models.CustomerNote) error {
	return s.repo.AddNote(ctx, note)
}

// GetNotes retrieves customer notes
func (s *CustomerService) GetNotes(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerNote, error) {
	return s.repo.GetNotes(ctx, tenantID, customerID)
}

// LogCommunication logs customer communication
func (s *CustomerService) LogCommunication(ctx context.Context, comm *models.CustomerCommunication) error {
	return s.repo.LogCommunication(ctx, comm)
}

// GetCommunicationHistory retrieves communication history
func (s *CustomerService) GetCommunicationHistory(ctx context.Context, tenantID string, customerID uuid.UUID, limit int) ([]models.CustomerCommunication, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.GetCommunications(ctx, tenantID, customerID, limit)
}

// GetPaymentMethods retrieves customer payment methods
func (s *CustomerService) GetPaymentMethods(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerPaymentMethod, error) {
	return s.repo.GetPaymentMethods(ctx, tenantID, customerID)
}

// DeletePaymentMethod deletes a payment method
func (s *CustomerService) DeletePaymentMethod(ctx context.Context, tenantID string, methodID uuid.UUID) error {
	return s.repo.DeletePaymentMethod(ctx, tenantID, methodID)
}

// GenerateVerificationToken generates a verification token for a customer
func (s *CustomerService) GenerateVerificationToken(ctx context.Context, tenantID string, customerID uuid.UUID) (string, error) {
	customer, err := s.repo.GetByID(ctx, tenantID, customerID)
	if err != nil {
		return "", fmt.Errorf("customer not found: %w", err)
	}

	if customer.EmailVerified {
		return "", fmt.Errorf("email is already verified")
	}

	// Generate a secure random token
	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour) // Token expires in 24 hours

	// Save the token
	if err := s.repo.SetVerificationToken(ctx, customerID, token, expiresAt); err != nil {
		return "", fmt.Errorf("failed to save verification token: %w", err)
	}

	return token, nil
}

// VerifyEmail verifies a customer's email using the verification token
func (s *CustomerService) VerifyEmail(ctx context.Context, token string) (*models.Customer, error) {
	customer, err := s.repo.GetByVerificationToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired verification token")
	}

	if customer.VerificationTokenExpiresAt != nil && time.Now().After(*customer.VerificationTokenExpiresAt) {
		return nil, fmt.Errorf("verification token has expired")
	}

	// Mark email as verified and clear the token
	if err := s.repo.MarkEmailVerified(ctx, customer.ID); err != nil {
		return nil, fmt.Errorf("failed to verify email: %w", err)
	}

	// Return updated customer
	customer.EmailVerified = true
	customer.VerificationToken = ""
	customer.VerificationTokenExpiresAt = nil

	// Send email verified notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromCustomer(customer)
			notification.StorefrontURL = s.tenantClient.BuildStorefrontURL(notifyCtx, customer.TenantID)

			if err := s.notificationClient.SendEmailVerifiedNotification(notifyCtx, notification); err != nil {
				log.Printf("[CustomerService] Failed to send email verified notification: %v", err)
			}
		}()
	}

	return customer, nil
}

// GetCustomerByEmail retrieves a customer by email
func (s *CustomerService) GetCustomerByEmail(ctx context.Context, tenantID, email string) (*models.Customer, error) {
	return s.repo.GetByEmail(ctx, tenantID, email)
}

// VerifyEmailByAddress marks a customer's email as verified using their email address
// This is used by the OTP verification flow where verification happens externally
// and we just need to mark the customer as verified and send the welcome email
func (s *CustomerService) VerifyEmailByAddress(ctx context.Context, tenantID, email string) (*models.Customer, error) {
	customer, err := s.repo.GetByEmail(ctx, tenantID, email)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// Already verified, return early
	if customer.EmailVerified {
		log.Printf("[CustomerService] Customer %s email already verified", email)
		return customer, nil
	}

	// Mark email as verified
	if err := s.repo.MarkEmailVerified(ctx, customer.ID); err != nil {
		return nil, fmt.Errorf("failed to verify email: %w", err)
	}

	// Update local copy
	customer.EmailVerified = true
	customer.VerificationToken = ""
	customer.VerificationTokenExpiresAt = nil

	log.Printf("[CustomerService] Email verified for customer %s via OTP", email)

	// Send welcome email notification after email verification (non-blocking)
	// This is the only place welcome emails are sent - after email is verified
	if s.notificationClient != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromCustomer(customer)
			notification.StorefrontURL = s.tenantClient.BuildStorefrontURL(notifyCtx, customer.TenantID)

			if err := s.notificationClient.SendCustomerWelcomeNotification(notifyCtx, notification); err != nil {
				log.Printf("[CustomerService] Failed to send welcome notification: %v", err)
			} else {
				log.Printf("[CustomerService] Welcome email sent to %s after email verification", email)
			}
		}()
	}

	return customer, nil
}

// LockCustomerRequest represents request to lock a customer account
type LockCustomerRequest struct {
	Reason string `json:"reason" binding:"required,min=10,max=500"`
}

// UnlockCustomerRequest represents request to unlock a customer account
type UnlockCustomerRequest struct {
	Reason string `json:"reason" binding:"required,min=10,max=500"`
}

// LockCustomer locks a customer account by setting status to BLOCKED
func (s *CustomerService) LockCustomer(ctx context.Context, tenantID string, customerID uuid.UUID, lockedByUserID uuid.UUID, reason string) (*models.Customer, error) {
	customer, err := s.repo.GetByID(ctx, tenantID, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// Validate customer is not already blocked
	if customer.Status == models.CustomerStatusBlocked {
		return nil, fmt.Errorf("customer is already blocked")
	}

	// Update customer status to blocked
	now := time.Now()
	customer.Status = models.CustomerStatusBlocked
	customer.LockReason = reason
	customer.LockedAt = &now
	customer.LockedBy = &lockedByUserID
	// Clear previous unlock fields
	customer.UnlockReason = ""
	customer.UnlockedAt = nil
	customer.UnlockedBy = nil

	if err := s.repo.Update(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to lock customer: %w", err)
	}

	// Invalidate cache
	s.repo.InvalidateCache(ctx, tenantID, customerID)

	// Send lock notification email (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := &clients.AccountLockedNotification{
				TenantID:      customer.TenantID,
				CustomerID:    customer.ID.String(),
				CustomerEmail: customer.Email,
				CustomerName:  customer.FirstName + " " + customer.LastName,
				Reason:        reason,
				StorefrontURL: s.tenantClient.BuildStorefrontURL(notifyCtx, customer.TenantID),
			}

			if err := s.notificationClient.SendAccountLockedNotification(notifyCtx, notification); err != nil {
				log.Printf("[CustomerService] Failed to send account locked notification: %v", err)
			}
		}()
	}

	log.Printf("[CustomerService] Customer %s locked by %s: %s", customerID, lockedByUserID, reason)

	return customer, nil
}

// UnlockCustomer unlocks a customer account by setting status to ACTIVE
func (s *CustomerService) UnlockCustomer(ctx context.Context, tenantID string, customerID uuid.UUID, unlockedByUserID uuid.UUID, reason string) (*models.Customer, error) {
	customer, err := s.repo.GetByID(ctx, tenantID, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// Validate customer is currently blocked
	if customer.Status != models.CustomerStatusBlocked {
		return nil, fmt.Errorf("customer is not blocked")
	}

	// Update customer status to active
	now := time.Now()
	customer.Status = models.CustomerStatusActive
	customer.UnlockReason = reason
	customer.UnlockedAt = &now
	customer.UnlockedBy = &unlockedByUserID
	// Keep lock fields for history (LockReason, LockedAt, LockedBy)

	if err := s.repo.Update(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to unlock customer: %w", err)
	}

	// Invalidate cache
	s.repo.InvalidateCache(ctx, tenantID, customerID)

	// Send unlock notification email (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := &clients.AccountUnlockedNotification{
				TenantID:      customer.TenantID,
				CustomerID:    customer.ID.String(),
				CustomerEmail: customer.Email,
				CustomerName:  customer.FirstName + " " + customer.LastName,
				StorefrontURL: s.tenantClient.BuildStorefrontURL(notifyCtx, customer.TenantID),
			}

			if err := s.notificationClient.SendAccountUnlockedNotification(notifyCtx, notification); err != nil {
				log.Printf("[CustomerService] Failed to send account unlocked notification: %v", err)
			}
		}()
	}

	log.Printf("[CustomerService] Customer %s unlocked by %s: %s", customerID, unlockedByUserID, reason)

	return customer, nil
}
