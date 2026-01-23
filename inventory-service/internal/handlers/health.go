package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns service health status (basic)
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "inventory-service",
	})
}

// ExtendedHealthCheck returns detailed health status including Redis
func (h *InventoryHandler) ExtendedHealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	health := gin.H{
		"status":  "healthy",
		"service": "inventory-service",
		"checks":  gin.H{},
	}

	checks := health["checks"].(gin.H)

	// Check Redis
	if err := h.repo.RedisHealth(ctx); err != nil {
		checks["redis"] = gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		checks["redis"] = gin.H{
			"status": "healthy",
		}
	}

	// Add cache stats if available
	if stats := h.repo.CacheStats(); stats != nil {
		checks["cache_stats"] = gin.H{
			"l1_hits":   stats.L1Hits,
			"l1_misses": stats.L1Misses,
			"l2_hits":   stats.L2Hits,
			"l2_misses": stats.L2Misses,
		}
	}

	// Determine overall health
	for _, check := range checks {
		if checkMap, ok := check.(gin.H); ok {
			if status, ok := checkMap["status"]; ok && status == "unhealthy" {
				health["status"] = "degraded"
				break
			}
		}
	}

	c.JSON(http.StatusOK, health)
}
