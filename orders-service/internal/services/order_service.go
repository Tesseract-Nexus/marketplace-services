package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Tesseract-Nexus/go-shared/security"
	"orders-service/internal/clients"
	"orders-service/internal/events"
	"orders-service/internal/models"
	"orders-service/internal/repository"
)

// OrderService defines the business logic interface for orders
type OrderService interface {
	CreateOrder(req CreateOrderRequest, tenantID string) (*models.Order, error)
	GetOrder(id uuid.UUID, tenantID string) (*models.Order, error)
	GetOrderByNumber(orderNumber string, tenantID string) (*models.Order, error)
	ListOrders(filters OrderListFilters, tenantID string) (*OrderListResponse, error)
	UpdateOrder(id uuid.UUID, req UpdateOrderRequest, tenantID string) (*models.Order, error)
	UpdateOrderStatus(id uuid.UUID, status models.OrderStatus, notes string, tenantID string) (*models.Order, error)
	UpdatePaymentStatus(orderID uuid.UUID, paymentStatus models.PaymentStatus, transactionID string, tenantID string) (*models.Order, error)
	UpdateFulfillmentStatus(id uuid.UUID, status models.FulfillmentStatus, notes string, tenantID string) (*models.Order, error)
	CancelOrder(id uuid.UUID, reason string, tenantID string) (*models.Order, error)
	RefundOrder(id uuid.UUID, amount *float64, reason string, tenantID string) (*models.Order, error)
	GetOrderTracking(id uuid.UUID, tenantID string) (*OrderTrackingResponse, error)
	AddShippingTracking(id uuid.UUID, carrier string, trackingNumber string, trackingUrl string, tenantID string) (*models.Order, error)
	GetValidStatusTransitions(id uuid.UUID, tenantID string) (*ValidTransitionsResponse, error)
	SplitOrder(id uuid.UUID, req models.SplitOrderRequest, userID *uuid.UUID, tenantID string) (*SplitOrderResponse, error)
	GetChildOrders(parentOrderID uuid.UUID, tenantID string) ([]models.Order, error)
	BatchGetOrders(ids []uuid.UUID, tenantID string) ([]*models.Order, error)
}

// DTOs and Request/Response types
type CreateOrderRequest struct {
	CustomerID uuid.UUID                    `json:"customerId" binding:"required"`
	Currency   string                       `json:"currency"`
	Items      []CreateOrderItemRequest     `json:"items" binding:"required,min=1"`
	Customer   CreateOrderCustomerRequest   `json:"customer" binding:"required"`
	Shipping   CreateOrderShippingRequest   `json:"shipping" binding:"required"`
	Payment    CreateOrderPaymentRequest    `json:"payment" binding:"required"`
	Discounts  []CreateOrderDiscountRequest `json:"discounts"`
	Notes          string                       `json:"notes"`
	StorefrontHost string                       `json:"storefrontHost,omitempty"` // Set from X-Storefront-Host header
	IdempotencyKey string                       `json:"-"`                        // Set from X-Idempotency-Key header
}

type CreateOrderItemRequest struct {
	ProductID   uuid.UUID `json:"productId" binding:"required"`
	ProductName string    `json:"productName" binding:"required"`
	SKU         string    `json:"sku" binding:"required"`
	Image       string    `json:"image,omitempty"` // Product image URL
	Quantity    int       `json:"quantity" binding:"required,min=1"`
	UnitPrice   float64   `json:"unitPrice" binding:"required,min=0"`
}

type CreateOrderCustomerRequest struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Phone     string `json:"phone"`
}

type CreateOrderShippingRequest struct {
	Method             string  `json:"method" binding:"required"`
	Carrier            string  `json:"carrier"`
	CourierServiceCode string  `json:"courierServiceCode"` // Carrier-specific courier ID for auto-selection
	Cost               float64 `json:"cost" binding:"min=0"`
	Street             string  `json:"street" binding:"required"`
	City               string  `json:"city" binding:"required"`
	State              string  `json:"state" binding:"required"`
	PostalCode         string  `json:"postalCode" binding:"required"`
	Country            string  `json:"country" binding:"required"`
	// Package dimensions (captured at checkout for accurate shipping)
	PackageWeight float64 `json:"packageWeight"` // Weight in kg
	PackageLength float64 `json:"packageLength"` // Length in cm
	PackageWidth  float64 `json:"packageWidth"`  // Width in cm
	PackageHeight float64 `json:"packageHeight"` // Height in cm
	// Rate breakdown (for admin transparency)
	BaseRate      float64 `json:"baseRate"`      // Original carrier rate before markup
	MarkupAmount  float64 `json:"markupAmount"`  // Markup amount applied
	MarkupPercent float64 `json:"markupPercent"` // Markup percentage (e.g., 10 for 10%)
}

type CreateOrderPaymentRequest struct {
	Method        string  `json:"method" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,min=0"`
	Currency      string  `json:"currency"`
	TransactionID string  `json:"transactionId"`
}

type CreateOrderDiscountRequest struct {
	CouponID     *uuid.UUID `json:"couponId"`
	CouponCode   string     `json:"couponCode"`
	DiscountType string     `json:"discountType" binding:"required"`
	Amount       float64    `json:"amount" binding:"required,min=0"`
	Description  string     `json:"description"`
}

type UpdateOrderRequest struct {
	Notes              string     `json:"notes"`
	ReceiptNumber      string     `json:"receiptNumber,omitempty"`
	InvoiceNumber      string     `json:"invoiceNumber,omitempty"`
	ReceiptDocumentID  *uuid.UUID `json:"receiptDocumentId,omitempty"`
	ReceiptShortURL    string     `json:"receiptShortUrl,omitempty"`
	ReceiptGeneratedAt *time.Time `json:"receiptGeneratedAt,omitempty"`
}

type OrderListFilters struct {
	VendorID      string // Vendor ID for marketplace isolation (Tenant -> Vendor -> Staff)
	CustomerID    *uuid.UUID
	CustomerEmail *string
	Status        *models.OrderStatus
	DateFrom      *time.Time
	DateTo        *time.Time
	Page          int
	Limit         int
}

type OrderListResponse struct {
	Orders []models.Order `json:"orders"`
	Total  int64          `json:"total"`
	Page   int            `json:"page"`
	Limit  int            `json:"limit"`
}

type OrderTrackingResponse struct {
	OrderID           uuid.UUID                `json:"orderId"`
	OrderNumber       string                   `json:"orderNumber"`
	Status            models.OrderStatus       `json:"status"`
	PaymentStatus     models.PaymentStatus     `json:"paymentStatus"`
	FulfillmentStatus models.FulfillmentStatus `json:"fulfillmentStatus"`
	Shipping          *models.OrderShipping    `json:"shipping"`
	Timeline          []models.OrderTimeline   `json:"timeline"`
	CurrentStatus     string                   `json:"currentStatus"`
	EstimatedDelivery *time.Time               `json:"estimatedDelivery"`
}

type ValidTransitionsResponse struct {
	OrderID                  uuid.UUID                  `json:"orderId"`
	CurrentOrderStatus       models.OrderStatus         `json:"currentOrderStatus"`
	CurrentPaymentStatus     models.PaymentStatus       `json:"currentPaymentStatus"`
	CurrentFulfillmentStatus models.FulfillmentStatus   `json:"currentFulfillmentStatus"`
	ValidOrderStatuses       []models.OrderStatus       `json:"validOrderStatuses"`
	ValidPaymentStatuses     []models.PaymentStatus     `json:"validPaymentStatuses"`
	ValidFulfillmentStatuses []models.FulfillmentStatus `json:"validFulfillmentStatuses"`
}

type SplitOrderResponse struct {
	OriginalOrder *models.Order      `json:"originalOrder"`
	NewOrder      *models.Order      `json:"newOrder"`
	Split         *models.OrderSplit `json:"split"`
}

type orderService struct {
	orderRepo                    repository.OrderRepository
	returnRepo                   *repository.ReturnRepository
	cancellationSettingsService  CancellationSettingsService
	productsClient               clients.ProductsClient
	taxClient                    clients.TaxClient
	customersClient              clients.CustomersClient
	notificationClient           clients.NotificationClient
	tenantClient                 clients.TenantClient
	shippingClient               clients.ShippingClient
	eventsPublisher              *events.Publisher // Optional: for real-time admin notifications via NATS
	guestTokenService            *GuestTokenService
}

// NewOrderService creates a new order service
func NewOrderService(orderRepo repository.OrderRepository, returnRepo *repository.ReturnRepository, cancellationSettingsService CancellationSettingsService, productsClient clients.ProductsClient, taxClient clients.TaxClient, customersClient clients.CustomersClient, notificationClient clients.NotificationClient, tenantClient clients.TenantClient, shippingClient clients.ShippingClient, eventsPublisher *events.Publisher, guestTokenService *GuestTokenService) OrderService {
	return &orderService{
		orderRepo:                    orderRepo,
		returnRepo:                   returnRepo,
		cancellationSettingsService:  cancellationSettingsService,
		productsClient:               productsClient,
		taxClient:                    taxClient,
		customersClient:              customersClient,
		notificationClient:           notificationClient,
		tenantClient:                 tenantClient,
		shippingClient:               shippingClient,
		eventsPublisher:              eventsPublisher,
		guestTokenService:            guestTokenService,
	}
}

