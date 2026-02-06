package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	marketingevents "marketing-service/internal/events"
	"marketing-service/internal/models"
	"marketing-service/internal/services"
)

// MarketingHandlers handles HTTP requests for marketing
type MarketingHandlers struct {
	service   *services.MarketingService
	publisher *marketingevents.Publisher
	logger    *logrus.Logger
}

// NewMarketingHandlers creates a new marketing handlers instance
func NewMarketingHandlers(service *services.MarketingService, publisher *marketingevents.Publisher, logger *logrus.Logger) *MarketingHandlers {
	return &MarketingHandlers{
		service:   service,
		publisher: publisher,
		logger:    logger,
	}
}

// ===== CAMPAIGNS =====

// CreateCampaign creates a new campaign
// POST /api/v1/campaigns
func (h *MarketingHandlers) CreateCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaign.TenantID = tenantID
	campaign.CreatedBy, _ = uuid.Parse(userID)

	if err := h.service.CreateCampaign(c.Request.Context(), &campaign); err != nil {
		h.logger.WithError(err).Error("Failed to create campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCampaignCreated(c.Request.Context(), tenantID, campaign.ID.String(), campaign.Name, string(campaign.Type), string(campaign.Channel), userID); err != nil {
			h.logger.WithError(err).Error("Failed to publish campaign created event")
		}
	}

	c.JSON(http.StatusCreated, campaign)
}

// GetCampaign retrieves a campaign by ID
// GET /api/v1/campaigns/:id
func (h *MarketingHandlers) GetCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	campaign, err := h.service.GetCampaign(c.Request.Context(), tenantID, id)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get campaign")
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// ListCampaigns retrieves campaigns with filters
// GET /api/v1/campaigns
func (h *MarketingHandlers) ListCampaigns(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	filter := &models.CampaignFilter{
		TenantID:    tenantID,
		SearchQuery: c.Query("search"),
		Limit:       h.getLimit(c),
		Offset:      h.getOffset(c),
	}

	if typeStr := c.Query("type"); typeStr != "" {
		t := models.CampaignType(typeStr)
		filter.Type = &t
	}
	if channelStr := c.Query("channel"); channelStr != "" {
		ch := models.CampaignChannel(channelStr)
		filter.Channel = &ch
	}
	if statusStr := c.Query("status"); statusStr != "" {
		st := models.CampaignStatus(statusStr)
		filter.Status = &st
	}

	campaigns, total, err := h.service.ListCampaigns(c.Request.Context(), filter)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list campaigns")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list campaigns"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"campaigns": campaigns,
		"total":     total,
		"limit":     filter.Limit,
		"offset":    filter.Offset,
	})
}

// UpdateCampaign updates a campaign
// PUT /api/v1/campaigns/:id
func (h *MarketingHandlers) UpdateCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaign.ID = id
	campaign.TenantID = tenantID

	if err := h.service.UpdateCampaign(c.Request.Context(), &campaign); err != nil {
		h.logger.WithError(err).Error("Failed to update campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update campaign"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCampaignUpdated(c.Request.Context(), tenantID, campaign.ID.String(), campaign.Name, string(campaign.Status), c.GetString("user_id")); err != nil {
			h.logger.WithError(err).Error("Failed to publish campaign updated event")
		}
	}

	c.JSON(http.StatusOK, campaign)
}

