package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/clients"
	"customers-service/internal/models"
	"customers-service/internal/repository"
)

// AbandonedCartService handles abandoned cart business logic
type AbandonedCartService struct {
	repo               *repository.AbandonedCartRepository
	customerRepo       *repository.CustomerRepository
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
}

// NewAbandonedCartService creates a new abandoned cart service
func NewAbandonedCartService(
	repo *repository.AbandonedCartRepository,
	customerRepo *repository.CustomerRepository,
	notificationClient *clients.NotificationClient,
	tenantClient *clients.TenantClient,
) *AbandonedCartService {
	return &AbandonedCartService{
		repo:               repo,
		customerRepo:       customerRepo,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
	}
}

// AbandonedCartListRequest represents request to list abandoned carts
type AbandonedCartListRequest struct {
	TenantID   string                       `json:"tenantId"`
	CustomerID *uuid.UUID                   `json:"customerId"`
	Status     *models.AbandonedCartStatus  `json:"status"`
	DateFrom   *time.Time                   `json:"dateFrom"`
	DateTo     *time.Time                   `json:"dateTo"`
	MinValue   *float64                     `json:"minValue"`
	MaxValue   *float64                     `json:"maxValue"`
	Page       int                          `json:"page"`
	Limit      int                          `json:"limit"`
	SortBy     string                       `json:"sortBy"`
	SortOrder  string                       `json:"sortOrder"`
}