// CreateOrder creates a new order with all related entities
func (s *orderService) CreateOrder(req CreateOrderRequest, tenantID string) (*models.Order, error) {
	// Idempotency check: if a key is provided, check for an existing order
	if req.IdempotencyKey != "" {
		existing, err := s.orderRepo.FindByIdempotencyKey(tenantID, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency key: %w", err)
		}
		if existing != nil {
			return existing, nil
		}
	}

	// Step 1: Check stock availability for all items
	stockCheckItems := make([]clients.StockCheckItem, len(req.Items))
	for i, item := range req.Items {
		stockCheckItems[i] = clients.StockCheckItem{
			ProductID: item.ProductID.String(),
			Quantity:  item.Quantity,
		}
	}

	stockResponse, err := s.productsClient.CheckStock(stockCheckItems, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to check stock availability: %w", err)
	}

	if !stockResponse.AllInStock {
		// Find which products are out of stock
		var outOfStockProducts []string
		for _, result := range stockResponse.Results {
			if !result.Available {
				outOfStockProducts = append(outOfStockProducts, fmt.Sprintf("%s (requested: %d, available: %d)",
					result.ProductName, result.Requested, result.InStock))
			}
		}
		return nil, fmt.Errorf("insufficient stock for products: %v", outOfStockProducts)
	}

	// Step 2: Calculate subtotal
	subtotal := s.calculateSubtotal(req.Items)
	discountAmount := s.calculateDiscountAmount(req.Discounts)

	// Step 3: Calculate taxes via tax-service
	taxReq := s.buildTaxCalculationRequest(req, subtotal, tenantID)
	taxResp, err := s.taxClient.CalculateTax(taxReq, tenantID)

	// Initialize tax amounts
	var taxAmount float64
	var taxBreakdown models.JSONB
	var cgst, sgst, igst, utgst, gstCess, vatAmount float64
	var isInterstate, isReverseCharge bool

	if err != nil {
		// Log warning but don't fail - use fallback tax calculation
		fmt.Printf("WARNING: Tax service unavailable, using fallback calculation: %v\n", err)
		taxAmount = subtotal * 0.085 // Fallback 8.5% tax
	} else {
		// Use tax service response
		taxAmount = taxResp.TaxAmount

		// Store tax breakdown as JSON
		if len(taxResp.TaxBreakdown) > 0 {
			breakdownJSON, _ := json.Marshal(taxResp.TaxBreakdown)
			taxBreakdown = models.JSONB(breakdownJSON)
		}

		// Extract GST summary (India)
		if taxResp.GSTSummary != nil {
			cgst = taxResp.GSTSummary.CGST
			sgst = taxResp.GSTSummary.SGST
			igst = taxResp.GSTSummary.IGST
			utgst = taxResp.GSTSummary.UTGST
			gstCess = taxResp.GSTSummary.Cess
			isInterstate = taxResp.GSTSummary.IsInterstate
		}

		// Extract VAT summary (EU)
		if taxResp.VATSummary != nil {
			vatAmount = taxResp.VATSummary.VATAmount
			isReverseCharge = taxResp.VATSummary.IsReverseCharge
		}
	}

	total := subtotal + taxAmount + req.Shipping.Cost - discountAmount

	// Create order entity with tax data
	order := &models.Order{
		ID:                uuid.New(),
		TenantID:          tenantID,
		CustomerID:        req.CustomerID,
		Status:            models.OrderStatusPlaced,
		PaymentStatus:     models.PaymentStatusPending,
		FulfillmentStatus: models.FulfillmentStatusUnfulfilled,
		Currency:          s.getCurrency(req.Currency),
		Subtotal:          subtotal,
		TaxAmount:         taxAmount,
		TaxBreakdown:      taxBreakdown,
		ShippingCost:      req.Shipping.Cost,
		DiscountAmount:    discountAmount,
		Total:             total,
		Notes:             req.Notes,
		CGST:              cgst,
		SGST:              sgst,
		IGST:              igst,
		UTGST:             utgst,
		GSTCess:           gstCess,
		IsInterstate:      isInterstate,
		VATAmount:         vatAmount,
		IsReverseCharge:   isReverseCharge,
		StorefrontHost:    req.StorefrontHost,
	}

	// Set idempotency key if provided
	if req.IdempotencyKey != "" {
		order.IdempotencyKey = &req.IdempotencyKey
	}

	// Create order items
	for _, itemReq := range req.Items {
		item := models.OrderItem{
			ID:          uuid.New(),
			OrderID:     order.ID,
			ProductID:   itemReq.ProductID,
			ProductName: itemReq.ProductName,
			SKU:         itemReq.SKU,
			Image:       itemReq.Image,
			Quantity:    itemReq.Quantity,
			UnitPrice:   itemReq.UnitPrice,
			TotalPrice:  itemReq.UnitPrice * float64(itemReq.Quantity),
		}
		order.Items = append(order.Items, item)
	}

	// Create customer info
	order.Customer = &models.OrderCustomer{
		ID:        uuid.New(),
		OrderID:   order.ID,
		FirstName: req.Customer.FirstName,
		LastName:  req.Customer.LastName,
		Email:     req.Customer.Email,
		Phone:     req.Customer.Phone,
	}

	// Create or get customer in customers-service immediately
	// This ensures customer record exists for guest checkout and admin visibility
	if s.customersClient != nil && req.Customer.Email != "" {
		customer, err := s.customersClient.GetOrCreateCustomer(clients.CreateCustomerRequest{
			Email:     req.Customer.Email,
			FirstName: req.Customer.FirstName,
			LastName:  req.Customer.LastName,
			Phone:     req.Customer.Phone,
		}, tenantID)
		if err != nil {
			// Log but don't fail order creation - customer can be created later
			fmt.Printf("[OrderService] Failed to create customer at order creation: %v\n", err)
		} else if customer != nil {
			// Update order with real customer ID from customers-service
			customerUUID, parseErr := uuid.Parse(customer.ID)
			if parseErr == nil {
				order.CustomerID = customerUUID
				fmt.Printf("[OrderService] Customer %s created/found for order, email: %s\n", customer.ID, security.MaskEmail(req.Customer.Email))
			}
		}
	}

	// Create shipping info
	order.Shipping = &models.OrderShipping{
		ID:                 uuid.New(),
		OrderID:            order.ID,
		Method:             req.Shipping.Method,
		Carrier:            req.Shipping.Carrier,
		CourierServiceCode: req.Shipping.CourierServiceCode,
		Cost:               req.Shipping.Cost,
		Street:             req.Shipping.Street,
		City:               req.Shipping.City,
		State:              req.Shipping.State,
		PostalCode:         req.Shipping.PostalCode,
		Country:            req.Shipping.Country,
		// Store package dimensions captured at checkout
		PackageWeight:      req.Shipping.PackageWeight,
		PackageLength:      req.Shipping.PackageLength,
		PackageWidth:       req.Shipping.PackageWidth,
		PackageHeight:      req.Shipping.PackageHeight,
		// Store rate breakdown for admin transparency
		BaseRate:           req.Shipping.BaseRate,
		MarkupAmount:       req.Shipping.MarkupAmount,
		MarkupPercent:      req.Shipping.MarkupPercent,
	}

	// Create payment info
	order.Payment = &models.OrderPayment{
		ID:            uuid.New(),
		OrderID:       order.ID,
		Method:        req.Payment.Method,
		Status:        models.PaymentStatusPending,
		Amount:        req.Payment.Amount,
		Currency:      s.getCurrency(req.Payment.Currency),
		TransactionID: req.Payment.TransactionID,
	}

	// Create discounts
	for _, discountReq := range req.Discounts {
		discount := models.OrderDiscount{
			ID:           uuid.New(),
			OrderID:      order.ID,
			CouponID:     discountReq.CouponID,
			CouponCode:   discountReq.CouponCode,
			DiscountType: discountReq.DiscountType,
			Amount:       discountReq.Amount,
			Description:  discountReq.Description,
		}
		order.Discounts = append(order.Discounts, discount)
	}

	// Step 3: Save to database
	if err := s.orderRepo.Create(order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// Step 4: Deduct inventory with idempotency key to prevent duplicate deductions
	inventoryItems := make([]clients.InventoryItem, len(req.Items))
	for i, item := range req.Items {
		inventoryItems[i] = clients.InventoryItem{
			ProductID: item.ProductID.String(),
			Quantity:  item.Quantity,
		}
	}

	// Use idempotency-enabled method to prevent duplicate deductions on retries
	if err := s.productsClient.DeductInventoryWithIdempotency(
		inventoryItems,
		fmt.Sprintf("Order %s created", order.OrderNumber),
		order.ID.String(),
		tenantID,
	); err != nil {
		// Log the error but don't fail the order creation
		// The idempotency key ensures this can be safely retried
		fmt.Printf("WARNING: Failed to deduct inventory for order %s: %v\n", order.OrderNumber, err)
	}

	// NOTE: Order confirmation email is sent AFTER payment succeeds (in UpdatePaymentStatus)
	// At this point, order is PLACED but not yet CONFIRMED (payment pending)
	// This prevents sending confirmation emails for abandoned checkouts

	// Publish order.created event for real-time admin notifications
	if s.eventsPublisher != nil {
		s.eventsPublisher.PublishOrderCreated(context.Background(), order, tenantID)
	}

	return order, nil
}

// GetOrder retrieves an order by ID
func (s *orderService) GetOrder(id uuid.UUID, tenantID string) (*models.Order, error) {
	return s.orderRepo.GetByID(id, tenantID)
}

// GetOrderByNumber retrieves an order by order number
func (s *orderService) GetOrderByNumber(orderNumber string, tenantID string) (*models.Order, error) {
	return s.orderRepo.GetByOrderNumber(orderNumber, tenantID)
}

// BatchGetOrders retrieves multiple orders by IDs in a single query
// Performance: Single database query instead of N queries
func (s *orderService) BatchGetOrders(ids []uuid.UUID, tenantID string) ([]*models.Order, error) {
	return s.orderRepo.BatchGetByIDs(ids, tenantID)
}

// ListOrders retrieves orders with filtering and pagination
func (s *orderService) ListOrders(filters OrderListFilters, tenantID string) (*OrderListResponse, error) {
	repoFilters := repository.OrderFilters{
		TenantID:      tenantID,
		VendorID:      filters.VendorID, // Vendor isolation for marketplace mode
		CustomerID:    filters.CustomerID,
		CustomerEmail: filters.CustomerEmail,
		Status:        filters.Status,
		DateFrom:      filters.DateFrom,
		DateTo:        filters.DateTo,
		Page:          filters.Page,
		Limit:         filters.Limit,
	}

	orders, total, err := s.orderRepo.List(repoFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	return &OrderListResponse{
		Orders: orders,
		Total:  total,
		Page:   filters.Page,
		Limit:  filters.Limit,
	}, nil
}

// UpdateOrder updates an existing order
func (s *orderService) UpdateOrder(id uuid.UUID, req UpdateOrderRequest, tenantID string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Notes != "" {
		order.Notes = req.Notes
	}

	// Update receipt fields if provided
	if req.ReceiptNumber != "" {
		order.ReceiptNumber = req.ReceiptNumber
	}
	if req.InvoiceNumber != "" {
		order.InvoiceNumber = req.InvoiceNumber
	}
	if req.ReceiptDocumentID != nil {
		order.ReceiptDocumentID = req.ReceiptDocumentID
	}
	if req.ReceiptShortURL != "" {
		order.ReceiptShortURL = req.ReceiptShortURL
	}
	if req.ReceiptGeneratedAt != nil {
		order.ReceiptGeneratedAt = req.ReceiptGeneratedAt
	}

	if err := s.orderRepo.Update(order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	return order, nil
}

// UpdateOrderStatus updates the status of an order with state machine validation
func (s *orderService) UpdateOrderStatus(id uuid.UUID, status models.OrderStatus, notes string, tenantID string) (*models.Order, error) {
	// Get the order before updating to check previous status
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	previousStatus := order.Status

	// Validate the status transition using state machine
	if err := models.ValidateOrderStatusTransition(previousStatus, status); err != nil {
		return nil, fmt.Errorf("invalid status transition: %w", err)
	}

	if err := s.orderRepo.UpdateStatus(id, status, notes, tenantID); err != nil {
		return nil, fmt.Errorf("failed to update order status: %w", err)
	}

	// Get updated order
	updatedOrder, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated order: %w", err)
	}

	// Send notification emails based on new status (only if status actually changed)
	if s.notificationClient != nil && previousStatus != status && updatedOrder.Customer != nil && updatedOrder.Customer.Email != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := s.buildOrderNotification(ctx, updatedOrder, tenantID)

			switch status {
			case models.OrderStatusShipped:
				if err := s.notificationClient.SendOrderShipped(ctx, notification); err != nil {
					fmt.Printf("WARNING: Failed to send order shipped email: %v\n", err)
				}
			case models.OrderStatusDelivered, models.OrderStatusCompleted:
				if err := s.notificationClient.SendOrderDelivered(ctx, notification); err != nil {
					fmt.Printf("WARNING: Failed to send order delivered email: %v\n", err)
				}
			case models.OrderStatusConfirmed:
				if err := s.notificationClient.SendOrderConfirmation(ctx, notification); err != nil {
					fmt.Printf("WARNING: Failed to send order confirmation email: %v\n", err)
				}
			case models.OrderStatusCancelled:
				notification.CancellationReason = notes
				notification.CancelledDate = time.Now().Format("January 2, 2006")
				if err := s.notificationClient.SendOrderCancelled(ctx, notification); err != nil {
					fmt.Printf("WARNING: Failed to send order cancelled email: %v\n", err)
				}
			}
		}()
	}

	// Publish order status change events for real-time admin notifications
	if s.eventsPublisher != nil && previousStatus != status {
		switch status {
		case models.OrderStatusShipped:
			s.eventsPublisher.PublishOrderShipped(context.Background(), updatedOrder, tenantID)
		case models.OrderStatusDelivered, models.OrderStatusCompleted:
			s.eventsPublisher.PublishOrderDelivered(context.Background(), updatedOrder, tenantID)
		case models.OrderStatusCancelled:
			s.eventsPublisher.PublishOrderCancelled(context.Background(), updatedOrder, notes, tenantID)
		default:
			s.eventsPublisher.PublishOrderStatusChanged(context.Background(), updatedOrder, string(previousStatus), string(status), tenantID)
		}
	}

	return updatedOrder, nil
}

