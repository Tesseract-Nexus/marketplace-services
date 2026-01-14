package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"vendor-service/internal/clients"
	"vendor-service/internal/models"
	"vendor-service/internal/repository"
)

// VendorService handles business logic for vendor management
type VendorService interface {
	// Core vendor operations
	CreateVendor(ctx context.Context, tenantID string, req *models.CreateVendorRequest) (*models.Vendor, error)
	GetVendor(tenantID string, id uuid.UUID) (*models.Vendor, error)
	UpdateVendor(tenantID string, id uuid.UUID, req *models.UpdateVendorRequest) (*models.Vendor, error)
	DeleteVendor(tenantID string, id uuid.UUID, deletedBy string) error
	ListVendors(tenantID string, filters *models.VendorFilters, page, limit int) ([]models.Vendor, *models.PaginationInfo, error)

	// Bulk operations
	BulkCreateVendors(ctx context.Context, tenantID string, vendors []models.CreateVendorRequest) ([]models.Vendor, error)

	// Status management
	UpdateVendorStatus(tenantID string, id uuid.UUID, status models.VendorStatus, updatedBy string) error
	UpdateValidationStatus(tenantID string, id uuid.UUID, status models.ValidationStatus, updatedBy string) error

	// Analytics
	GetVendorAnalytics(tenantID string) (*models.VendorAnalytics, error)

	// Address management
	AddVendorAddress(tenantID string, vendorID uuid.UUID, address *models.VendorAddress) error
	UpdateVendorAddress(tenantID string, addressID uuid.UUID, updates map[string]interface{}) error
	DeleteVendorAddress(tenantID string, addressID uuid.UUID) error
	GetVendorAddresses(tenantID string, vendorID uuid.UUID) ([]models.VendorAddress, error)

	// Payment management
	AddVendorPayment(tenantID string, vendorID uuid.UUID, payment *models.VendorPayment) error
	UpdateVendorPayment(tenantID string, paymentID uuid.UUID, updates map[string]interface{}) error
	DeleteVendorPayment(tenantID string, paymentID uuid.UUID) error
	GetVendorPayments(tenantID string, vendorID uuid.UUID) ([]models.VendorPayment, error)

	// Contract management
	GetExpiringContracts(tenantID string, days int) ([]models.Vendor, error)
	UpdatePerformanceRating(tenantID string, id uuid.UUID, rating float64, updatedBy string) error
}

type vendorService struct {
	repo        repository.VendorRepository
	staffClient *clients.StaffClient
}

// NewVendorService creates a new vendor service instance
func NewVendorService(repo repository.VendorRepository) VendorService {
	return &vendorService{
		repo:        repo,
		staffClient: clients.NewStaffClient(),
	}
}