// DeleteCampaign deletes a campaign
// DELETE /api/v1/campaigns/:id
func (h *MarketingHandlers) DeleteCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	if err := h.service.DeleteCampaign(c.Request.Context(), tenantID, id); err != nil {
		h.logger.WithError(err).Error("Failed to delete campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete campaign"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCampaignDeleted(c.Request.Context(), tenantID, id.String(), "", c.GetString("user_id")); err != nil {
			h.logger.WithError(err).Error("Failed to publish campaign deleted event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaign deleted successfully"})
}

// SendCampaign sends a campaign
// POST /api/v1/campaigns/:id/send
func (h *MarketingHandlers) SendCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	if err := h.service.SendCampaign(c.Request.Context(), tenantID, id); err != nil {
		h.logger.WithError(err).Error("Failed to send campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCampaignSent(c.Request.Context(), tenantID, id.String(), "", "", "", 0, c.GetString("user_id")); err != nil {
			h.logger.WithError(err).Error("Failed to publish campaign sent event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaign sent successfully"})
}

// ScheduleCampaign schedules a campaign
// POST /api/v1/campaigns/:id/schedule
func (h *MarketingHandlers) ScheduleCampaign(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid campaign ID"})
		return
	}

	var req struct {
		ScheduledAt time.Time `json:"scheduledAt" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ScheduleCampaign(c.Request.Context(), tenantID, id, req.ScheduledAt); err != nil {
		h.logger.WithError(err).Error("Failed to schedule campaign")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to schedule campaign"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCampaignScheduled(c.Request.Context(), tenantID, id.String(), "", req.ScheduledAt.Format(time.RFC3339), c.GetString("user_id")); err != nil {
			h.logger.WithError(err).Error("Failed to publish campaign scheduled event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaign scheduled successfully"})
}

// GetCampaignStats retrieves campaign statistics
// GET /api/v1/campaigns/stats
func (h *MarketingHandlers) GetCampaignStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	stats, err := h.service.GetCampaignStats(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get campaign stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ===== SEGMENTS =====

// CreateSegment creates a new customer segment
// POST /api/v1/segments
func (h *MarketingHandlers) CreateSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var segment models.CustomerSegment
	if err := c.ShouldBindJSON(&segment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segment.TenantID = tenantID
	segment.CreatedBy, _ = uuid.Parse(userID)

	if err := h.service.CreateSegment(c.Request.Context(), &segment); err != nil {
		h.logger.WithError(err).Error("Failed to create segment")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create segment"})
		return
	}

	c.JSON(http.StatusCreated, segment)
}

// GetSegment retrieves a segment by ID
// GET /api/v1/segments/:id
func (h *MarketingHandlers) GetSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment ID"})
		return
	}

	segment, err := h.service.GetSegment(c.Request.Context(), tenantID, id)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get segment")
		c.JSON(http.StatusNotFound, gin.H{"error": "Segment not found"})
		return
	}

	c.JSON(http.StatusOK, segment)
}

// ListSegments retrieves segments with filters
// GET /api/v1/segments
func (h *MarketingHandlers) ListSegments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	filter := &models.SegmentFilter{
		TenantID:    tenantID,
		SearchQuery: c.Query("search"),
		Limit:       h.getLimit(c),
		Offset:      h.getOffset(c),
	}

	if typeStr := c.Query("type"); typeStr != "" {
		t := models.SegmentType(typeStr)
		filter.Type = &t
	}
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		filter.IsActive = &isActive
	}

	segments, total, err := h.service.ListSegments(c.Request.Context(), filter)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list segments")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list segments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"segments": segments,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

// UpdateSegment updates a segment
// PUT /api/v1/segments/:id
func (h *MarketingHandlers) UpdateSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment ID"})
		return
	}

	var segment models.CustomerSegment
	if err := c.ShouldBindJSON(&segment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segment.ID = id
	segment.TenantID = tenantID

	if err := h.service.UpdateSegment(c.Request.Context(), &segment); err != nil {
		h.logger.WithError(err).Error("Failed to update segment")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update segment"})
		return
	}

	c.JSON(http.StatusOK, segment)
}

// DeleteSegment deletes a segment
// DELETE /api/v1/segments/:id
func (h *MarketingHandlers) DeleteSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment ID"})
		return
	}

	if err := h.service.DeleteSegment(c.Request.Context(), tenantID, id); err != nil {
		h.logger.WithError(err).Error("Failed to delete segment")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete segment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Segment deleted successfully"})
}

// ===== ABANDONED CARTS =====

// CreateAbandonedCart creates an abandoned cart record
// POST /api/v1/abandoned-carts
func (h *MarketingHandlers) CreateAbandonedCart(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var cart models.AbandonedCart
	if err := c.ShouldBindJSON(&cart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart.TenantID = tenantID

	if err := h.service.CreateAbandonedCart(c.Request.Context(), &cart); err != nil {
		h.logger.WithError(err).Error("Failed to create abandoned cart")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create abandoned cart"})
		return
	}

	c.JSON(http.StatusCreated, cart)
}

// ListAbandonedCarts retrieves abandoned carts with filters
// GET /api/v1/abandoned-carts
func (h *MarketingHandlers) ListAbandonedCarts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	filter := &models.AbandonedCartFilter{
		TenantID: tenantID,
		Limit:    h.getLimit(c),
		Offset:   h.getOffset(c),
	}

	if customerIDStr := c.Query("customer_id"); customerIDStr != "" {
		customerID, err := uuid.Parse(customerIDStr)
		if err == nil {
			filter.CustomerID = &customerID
		}
	}
	if statusStr := c.Query("status"); statusStr != "" {
		st := models.AbandonedStatus(statusStr)
		filter.Status = &st
	}
	if minAmountStr := c.Query("min_amount"); minAmountStr != "" {
		if minAmount, err := strconv.ParseFloat(minAmountStr, 64); err == nil {
			filter.MinAmount = &minAmount
		}
	}
	if maxAmountStr := c.Query("max_amount"); maxAmountStr != "" {
		if maxAmount, err := strconv.ParseFloat(maxAmountStr, 64); err == nil {
			filter.MaxAmount = &maxAmount
		}
	}

	carts, total, err := h.service.ListAbandonedCarts(c.Request.Context(), filter)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list abandoned carts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list abandoned carts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"carts":  carts,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GetAbandonedCartStats retrieves statistics
// GET /api/v1/abandoned-carts/stats
func (h *MarketingHandlers) GetAbandonedCartStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	fromDate, toDate := h.getDateRange(c)

	stats, err := h.service.GetAbandonedCartStats(c.Request.Context(), tenantID, fromDate, toDate)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get abandoned cart stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ===== LOYALTY PROGRAM =====

// CreateLoyaltyProgram creates a loyalty program
// POST /api/v1/loyalty/program
func (h *MarketingHandlers) CreateLoyaltyProgram(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var program models.LoyaltyProgram
	if err := c.ShouldBindJSON(&program); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	program.TenantID = tenantID

	if err := h.service.CreateLoyaltyProgram(c.Request.Context(), &program); err != nil {
		h.logger.WithError(err).Error("Failed to create loyalty program")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create loyalty program"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishLoyaltyProgramCreated(c.Request.Context(), tenantID, program.ID.String(), program.Name); err != nil {
			h.logger.WithError(err).Error("Failed to publish loyalty program created event")
		}
	}

	c.JSON(http.StatusCreated, program)
}

// GetLoyaltyProgram retrieves a loyalty program
// GET /api/v1/loyalty/program
func (h *MarketingHandlers) GetLoyaltyProgram(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	program, err := h.service.GetLoyaltyProgram(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get loyalty program")
		c.JSON(http.StatusNotFound, gin.H{"error": "Loyalty program not found"})
		return
	}

	c.JSON(http.StatusOK, program)
}

// UpdateLoyaltyProgram updates a loyalty program
// PUT /api/v1/loyalty/program
func (h *MarketingHandlers) UpdateLoyaltyProgram(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var program models.LoyaltyProgram
	if err := c.ShouldBindJSON(&program); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	program.TenantID = tenantID

	if err := h.service.UpdateLoyaltyProgram(c.Request.Context(), &program); err != nil {
		h.logger.WithError(err).Error("Failed to update loyalty program")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update loyalty program"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishLoyaltyProgramUpdated(c.Request.Context(), tenantID, program.ID.String(), program.Name); err != nil {
			h.logger.WithError(err).Error("Failed to publish loyalty program updated event")
		}
	}

	c.JSON(http.StatusOK, program)
}

// GetCustomerLoyalty retrieves a customer's loyalty account
// GET /api/v1/loyalty/customers/:customer_id
func (h *MarketingHandlers) GetCustomerLoyalty(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	loyalty, err := h.service.GetCustomerLoyalty(c.Request.Context(), tenantID, customerID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get customer loyalty")
		c.JSON(http.StatusNotFound, gin.H{"error": "Loyalty account not found"})
		return
	}

	c.JSON(http.StatusOK, loyalty)
}

// EnrollCustomer enrolls a customer in the loyalty program
// POST /api/v1/loyalty/customers/:customer_id/enroll
func (h *MarketingHandlers) EnrollCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	// Check for optional referral code and date of birth in request body
	var req struct {
		ReferralCode string     `json:"referralCode"`
		DateOfBirth  *time.Time `json:"dateOfBirth"`
	}
	c.ShouldBindJSON(&req) // Ignore error - fields are optional

	loyalty, err := h.service.EnrollCustomerWithReferral(c.Request.Context(), tenantID, customerID, req.ReferralCode, req.DateOfBirth)
	if err != nil {
		h.logger.WithError(err).Error("Failed to enroll customer")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enroll customer"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCustomerEnrolled(c.Request.Context(), tenantID, "", "", customerID.String()); err != nil {
			h.logger.WithError(err).Error("Failed to publish customer enrolled event")
		}
	}

	c.JSON(http.StatusCreated, loyalty)
}

// GetReferralStats retrieves referral statistics for a customer
// GET /api/v1/loyalty/customers/:customer_id/referrals/stats
func (h *MarketingHandlers) GetReferralStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	stats, err := h.service.GetReferralStats(c.Request.Context(), tenantID, customerID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get referral stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get referral stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetReferrals retrieves referrals made by a customer
// GET /api/v1/loyalty/customers/:customer_id/referrals
func (h *MarketingHandlers) GetReferrals(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	referrals, total, err := h.service.GetReferrals(c.Request.Context(), tenantID, customerID, h.getLimit(c), h.getOffset(c))
	if err != nil {
		h.logger.WithError(err).Error("Failed to get referrals")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get referrals"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"referrals": referrals,
		"total":     total,
	})
}

// RedeemPoints redeems loyalty points
// POST /api/v1/loyalty/customers/:customer_id/redeem
func (h *MarketingHandlers) RedeemPoints(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		Points      int    `json:"points" binding:"required"`
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.RedeemPoints(c.Request.Context(), tenantID, customerID, req.Points, req.Description); err != nil {
		h.logger.WithError(err).Error("Failed to redeem points")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishPointsRedeemed(c.Request.Context(), tenantID, customerID.String(), req.Points, req.Description); err != nil {
			h.logger.WithError(err).Error("Failed to publish points redeemed event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Points redeemed successfully"})
}

// GetLoyaltyTransactions retrieves transactions for a customer
// GET /api/v1/loyalty/customers/:customer_id/transactions
func (h *MarketingHandlers) GetLoyaltyTransactions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	txns, total, err := h.service.GetLoyaltyTransactions(c.Request.Context(), tenantID, customerID, h.getLimit(c), h.getOffset(c))
	if err != nil {
		h.logger.WithError(err).Error("Failed to get loyalty transactions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txns,
		"total":        total,
	})
}

// ===== COUPONS =====

// CreateCoupon creates a new coupon
// POST /api/v1/coupons
func (h *MarketingHandlers) CreateCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var coupon models.CouponCode
	if err := c.ShouldBindJSON(&coupon); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	coupon.TenantID = tenantID
	coupon.CreatedBy, _ = uuid.Parse(userID)

	if err := h.service.CreateCoupon(c.Request.Context(), &coupon); err != nil {
		h.logger.WithError(err).Error("Failed to create coupon")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCouponCreated(c.Request.Context(), tenantID, coupon.ID.String(), coupon.Code, string(coupon.Type), coupon.DiscountValue, userID); err != nil {
			h.logger.WithError(err).Error("Failed to publish coupon created event")
		}
	}

	c.JSON(http.StatusCreated, coupon)
}

// GetCoupon retrieves a coupon by ID
// GET /api/v1/coupons/:id
func (h *MarketingHandlers) GetCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon ID"})
		return
	}

	coupon, err := h.service.GetCoupon(c.Request.Context(), tenantID, id)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get coupon")
		c.JSON(http.StatusNotFound, gin.H{"error": "Coupon not found"})
		return
	}

	c.JSON(http.StatusOK, coupon)
}

// ListCoupons retrieves all coupons
// GET /api/v1/coupons
func (h *MarketingHandlers) ListCoupons(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	coupons, total, err := h.service.ListCoupons(c.Request.Context(), tenantID, h.getLimit(c), h.getOffset(c))
	if err != nil {
		h.logger.WithError(err).Error("Failed to list coupons")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list coupons"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"coupons": coupons,
		"total":   total,
	})
}

// UpdateCoupon updates a coupon
// PUT /api/v1/coupons/:id
func (h *MarketingHandlers) UpdateCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon ID"})
		return
	}

	var coupon models.CouponCode
	if err := c.ShouldBindJSON(&coupon); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	coupon.ID = id
	coupon.TenantID = tenantID

	if err := h.service.UpdateCoupon(c.Request.Context(), &coupon); err != nil {
		h.logger.WithError(err).Error("Failed to update coupon")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCouponUpdated(c.Request.Context(), tenantID, coupon.ID.String(), coupon.Code, ""); err != nil {
			h.logger.WithError(err).Error("Failed to publish coupon updated event")
		}
	}

	c.JSON(http.StatusOK, coupon)
}

// DeleteCoupon deletes a coupon
// DELETE /api/v1/coupons/:id
func (h *MarketingHandlers) DeleteCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon ID"})
		return
	}

	if err := h.service.DeleteCoupon(c.Request.Context(), tenantID, id); err != nil {
		h.logger.WithError(err).Error("Failed to delete coupon")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete coupon"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCouponDeleted(c.Request.Context(), tenantID, id.String(), ""); err != nil {
			h.logger.WithError(err).Error("Failed to publish coupon deleted event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Coupon deleted successfully"})
}

// ValidateCoupon validates a coupon for use
// POST /api/v1/coupons/validate
func (h *MarketingHandlers) ValidateCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req struct {
		Code        string    `json:"code" binding:"required"`
		CustomerID  uuid.UUID `json:"customerId" binding:"required"`
		OrderAmount float64   `json:"orderAmount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	coupon, err := h.service.ValidateCoupon(c.Request.Context(), tenantID, req.Code, req.CustomerID, req.OrderAmount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "valid": false})
		return
	}

	discount := h.service.ApplyCoupon(c.Request.Context(), coupon, req.OrderAmount)

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"coupon":   coupon,
		"discount": discount,
	})
}