// CancelOrder cancels an order with full workflow automation
// This includes: cancellation fee calculation, return record creation, automatic refund processing,
// timeline event logging, and multi-tenant isolation
func (s *orderService) CancelOrder(id uuid.UUID, reason string, tenantID string) (*models.Order, error) {
	ctx := context.Background()

	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, err
	}

	// Get cancellation settings for the tenant
	var cancellationFee float64
	var canCancel bool
	var windowName string
	var settings *models.CancellationSettings

	if s.cancellationSettingsService != nil {
		settings, err = s.cancellationSettingsService.GetSettings(ctx, tenantID, "")
		if err != nil {
			fmt.Printf("WARNING: Failed to get cancellation settings for tenant %s: %v\n", tenantID, err)
		}

		if settings != nil {
			canCancel, cancellationFee, windowName = s.cancellationSettingsService.CanCancelOrder(ctx, order, settings)
			if !canCancel && windowName != "" {
				return nil, fmt.Errorf("order cannot be cancelled: %s", windowName)
			}
		}
	}

	// Validate the status transition using state machine
	if err := models.ValidateOrderStatusTransition(order.Status, models.OrderStatusCancelled); err != nil {
		return nil, fmt.Errorf("order cannot be cancelled: %w", err)
	}

	// Update status to cancelled with reason and fee information
	cancellationNotes := fmt.Sprintf("Cancelled: %s", reason)
	if cancellationFee > 0 {
		cancellationNotes = fmt.Sprintf("Cancelled: %s (Fee: $%.2f)", reason, cancellationFee)
	}
	if err := s.orderRepo.UpdateStatus(id, models.OrderStatusCancelled, cancellationNotes, tenantID); err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	// Get customer name for the timeline event (show who cancelled the order)
	cancelledBy := "customer"
	if order.Customer != nil {
		customerName := strings.TrimSpace(order.Customer.FirstName + " " + order.Customer.LastName)
		if customerName != "" {
			cancelledBy = customerName
		}
	}

	// Add timeline event for the cancellation with the customer's name
	s.orderRepo.AddTimelineEventByName(id, "ORDER_CANCELLED", cancellationNotes, cancelledBy, tenantID)

	// Restore inventory with idempotency key to prevent duplicate restorations
	inventoryItems := make([]clients.InventoryItem, len(order.Items))
	for i, item := range order.Items {
		inventoryItems[i] = clients.InventoryItem{
			ProductID: item.ProductID.String(),
			Quantity:  item.Quantity,
		}
	}

	// Use idempotency-enabled method to prevent duplicate restorations on retries
	if err := s.productsClient.RestoreInventoryWithIdempotency(
		inventoryItems,
		fmt.Sprintf("Order %s cancelled", order.OrderNumber),
		order.ID.String(),
		tenantID,
	); err != nil {
		// Log the error but don't fail the cancellation
		// The idempotency key ensures this can be safely retried
		fmt.Printf("WARNING: Failed to restore inventory for order %s: %v\n", order.OrderNumber, err)
	}

	// Get updated order
	updatedOrder, _ := s.orderRepo.GetByID(id, tenantID)

	// Create a Return record to track the refund process
	if s.returnRepo != nil && updatedOrder != nil {
		refundAmount := updatedOrder.Total - cancellationFee
		if refundAmount < 0 {
			refundAmount = 0
		}

		// Determine refund method from settings
		refundMethod := models.RefundMethodOriginal
		if settings != nil && settings.RefundMethod == "store_credit" {
			refundMethod = models.RefundMethodStoreCredit
		}

		// Map cancellation reason to return reason
		returnReason := models.ReturnReasonChangedMind
		switch reason {
		case "FOUND_BETTER_PRICE", "BETTER_PRICE":
			returnReason = models.ReturnReasonBetterPrice
		case "ORDERED_BY_MISTAKE", "MISTAKE":
			returnReason = models.ReturnReasonChangedMind
		case "SHIPPING_TOO_SLOW", "DELAYED":
			returnReason = models.ReturnReasonNoLongerNeeded
		case "PAYMENT_ISSUE":
			returnReason = models.ReturnReasonOther
		case "OTHER":
			returnReason = models.ReturnReasonOther
		}

		// Create return record for tracking refund
		returnRecord := &models.Return{
			TenantID:      tenantID,
			OrderID:       order.ID,
			CustomerID:    order.CustomerID,
			Status:        models.ReturnStatusPending,
			Reason:        returnReason,
			ReturnType:    models.ReturnTypeRefund,
			CustomerNotes: fmt.Sprintf("Order cancelled by customer. Reason: %s", reason),
			RefundAmount:  refundAmount,
			RefundMethod:  refundMethod,
			RestockingFee: cancellationFee,
		}

		// Create return items from order items
		for _, item := range order.Items {
			returnItem := models.ReturnItem{
				OrderItemID:  item.ID,
				ProductID:    item.ProductID,
				ProductName:  item.ProductName,
				SKU:          item.SKU,
				Quantity:     item.Quantity,
				UnitPrice:    item.UnitPrice,
				RefundAmount: float64(item.Quantity) * item.UnitPrice,
				Reason:       returnReason,
				ItemNotes:    "Order cancellation",
			}
			returnRecord.Items = append(returnRecord.Items, returnItem)
		}

		if err := s.returnRepo.CreateReturn(returnRecord); err != nil {
			fmt.Printf("WARNING: Failed to create return record for cancelled order %s: %v\n", order.OrderNumber, err)
		} else {
			fmt.Printf("Created return record %s for cancelled order %s (refund: $%.2f, fee: $%.2f)\n",
				returnRecord.RMANumber, order.OrderNumber, refundAmount, cancellationFee)

			// Auto-complete the return if auto-refund is enabled and order was paid
			if settings != nil && settings.AutoRefundEnabled && updatedOrder.PaymentStatus == models.PaymentStatusPaid {
				now := time.Now()
				returnRecord.Status = models.ReturnStatusCompleted
				returnRecord.RefundProcessedAt = &now
				returnRecord.AdminNotes = "Auto-approved and processed due to order cancellation with auto-refund enabled"

				if err := s.returnRepo.UpdateReturn(returnRecord); err != nil {
					fmt.Printf("WARNING: Failed to auto-complete return for order %s: %v\n", order.OrderNumber, err)
				} else {
					// Add timeline entry for auto-completion
					timeline := &models.ReturnTimeline{
						ReturnID:  returnRecord.ID,
						Status:    models.ReturnStatusCompleted,
						Message:   fmt.Sprintf("Return auto-approved and refund of $%.2f processed (cancellation fee: $%.2f)", refundAmount, cancellationFee),
						CreatedAt: now,
					}
					s.returnRepo.AddTimelineEntry(timeline)

					// Update order payment status to refunded
					if refundAmount > 0 {
						_, _ = s.RefundOrder(id, &refundAmount, "Order cancelled - automatic refund", tenantID)
					}
				}
			}
		}
	}

	// Send order cancelled email via notification-service
	if s.notificationClient != nil && updatedOrder != nil && updatedOrder.Customer != nil && updatedOrder.Customer.Email != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := s.buildOrderNotification(ctx, updatedOrder, tenantID)
			notification.CancellationReason = reason
			notification.CancelledDate = time.Now().Format("January 2, 2006")
			if err := s.notificationClient.SendOrderCancelled(ctx, notification); err != nil {
				fmt.Printf("WARNING: Failed to send order cancelled email: %v\n", err)
			}
		}()
	}

	// Publish order.cancelled event for real-time admin notifications
	if s.eventsPublisher != nil && updatedOrder != nil {
		s.eventsPublisher.PublishOrderCancelled(context.Background(), updatedOrder, reason, tenantID)
	}

	return updatedOrder, nil
}