// AbandonedCartListResponse represents response for listing abandoned carts
type AbandonedCartListResponse struct {
	Carts      []models.AbandonedCart `json:"carts"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	Limit      int                    `json:"limit"`
	TotalPages int                    `json:"totalPages"`
}

// List retrieves abandoned carts with filters and pagination
func (s *AbandonedCartService) List(ctx context.Context, req AbandonedCartListRequest) (*AbandonedCartListResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	offset := (req.Page - 1) * req.Limit

	filter := repository.AbandonedCartFilter{
		TenantID:   req.TenantID,
		CustomerID: req.CustomerID,
		Status:     req.Status,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		MinValue:   req.MinValue,
		MaxValue:   req.MaxValue,
		Limit:      req.Limit,
		Offset:     offset,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
	}

	carts, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list abandoned carts: %w", err)
	}

	totalPages := int(total) / req.Limit
	if int(total)%req.Limit != 0 {
		totalPages++
	}

	return &AbandonedCartListResponse{
		Carts:      carts,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetByID retrieves an abandoned cart by ID
func (s *AbandonedCartService) GetByID(ctx context.Context, tenantID string, cartID uuid.UUID) (*models.AbandonedCart, error) {
	return s.repo.GetByID(ctx, tenantID, cartID)
}

// GetStats gets abandoned cart statistics
func (s *AbandonedCartService) GetStats(ctx context.Context, tenantID string, from, to time.Time) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx, tenantID, from, to)
}

// GetSettings gets tenant settings for abandoned cart recovery
func (s *AbandonedCartService) GetSettings(ctx context.Context, tenantID string) (*models.AbandonedCartSettings, error) {
	return s.repo.GetSettings(ctx, tenantID)
}

// UpdateSettings updates tenant settings
func (s *AbandonedCartService) UpdateSettings(ctx context.Context, settings *models.AbandonedCartSettings) error {
	return s.repo.UpsertSettings(ctx, settings)
}

// DetectAbandonedCarts finds and marks carts as abandoned
func (s *AbandonedCartService) DetectAbandonedCarts(ctx context.Context, tenantID string) (int, error) {
	// Get settings for this tenant
	settings, err := s.repo.GetSettings(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to get settings: %w", err)
	}

	log.Printf("[AbandonedCartService] Detection settings for tenant %s: enabled=%v, threshold=%d minutes",
		tenantID, settings.Enabled, settings.AbandonmentThresholdMinutes)

	if !settings.Enabled {
		return 0, nil
	}

	// Find carts that have been inactive
	inactiveCarts, err := s.repo.FindAbandonedCarts(ctx, tenantID, settings.AbandonmentThresholdMinutes)
	if err != nil {
		return 0, fmt.Errorf("failed to find inactive carts: %w", err)
	}

	log.Printf("[AbandonedCartService] Found %d inactive carts for tenant %s", len(inactiveCarts), tenantID)

	count := 0
	for _, cart := range inactiveCarts {
		// Check if already tracked
		existing, err := s.repo.GetByCartID(ctx, tenantID, cart.ID)
		if err != nil {
			log.Printf("[AbandonedCartService] Error checking existing abandoned cart: %v", err)
			continue
		}

		if existing != nil {
			// Already tracked
			continue
		}

		// Get customer info
		customer, err := s.customerRepo.GetByID(ctx, tenantID, cart.CustomerID)
		if err != nil {
			log.Printf("[AbandonedCartService] Error getting customer %s: %v", cart.CustomerID, err)
			continue
		}

		// Calculate first reminder time
		firstReminderAt := time.Now().Add(time.Duration(settings.FirstReminderHours) * time.Hour)

		// Create abandoned cart record
		abandoned := &models.AbandonedCart{
			TenantID:          tenantID,
			CartID:            cart.ID,
			CustomerID:        cart.CustomerID,
			Status:            models.AbandonedCartStatusPending,
			Items:             cart.Items,
			Subtotal:          cart.Subtotal,
			ItemCount:         cart.ItemCount,
			CustomerEmail:     customer.Email,
			CustomerFirstName: customer.FirstName,
			CustomerLastName:  customer.LastName,
			AbandonedAt:       time.Now(),
			LastCartActivity:  cart.LastItemChange,
			NextReminderAt:    &firstReminderAt,
		}

		if err := s.repo.Create(ctx, abandoned); err != nil {
			log.Printf("[AbandonedCartService] Error creating abandoned cart: %v", err)
			continue
		}

		count++
	}

	return count, nil
}

// SendReminders sends reminder emails for abandoned carts
// If cartIDs is provided, only sends to those specific carts; otherwise sends to all due carts
func (s *AbandonedCartService) SendReminders(ctx context.Context, tenantID string, cartIDs []uuid.UUID) (int, error) {
	// Get settings
	settings, err := s.repo.GetSettings(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to get settings: %w", err)
	}

	if !settings.Enabled {
		return 0, nil
	}

	var carts []models.AbandonedCart
	if len(cartIDs) > 0 {
		// Get specific carts by IDs
		carts, err = s.repo.GetByIDs(ctx, tenantID, cartIDs)
		if err != nil {
			return 0, fmt.Errorf("failed to get carts by IDs: %w", err)
		}
	} else {
		// Get carts due for reminder
		carts, err = s.repo.GetDueForReminder(ctx, tenantID)
		if err != nil {
			return 0, fmt.Errorf("failed to get carts due for reminder: %w", err)
		}
	}

	count := 0
	for _, cart := range carts {
		if cart.ReminderCount >= settings.MaxReminders {
			// No more reminders allowed
			continue
		}

		// Determine next reminder time based on reminder count
		var nextReminderHours int
		var template string
		switch cart.ReminderCount {
		case 0:
			template = settings.ReminderEmailTemplate1
			nextReminderHours = settings.SecondReminderHours
		case 1:
			template = settings.ReminderEmailTemplate2
			nextReminderHours = settings.ThirdReminderHours
		default:
			template = settings.ReminderEmailTemplate3
			nextReminderHours = 0 // No more reminders
		}

		// Check if this reminder should include a discount
		discountOffered := ""
		if settings.OfferDiscountOnReminder > 0 && cart.ReminderCount+1 == settings.OfferDiscountOnReminder {
			discountOffered = settings.DiscountCode
		}

		// Send the reminder via notification service
		if s.notificationClient != nil {
			// Parse cart items for the email
			var items []models.CartItem
			if err := json.Unmarshal(cart.Items, &items); err != nil {
				log.Printf("[AbandonedCartService] Error parsing cart items: %v", err)
			}

			// Convert cart items to map format for the email template
			cartItemsMap := make([]map[string]interface{}, len(items))
			for i, item := range items {
				cartItemsMap[i] = map[string]interface{}{
					"name":     item.Name,
					"price":    item.Price,
					"quantity": item.Quantity,
					"image":    item.Image,
				}
			}

			storefrontURL := s.tenantClient.BuildStorefrontURL(ctx, tenantID)
			cartRecoveryURL := fmt.Sprintf("%s/cart?recover=%s", storefrontURL, cart.CartID.String())

			customerName := cart.CustomerFirstName
			if cart.CustomerLastName != "" {
				customerName = cart.CustomerFirstName + " " + cart.CustomerLastName
			}

			notification := &clients.AbandonedCartReminderNotification{
				TenantID:        tenantID,
				CustomerID:      cart.CustomerID.String(),
				CustomerEmail:   cart.CustomerEmail,
				CustomerName:    customerName,
				CartID:          cart.ID.String(),
				CartItems:       cartItemsMap,
				CartTotal:       cart.Subtotal,
				ReminderNumber:  cart.ReminderCount + 1,
				DiscountCode:    discountOffered,
				StorefrontURL:   storefrontURL,
				CartRecoveryURL: cartRecoveryURL,
			}

			log.Printf("[AbandonedCartService] Sending reminder %d to %s for cart %s (discount: %s)",
				cart.ReminderCount+1, cart.CustomerEmail, cart.ID, discountOffered)

			if err := s.notificationClient.SendAbandonedCartReminder(ctx, notification); err != nil {
				log.Printf("[AbandonedCartService] Error sending reminder email: %v", err)
				// Continue processing other carts even if one fails
			}
		}

		// Create recovery attempt record
		attempt := &models.AbandonedCartRecoveryAttempt{
			AbandonedCartID: cart.ID,
			TenantID:        tenantID,
			AttemptType:     "email",
			AttemptNumber:   cart.ReminderCount + 1,
			Status:          "sent",
			MessageTemplate: template,
			DiscountOffered: discountOffered,
			SentAt:          time.Now(),
		}

		if err := s.repo.CreateRecoveryAttempt(ctx, attempt); err != nil {
			log.Printf("[AbandonedCartService] Error creating recovery attempt: %v", err)
		}

		// Update cart with new reminder info
		var nextReminderAt *time.Time
		if nextReminderHours > 0 && cart.ReminderCount+1 < settings.MaxReminders {
			t := time.Now().Add(time.Duration(nextReminderHours) * time.Hour)
			nextReminderAt = &t
		}

		cart.MarkAsReminded(nextReminderAt)

		if err := s.repo.Update(ctx, &cart); err != nil {
			log.Printf("[AbandonedCartService] Error updating abandoned cart: %v", err)
		}

		count++
	}

	return count, nil
}

// MarkAsRecovered marks an abandoned cart as recovered when order is placed
func (s *AbandonedCartService) MarkAsRecovered(ctx context.Context, cartID uuid.UUID, orderID uuid.UUID, source string, discountUsed string, orderValue float64) error {
	return s.repo.MarkAsRecovered(ctx, cartID, orderID, source, discountUsed, orderValue)
}

// ExpireOldCarts marks old abandoned carts as expired
func (s *AbandonedCartService) ExpireOldCarts(ctx context.Context, tenantID string) (int64, error) {
	settings, err := s.repo.GetSettings(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to get settings: %w", err)
	}

	return s.repo.ExpireOldCarts(ctx, tenantID, settings.ExpirationDays)
}

// GetRecoveryAttempts gets all recovery attempts for an abandoned cart
func (s *AbandonedCartService) GetRecoveryAttempts(ctx context.Context, abandonedCartID uuid.UUID) ([]models.AbandonedCartRecoveryAttempt, error) {
	return s.repo.GetRecoveryAttempts(ctx, abandonedCartID)
}

// Delete deletes an abandoned cart
func (s *AbandonedCartService) Delete(ctx context.Context, tenantID string, cartID uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, cartID)
}
