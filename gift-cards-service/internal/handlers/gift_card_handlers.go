package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gift-cards-service/internal/models"
	"gift-cards-service/internal/repository"
)

type GiftCardHandler struct {
	repo *repository.GiftCardRepository
}

func NewGiftCardHandler(repo *repository.GiftCardRepository) *GiftCardHandler {
	return &GiftCardHandler{repo: repo}
}

// CreateGiftCard creates a new gift card
func (h *GiftCardHandler) CreateGiftCard(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, userExists := c.Get("user_id")

	var req models.CreateGiftCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	giftCard := &models.GiftCard{
		InitialBalance: req.InitialBalance,
		CurrencyCode:   "USD",
		RecipientEmail: req.RecipientEmail,
		RecipientName:  req.RecipientName,
		SenderName:     req.SenderName,
		Message:        req.Message,
		ExpiresAt:      req.ExpiresAt,
		Metadata:       req.Metadata,
	}

	// Set CreatedBy only if user is authenticated
	if userExists && userID != nil {
		if uid, ok := userID.(string); ok && uid != "" {
			giftCard.CreatedBy = stringPtr(uid)
		}
	}

	if req.CurrencyCode != "" {
		giftCard.CurrencyCode = req.CurrencyCode
	}

	if err := h.repo.CreateGiftCard(tenantID.(string), giftCard); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create gift card",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.GiftCardResponse{
		Success: true,
		Data:    giftCard,
		Message: stringPtr("Gift card created successfully"),
	})
}

// GetGiftCard retrieves a gift card by ID
func (h *GiftCardHandler) GetGiftCard(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid gift card ID",
			},
		})
		return
	}

	giftCard, err := h.repo.GetGiftCardByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Gift card not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GiftCardResponse{
		Success: true,
		Data:    giftCard,
	})
}

// ListGiftCards retrieves gift cards with filters
func (h *GiftCardHandler) ListGiftCards(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	req := &models.SearchGiftCardsRequest{
		Page:  page,
		Limit: limit,
	}

	// Parse filters
	if query := c.Query("query"); query != "" {
		req.Query = &query
	}

	if status := c.Query("status"); status != "" {
		req.Status = []models.GiftCardStatus{models.GiftCardStatus(status)}
	}

	if recipientEmail := c.Query("recipientEmail"); recipientEmail != "" {
		req.RecipientEmail = &recipientEmail
	}

	if minBalance := c.Query("minBalance"); minBalance != "" {
		if val, err := strconv.ParseFloat(minBalance, 64); err == nil {
			req.MinBalance = &val
		}
	}

	if maxBalance := c.Query("maxBalance"); maxBalance != "" {
		if val, err := strconv.ParseFloat(maxBalance, 64); err == nil {
			req.MaxBalance = &val
		}
	}

	giftCards, total, err := h.repo.ListGiftCards(tenantID.(string), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve gift cards",
			},
		})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	c.JSON(http.StatusOK, models.GiftCardListResponse{
		Success:    true,
		Data:       giftCards,
		Pagination: pagination,
	})
}

// CheckBalance checks the balance of a gift card
func (h *GiftCardHandler) CheckBalance(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.CheckBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	giftCard, err := h.repo.GetGiftCardByCode(tenantID.(string), req.Code)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Gift card not found",
			},
		})
		return
	}

	// Check if expired
	if giftCard.ExpiresAt != nil && giftCard.ExpiresAt.Before(time.Now()) && giftCard.Status == models.GiftCardStatusActive {
		giftCard.Status = models.GiftCardStatusExpired
		h.repo.UpdateGiftCardStatus(tenantID.(string), giftCard.ID, models.GiftCardStatusExpired)
	}

	response := models.BalanceResponse{
		Success: true,
		Data: &struct {
			Code         string                `json:"code"`
			Balance      float64               `json:"balance"`
			CurrencyCode string                `json:"currencyCode"`
			Status       models.GiftCardStatus `json:"status"`
			ExpiresAt    *time.Time            `json:"expiresAt,omitempty"`
		}{
			Code:         giftCard.Code,
			Balance:      giftCard.CurrentBalance,
			CurrencyCode: giftCard.CurrencyCode,
			Status:       giftCard.Status,
			ExpiresAt:    giftCard.ExpiresAt,
		},
	}

	c.JSON(http.StatusOK, response)
}