// RefundOrder processes a refund for an order
func (s *orderService) RefundOrder(id uuid.UUID, amount *float64, reason string, tenantID string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, err
	}

	// Determine refund amount
	refundAmount := order.Total
	if amount != nil {
		refundAmount = *amount
	}

	// Validate refund amount
	if refundAmount <= 0 || refundAmount > order.Total {
		return nil, fmt.Errorf("invalid refund amount: %f", refundAmount)
	}

	// Update payment status to refunded (order status stays as-is, payment status tracks refund)
	refundPaymentStatus := models.PaymentStatusRefunded
	if refundAmount < order.Total {
		refundPaymentStatus = models.PaymentStatusPartiallyRefunded
	}
	if err := s.orderRepo.UpdatePaymentStatus(id, refundPaymentStatus, "", nil, tenantID); err != nil {
		return nil, fmt.Errorf("failed to refund order: %w", err)
	}

	// Send order refunded email via notification-service
	updatedOrder, _ := s.orderRepo.GetByID(id, tenantID)
	if s.notificationClient != nil && updatedOrder != nil && updatedOrder.Customer != nil && updatedOrder.Customer.Email != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := s.buildOrderNotification(ctx, updatedOrder, tenantID)
			notification.RefundAmount = fmt.Sprintf("%.2f", refundAmount)
			notification.RefundDays = "5-7 business days"
			if err := s.notificationClient.SendOrderRefunded(ctx, notification); err != nil {
				fmt.Printf("WARNING: Failed to send order refunded email: %v\n", err)
			}
		}()
	}

	// Publish order.refunded event for real-time admin notifications
	if s.eventsPublisher != nil && updatedOrder != nil {
		s.eventsPublisher.PublishOrderRefunded(context.Background(), updatedOrder, refundAmount, reason, tenantID)
	}

	return updatedOrder, nil
}

// UpdatePaymentStatus updates the payment status of an order with state machine validation
func (s *orderService) UpdatePaymentStatus(orderID uuid.UUID, paymentStatus models.PaymentStatus, transactionID string, tenantID string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(orderID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// Validate the payment status transition using state machine
	if err := models.ValidatePaymentStatusTransition(order.PaymentStatus, paymentStatus); err != nil {
		return nil, fmt.Errorf("invalid payment status transition: %w", err)
	}

	// Update payment status
	now := time.Now()
	if err := s.orderRepo.UpdatePaymentStatus(orderID, paymentStatus, transactionID, &now, tenantID); err != nil {
		return nil, fmt.Errorf("failed to update payment status: %w", err)
	}

	// If payment completed (PAID), update order status to confirmed and record customer order
	if paymentStatus == models.PaymentStatusPaid && order.Status == models.OrderStatusPlaced {
		if err := s.orderRepo.UpdateStatus(orderID, models.OrderStatusConfirmed, "Payment completed", tenantID); err != nil {
			// Log but don't fail
			fmt.Printf("WARNING: Failed to update order status to confirmed: %v\n", err)
		}

		// Record order in customers-service to update customer statistics
		s.recordCustomerOrder(order, tenantID)

		// Send order confirmation email via notification-service
		updatedOrder, _ := s.orderRepo.GetByID(orderID, tenantID)
		if s.notificationClient != nil && updatedOrder != nil && updatedOrder.Customer != nil && updatedOrder.Customer.Email != "" {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				notification := s.buildOrderNotification(ctx, updatedOrder, tenantID)

				if err := s.notificationClient.SendOrderConfirmation(ctx, notification); err != nil {
					fmt.Printf("WARNING: Failed to send order confirmation email: %v\n", err)
				}
			}()
		}

		// Publish order.confirmed and payment.captured events for real-time admin notifications
		if s.eventsPublisher != nil && updatedOrder != nil {
			s.eventsPublisher.PublishOrderConfirmed(context.Background(), updatedOrder, tenantID)
			s.eventsPublisher.PublishPaymentReceived(context.Background(), updatedOrder, transactionID, tenantID)
		}

		// Auto-create shipment using customer's selected carrier from checkout
		if s.shippingClient != nil && updatedOrder != nil && updatedOrder.Shipping != nil {
			go s.autoCreateShipment(updatedOrder, tenantID)
		}
	}

	return s.orderRepo.GetByID(orderID, tenantID)
}

