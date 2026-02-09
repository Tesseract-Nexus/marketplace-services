package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"marketplace-connector-service/internal/middleware"
	"marketplace-connector-service/internal/services"
)

// AuditHandler handles audit log HTTP requests for compliance queries
type AuditHandler struct {
	auditService *services.AuditService
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(auditService *services.AuditService) *AuditHandler {
	return &AuditHandler{
		auditService: auditService,
	}
}

// GetAuditLogs retrieves audit logs for compliance queries
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	opts := &services.AuditLogOptions{}

	// Parse query parameters
	if actorID := c.Query("actorId"); actorID != "" {
		opts.ActorID = actorID
	}
	if action := c.Query("action"); action != "" {
		opts.Action = action
	}
	if resourceType := c.Query("resourceType"); resourceType != "" {
		opts.ResourceType = resourceType
	}
	if resourceID := c.Query("resourceId"); resourceID != "" {
		opts.ResourceID = resourceID
	}
	if c.Query("piiOnly") == "true" {
		opts.PIIOnly = true
	}
	if startDate := c.Query("startDate"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			opts.StartDate = t
		}
	}
	if endDate := c.Query("endDate"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			opts.EndDate = t
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	} else {
		opts.Limit = 50 // Default limit
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	logs, total, err := h.auditService.GetAuditLogs(c.Request.Context(), tenantID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   logs,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	})
}

// GetPIIAccessLogs retrieves PII access logs for compliance reporting
func (h *AuditHandler) GetPIIAccessLogs(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	opts := &services.AuditLogOptions{
		PIIOnly: true,
		Limit:   50,
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}
	if startDate := c.Query("startDate"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			opts.StartDate = t
		}
	}
	if endDate := c.Query("endDate"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			opts.EndDate = t
		}
	}

	logs, total, err := h.auditService.GetAuditLogs(c.Request.Context(), tenantID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve PII access logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   logs,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	})
}
