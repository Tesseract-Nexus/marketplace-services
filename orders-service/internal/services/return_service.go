package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"orders-service/internal/clients"
	"orders-service/internal/models"
	"orders-service/internal/repository"
)

type ReturnService struct {
	returnRepo    *repository.ReturnRepository
	orderRepo     repository.OrderRepository
	paymentClient clients.PaymentClient
}

func NewReturnService(returnRepo *repository.ReturnRepository, orderRepo repository.OrderRepository, paymentClient clients.PaymentClient) *ReturnService {
	return &ReturnService{
		returnRepo:    returnRepo,
		orderRepo:     orderRepo,
		paymentClient: paymentClient,
	}
}

// CreateReturnRequest creates a new return request
func (s *ReturnService) CreateReturnRequest(req *CreateReturnRequest) (*models.Return, error) {
	// Validate order exists
	order, err := s.orderRepo.GetByID(req.OrderID, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// Check if order can be returned
	if !s.canReturnOrder(order) {
		return nil, fmt.Errorf("order cannot be returned (status: %s)", order.Status)
	}

	// Get return policy
	policy, err := s.returnRepo.GetReturnPolicy(req.TenantID)
	if err != nil {
		// Use default policy if not found
		policy = &models.ReturnPolicy{
			ReturnWindowDays: 30,
			AllowExchange:    true,
			AllowStoreCredit: true,
		}
	}

	// Check if within return window
	if !s.isWithinReturnWindow(order.CreatedAt, policy.ReturnWindowDays) {
		return nil, fmt.Errorf("return window has expired (%d days)", policy.ReturnWindowDays)
	}

	// Validate return items
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("at least one item must be selected for return")
	}

	// Create return
	ret := &models.Return{
		TenantID:       req.TenantID,
		OrderID:        req.OrderID,
		CustomerID:     order.CustomerID,
		Status:         models.ReturnStatusPending,
		Reason:         req.Reason,
		ReturnType:     req.ReturnType,
		CustomerNotes:  req.CustomerNotes,
		Items:          make([]models.ReturnItem, 0),
	}

	// Add return items
	for _, itemReq := range req.Items {
		// Find order item
		var orderItem *models.OrderItem
		for i := range order.Items {
			if order.Items[i].ID == itemReq.OrderItemID {
				orderItem = &order.Items[i]
				break
			}
		}

		if orderItem == nil {
			return nil, fmt.Errorf("order item %s not found", itemReq.OrderItemID)
		}

		// Validate quantity
		if itemReq.Quantity <= 0 || itemReq.Quantity > orderItem.Quantity {
			return nil, fmt.Errorf("invalid quantity for item %s", orderItem.ProductName)
		}

		returnItem := models.ReturnItem{
			OrderItemID: itemReq.OrderItemID,
			ProductID:   orderItem.ProductID,
			ProductName: orderItem.ProductName,
			SKU:         orderItem.SKU,
			Quantity:    itemReq.Quantity,
			UnitPrice:   orderItem.UnitPrice,
			RefundAmount: orderItem.UnitPrice * float64(itemReq.Quantity),
			Reason:      itemReq.Reason,
			ItemNotes:   itemReq.Notes,
		}

		ret.Items = append(ret.Items, returnItem)
	}

	// Calculate refund amount
	totalRefund := 0.0
	for _, item := range ret.Items {
		totalRefund += item.RefundAmount
	}

	// Apply restocking fee if applicable
	if policy.RestockingFeePercent > 0 {
		ret.RestockingFee = totalRefund * (policy.RestockingFeePercent / 100.0)
	}

	ret.RefundAmount = totalRefund - ret.RestockingFee

	// Auto-approve if enabled in policy
	if policy.AutoApproveReturns {
		ret.Status = models.ReturnStatusApproved
		now := time.Now()
		ret.ApprovedAt = &now
	}

	// Create return in database
	if err := s.returnRepo.CreateReturn(ret); err != nil {
		return nil, fmt.Errorf("failed to create return: %w", err)
	}

	return ret, nil
}

// GetReturn retrieves a return by ID
func (s *ReturnService) GetReturn(id uuid.UUID) (*models.Return, error) {
	return s.returnRepo.GetReturnByID(id)
}

// GetReturnByRMA retrieves a return by RMA number
func (s *ReturnService) GetReturnByRMA(rmaNumber string) (*models.Return, error) {
	return s.returnRepo.GetReturnByRMANumber(rmaNumber)
}

// ListReturns lists returns with filters
func (s *ReturnService) ListReturns(tenantID string, filters map[string]interface{}, page, pageSize int) ([]models.Return, int64, error) {
	return s.returnRepo.ListReturns(tenantID, filters, page, pageSize)
}