// recordCustomerOrder notifies customers-service about the completed order
// This is called after payment confirmation to update customer statistics
func (s *orderService) recordCustomerOrder(order *models.Order, tenantID string) {
	if s.customersClient == nil {
		fmt.Printf("[OrderService] Customers client not configured, skipping customer order recording\n")
		return
	}

	var customerID string
	var retryCount int
	const maxRetries = 3

	// If order already has a valid customer ID (set during order creation), use it
	if order.CustomerID != uuid.Nil {
		customerID = order.CustomerID.String()
		fmt.Printf("[OrderService] Using existing customer ID %s for order %s\n", customerID, order.OrderNumber)
	} else if order.Customer != nil && order.Customer.Email != "" {
		// Customer wasn't created at order time (maybe service was down)
		// Try to find or create customer by email with retry logic
		for retryCount < maxRetries {
			customer, err := s.customersClient.GetOrCreateCustomer(clients.CreateCustomerRequest{
				Email:     order.Customer.Email,
				FirstName: order.Customer.FirstName,
				LastName:  order.Customer.LastName,
				Phone:     order.Customer.Phone,
			}, tenantID)
			if err != nil {
				retryCount++
				fmt.Printf("[OrderService] Failed to get/create customer (attempt %d/%d): %v\n", retryCount, maxRetries, err)
				if retryCount < maxRetries {
					time.Sleep(time.Duration(retryCount) * time.Second) // Exponential backoff
					continue
				}
				return
			}
			customerID = customer.ID
			fmt.Printf("[OrderService] Customer %s created/found on payment confirmation for order %s\n", customerID, order.OrderNumber)

			// Update order with customer ID for future reference
			if customerUUID, parseErr := uuid.Parse(customerID); parseErr == nil {
				if updateErr := s.orderRepo.UpdateCustomerID(order.ID, customerUUID, tenantID); updateErr != nil {
					fmt.Printf("[OrderService] Failed to update order with customer ID: %v\n", updateErr)
				}
			}
			break
		}
	} else {
		fmt.Printf("[OrderService] No customer info available for order %s, skipping\n", order.ID)
		return
	}

	if customerID == "" {
		fmt.Printf("[OrderService] Could not determine customer ID for order %s, skipping order recording\n", order.OrderNumber)
		return
	}

	// Record the order with retry logic
	for retryCount = 0; retryCount < maxRetries; retryCount++ {
		err := s.customersClient.RecordOrder(customerID, clients.RecordOrderRequest{
			OrderID:     order.ID.String(),
			OrderNumber: order.OrderNumber,
			TotalAmount: order.Total,
		}, tenantID)

		if err != nil {
			fmt.Printf("[OrderService] Failed to record customer order (attempt %d/%d): %v\n", retryCount+1, maxRetries, err)
			if retryCount < maxRetries-1 {
				time.Sleep(time.Duration(retryCount+1) * time.Second)
				continue
			}
		} else {
			fmt.Printf("[OrderService] Successfully recorded order %s for customer %s\n", order.OrderNumber, customerID)
			break
		}
	}
}

// autoCreateShipment creates a shipment automatically after payment is confirmed
// Uses the carrier and shipping cost selected by customer at checkout
func (s *orderService) autoCreateShipment(order *models.Order, tenantID string) {
	if order.Shipping == nil {
		fmt.Printf("[OrderService] No shipping info for order %s, skipping auto-shipment\n", order.OrderNumber)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch warehouse address from shipping settings
	var fromAddress *clients.ShipmentAddress
	settings, err := s.shippingClient.GetShippingSettings(ctx, tenantID)
	if err != nil {
		fmt.Printf("[OrderService] Warning: Failed to fetch shipping settings for order %s: %v\n", order.OrderNumber, err)
		// Continue with nil - shipping service may have defaults configured
	} else if settings != nil && settings.Warehouse != nil {
		fromAddress = settings.Warehouse
		fmt.Printf("[OrderService] Using warehouse address: %s, %s for order %s\n",
			fromAddress.City, fromAddress.Country, order.OrderNumber)
	}

	// If no warehouse configured, skip auto-shipment
	if fromAddress == nil {
		fmt.Printf("[OrderService] No warehouse address configured for tenant, skipping auto-shipment for order %s\n", order.OrderNumber)
		return
	}

	// Build order items for shipment
	var shipmentItems []clients.ShipmentItem
	for _, item := range order.Items {
		shipmentItems = append(shipmentItems, clients.ShipmentItem{
			Name:     item.ProductName,
			SKU:      item.SKU,
			Quantity: item.Quantity,
			Price:    item.UnitPrice,
		})
	}

	weight, length, width, height := s.calculateShipmentMetrics(order, tenantID)

	// Build shipment request using order's shipping info
	req := &clients.CreateShipmentRequest{
		TenantID:    tenantID,
		OrderID:     order.ID.String(),
		OrderNumber: order.OrderNumber,
		FromAddress: fromAddress,
		ToAddress: &clients.ShipmentAddress{
			Name:       fmt.Sprintf("%s %s", order.Customer.FirstName, order.Customer.LastName),
			Phone:      order.Customer.Phone,
			Email:      order.Customer.Email,
			Street:     order.Shipping.Street,
			City:       order.Shipping.City,
			State:      order.Shipping.State,
			PostalCode: order.Shipping.PostalCode,
			Country:    order.Shipping.Country,
		},
		Weight:             weight,
		Length:             length,
		Width:              width,
		Height:             height,
		Carrier:            order.Shipping.Carrier,
		CourierServiceCode: order.Shipping.CourierServiceCode, // Customer's selected courier for auto-assignment
		ShippingCost:       order.ShippingCost,
		Items:              shipmentItems,
		OrderValue:         order.Subtotal,
	}

	shipment, err := s.shippingClient.CreateShipment(ctx, req)
	if err != nil {
		fmt.Printf("[OrderService] Auto-shipment creation failed for order %s: %v\n", order.OrderNumber, err)
		return
	}

	fmt.Printf("[OrderService] Auto-shipment created for order %s: shipment=%s, carrier=%s\n",
		order.OrderNumber, shipment.ID, shipment.Carrier)
}

func (s *orderService) calculateShipmentMetrics(order *models.Order, tenantID string) (float64, float64, float64, float64) {
	const (
		defaultWeight = 0.5
		defaultLength = 20
		defaultWidth  = 15
		defaultHeight = 10
	)

	// First, check if the order has stored package metrics from checkout
	// These are more reliable as they were calculated when customer selected shipping
	if order.Shipping != nil {
		hasStoredMetrics := order.Shipping.PackageWeight > 0 ||
			order.Shipping.PackageLength > 0 ||
			order.Shipping.PackageWidth > 0 ||
			order.Shipping.PackageHeight > 0

		if hasStoredMetrics {
			weight := order.Shipping.PackageWeight
			length := order.Shipping.PackageLength
			width := order.Shipping.PackageWidth
			height := order.Shipping.PackageHeight

			// Apply defaults only where needed
			if weight <= 0 {
				weight = defaultWeight
			}
			if length <= 0 {
				length = defaultLength
			}
			if width <= 0 {
				width = defaultWidth
			}
			if height <= 0 {
				height = defaultHeight
			}

			fmt.Printf("[OrderService] Using stored package metrics for order %s: %.2fkg, %dx%dx%d cm\n",
				order.OrderNumber, weight, int(length), int(width), int(height))
			return weight, length, width, height
		}
	}

	// Fallback: calculate from product data if no stored metrics
	var totalWeightKg float64
	var maxLengthCm float64
	var maxWidthCm float64
	var maxHeightCm float64

	for _, item := range order.Items {
		product, err := s.productsClient.GetProduct(item.ProductID.String(), tenantID)
		if err != nil {
			fmt.Printf("[OrderService] Warning: failed to fetch product %s for shipping metrics: %v\n", item.ProductID.String(), err)
			continue
		}

		if product.Weight != nil {
			if weightValue, weightUnit, ok := parseValueAndUnit(*product.Weight); ok {
				weightKg := normalizeWeightToKg(weightValue, weightUnit)
				if weightKg > 0 {
					totalWeightKg += weightKg * float64(item.Quantity)
				}
			}
		}

		if product.Dimensions != nil {
			unit := strings.TrimSpace(product.Dimensions.Unit)
			if lengthValue, ok := parseFloatValue(product.Dimensions.Length); ok {
				lengthCm := normalizeLengthToCm(lengthValue, unit)
				if lengthCm > maxLengthCm {
					maxLengthCm = lengthCm
				}
			}
			if widthValue, ok := parseFloatValue(product.Dimensions.Width); ok {
				widthCm := normalizeLengthToCm(widthValue, unit)
				if widthCm > maxWidthCm {
					maxWidthCm = widthCm
				}
			}
			if heightValue, ok := parseFloatValue(product.Dimensions.Height); ok {
				heightCm := normalizeLengthToCm(heightValue, unit)
				if heightCm > maxHeightCm {
					maxHeightCm = heightCm
				}
			}
		}
	}

	weight := totalWeightKg
	if weight <= 0 {
		weight = defaultWeight
	}
	length := maxLengthCm
	width := maxWidthCm
	height := maxHeightCm
	if length <= 0 {
		length = defaultLength
	}
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}

	return weight, length, width, height
}