// CreateVendor creates a new vendor with validation
// For marketplace vendors (IsOwnerVendor=false), this also seeds vendor-specific RBAC roles
func (s *vendorService) CreateVendor(ctx context.Context, tenantID string, req *models.CreateVendorRequest) (*models.Vendor, error) {
	// Validate tenant ID
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}

	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check if email already exists
	existing, _ := s.repo.GetByEmail(tenantID, req.Email)
	if existing != nil {
		return nil, errors.New("vendor with this email already exists")
	}

	// Determine vendor status
	// Owner vendors (created during tenant onboarding) are auto-approved since they ARE the tenant
	// Marketplace vendors (external) start in PENDING status and require manual approval
	isOwnerVendor := req.IsOwnerVendor != nil && *req.IsOwnerVendor
	status := models.VendorStatusPending
	if isOwnerVendor {
		status = models.VendorStatusActive
	}

	// Create vendor model
	vendor := &models.Vendor{
		Name:             req.Name,
		Details:          req.Details,
		Location:         req.Location,
		PrimaryContact:   req.PrimaryContact,
		SecondaryContact: req.SecondaryContact,
		Email:            req.Email,
		CommissionRate:   req.CommissionRate,
		Status:           status,
		ValidationStatus: models.ValidationStatusNotStarted,
		IsActive:         true,
		// IsOwnerVendor: TRUE for tenant's own vendor (created during onboarding)
		// FALSE for external marketplace vendors
		IsOwnerVendor: isOwnerVendor,
		CustomFields:  req.CustomFields,
		CreatedBy:     &tenantID,
	}

	// If a specific ID is provided, use it instead of auto-generating
	// This is used during tenant onboarding to ensure Tenant.ID == Vendor.ID
	if req.ID != nil {
		vendor.ID = *req.ID
	}

	// Create vendor
	if err := s.repo.Create(tenantID, vendor); err != nil {
		return nil, fmt.Errorf("failed to create vendor: %w", err)
	}

	// For marketplace vendors (non-owner), seed vendor-specific RBAC roles
	// Owner vendors use tenant-level roles (store_owner, store_admin, etc.)
	// Marketplace vendors need vendor-scoped roles (vendor_owner, vendor_admin, etc.)
	if !vendor.IsOwnerVendor {
		if err := s.staffClient.SeedVendorRoles(ctx, tenantID, vendor.ID.String()); err != nil {
			// Log error but don't fail vendor creation - roles can be seeded manually
			fmt.Printf("[VENDOR] Warning: Failed to seed vendor roles for vendor %s: %v\n", vendor.ID, err)
		}
	}

	// Add addresses if provided
	for _, addrReq := range req.Addresses {
		address := &models.VendorAddress{
			VendorID:     vendor.ID,
			AddressType:  addrReq.AddressType,
			AddressLine1: addrReq.AddressLine1,
			AddressLine2: addrReq.AddressLine2,
			City:         addrReq.City,
			State:        addrReq.State,
			Country:      addrReq.Country,
			PostalCode:   addrReq.PostalCode,
			IsDefault:    addrReq.IsDefault != nil && *addrReq.IsDefault,
		}
		if err := s.repo.AddAddress(tenantID, vendor.ID, address); err != nil {
			// Log error but don't fail vendor creation
			fmt.Printf("Failed to add address: %v\n", err)
		}
	}

	// Add payment details if provided
	for _, payReq := range req.Payments {
		payment := &models.VendorPayment{
			VendorID:          vendor.ID,
			AccountHolderName: payReq.AccountHolderName,
			BankName:          payReq.BankName,
			AccountNumber:     payReq.AccountNumber,
			RoutingNumber:     payReq.RoutingNumber,
			SwiftCode:         payReq.SwiftCode,
			TaxIdentifier:     payReq.TaxIdentifier,
			Currency:          payReq.Currency,
			PaymentMethod:     payReq.PaymentMethod,
			IsDefault:         payReq.IsDefault != nil && *payReq.IsDefault,
		}
		if err := s.repo.AddPayment(tenantID, vendor.ID, payment); err != nil {
			// Log error but don't fail vendor creation
			fmt.Printf("Failed to add payment: %v\n", err)
		}
	}

	// Reload vendor with addresses and payments
	vendor, err := s.repo.GetByID(tenantID, vendor.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload vendor: %w", err)
	}

	return vendor, nil
}

// GetVendor retrieves a vendor by ID
func (s *vendorService) GetVendor(tenantID string, id uuid.UUID) (*models.Vendor, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}

	vendor, err := s.repo.GetByID(tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get vendor: %w", err)
	}

	return vendor, nil
}