// ===== STOREFRONT HANDLERS =====
// These handlers are for public storefront operations
// Customer ID comes from X-Customer-ID header instead of URL param

// GetStorefrontCustomerLoyalty retrieves a customer's loyalty account for storefront
// GET /api/v1/storefront/loyalty/customer
func (h *MarketingHandlers) GetStorefrontCustomerLoyalty(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerIDStr := c.GetHeader("X-Customer-ID")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Customer-ID header"})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	loyalty, err := h.service.GetCustomerLoyalty(c.Request.Context(), tenantID, customerID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get customer loyalty")
		c.JSON(http.StatusNotFound, gin.H{"error": "Loyalty account not found"})
		return
	}

	c.JSON(http.StatusOK, loyalty)
}

// StorefrontEnrollCustomer enrolls a customer in the loyalty program from storefront
// POST /api/v1/storefront/loyalty/enroll
func (h *MarketingHandlers) StorefrontEnrollCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerIDStr := c.GetHeader("X-Customer-ID")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Customer-ID header"})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	// Check for optional referral code and date of birth in request body
	var req struct {
		ReferralCode string     `json:"referralCode"`
		DateOfBirth  *time.Time `json:"dateOfBirth"`
	}
	c.ShouldBindJSON(&req) // Ignore error - fields are optional

	loyalty, err := h.service.EnrollCustomerWithReferral(c.Request.Context(), tenantID, customerID, req.ReferralCode, req.DateOfBirth)
	if err != nil {
		h.logger.WithError(err).Error("Failed to enroll customer")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enroll customer"})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishCustomerEnrolled(c.Request.Context(), tenantID, "", "", customerID.String()); err != nil {
			h.logger.WithError(err).Error("Failed to publish customer enrolled event")
		}
	}

	c.JSON(http.StatusCreated, loyalty)
}