func parseFloatValue(value string) (float64, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		if parsedValue, _, ok := parseValueAndUnit(trimmed); ok {
			return parsedValue, true
		}
		return 0, false
	}
	return parsed, true
}

func parseValueAndUnit(value string) (float64, string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, "", false
	}

	parts := strings.Fields(trimmed)
	if len(parts) >= 1 {
		if parsed, err := strconv.ParseFloat(parts[0], 64); err == nil {
			unit := ""
			if len(parts) > 1 {
				unit = strings.TrimSpace(parts[1])
			}
			return parsed, unit, true
		}
	}

	numericEnd := 0
	for numericEnd < len(trimmed) {
		ch := trimmed[numericEnd]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			numericEnd++
			continue
		}
		break
	}
	if numericEnd == 0 {
		return 0, "", false
	}

	parsed, err := strconv.ParseFloat(trimmed[:numericEnd], 64)
	if err != nil {
		return 0, "", false
	}

	unit := strings.TrimSpace(trimmed[numericEnd:])
	return parsed, unit, true
}

func normalizeWeightToKg(value float64, unit string) float64 {
	normalizedUnit := strings.ToLower(strings.TrimSpace(unit))
	switch normalizedUnit {
	case "g":
		return value / 1000
	case "lb", "lbs", "pound", "pounds":
		return value * 0.45359237
	case "oz", "ounce", "ounces":
		return value * 0.0283495231
	default:
		return value
	}
}

func normalizeLengthToCm(value float64, unit string) float64 {
	normalizedUnit := strings.ToLower(strings.TrimSpace(unit))
	switch normalizedUnit {
	case "m", "meter", "meters":
		return value * 100
	case "in", "inch", "inches":
		return value * 2.54
	default:
		return value
	}
}

// GetOrderTracking retrieves tracking information for an order
func (s *orderService) GetOrderTracking(id uuid.UUID, tenantID string) (*OrderTrackingResponse, error) {
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, err
	}

	timeline, err := s.orderRepo.GetTimelineByOrderID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order timeline: %w", err)
	}

	var estimatedDelivery *time.Time
	if order.Shipping != nil {
		estimatedDelivery = order.Shipping.EstimatedDelivery
	}

	return &OrderTrackingResponse{
		OrderID:           order.ID,
		OrderNumber:       order.OrderNumber,
		Status:            order.Status,
		Shipping:          order.Shipping,
		Timeline:          timeline,
		CurrentStatus:     string(order.Status),
		EstimatedDelivery: estimatedDelivery,
	}, nil
}

// AddShippingTracking adds tracking information to an order's shipping details
func (s *orderService) AddShippingTracking(id uuid.UUID, carrier string, trackingNumber string, trackingUrl string, tenantID string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, err
	}

	// Validate that order has shipping info
	if order.Shipping == nil {
		return nil, fmt.Errorf("order has no shipping information")
	}

	// Validate order status (should be confirmed or processing to add tracking)
	if order.Status != models.OrderStatusConfirmed && order.Status != models.OrderStatusProcessing {
		return nil, fmt.Errorf("cannot add tracking to order with status: %s", order.Status)
	}

	// Update shipping with tracking information
	if err := s.orderRepo.UpdateShippingTracking(id, carrier, trackingNumber, trackingUrl, tenantID); err != nil {
		return nil, fmt.Errorf("failed to add shipping tracking: %w", err)
	}

	// Update fulfillment status to DISPATCHED when tracking is added
	statusDescription := fmt.Sprintf("Shipping tracking added: %s - %s", carrier, trackingNumber)
	if err := s.orderRepo.UpdateFulfillmentStatus(id, models.FulfillmentStatusDispatched, statusDescription, tenantID); err != nil {
		// Log but don't fail - tracking was added successfully
		fmt.Printf("WARNING: Failed to update fulfillment status to dispatched: %v\n", err)
	}

	// Send order shipped email via notification-service
	updatedOrder, _ := s.orderRepo.GetByID(id, tenantID)
	if s.notificationClient != nil && updatedOrder != nil && updatedOrder.Customer != nil && updatedOrder.Customer.Email != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := s.buildOrderNotification(ctx, updatedOrder, tenantID)
			notification.Carrier = carrier
			notification.TrackingNumber = trackingNumber
			notification.TrackingURL = trackingUrl
			if err := s.notificationClient.SendOrderShipped(ctx, notification); err != nil {
				fmt.Printf("WARNING: Failed to send order shipped email: %v\n", err)
			}
		}()
	}

	// Publish order.shipped event for real-time admin notifications
	if s.eventsPublisher != nil && updatedOrder != nil {
		s.eventsPublisher.PublishOrderShipped(context.Background(), updatedOrder, tenantID)
	}

	// Return updated order
	return updatedOrder, nil
}

// Helper methods
func (s *orderService) calculateSubtotal(items []CreateOrderItemRequest) float64 {
	var subtotal float64
	for _, item := range items {
		subtotal += item.UnitPrice * float64(item.Quantity)
	}
	return subtotal
}

// buildTaxCalculationRequest constructs a tax calculation request from order data
func (s *orderService) buildTaxCalculationRequest(req CreateOrderRequest, subtotal float64, tenantID string) *clients.TaxCalculationRequest {
	// Build line items for tax calculation
	lineItems := make([]clients.LineItemInput, len(req.Items))
	for i, item := range req.Items {
		itemSubtotal := item.UnitPrice * float64(item.Quantity)
		lineItems[i] = clients.LineItemInput{
			ProductID: item.ProductID.String(),
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			Subtotal:  itemSubtotal,
			// HSNCode and SACCode would come from product metadata
			// These can be passed in via CreateOrderItemRequest if available
		}
	}

	// Build shipping address
	shippingAddress := clients.AddressInput{
		City:        req.Shipping.City,
		State:       req.Shipping.State,
		Zip:         req.Shipping.PostalCode,
		Country:     req.Shipping.Country,
		CountryCode: s.getCountryCode(req.Shipping.Country),
		StateCode:   s.getStateCode(req.Shipping.State, req.Shipping.Country),
	}

	return &clients.TaxCalculationRequest{
		TenantID:        tenantID,
		ShippingAddress: shippingAddress,
		LineItems:       lineItems,
		ShippingAmount:  req.Shipping.Cost,
	}
}

// getCountryCode returns the ISO 3166-1 alpha-2 country code
func (s *orderService) getCountryCode(country string) string {
	// Common country mappings
	countryCodeMap := map[string]string{
		"India":          "IN",
		"United States":  "US",
		"USA":            "US",
		"United Kingdom": "GB",
		"UK":             "GB",
		"Canada":         "CA",
		"Australia":      "AU",
		"Germany":        "DE",
		"France":         "FR",
		"Singapore":      "SG",
		"New Zealand":    "NZ",
	}

	if code, ok := countryCodeMap[country]; ok {
		return code
	}
	// If already a 2-letter code, return as-is
	if len(country) == 2 {
		return country
	}
	return country
}