// UpdateVendor updates vendor information
func (s *vendorService) UpdateVendor(tenantID string, id uuid.UUID, req *models.UpdateVendorRequest) (*models.Vendor, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}

	// Validate update request
	if err := s.validateUpdateRequest(req); err != nil {
		return nil, err
	}

	// Check if vendor exists
	existing, err := s.repo.GetByID(tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("vendor not found: %w", err)
	}

	// If email is being updated, check for duplicates
	if req.Email != nil && *req.Email != existing.Email {
		duplicate, _ := s.repo.GetByEmail(tenantID, *req.Email)
		if duplicate != nil && duplicate.ID != id {
			return nil, errors.New("vendor with this email already exists")
		}
	}

	// Update vendor
	if err := s.repo.Update(tenantID, id, req); err != nil {
		return nil, fmt.Errorf("failed to update vendor: %w", err)
	}

	// Return updated vendor
	return s.repo.GetByID(tenantID, id)
}

// DeleteVendor soft deletes a vendor
func (s *vendorService) DeleteVendor(tenantID string, id uuid.UUID, deletedBy string) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}

	return s.repo.Delete(tenantID, id, deletedBy)
}

// ListVendors retrieves vendors with filters and pagination
func (s *vendorService) ListVendors(tenantID string, filters *models.VendorFilters, page, limit int) ([]models.Vendor, *models.PaginationInfo, error) {
	if tenantID == "" {
		return nil, nil, errors.New("tenant ID is required")
	}

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return s.repo.List(tenantID, filters, page, limit)
}

// BulkCreateVendors creates multiple vendors
// For marketplace vendors (IsOwnerVendor=false), this also seeds vendor-specific RBAC roles
func (s *vendorService) BulkCreateVendors(ctx context.Context, tenantID string, requests []models.CreateVendorRequest) ([]models.Vendor, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}

	var vendors []models.Vendor
	for _, req := range requests {
		// Validate each request
		if err := s.validateCreateRequest(&req); err != nil {
			return nil, fmt.Errorf("validation failed for vendor %s: %w", req.Name, err)
		}

		vendor := models.Vendor{
			Name:             req.Name,
			Details:          req.Details,
			Location:         req.Location,
			PrimaryContact:   req.PrimaryContact,
			SecondaryContact: req.SecondaryContact,
			Email:            req.Email,
			CommissionRate:   req.CommissionRate,
			Status:           models.VendorStatusPending,
			ValidationStatus: models.ValidationStatusNotStarted,
			IsActive:         true,
			IsOwnerVendor:    req.IsOwnerVendor != nil && *req.IsOwnerVendor,
			CustomFields:     req.CustomFields,
			CreatedBy:        &tenantID,
		}
		vendors = append(vendors, vendor)
	}

	if err := s.repo.BulkCreate(tenantID, vendors); err != nil {
		return nil, fmt.Errorf("failed to bulk create vendors: %w", err)
	}

	// Seed roles for marketplace vendors (non-owner vendors)
	for _, vendor := range vendors {
		if !vendor.IsOwnerVendor {
			if err := s.staffClient.SeedVendorRoles(ctx, tenantID, vendor.ID.String()); err != nil {
				// Log error but don't fail - roles can be seeded manually
				fmt.Printf("[VENDOR] Warning: Failed to seed vendor roles for vendor %s: %v\n", vendor.ID, err)
			}
		}
	}

	return vendors, nil
}

// UpdateVendorStatus updates the vendor status
func (s *vendorService) UpdateVendorStatus(tenantID string, id uuid.UUID, status models.VendorStatus, updatedBy string) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}

	// Validate status transition
	vendor, err := s.repo.GetByID(tenantID, id)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	if !s.isValidStatusTransition(vendor.Status, status) {
		return fmt.Errorf("invalid status transition from %s to %s", vendor.Status, status)
	}

	return s.repo.UpdateStatus(tenantID, id, status, updatedBy)
}

// UpdateValidationStatus updates the vendor validation status
func (s *vendorService) UpdateValidationStatus(tenantID string, id uuid.UUID, status models.ValidationStatus, updatedBy string) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}

	return s.repo.UpdateValidationStatus(tenantID, id, status, updatedBy)
}

