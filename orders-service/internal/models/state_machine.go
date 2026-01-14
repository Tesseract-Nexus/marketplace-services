package models

import "fmt"

// ValidOrderTransitions defines valid state transitions for OrderStatus
// Flow: PLACED → CONFIRMED → PROCESSING → SHIPPED → DELIVERED
// CANCELLED can be reached from any non-terminal state
var ValidOrderTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusPlaced:     {OrderStatusConfirmed, OrderStatusCancelled},
	OrderStatusConfirmed:  {OrderStatusProcessing, OrderStatusShipped, OrderStatusCancelled}, // Can skip PROCESSING
	OrderStatusProcessing: {OrderStatusShipped, OrderStatusCancelled},
	OrderStatusShipped:    {OrderStatusDelivered, OrderStatusCancelled},
	OrderStatusDelivered:  {OrderStatusCompleted}, // Can mark as completed after delivery
	OrderStatusCompleted:  {},                     // Terminal state
	OrderStatusCancelled:  {},                     // Terminal state
}

// ValidPaymentTransitions defines valid state transitions for PaymentStatus
var ValidPaymentTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentStatusPending:           {PaymentStatusPaid, PaymentStatusFailed},
	PaymentStatusPaid:              {PaymentStatusPartiallyRefunded, PaymentStatusRefunded},
	PaymentStatusFailed:            {PaymentStatusPending}, // Allow retry
	PaymentStatusPartiallyRefunded: {PaymentStatusRefunded},
	PaymentStatusRefunded:          {}, // Terminal state
}

// ValidFulfillmentTransitions defines valid state transitions for FulfillmentStatus
var ValidFulfillmentTransitions = map[FulfillmentStatus][]FulfillmentStatus{
	FulfillmentStatusUnfulfilled:    {FulfillmentStatusProcessing},
	FulfillmentStatusProcessing:     {FulfillmentStatusPacked, FulfillmentStatusUnfulfilled}, // Can go back if issue found
	FulfillmentStatusPacked:         {FulfillmentStatusDispatched, FulfillmentStatusProcessing},
	FulfillmentStatusDispatched:     {FulfillmentStatusInTransit},
	FulfillmentStatusInTransit:      {FulfillmentStatusOutForDelivery, FulfillmentStatusReturned},
	FulfillmentStatusOutForDelivery: {FulfillmentStatusDelivered, FulfillmentStatusFailedDelivery},
	FulfillmentStatusDelivered:      {}, // Terminal state
	FulfillmentStatusFailedDelivery: {FulfillmentStatusOutForDelivery, FulfillmentStatusReturned}, // Retry or return
	FulfillmentStatusReturned:       {FulfillmentStatusProcessing}, // Can be reprocessed
}

// CanTransitionOrderStatus checks if a transition from one order status to another is valid
func CanTransitionOrderStatus(from, to OrderStatus) bool {
	validTransitions, exists := ValidOrderTransitions[from]
	if !exists {
		return false
	}
	for _, validTo := range validTransitions {
		if validTo == to {
			return true
		}
	}
	return false
}

// CanTransitionPaymentStatus checks if a transition from one payment status to another is valid
func CanTransitionPaymentStatus(from, to PaymentStatus) bool {
	validTransitions, exists := ValidPaymentTransitions[from]
	if !exists {
		return false
	}
	for _, validTo := range validTransitions {
		if validTo == to {
			return true
		}
	}
	return false
}

// CanTransitionFulfillmentStatus checks if a transition from one fulfillment status to another is valid
func CanTransitionFulfillmentStatus(from, to FulfillmentStatus) bool {
	validTransitions, exists := ValidFulfillmentTransitions[from]
	if !exists {
		return false
	}
	for _, validTo := range validTransitions {
		if validTo == to {
			return true
		}
	}
	return false
}

