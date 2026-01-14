package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns the health status of the service
// @Summary Health check
// @Description Returns the health status of the reviews service
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "reviews-service",
		"version":   "1.0.0",
		"timestamp": c.Request.Header.Get("X-Request-Time"),
	})
}