// GetVendorAnalytics retrieves vendor analytics
func (s *vendorService) GetVendorAnalytics(tenantID string) (*models.VendorAnalytics, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}

	return s.repo.GetAnalytics(tenantID)
}

// Address management methods
func (s *vendorService) AddVendorAddress(tenantID string, vendorID uuid.UUID, address *models.VendorAddress) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	return s.repo.AddAddress(tenantID, vendorID, address)
}

func (s *vendorService) UpdateVendorAddress(tenantID string, addressID uuid.UUID, updates map[string]interface{}) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	return s.repo.UpdateAddress(tenantID, addressID, updates)
}

func (s *vendorService) DeleteVendorAddress(tenantID string, addressID uuid.UUID) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	return s.repo.DeleteAddress(tenantID, addressID)
}

func (s *vendorService) GetVendorAddresses(tenantID string, vendorID uuid.UUID) ([]models.VendorAddress, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}
	return s.repo.GetAddresses(tenantID, vendorID)
}

// Payment management methods
func (s *vendorService) AddVendorPayment(tenantID string, vendorID uuid.UUID, payment *models.VendorPayment) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	return s.repo.AddPayment(tenantID, vendorID, payment)
}

func (s *vendorService) UpdateVendorPayment(tenantID string, paymentID uuid.UUID, updates map[string]interface{}) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	return s.repo.UpdatePayment(tenantID, paymentID, updates)
}

func (s *vendorService) DeleteVendorPayment(tenantID string, paymentID uuid.UUID) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}
	return s.repo.DeletePayment(tenantID, paymentID)
}

func (s *vendorService) GetVendorPayments(tenantID string, vendorID uuid.UUID) ([]models.VendorPayment, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}
	return s.repo.GetPayments(tenantID, vendorID)
}

// Contract management methods
func (s *vendorService) GetExpiringContracts(tenantID string, days int) ([]models.Vendor, error) {
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}
	return s.repo.GetExpiringContracts(tenantID, days)
}

func (s *vendorService) UpdatePerformanceRating(tenantID string, id uuid.UUID, rating float64, updatedBy string) error {
	if tenantID == "" {
		return errors.New("tenant ID is required")
	}

	if rating < 0 || rating > 5 {
		return errors.New("rating must be between 0 and 5")
	}

	return s.repo.UpdatePerformanceRating(tenantID, id, rating, updatedBy)
}

// Helper methods

func (s *vendorService) validateCreateRequest(req *models.CreateVendorRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("vendor name is required")
	}
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("vendor email is required")
	}
	if req.CommissionRate < 0 || req.CommissionRate > 100 {
		return errors.New("commission rate must be between 0 and 100")
	}
	return nil
}

func (s *vendorService) validateUpdateRequest(req *models.UpdateVendorRequest) error {
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return errors.New("vendor name cannot be empty")
	}
	if req.Email != nil && strings.TrimSpace(*req.Email) == "" {
		return errors.New("vendor email cannot be empty")
	}
	if req.CommissionRate != nil && (*req.CommissionRate < 0 || *req.CommissionRate > 100) {
		return errors.New("commission rate must be between 0 and 100")
	}
	return nil
}

func (s *vendorService) isValidStatusTransition(from, to models.VendorStatus) bool {
	// Define valid status transitions
	validTransitions := map[models.VendorStatus][]models.VendorStatus{
		models.VendorStatusPending: {
			models.VendorStatusActive,
			models.VendorStatusSuspended,
			models.VendorStatusTerminated,
		},
		models.VendorStatusActive: {
			models.VendorStatusSuspended,
			models.VendorStatusInactive,
		},
		models.VendorStatusSuspended: {
			models.VendorStatusActive,
			models.VendorStatusInactive,
		},
		models.VendorStatusInactive: {
			models.VendorStatusActive,
			models.VendorStatusSuspended,
		},
		models.VendorStatusTerminated: {
			models.VendorStatusPending, // Allow resubmission
		},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}

	return false
}
