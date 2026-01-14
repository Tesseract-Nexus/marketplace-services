package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var db *gorm.DB

// SetDB sets the database connection for health checks
func SetDB(database *gorm.DB) {
	db = database
}

// HealthCheck provides a health check endpoint
// @Summary Health check
// @Description Check if the service is healthy
// @Tags health
// @Produce json
// @Success 200 {object} gin.H
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "staff-service",
		"version":   "2.0.0",
		"timestamp": time.Now().UTC(),
	})
}

// ReadinessCheck provides a readiness check endpoint
// @Summary Readiness check
// @Description Check if the service is ready to handle requests
// @Tags health
// @Produce json
// @Success 200 {object} gin.H
// @Failure 503 {object} gin.H
// @Router /ready [get]
func ReadinessCheck(c *gin.Context) {
	// Check database connectivity
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "unhealthy",
				"service":   "staff-service",
				"version":   "2.0.0",
				"timestamp": time.Now().UTC(),
				"error":     "failed to get database connection",
			})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "unhealthy",
				"service":   "staff-service",
				"version":   "2.0.0",
				"timestamp": time.Now().UTC(),
				"error":     "database connection failed",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"service":   "staff-service",
		"version":   "2.0.0",
		"timestamp": time.Now().UTC(),
		"checks": gin.H{
			"database": "connected",
		},
	})
}