// ApproveReturn approves a return request
func (s *ReturnService) ApproveReturn(returnID, approvedBy uuid.UUID, notes string) error {
	// Get return
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	// Check if can approve
	if !ret.CanApprove() {
		return fmt.Errorf("return cannot be approved (current status: %s)", ret.Status)
	}

	// Approve return
	if err := s.returnRepo.ApproveReturn(returnID, approvedBy, notes); err != nil {
		return err
	}

	// TODO: Send notification to customer
	// TODO: Generate return shipping label

	return nil
}

// RejectReturn rejects a return request
func (s *ReturnService) RejectReturn(returnID, rejectedBy uuid.UUID, reason string) error {
	// Get return
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	// Check if can reject
	if !ret.CanReject() {
		return fmt.Errorf("return cannot be rejected (current status: %s)", ret.Status)
	}

	// Reject return
	if err := s.returnRepo.RejectReturn(returnID, rejectedBy, reason); err != nil {
		return err
	}

	// TODO: Send notification to customer

	return nil
}

// MarkReturnInTransit marks return as in transit
func (s *ReturnService) MarkReturnInTransit(returnID uuid.UUID, trackingNumber, carrier string, userID *uuid.UUID) error {
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	if ret.Status != models.ReturnStatusApproved {
		return fmt.Errorf("return must be approved before marking in transit")
	}

	ret.Status = models.ReturnStatusInTransit
	ret.ReturnTrackingNumber = trackingNumber
	ret.ReturnCarrier = carrier

	if err := s.returnRepo.UpdateReturn(ret); err != nil {
		return err
	}

	message := fmt.Sprintf("Items in transit - Carrier: %s, Tracking: %s", carrier, trackingNumber)
	return s.returnRepo.UpdateReturnStatus(returnID, models.ReturnStatusInTransit, message, userID)
}

// MarkReturnReceived marks return as received
func (s *ReturnService) MarkReturnReceived(returnID uuid.UUID, userID *uuid.UUID) error {
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	if ret.Status != models.ReturnStatusInTransit {
		return fmt.Errorf("return must be in transit before marking as received")
	}

	return s.returnRepo.UpdateReturnStatus(returnID, models.ReturnStatusReceived, "Items received at warehouse", userID)
}

// InspectReturn inspects returned items
func (s *ReturnService) InspectReturn(returnID, inspectedBy uuid.UUID, inspectionNotes string, itemConditions map[uuid.UUID]ItemCondition) error {
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	if ret.Status != models.ReturnStatusReceived {
		return fmt.Errorf("return must be received before inspection")
	}

	// Update item conditions
	for i := range ret.Items {
		if condition, ok := itemConditions[ret.Items[i].ID]; ok {
			ret.Items[i].ReceivedCondition = condition.Condition
			ret.Items[i].IsDefective = condition.IsDefective
			ret.Items[i].CanResell = condition.CanResell

			// Adjust refund amount based on condition
			if !condition.CanResell {
				ret.Items[i].RefundAmount = ret.Items[i].RefundAmount * 0.8 // 20% reduction for non-resellable
			}
		}
	}

	ret.InspectionNotes = inspectionNotes
	now := time.Now()
	ret.InspectedAt = &now
	ret.InspectedBy = &inspectedBy

	if err := s.returnRepo.UpdateReturn(ret); err != nil {
		return err
	}

	return s.returnRepo.UpdateReturnStatus(returnID, models.ReturnStatusInspecting, "Items inspected", &inspectedBy)
}

// CompleteReturn completes return and processes refund
func (s *ReturnService) CompleteReturn(returnID, processedBy uuid.UUID, refundMethod models.RefundMethod) error {
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	if !ret.CanComplete() {
		return fmt.Errorf("return cannot be completed (current status: %s)", ret.Status)
	}

	// Calculate final refund amount
	refundAmount := ret.CalculateRefundAmount()

	// Process refund
	if err := s.processRefund(ret, refundAmount, refundMethod); err != nil {
		return fmt.Errorf("failed to process refund: %w", err)
	}

	// Complete return in database
	if err := s.returnRepo.CompleteReturn(returnID, processedBy, refundAmount, refundMethod); err != nil {
		return err
	}

	// TODO: Update inventory for returned items
	// TODO: Send notification to customer

	return nil
}

// CancelReturn cancels a return request
func (s *ReturnService) CancelReturn(returnID uuid.UUID, userID *uuid.UUID, reason string) error {
	ret, err := s.returnRepo.GetReturnByID(returnID)
	if err != nil {
		return err
	}

	if !ret.CanCancel() {
		return fmt.Errorf("return cannot be cancelled (current status: %s)", ret.Status)
	}

	return s.returnRepo.CancelReturn(returnID, userID, reason)
}

// GetReturnStats retrieves return statistics
func (s *ReturnService) GetReturnStats(tenantID string) (map[string]interface{}, error) {
	return s.returnRepo.GetReturnStats(tenantID)
}

// GetReturnsByOrder retrieves returns for a specific order
func (s *ReturnService) GetReturnsByOrder(orderID uuid.UUID) ([]models.Return, error) {
	return s.returnRepo.GetReturnsByOrderID(orderID)
}

