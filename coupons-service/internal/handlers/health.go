package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck handles health check requests
// @Summary Health check endpoint
// @Description Check if the service is running
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "coupons-service",
		"version": "1.0.0",
	})
}