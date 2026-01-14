package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
	"marketplace-connector-service/internal/services"
)

// SyncHandler handles sync job endpoints
type SyncHandler struct {
	service     *services.SyncService
	mappingRepo *repository.MappingRepository
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(service *services.SyncService, mappingRepo *repository.MappingRepository) *SyncHandler {
	return &SyncHandler{
		service:     service,
		mappingRepo: mappingRepo,
	}
}

// ListJobs returns all sync jobs for a tenant
func (h *SyncHandler) ListJobs(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	opts := &repository.SyncListOptions{
		Status:   c.Query("status"),
		SyncType: c.Query("syncType"),
	}

	if connectionID := c.Query("connectionId"); connectionID != "" {
		if id, err := uuid.Parse(connectionID); err == nil {
			opts.ConnectionID = id
		}
	}

	jobs, total, err := h.service.ListJobs(c.Request.Context(), tenantID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  jobs,
		"total": total,
	})
}

// CreateJob creates a new sync job
func (h *SyncHandler) CreateJob(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	var req services.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.service.CreateJob(c.Request.Context(), tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": job})
}

// GetJob returns a single sync job
func (h *SyncHandler) GetJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	job, err := h.service.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	// Verify tenant
	tenantID := c.GetString("tenantId")
	if job.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": job})
}

// CancelJob cancels a running sync job
func (h *SyncHandler) CancelJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.CancelJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "job cancelled"})
}

// GetJobLogs returns logs for a sync job
func (h *SyncHandler) GetJobLogs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	logs, err := h.service.GetJobLogs(c.Request.Context(), id, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// GetStats returns sync statistics
func (h *SyncHandler) GetStats(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	var connectionID *uuid.UUID
	if connIDStr := c.Query("connectionId"); connIDStr != "" {
		if id, err := uuid.Parse(connIDStr); err == nil {
			connectionID = &id
		}
	}

	stats, err := h.service.GetStats(c.Request.Context(), tenantID, connectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// ListProductMappings returns product mappings with pagination
func (h *SyncHandler) ListProductMappings(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	opts := repository.MappingListOptions{
		TenantID:   tenantID,
		SyncStatus: c.Query("syncStatus"),
	}

	// Parse connectionId
	if connectionID := c.Query("connectionId"); connectionID != "" {
		if id, err := uuid.Parse(connectionID); err == nil {
			opts.ConnectionID = id
		}
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			opts.Limit = l
		}
	}
	if opts.Limit == 0 {
		opts.Limit = 50 // default limit
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			opts.Offset = o
		}
	}

	mappings, total, err := h.mappingRepo.ListProductMappings(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   mappings,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	})
}

// CreateProductMappingRequest represents the request to create a product mapping
type CreateProductMappingRequest struct {
	ConnectionID      uuid.UUID  `json:"connectionId" binding:"required"`
	InternalProductID uuid.UUID  `json:"internalProductId" binding:"required"`
	InternalVariantID *uuid.UUID `json:"internalVariantId"`
	ExternalProductID string     `json:"externalProductId" binding:"required"`
	ExternalVariantID *string    `json:"externalVariantId"`
	ExternalSKU       *string    `json:"externalSku"`
}

// CreateProductMapping creates a manual product mapping
func (h *SyncHandler) CreateProductMapping(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	var req CreateProductMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	mapping := &models.MarketplaceProductMapping{
		ConnectionID:      req.ConnectionID,
		TenantID:          tenantID,
		InternalProductID: req.InternalProductID,
		InternalVariantID: req.InternalVariantID,
		ExternalProductID: req.ExternalProductID,
		ExternalVariantID: req.ExternalVariantID,
		ExternalSKU:       req.ExternalSKU,
		SyncStatus:        models.MappingSynced,
		LastSyncedAt:      &now,
	}

	if err := h.mappingRepo.UpsertProductMapping(c.Request.Context(), mapping); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": mapping})
}