// GetReturnsByCustomer retrieves returns for a specific customer
func (s *ReturnService) GetReturnsByCustomer(customerID uuid.UUID, page, pageSize int) ([]models.Return, int64, error) {
	return s.returnRepo.GetReturnsByCustomerID(customerID, page, pageSize)
}

// GetReturnPolicy retrieves return policy
func (s *ReturnService) GetReturnPolicy(tenantID string) (*models.ReturnPolicy, error) {
	return s.returnRepo.GetReturnPolicy(tenantID)
}

// UpdateReturnPolicy updates return policy
func (s *ReturnService) UpdateReturnPolicy(policy *models.ReturnPolicy) error {
	return s.returnRepo.UpsertReturnPolicy(policy)
}

// Helper methods

func (s *ReturnService) canReturnOrder(order *models.Order) bool {
	// Orders can be returned if delivered or completed
	if order.Status == models.OrderStatusDelivered || order.Status == models.OrderStatusCompleted {
		return true
	}
	// Also allow returns if order is in processing and fulfillment has started
	if order.Status == models.OrderStatusProcessing {
		return order.FulfillmentStatus == models.FulfillmentStatusDispatched ||
			order.FulfillmentStatus == models.FulfillmentStatusInTransit ||
			order.FulfillmentStatus == models.FulfillmentStatusOutForDelivery ||
			order.FulfillmentStatus == models.FulfillmentStatusDelivered
	}
	// Allow returns for shipped orders
	if order.Status == models.OrderStatusShipped {
		return true
	}
	return false
}

func (s *ReturnService) isWithinReturnWindow(orderDate time.Time, windowDays int) bool {
	deadline := orderDate.AddDate(0, 0, windowDays)
	return time.Now().Before(deadline)
}

func (s *ReturnService) processRefund(ret *models.Return, amount float64, method models.RefundMethod) error {
	if amount <= 0 {
		return fmt.Errorf("refund amount must be greater than 0")
	}

	// Validate refund method
	switch method {
	case models.RefundMethodOriginal:
		// Process refund to original payment method via payment-service
		if s.paymentClient == nil {
			return fmt.Errorf("payment client not configured")
		}

		// Get payments for this order
		payments, err := s.paymentClient.GetPaymentsByOrder(ret.OrderID, ret.TenantID)
		if err != nil {
			return fmt.Errorf("failed to get payments for order: %w", err)
		}

		// Find the successful payment to refund
		var paymentToRefund *clients.Payment
		for i := range payments {
			if payments[i].Status == "COMPLETED" || payments[i].Status == "CAPTURED" {
				paymentToRefund = &payments[i]
				break
			}
		}

		if paymentToRefund == nil {
			return fmt.Errorf("no completed payment found for order")
		}

		// Create refund request
		refundReq := clients.CreateRefundRequest{
			Amount: amount,
			Reason: fmt.Sprintf("Return refund for RMA %s", ret.RMANumber),
			Notes:  fmt.Sprintf("Return ID: %s, Reason: %s", ret.ID.String(), ret.Reason),
		}

		// Parse payment ID
		paymentID, err := uuid.Parse(paymentToRefund.ID)
		if err != nil {
			return fmt.Errorf("invalid payment ID: %w", err)
		}

		// Call payment service to create refund
		refundResp, err := s.paymentClient.CreateRefund(paymentID, refundReq, ret.TenantID)
		if err != nil {
			return fmt.Errorf("failed to process refund: %w", err)
		}

		// Log successful refund
		fmt.Printf("Refund processed successfully: ID=%s, Amount=%.2f, Status=%s\n",
			refundResp.ID, refundResp.Amount, refundResp.Status)

	case models.RefundMethodStoreCredit:
		// Store credit is not yet implemented
		return fmt.Errorf("store credit refunds are not yet implemented")

	case models.RefundMethodBankTransfer:
		// Bank transfer is not yet implemented
		return fmt.Errorf("bank transfer refunds are not yet implemented")

	default:
		return fmt.Errorf("invalid refund method: %s", method)
	}

	return nil
}

// DTOs

type CreateReturnRequest struct {
	TenantID      string                `json:"tenantId"`
	OrderID       uuid.UUID             `json:"orderId"`
	Reason        models.ReturnReason   `json:"reason"`
	ReturnType    models.ReturnType     `json:"returnType"`
	CustomerNotes string                `json:"customerNotes"`
	Items         []ReturnItemRequest   `json:"items"`
}

type ReturnItemRequest struct {
	OrderItemID uuid.UUID           `json:"orderItemId"`
	Quantity    int                 `json:"quantity"`
	Reason      models.ReturnReason `json:"reason"`
	Notes       string              `json:"notes"`
}

type ItemCondition struct {
	Condition   string `json:"condition"`
	IsDefective bool   `json:"isDefective"`
	CanResell   bool   `json:"canResell"`
}
