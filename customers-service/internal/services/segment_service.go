package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"customers-service/internal/models"
	"customers-service/internal/repository"
)

// SegmentService handles segment business logic
type SegmentService struct {
	repo *repository.SegmentRepository
}

// NewSegmentService creates a new segment service
func NewSegmentService(repo *repository.SegmentRepository) *SegmentService {
	return &SegmentService{repo: repo}
}

// CreateSegmentRequest represents a request to create a segment
type CreateSegmentRequest struct {
	TenantID    string       `json:"tenantId" binding:"required"`
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	Rules       models.JSONB `json:"rules"`
	IsDynamic   bool         `json:"isDynamic"`
	IsActive    *bool        `json:"isActive"`
}

// UpdateSegmentRequest represents a request to update a segment
type UpdateSegmentRequest struct {
	Name        *string       `json:"name"`
	Description *string       `json:"description"`
	Rules       *models.JSONB `json:"rules"`
	IsDynamic   *bool         `json:"isDynamic"`
	IsActive    *bool         `json:"isActive"`
}

// ListSegments returns all segments for a tenant
func (s *SegmentService) ListSegments(ctx context.Context, tenantID string) ([]models.CustomerSegment, error) {
	return s.repo.ListSegments(ctx, tenantID)
}

// GetSegment returns a specific segment
func (s *SegmentService) GetSegment(ctx context.Context, tenantID string, segmentID uuid.UUID) (*models.CustomerSegment, error) {
	return s.repo.GetSegment(ctx, tenantID, segmentID)
}

// CreateSegment creates a new segment
func (s *SegmentService) CreateSegment(ctx context.Context, req CreateSegmentRequest) (*models.CustomerSegment, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	segment := &models.CustomerSegment{
		TenantID:      req.TenantID,
		Name:          req.Name,
		Description:   req.Description,
		Rules:         req.Rules,
		IsDynamic:     req.IsDynamic,
		IsActive:      isActive,
		CustomerCount: 0,
	}

	if err := s.repo.CreateSegment(ctx, segment); err != nil {
		return nil, err
	}

	return segment, nil
}

// UpdateSegment updates an existing segment
func (s *SegmentService) UpdateSegment(ctx context.Context, tenantID string, segmentID uuid.UUID, req UpdateSegmentRequest) (*models.CustomerSegment, error) {
	segment, err := s.repo.GetSegment(ctx, tenantID, segmentID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		segment.Name = *req.Name
	}
	if req.Description != nil {
		segment.Description = *req.Description
	}
	if req.Rules != nil {
		segment.Rules = *req.Rules
	}
	if req.IsDynamic != nil {
		segment.IsDynamic = *req.IsDynamic
	}
	if req.IsActive != nil {
		segment.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateSegment(ctx, segment); err != nil {
		return nil, err
	}

	return segment, nil
}

// DeleteSegment deletes a segment
func (s *SegmentService) DeleteSegment(ctx context.Context, tenantID string, segmentID uuid.UUID) error {
	return s.repo.DeleteSegment(ctx, tenantID, segmentID)
}

// AddCustomersToSegment adds customers to a segment (manual, not auto-added)
func (s *SegmentService) AddCustomersToSegment(ctx context.Context, tenantID string, segmentID uuid.UUID, customerIDs []uuid.UUID) error {
	if len(customerIDs) == 0 {
		return errors.New("no customer IDs provided")
	}

	return s.repo.AddCustomersToSegmentManual(ctx, tenantID, segmentID, customerIDs)
}

// RemoveCustomersFromSegment removes customers from a segment
func (s *SegmentService) RemoveCustomersFromSegment(ctx context.Context, tenantID string, segmentID uuid.UUID, customerIDs []uuid.UUID) error {
	if len(customerIDs) == 0 {
		return errors.New("no customer IDs provided")
	}

	return s.repo.RemoveCustomersFromSegment(ctx, tenantID, segmentID, customerIDs)
}

// GetSegmentCustomers returns all customers in a segment
func (s *SegmentService) GetSegmentCustomers(ctx context.Context, tenantID string, segmentID uuid.UUID) ([]models.Customer, error) {
	return s.repo.GetSegmentCustomers(ctx, tenantID, segmentID)
}