// ValidateOrderStatusTransition returns an error if the transition is invalid
func ValidateOrderStatusTransition(from, to OrderStatus) error {
	if !CanTransitionOrderStatus(from, to) {
		return fmt.Errorf("invalid order status transition from %s to %s", from, to)
	}
	return nil
}

// ValidatePaymentStatusTransition returns an error if the transition is invalid
func ValidatePaymentStatusTransition(from, to PaymentStatus) error {
	if !CanTransitionPaymentStatus(from, to) {
		return fmt.Errorf("invalid payment status transition from %s to %s", from, to)
	}
	return nil
}

// ValidateFulfillmentStatusTransition returns an error if the transition is invalid
func ValidateFulfillmentStatusTransition(from, to FulfillmentStatus) error {
	if !CanTransitionFulfillmentStatus(from, to) {
		return fmt.Errorf("invalid fulfillment status transition from %s to %s", from, to)
	}
	return nil
}

// GetNextValidOrderStatuses returns the list of valid next statuses for an order
func GetNextValidOrderStatuses(current OrderStatus) []OrderStatus {
	return ValidOrderTransitions[current]
}

// GetNextValidPaymentStatuses returns the list of valid next statuses for payment
func GetNextValidPaymentStatuses(current PaymentStatus) []PaymentStatus {
	return ValidPaymentTransitions[current]
}

// GetNextValidFulfillmentStatuses returns the list of valid next statuses for fulfillment
func GetNextValidFulfillmentStatuses(current FulfillmentStatus) []FulfillmentStatus {
	return ValidFulfillmentTransitions[current]
}

// IsTerminalOrderStatus checks if the order status is a terminal state
func IsTerminalOrderStatus(status OrderStatus) bool {
	return len(ValidOrderTransitions[status]) == 0
}

// IsTerminalPaymentStatus checks if the payment status is a terminal state
func IsTerminalPaymentStatus(status PaymentStatus) bool {
	return len(ValidPaymentTransitions[status]) == 0
}

// IsTerminalFulfillmentStatus checks if the fulfillment status is a terminal state
func IsTerminalFulfillmentStatus(status FulfillmentStatus) bool {
	return len(ValidFulfillmentTransitions[status]) == 0
}

// OrderStatusDisplayName returns a human-readable name for the order status
func (s OrderStatus) DisplayName() string {
	switch s {
	case OrderStatusPlaced:
		return "Order Placed"
	case OrderStatusConfirmed:
		return "Confirmed"
	case OrderStatusProcessing:
		return "Processing"
	case OrderStatusShipped:
		return "Shipped"
	case OrderStatusDelivered:
		return "Delivered"
	case OrderStatusCompleted:
		return "Completed"
	case OrderStatusCancelled:
		return "Cancelled"
	default:
		return string(s)
	}
}

// PaymentStatusDisplayName returns a human-readable name for the payment status
func (s PaymentStatus) DisplayName() string {
	switch s {
	case PaymentStatusPending:
		return "Pending"
	case PaymentStatusPaid:
		return "Paid"
	case PaymentStatusFailed:
		return "Failed"
	case PaymentStatusPartiallyRefunded:
		return "Partially Refunded"
	case PaymentStatusRefunded:
		return "Refunded"
	default:
		return string(s)
	}
}

// FulfillmentStatusDisplayName returns a human-readable name for the fulfillment status
func (s FulfillmentStatus) DisplayName() string {
	switch s {
	case FulfillmentStatusUnfulfilled:
		return "Unfulfilled"
	case FulfillmentStatusProcessing:
		return "Processing"
	case FulfillmentStatusPacked:
		return "Packed"
	case FulfillmentStatusDispatched:
		return "Dispatched"
	case FulfillmentStatusInTransit:
		return "In Transit"
	case FulfillmentStatusOutForDelivery:
		return "Out for Delivery"
	case FulfillmentStatusDelivered:
		return "Delivered"
	case FulfillmentStatusFailedDelivery:
		return "Delivery Failed"
	case FulfillmentStatusReturned:
		return "Returned"
	default:
		return string(s)
	}
}