// RedeemGiftCard redeems a gift card
func (h *GiftCardHandler) RedeemGiftCard(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.RedeemGiftCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	var uid *uuid.UUID
	if userID != nil {
		parsed, err := uuid.Parse(userID.(string))
		if err == nil {
			uid = &parsed
		}
	}

	giftCard, err := h.repo.RedeemGiftCard(tenantID.(string), req.Code, req.Amount, nil, uid)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "REDEMPTION_FAILED"

		if err.Error() == "gift card is not active" {
			statusCode = http.StatusBadRequest
			errorCode = "CARD_NOT_ACTIVE"
		} else if err.Error() == "gift card has expired" {
			statusCode = http.StatusBadRequest
			errorCode = "CARD_EXPIRED"
		} else if err.Error() == "insufficient balance" {
			statusCode = http.StatusBadRequest
			errorCode = "INSUFFICIENT_BALANCE"
		}

		c.JSON(statusCode, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GiftCardResponse{
		Success: true,
		Data:    giftCard,
		Message: stringPtr("Gift card redeemed successfully"),
	})
}

// ApplyGiftCard applies a gift card to an order
func (h *GiftCardHandler) ApplyGiftCard(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.ApplyGiftCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	// Get gift card to validate
	giftCard, err := h.repo.GetGiftCardByCode(tenantID.(string), req.Code)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Gift card not found",
			},
		})
		return
	}

	// Validate gift card
	if giftCard.Status != models.GiftCardStatusActive {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CARD_NOT_ACTIVE",
				Message: "Gift card is not active",
			},
		})
		return
	}

	if giftCard.ExpiresAt != nil && giftCard.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CARD_EXPIRED",
				Message: "Gift card has expired",
			},
		})
		return
	}

	if giftCard.CurrentBalance <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INSUFFICIENT_BALANCE",
				Message: "Gift card has no remaining balance",
			},
		})
		return
	}

	// Return gift card details for application
	c.JSON(http.StatusOK, models.GiftCardResponse{
		Success: true,
		Data:    giftCard,
		Message: stringPtr("Gift card is valid and can be applied"),
	})
}

// UpdateGiftCard updates a gift card
func (h *GiftCardHandler) UpdateGiftCard(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid gift card ID",
			},
		})
		return
	}

	var updates models.GiftCard
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	updates.UpdatedBy = stringPtr(userID.(string))

	if err := h.repo.UpdateGiftCard(tenantID.(string), id, &updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update gift card",
			},
		})
		return
	}

	giftCard, _ := h.repo.GetGiftCardByID(tenantID.(string), id)

	c.JSON(http.StatusOK, models.GiftCardResponse{
		Success: true,
		Data:    giftCard,
		Message: stringPtr("Gift card updated successfully"),
	})
}

// UpdateGiftCardStatus updates gift card status
func (h *GiftCardHandler) UpdateGiftCardStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid gift card ID",
			},
		})
		return
	}

	var req struct {
		Status models.GiftCardStatus `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	if err := h.repo.UpdateGiftCardStatus(tenantID.(string), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update gift card status",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Gift card status updated successfully"),
	})
}

// DeleteGiftCard deletes a gift card
func (h *GiftCardHandler) DeleteGiftCard(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid gift card ID",
			},
		})
		return
	}

	if err := h.repo.DeleteGiftCard(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete gift card",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Gift card deleted successfully"),
	})
}

// GetGiftCardStats returns gift card statistics
func (h *GiftCardHandler) GetGiftCardStats(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	stats, err := h.repo.GetGiftCardStats(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve gift card statistics",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GiftCardStatsResponse{
		Success: true,
		Data:    stats,
	})
}

// GetTransactionHistory returns transaction history for a gift card
func (h *GiftCardHandler) GetTransactionHistory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid gift card ID",
			},
		})
		return
	}

	transactions, err := h.repo.GetTransactionHistory(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve transaction history",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    transactions,
	})
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