// getStateCode returns the state code for tax determination
func (s *orderService) getStateCode(state, country string) string {
	// For US states
	usStateCodeMap := map[string]string{
		"California": "CA", "Texas": "TX", "New York": "NY", "Florida": "FL",
		"Illinois": "IL", "Pennsylvania": "PA", "Ohio": "OH", "Georgia": "GA",
		"North Carolina": "NC", "Michigan": "MI", "New Jersey": "NJ", "Virginia": "VA",
		"Washington": "WA", "Arizona": "AZ", "Massachusetts": "MA", "Tennessee": "TN",
		"Indiana": "IN", "Missouri": "MO", "Maryland": "MD", "Wisconsin": "WI",
		"Colorado": "CO", "Minnesota": "MN", "South Carolina": "SC", "Alabama": "AL",
		"Louisiana": "LA", "Kentucky": "KY", "Oregon": "OR", "Oklahoma": "OK",
		"Connecticut": "CT", "Utah": "UT", "Iowa": "IA", "Nevada": "NV",
		"Arkansas": "AR", "Mississippi": "MS", "Kansas": "KS", "New Mexico": "NM",
		"Nebraska": "NE", "Idaho": "ID", "West Virginia": "WV", "Hawaii": "HI",
		"New Hampshire": "NH", "Maine": "ME", "Montana": "MT", "Rhode Island": "RI",
		"Delaware": "DE", "South Dakota": "SD", "North Dakota": "ND", "Alaska": "AK",
		"Vermont": "VT", "Wyoming": "WY",
	}

	// For Indian states (GST state codes)
	indiaStateCodeMap := map[string]string{
		"Maharashtra": "MH", "Karnataka": "KA", "Tamil Nadu": "TN", "Telangana": "TS",
		"Gujarat": "GJ", "West Bengal": "WB", "Rajasthan": "RJ", "Uttar Pradesh": "UP",
		"Andhra Pradesh": "AP", "Madhya Pradesh": "MP", "Kerala": "KL", "Bihar": "BR",
		"Punjab": "PB", "Haryana": "HR", "Delhi": "DL", "Odisha": "OD",
		"Jharkhand": "JH", "Chhattisgarh": "CG", "Assam": "AS", "Uttarakhand": "UK",
		"Goa": "GA", "Himachal Pradesh": "HP", "Jammu and Kashmir": "JK",
	}

	// Check US states
	if code, ok := usStateCodeMap[state]; ok {
		return code
	}

	// Check Indian states
	if code, ok := indiaStateCodeMap[state]; ok {
		return code
	}

	// If already a 2-letter code, return as-is
	if len(state) == 2 {
		return state
	}

	return state
}

func (s *orderService) calculateDiscountAmount(discounts []CreateOrderDiscountRequest) float64 {
	var total float64
	for _, discount := range discounts {
		total += discount.Amount
	}
	return total
}

func (s *orderService) getCurrency(currency string) string {
	if currency == "" {
		return "USD"
	}
	return currency
}

// UpdateFulfillmentStatus updates the fulfillment status of an order with state machine validation
func (s *orderService) UpdateFulfillmentStatus(id uuid.UUID, status models.FulfillmentStatus, notes string, tenantID string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Validate the fulfillment status transition using state machine
	if err := models.ValidateFulfillmentStatusTransition(order.FulfillmentStatus, status); err != nil {
		return nil, fmt.Errorf("invalid fulfillment status transition: %w", err)
	}

	// Check that order is in a valid state for fulfillment updates
	if order.Status == models.OrderStatusCancelled {
		return nil, fmt.Errorf("cannot update fulfillment status for cancelled order")
	}
	if order.PaymentStatus != models.PaymentStatusPaid && order.PaymentStatus != models.PaymentStatusPartiallyRefunded {
		return nil, fmt.Errorf("cannot update fulfillment status before payment is confirmed")
	}

	// Update fulfillment status
	if err := s.orderRepo.UpdateFulfillmentStatus(id, status, notes, tenantID); err != nil {
		return nil, fmt.Errorf("failed to update fulfillment status: %w", err)
	}

	// Auto-update order status based on fulfillment status
	updatedOrder, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated order: %w", err)
	}

	// If fulfillment started, move order to PROCESSING
	if status == models.FulfillmentStatusProcessing && updatedOrder.Status == models.OrderStatusConfirmed {
		if err := s.orderRepo.UpdateStatus(id, models.OrderStatusProcessing, "Fulfillment started", tenantID); err != nil {
			fmt.Printf("WARNING: Failed to update order status to processing: %v\n", err)
		}
	}

	// If delivered, move order to COMPLETED
	if status == models.FulfillmentStatusDelivered && updatedOrder.Status == models.OrderStatusProcessing {
		if err := s.orderRepo.UpdateStatus(id, models.OrderStatusCompleted, "Order delivered", tenantID); err != nil {
			fmt.Printf("WARNING: Failed to update order status to completed: %v\n", err)
		}

		// Send order delivered email
		finalOrder, _ := s.orderRepo.GetByID(id, tenantID)
		if s.notificationClient != nil && finalOrder != nil && finalOrder.Customer != nil && finalOrder.Customer.Email != "" {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				notification := s.buildOrderNotification(ctx, finalOrder, tenantID)
				notification.DeliveryDate = time.Now().Format("January 2, 2006")
				if err := s.notificationClient.SendOrderDelivered(ctx, notification); err != nil {
					fmt.Printf("WARNING: Failed to send order delivered email: %v\n", err)
				}
			}()
		}
	}

	// Send order shipped email when dispatched
	if status == models.FulfillmentStatusDispatched && s.notificationClient != nil && updatedOrder.Customer != nil && updatedOrder.Customer.Email != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := s.buildOrderNotification(ctx, updatedOrder, tenantID)
			if err := s.notificationClient.SendOrderShipped(ctx, notification); err != nil {
				fmt.Printf("WARNING: Failed to send order shipped email: %v\n", err)
			}
		}()
	}

	return s.orderRepo.GetByID(id, tenantID)
}

// GetValidStatusTransitions returns the valid next status options for an order
func (s *orderService) GetValidStatusTransitions(id uuid.UUID, tenantID string) (*ValidTransitionsResponse, error) {
	order, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &ValidTransitionsResponse{
		OrderID:                  order.ID,
		CurrentOrderStatus:       order.Status,
		CurrentPaymentStatus:     order.PaymentStatus,
		CurrentFulfillmentStatus: order.FulfillmentStatus,
		ValidOrderStatuses:       models.GetNextValidOrderStatuses(order.Status),
		ValidPaymentStatuses:     models.GetNextValidPaymentStatuses(order.PaymentStatus),
		ValidFulfillmentStatuses: models.GetNextValidFulfillmentStatuses(order.FulfillmentStatus),
	}, nil
}

// buildOrderNotification builds an OrderNotification from an order model
func (s *orderService) buildOrderNotification(ctx context.Context, order *models.Order, tenantID string) *clients.OrderNotification {
	notification := &clients.OrderNotification{
		TenantID:    tenantID,
		OrderID:     order.ID.String(),
		OrderNumber: order.OrderNumber,
		OrderDate:   order.CreatedAt.Format("January 2, 2006"),
		OrderStatus: string(order.Status),
		Currency:    order.Currency,
		Subtotal:    fmt.Sprintf("%.2f", order.Subtotal),
		Discount:    fmt.Sprintf("%.2f", order.DiscountAmount),
		Shipping:    fmt.Sprintf("%.2f", order.ShippingCost),
		Tax:         fmt.Sprintf("%.2f", order.TaxAmount),
		Total:       fmt.Sprintf("%.2f", order.Total),
	}

	// Set customer details
	if order.Customer != nil {
		notification.CustomerEmail = order.Customer.Email
		notification.CustomerName = fmt.Sprintf("%s %s", order.Customer.FirstName, order.Customer.LastName)
	}

	// Build URLs using storefront host (supports custom domains) or fallback to tenant client
	storefrontBase := ""
	if order.StorefrontHost != "" {
		storefrontBase = "https://" + order.StorefrontHost
	}

	if storefrontBase != "" {
		// Use the actual storefront host (custom domain or default subdomain)
		notification.OrderDetailsURL = storefrontBase + "/account/orders/" + order.ID.String()
		notification.TrackingURL = storefrontBase + "/account/orders/" + order.ID.String() + "/track"
		notification.ReviewURL = storefrontBase + "/account/orders/" + order.ID.String() + "/review"
		notification.ShopURL = storefrontBase
	} else if s.tenantClient != nil {
		// Fallback to tenant slug-based URL building
		notification.OrderDetailsURL = s.tenantClient.BuildOrderURL(ctx, tenantID, order.ID.String())
		notification.TrackingURL = s.tenantClient.BuildOrderTrackingURL(ctx, tenantID, order.ID.String())
		notification.ReviewURL = s.tenantClient.BuildReviewURL(ctx, tenantID, order.ID.String())
		notification.ShopURL = s.tenantClient.BuildShopURL(ctx, tenantID)
	}

	// Override OrderDetailsURL with guest order URL so all emails
	// (confirmation, shipped, delivered, cancelled, refunded) contain
	// a token-based link that works without authentication.
	if s.guestTokenService != nil && order.Customer != nil && order.Customer.Email != "" {
		token := s.guestTokenService.GenerateToken(
			order.ID.String(), order.OrderNumber, order.Customer.Email)
		if storefrontBase != "" {
			notification.OrderDetailsURL = fmt.Sprintf("%s/orders/guest?token=%s&order=%s&email=%s",
				storefrontBase, url.QueryEscape(token), url.QueryEscape(order.OrderNumber), url.QueryEscape(order.Customer.Email))
		} else if s.tenantClient != nil {
			notification.OrderDetailsURL = s.tenantClient.BuildGuestOrderURL(
				ctx, tenantID, order.OrderNumber, token, order.Customer.Email)
		}
		// Guest cancel URL is the same page (it has cancel functionality)
		notification.GuestCancelURL = notification.OrderDetailsURL
	}

	// Build order items
	for _, item := range order.Items {
		notification.Items = append(notification.Items, clients.OrderItem{
			Name:     item.ProductName,
			SKU:      item.SKU,
			Quantity: item.Quantity,
			Price:    fmt.Sprintf("%.2f", item.UnitPrice),
			Currency: order.Currency,
		})
	}

	// Set shipping address
	if order.Shipping != nil {
		notification.ShippingAddress = &clients.Address{
			Name:       notification.CustomerName,
			Line1:      order.Shipping.Street,
			City:       order.Shipping.City,
			State:      order.Shipping.State,
			PostalCode: order.Shipping.PostalCode,
			Country:    order.Shipping.Country,
		}

		// Set carrier and tracking if available
		notification.Carrier = order.Shipping.Carrier
		notification.TrackingNumber = order.Shipping.TrackingNumber
		if order.Shipping.EstimatedDelivery != nil {
			notification.EstimatedDelivery = order.Shipping.EstimatedDelivery.Format("January 2, 2006")
		}
	}

	// Set payment method
	if order.Payment != nil {
		notification.PaymentMethod = order.Payment.Method
	}

	return notification
}