// StorefrontGetReferralStats retrieves referral statistics for a customer from storefront
// GET /api/v1/storefront/loyalty/referrals/stats
func (h *MarketingHandlers) StorefrontGetReferralStats(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerIDStr := c.GetHeader("X-Customer-ID")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Customer-ID header"})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	stats, err := h.service.GetReferralStats(c.Request.Context(), tenantID, customerID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get referral stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get referral stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// StorefrontGetReferrals retrieves referrals made by a customer from storefront
// GET /api/v1/storefront/loyalty/referrals
func (h *MarketingHandlers) StorefrontGetReferrals(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerIDStr := c.GetHeader("X-Customer-ID")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Customer-ID header"})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	referrals, total, err := h.service.GetReferrals(c.Request.Context(), tenantID, customerID, h.getLimit(c), h.getOffset(c))
	if err != nil {
		h.logger.WithError(err).Error("Failed to get referrals")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get referrals"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"referrals": referrals,
		"total":     total,
	})
}

// StorefrontRedeemPoints redeems loyalty points from storefront
// POST /api/v1/storefront/loyalty/redeem
func (h *MarketingHandlers) StorefrontRedeemPoints(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerIDStr := c.GetHeader("X-Customer-ID")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Customer-ID header"})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		Points      int    `json:"points" binding:"required"`
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.RedeemPoints(c.Request.Context(), tenantID, customerID, req.Points, req.Description); err != nil {
		h.logger.WithError(err).Error("Failed to redeem points")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.publisher != nil {
		if err := h.publisher.PublishPointsRedeemed(c.Request.Context(), tenantID, customerID.String(), req.Points, req.Description); err != nil {
			h.logger.WithError(err).Error("Failed to publish points redeemed event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Points redeemed successfully"})
}

// GetStorefrontLoyaltyTransactions retrieves transactions for a customer from storefront
// GET /api/v1/storefront/loyalty/transactions
func (h *MarketingHandlers) GetStorefrontLoyaltyTransactions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	customerIDStr := c.GetHeader("X-Customer-ID")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Customer-ID header"})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	txns, total, err := h.service.GetLoyaltyTransactions(c.Request.Context(), tenantID, customerID, h.getLimit(c), h.getOffset(c))
	if err != nil {
		h.logger.WithError(err).Error("Failed to get loyalty transactions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txns,
		"total":        total,
	})
}

// ===== BIRTHDAY BONUSES =====

// TriggerBirthdayBonuses triggers birthday bonus processing for a tenant
// POST /api/v1/loyalty/birthday-bonuses
func (h *MarketingHandlers) TriggerBirthdayBonuses(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	awarded, err := h.service.AwardBirthdayBonuses(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to process birthday bonuses")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process birthday bonuses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Birthday bonuses processed",
		"awarded": awarded,
	})
}

// ===== HELPER FUNCTIONS =====

func (h *MarketingHandlers) getLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

func (h *MarketingHandlers) getOffset(c *gin.Context) int {
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}
	return offset
}

func (h *MarketingHandlers) getDateRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	to := now
	from := now.AddDate(0, 0, -30) // Default: last 30 days

	if fromStr := c.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = parsed
		}
	}

	return from, to
}
