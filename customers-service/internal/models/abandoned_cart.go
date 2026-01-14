package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AbandonedCartStatus represents the status of an abandoned cart
type AbandonedCartStatus string

const (
	AbandonedCartStatusPending   AbandonedCartStatus = "PENDING"   // Cart abandoned, no action taken
	AbandonedCartStatusReminded  AbandonedCartStatus = "REMINDED"  // Reminder email(s) sent
	AbandonedCartStatusRecovered AbandonedCartStatus = "RECOVERED" // Customer completed purchase
	AbandonedCartStatusExpired   AbandonedCartStatus = "EXPIRED"   // Cart expired (too old to recover)
)

// AbandonedCart tracks abandoned shopping carts for recovery campaigns
type AbandonedCart struct {
	ID         uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID   string              `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	CartID     uuid.UUID           `json:"cartId" gorm:"type:uuid;not null;index"`
	CustomerID uuid.UUID           `json:"customerId" gorm:"type:uuid;not null;index"`
	Status     AbandonedCartStatus `json:"status" gorm:"type:varchar(20);not null;default:'PENDING'"`

	// Cart snapshot at time of abandonment
	Items     JSONB   `json:"items" gorm:"type:jsonb;not null"`
	Subtotal  float64 `json:"subtotal" gorm:"type:decimal(12,2);not null"`
	ItemCount int     `json:"itemCount" gorm:"not null"`

	// Customer info for outreach
	CustomerEmail     string `json:"customerEmail" gorm:"type:varchar(255)"`
	CustomerFirstName string `json:"customerFirstName" gorm:"type:varchar(100)"`
	CustomerLastName  string `json:"customerLastName" gorm:"type:varchar(100)"`

	// Abandonment tracking
	AbandonedAt time.Time `json:"abandonedAt" gorm:"not null"`
	LastCartActivity time.Time `json:"lastCartActivity" gorm:"not null"` // When cart was last updated

	// Recovery tracking
	ReminderCount    int        `json:"reminderCount" gorm:"default:0"`        // Number of reminders sent
	LastReminderAt   *time.Time `json:"lastReminderAt"`                        // When last reminder was sent
	NextReminderAt   *time.Time `json:"nextReminderAt"`                        // Scheduled next reminder
	RecoveredAt      *time.Time `json:"recoveredAt"`                           // When cart was recovered
	RecoveredOrderID *uuid.UUID `json:"recoveredOrderId" gorm:"type:uuid"`     // Order created from recovery
	ExpiredAt        *time.Time `json:"expiredAt"`                             // When cart was marked expired

	// Analytics
	RecoverySource string  `json:"recoverySource" gorm:"type:varchar(50)"` // email_reminder, discount_offer, direct
	DiscountUsed   string  `json:"discountUsed" gorm:"type:varchar(50)"`   // Coupon code if any
	RecoveredValue float64 `json:"recoveredValue" gorm:"type:decimal(12,2);default:0"` // Final order value

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships (loaded via joins)
	Customer *Customer     `json:"customer,omitempty" gorm:"foreignKey:CustomerID;references:ID"`
	Cart     *CustomerCart `json:"cart,omitempty" gorm:"foreignKey:CartID;references:ID"`
}

// TableName returns the table name for GORM
func (AbandonedCart) TableName() string {
	return "abandoned_carts"
}

// AbandonedCartRecoveryAttempt tracks individual recovery attempts
type AbandonedCartRecoveryAttempt struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	AbandonedCartID  uuid.UUID `json:"abandonedCartId" gorm:"type:uuid;not null;index"`
	TenantID         string    `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	AttemptType      string    `json:"attemptType" gorm:"type:varchar(50);not null"` // email, sms, push
	AttemptNumber    int       `json:"attemptNumber" gorm:"not null"`                // 1st, 2nd, 3rd attempt
	Status           string    `json:"status" gorm:"type:varchar(20);not null"`      // sent, delivered, opened, clicked, failed
	MessageTemplate  string    `json:"messageTemplate" gorm:"type:varchar(100)"`     // Template used
	DiscountOffered  string    `json:"discountOffered" gorm:"type:varchar(50)"`      // Coupon offered if any
	ExternalID       string    `json:"externalId" gorm:"type:varchar(255)"`          // Email service message ID
	SentAt           time.Time `json:"sentAt" gorm:"not null"`
	DeliveredAt      *time.Time `json:"deliveredAt"`
	OpenedAt         *time.Time `json:"openedAt"`
	ClickedAt        *time.Time `json:"clickedAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

// TableName returns the table name for GORM
func (AbandonedCartRecoveryAttempt) TableName() string {
	return "abandoned_cart_recovery_attempts"
}

// AbandonedCartSettings stores tenant-specific settings for abandoned cart recovery
type AbandonedCartSettings struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID string    `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex"`

	// Abandonment detection
	AbandonmentThresholdMinutes int  `json:"abandonmentThresholdMinutes" gorm:"default:60"` // Minutes before cart is considered abandoned
	ExpirationDays              int  `json:"expirationDays" gorm:"default:30"`              // Days before abandoned cart expires
	Enabled                     bool `json:"enabled" gorm:"default:true"`

	// Reminder schedule (intervals in hours after abandonment)
	FirstReminderHours  int `json:"firstReminderHours" gorm:"default:1"`   // 1 hour after abandonment
	SecondReminderHours int `json:"secondReminderHours" gorm:"default:24"` // 24 hours after abandonment
	ThirdReminderHours  int `json:"thirdReminderHours" gorm:"default:72"`  // 72 hours after abandonment
	MaxReminders        int `json:"maxReminders" gorm:"default:3"`

	// Incentives
	OfferDiscountOnReminder int    `json:"offerDiscountOnReminder" gorm:"default:2"`      // Which reminder to offer discount (0=none)
	DiscountType            string `json:"discountType" gorm:"type:varchar(20)"`          // percentage, fixed
	DiscountValue           float64 `json:"discountValue" gorm:"type:decimal(10,2)"`
	DiscountCode            string `json:"discountCode" gorm:"type:varchar(50)"`

	// Email templates
	ReminderEmailTemplate1 string `json:"reminderEmailTemplate1" gorm:"type:varchar(100)"` // Template for 1st reminder
	ReminderEmailTemplate2 string `json:"reminderEmailTemplate2" gorm:"type:varchar(100)"` // Template for 2nd reminder
	ReminderEmailTemplate3 string `json:"reminderEmailTemplate3" gorm:"type:varchar(100)"` // Template for 3rd reminder

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName returns the table name for GORM
func (AbandonedCartSettings) TableName() string {
	return "abandoned_cart_settings"
}

// IsRecoverable returns true if the abandoned cart can still be recovered
func (ac *AbandonedCart) IsRecoverable() bool {
	return ac.Status == AbandonedCartStatusPending || ac.Status == AbandonedCartStatusReminded
}

// MarkAsReminded updates the cart status after sending a reminder
func (ac *AbandonedCart) MarkAsReminded(nextReminderAt *time.Time) {
	now := time.Now()
	ac.Status = AbandonedCartStatusReminded
	ac.ReminderCount++
	ac.LastReminderAt = &now
	ac.NextReminderAt = nextReminderAt
}

// MarkAsRecovered updates the cart when the customer completes purchase
func (ac *AbandonedCart) MarkAsRecovered(orderID uuid.UUID, source string, discountUsed string, orderValue float64) {
	now := time.Now()
	ac.Status = AbandonedCartStatusRecovered
	ac.RecoveredAt = &now
	ac.RecoveredOrderID = &orderID
	ac.RecoverySource = source
	ac.DiscountUsed = discountUsed
	ac.RecoveredValue = orderValue
	ac.NextReminderAt = nil
}

// MarkAsExpired marks the cart as expired
func (ac *AbandonedCart) MarkAsExpired() {
	now := time.Now()
	ac.Status = AbandonedCartStatusExpired
	ac.ExpiredAt = &now
	ac.NextReminderAt = nil
}