// SplitOrder splits an order into two orders
func (s *orderService) SplitOrder(id uuid.UUID, req models.SplitOrderRequest, userID *uuid.UUID, tenantID string) (*SplitOrderResponse, error) {
	// Get the original order
	originalOrder, err := s.orderRepo.GetByID(id, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get original order: %w", err)
	}

	// Validate order can be split
	if originalOrder.Status == models.OrderStatusCancelled {
		return nil, fmt.Errorf("cannot split a cancelled order")
	}
	if originalOrder.Status == models.OrderStatusCompleted || originalOrder.Status == models.OrderStatusDelivered {
		return nil, fmt.Errorf("cannot split a completed/delivered order")
	}

	// Validate items
	if len(req.ItemIDs) == 0 {
		return nil, fmt.Errorf("at least one item must be selected for splitting")
	}
	if len(req.ItemIDs) >= len(originalOrder.Items) {
		return nil, fmt.Errorf("cannot split all items - at least one item must remain in the original order")
	}

	// Find the items to move
	var itemsToMove []models.OrderItem
	var itemsToKeep []models.OrderItem
	itemIDMap := make(map[uuid.UUID]bool)
	for _, itemID := range req.ItemIDs {
		itemIDMap[itemID] = true
	}

	for _, item := range originalOrder.Items {
		if itemIDMap[item.ID] {
			itemsToMove = append(itemsToMove, item)
		} else {
			itemsToKeep = append(itemsToKeep, item)
		}
	}

	if len(itemsToMove) == 0 {
		return nil, fmt.Errorf("none of the specified items were found in the order")
	}

	// Calculate totals for the new order
	var newSubtotal, newTaxAmount float64
	for _, item := range itemsToMove {
		newSubtotal += item.TotalPrice
		newTaxAmount += item.TaxAmount
	}

	// Calculate totals for the original order
	var originalSubtotal, originalTaxAmount float64
	for _, item := range itemsToKeep {
		originalSubtotal += item.TotalPrice
		originalTaxAmount += item.TaxAmount
	}

	// Create new order
	newOrder := &models.Order{
		TenantID:          tenantID,
		CustomerID:        originalOrder.CustomerID,
		Status:            originalOrder.Status,
		PaymentStatus:     originalOrder.PaymentStatus,
		FulfillmentStatus: models.FulfillmentStatusUnfulfilled, // Reset fulfillment for split order
		Currency:          originalOrder.Currency,
		Subtotal:          newSubtotal,
		TaxAmount:         newTaxAmount,
		ShippingCost:      0, // Shipping handled separately
		DiscountAmount:    0, // Discounts stay with original order
		Total:             newSubtotal + newTaxAmount,
		Notes:             fmt.Sprintf("Split from order %s - %s", originalOrder.OrderNumber, req.Reason),
		ParentOrderID:     &originalOrder.ID,
		IsSplit:           true,
		SplitReason:       string(req.SplitType),
	}

	// Create items for new order (with new IDs)
	for _, item := range itemsToMove {
		newItem := models.OrderItem{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			SKU:         item.SKU,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TotalPrice:  item.TotalPrice,
			TaxAmount:   item.TaxAmount,
			TaxRate:     item.TaxRate,
			HSNCode:     item.HSNCode,
			SACCode:     item.SACCode,
			GSTSlab:     item.GSTSlab,
			CGSTAmount:  item.CGSTAmount,
			SGSTAmount:  item.SGSTAmount,
			IGSTAmount:  item.IGSTAmount,
		}
		newOrder.Items = append(newOrder.Items, newItem)
	}

	// Copy customer info
	if originalOrder.Customer != nil {
		newOrder.Customer = &models.OrderCustomer{
			FirstName: originalOrder.Customer.FirstName,
			LastName:  originalOrder.Customer.LastName,
			Email:     originalOrder.Customer.Email,
			Phone:     originalOrder.Customer.Phone,
		}
	}

	// Copy shipping info
	if originalOrder.Shipping != nil {
		newOrder.Shipping = &models.OrderShipping{
			Method:      originalOrder.Shipping.Method,
			Carrier:     originalOrder.Shipping.Carrier,
			Cost:        0, // Shipping cost stays with original
			Street:      originalOrder.Shipping.Street,
			City:        originalOrder.Shipping.City,
			State:       originalOrder.Shipping.State,
			StateCode:   originalOrder.Shipping.StateCode,
			PostalCode:  originalOrder.Shipping.PostalCode,
			Country:     originalOrder.Shipping.Country,
			CountryCode: originalOrder.Shipping.CountryCode,
		}
	}

	// Copy payment info
	if originalOrder.Payment != nil {
		newOrder.Payment = &models.OrderPayment{
			Method:   originalOrder.Payment.Method,
			Status:   originalOrder.Payment.Status,
			Amount:   newSubtotal + newTaxAmount,
			Currency: originalOrder.Payment.Currency,
		}
	}

	// Create the new order in database
	if err := s.orderRepo.Create(newOrder); err != nil {
		return nil, fmt.Errorf("failed to create split order: %w", err)
	}

	// Update original order totals
	originalOrder.Subtotal = originalSubtotal
	originalOrder.TaxAmount = originalTaxAmount
	originalOrder.Total = originalSubtotal + originalTaxAmount + originalOrder.ShippingCost - originalOrder.DiscountAmount
	originalOrder.IsSplit = true

	// Remove items that were moved
	if err := s.orderRepo.RemoveItems(originalOrder.ID, req.ItemIDs, tenantID); err != nil {
		return nil, fmt.Errorf("failed to remove items from original order: %w", err)
	}

	// Update original order totals
	if err := s.orderRepo.UpdateTotals(originalOrder.ID, originalSubtotal, originalTaxAmount, originalOrder.Total, tenantID); err != nil {
		return nil, fmt.Errorf("failed to update original order totals: %w", err)
	}

	// Convert item IDs to JSON for storage
	itemIDsJSON, _ := json.Marshal(req.ItemIDs)

	// Create split record
	split := &models.OrderSplit{
		TenantID:        tenantID,
		OriginalOrderID: originalOrder.ID,
		NewOrderID:      newOrder.ID,
		SplitType:       req.SplitType,
		ItemIDs:         models.JSONB(itemIDsJSON),
		Reason:          req.Reason,
		CreatedBy:       userID,
	}

	if err := s.orderRepo.CreateSplit(split); err != nil {
		return nil, fmt.Errorf("failed to create split record: %w", err)
	}

	// Add timeline events
	splitDescription := fmt.Sprintf("Order split - Items moved to new order %s. Reason: %s", newOrder.OrderNumber, req.Reason)
	s.orderRepo.AddTimelineEvent(originalOrder.ID, "ORDER_SPLIT", splitDescription, userID, tenantID)

	newOrderDescription := fmt.Sprintf("Created from order split of %s", originalOrder.OrderNumber)
	s.orderRepo.AddTimelineEvent(newOrder.ID, "ORDER_CREATED_FROM_SPLIT", newOrderDescription, userID, tenantID)

	// Reload orders with relationships
	updatedOriginal, _ := s.orderRepo.GetByID(originalOrder.ID, tenantID)
	updatedNew, _ := s.orderRepo.GetByID(newOrder.ID, tenantID)

	return &SplitOrderResponse{
		OriginalOrder: updatedOriginal,
		NewOrder:      updatedNew,
		Split:         split,
	}, nil
}

// GetChildOrders returns all child orders for a parent order
func (s *orderService) GetChildOrders(parentOrderID uuid.UUID, tenantID string) ([]models.Order, error) {
	return s.orderRepo.GetChildOrders(parentOrderID, tenantID)
}