// DeleteProductMapping deletes a product mapping
func (h *SyncHandler) DeleteProductMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mapping id"})
		return
	}

	if err := h.mappingRepo.DeleteProductMapping(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "mapping deleted"})
}

// ListOrderMappings returns order mappings with pagination
func (h *SyncHandler) ListOrderMappings(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	opts := repository.MappingListOptions{
		TenantID:   tenantID,
		SyncStatus: c.Query("syncStatus"),
	}

	// Parse connectionId
	if connectionID := c.Query("connectionId"); connectionID != "" {
		if id, err := uuid.Parse(connectionID); err == nil {
			opts.ConnectionID = id
		}
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			opts.Limit = l
		}
	}
	if opts.Limit == 0 {
		opts.Limit = 50 // default limit
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			opts.Offset = o
		}
	}

	mappings, total, err := h.mappingRepo.ListOrderMappings(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   mappings,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	})
}

// ListInventoryMappings returns inventory mappings with pagination
func (h *SyncHandler) ListInventoryMappings(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	opts := repository.MappingListOptions{
		TenantID:   tenantID,
		SyncStatus: c.Query("syncStatus"),
	}

	// Parse connectionId
	if connectionID := c.Query("connectionId"); connectionID != "" {
		if id, err := uuid.Parse(connectionID); err == nil {
			opts.ConnectionID = id
		}
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			opts.Limit = l
		}
	}
	if opts.Limit == 0 {
		opts.Limit = 50 // default limit
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			opts.Offset = o
		}
	}

	mappings, total, err := h.mappingRepo.ListInventoryMappings(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   mappings,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	})
}

// CreateInventoryMappingRequest represents the request to create an inventory mapping
type CreateInventoryMappingRequest struct {
	ConnectionID       uuid.UUID  `json:"connectionId" binding:"required"`
	InternalProductID  uuid.UUID  `json:"internalProductId" binding:"required"`
	InternalVariantID  *uuid.UUID `json:"internalVariantId"`
	InternalSKU        string     `json:"internalSku" binding:"required"`
	ExternalSKU        string     `json:"externalSku" binding:"required"`
	ExternalLocationID *string    `json:"externalLocationId"`
}

// CreateInventoryMapping creates a manual inventory mapping
func (h *SyncHandler) CreateInventoryMapping(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	var req CreateInventoryMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mapping := &models.MarketplaceInventoryMapping{
		ConnectionID:       req.ConnectionID,
		TenantID:           tenantID,
		InternalProductID:  req.InternalProductID,
		InternalVariantID:  req.InternalVariantID,
		InternalSKU:        req.InternalSKU,
		ExternalSKU:        req.ExternalSKU,
		ExternalLocationID: req.ExternalLocationID,
		SyncStatus:         models.MappingSynced,
	}

	if err := h.mappingRepo.UpsertInventoryMapping(c.Request.Context(), mapping); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": mapping})
}

// DeleteInventoryMapping deletes an inventory mapping
func (h *SyncHandler) DeleteInventoryMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mapping id"})
		return
	}

	if err := h.mappingRepo.DeleteInventoryMapping(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "mapping deleted"})
}

// GetProductMapping retrieves a single product mapping by ID
func (h *SyncHandler) GetProductMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mapping id"})
		return
	}

	mapping, err := h.mappingRepo.GetProductMappingByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}

	// Verify tenant
	tenantID := c.GetString("tenantId")
	if mapping.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": mapping})
}

// GetOrderMapping retrieves a single order mapping by ID
func (h *SyncHandler) GetOrderMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mapping id"})
		return
	}

	mapping, err := h.mappingRepo.GetOrderMappingByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}

	// Verify tenant
	tenantID := c.GetString("tenantId")
	if mapping.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": mapping})
}

// GetInventoryMapping retrieves a single inventory mapping by ID
func (h *SyncHandler) GetInventoryMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mapping id"})
		return
	}

	mapping, err := h.mappingRepo.GetInventoryMappingByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}

	// Verify tenant
	tenantID := c.GetString("tenantId")
	if mapping.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": mapping})
}
