package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Campaign represents a marketing campaign
type Campaign struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string          `gorm:"type:varchar(100);not null;index:idx_campaigns_tenant" json:"tenantId"`
	Name        string          `gorm:"type:varchar(255);not null" json:"name"`
	Description string          `gorm:"type:text" json:"description"`
	Type        CampaignType    `gorm:"type:varchar(50);not null" json:"type"`
	Channel     CampaignChannel `gorm:"type:varchar(50);not null" json:"channel"`
	Status      CampaignStatus  `gorm:"type:varchar(50);not null;default:'DRAFT'" json:"status"`

	// Segmentation
	SegmentID   *uuid.UUID      `gorm:"type:uuid;index:idx_campaigns_segment" json:"segmentId,omitempty"`
	TargetAll   bool            `gorm:"default:false" json:"targetAll"`

	// Content
	Subject     string          `gorm:"type:varchar(500)" json:"subject,omitempty"`
	Content     string          `gorm:"type:text" json:"content"`
	TemplateID  *uuid.UUID      `gorm:"type:uuid" json:"templateId,omitempty"`

	// Scheduling
	ScheduledAt *time.Time      `json:"scheduledAt,omitempty"`
	SentAt      *time.Time      `json:"sentAt,omitempty"`

	// Analytics
	TotalRecipients int64        `gorm:"default:0" json:"totalRecipients"`
	Sent            int64        `gorm:"default:0" json:"sent"`
	Delivered       int64        `gorm:"default:0" json:"delivered"`
	Opened          int64        `gorm:"default:0" json:"opened"`
	Clicked         int64        `gorm:"default:0" json:"clicked"`
	Converted       int64        `gorm:"default:0" json:"converted"`
	Unsubscribed    int64        `gorm:"default:0" json:"unsubscribed"`
	Failed          int64        `gorm:"default:0" json:"failed"`
	Revenue         float64      `gorm:"type:decimal(15,2);default:0" json:"revenue"`

	// Metadata
	Metadata    datatypes.JSON  `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedBy   uuid.UUID       `gorm:"type:uuid" json:"createdBy"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deletedAt,omitempty"`
}

// CampaignType represents the type of campaign
type CampaignType string

const (
	CampaignTypePromotion      CampaignType = "PROMOTION"
	CampaignTypeAbandonedCart  CampaignType = "ABANDONED_CART"
	CampaignTypeWelcome        CampaignType = "WELCOME"
	CampaignTypeWinback        CampaignType = "WINBACK"
	CampaignTypeProductLaunch  CampaignType = "PRODUCT_LAUNCH"
	CampaignTypeNewsletter     CampaignType = "NEWSLETTER"
	CampaignTypeTransactional  CampaignType = "TRANSACTIONAL"
	CampaignTypeReEngagement   CampaignType = "RE_ENGAGEMENT"
)

// CampaignChannel represents the communication channel
type CampaignChannel string

const (
	CampaignChannelEmail    CampaignChannel = "EMAIL"
	CampaignChannelSMS      CampaignChannel = "SMS"
	CampaignChannelPush     CampaignChannel = "PUSH"
	CampaignChannelInApp    CampaignChannel = "IN_APP"
	CampaignChannelMulti    CampaignChannel = "MULTI"
)

// CampaignStatus represents the campaign status
type CampaignStatus string

const (
	CampaignStatusDraft      CampaignStatus = "DRAFT"
	CampaignStatusScheduled  CampaignStatus = "SCHEDULED"
	CampaignStatusSending    CampaignStatus = "SENDING"
	CampaignStatusSent       CampaignStatus = "SENT"
	CampaignStatusPaused     CampaignStatus = "PAUSED"
	CampaignStatusCancelled  CampaignStatus = "CANCELLED"
	CampaignStatusCompleted  CampaignStatus = "COMPLETED"
)

// CustomerSegment represents a customer segment for targeting
type CustomerSegment struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string          `gorm:"type:varchar(100);not null;index:idx_segments_tenant" json:"tenantId"`
	Name        string          `gorm:"type:varchar(255);not null" json:"name"`
	Description string          `gorm:"type:text" json:"description"`
	Type        SegmentType     `gorm:"type:varchar(50);not null" json:"type"`

	// Rules
	Rules       datatypes.JSON  `gorm:"type:jsonb;not null" json:"rules"`

	// Statistics
	CustomerCount int64         `gorm:"default:0" json:"customerCount"`
	LastCalculated *time.Time   `json:"lastCalculated,omitempty"`

	// Metadata
	IsActive    bool            `gorm:"default:true" json:"isActive"`
	CreatedBy   uuid.UUID       `gorm:"type:uuid" json:"createdBy"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deletedAt,omitempty"`
}

// SegmentType represents the type of segment
type SegmentType string

const (
	SegmentTypeStatic   SegmentType = "STATIC"   // Manual customer list
	SegmentTypeDynamic  SegmentType = "DYNAMIC"  // Rule-based, auto-updating
)

// SegmentRule represents a rule for customer segmentation
type SegmentRule struct {
	Field    string      `json:"field"`    // e.g., "total_orders", "total_spent", "last_order_date"
	Operator string      `json:"operator"` // e.g., "gt", "lt", "eq", "between", "in"
	Value    interface{} `json:"value"`    // Value to compare against
}

// AbandonedCart represents an abandoned cart for recovery
type AbandonedCart struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string          `gorm:"type:varchar(100);not null;index:idx_abandoned_carts_tenant" json:"tenantId"`
	CustomerID      uuid.UUID       `gorm:"type:uuid;not null;index:idx_abandoned_carts_customer" json:"customerId"`
	SessionID       string          `gorm:"type:varchar(255);index" json:"sessionId"`

	// Cart details
	CartItems       datatypes.JSON  `gorm:"type:jsonb;not null" json:"cartItems"`
	TotalAmount     float64         `gorm:"type:decimal(15,2);not null" json:"totalAmount"`
	ItemCount       int             `gorm:"not null" json:"itemCount"`

	// Recovery
	Status          AbandonedStatus `gorm:"type:varchar(50);not null;default:'PENDING'" json:"status"`
	RecoveryAttempts int            `gorm:"default:0" json:"recoveryAttempts"`
	LastReminderSent *time.Time     `json:"lastReminderSent,omitempty"`

	// Outcome
	RecoveredAt     *time.Time      `json:"recoveredAt,omitempty"`
	OrderID         *uuid.UUID      `gorm:"type:uuid" json:"orderId,omitempty"`
	RecoveredAmount float64         `gorm:"type:decimal(15,2);default:0" json:"recoveredAmount"`

	// Metadata
	AbandonedAt     time.Time       `gorm:"not null;index" json:"abandonedAt"`
	ExpiresAt       time.Time       `gorm:"not null;index" json:"expiresAt"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updatedAt"`
}

// AbandonedStatus represents the status of an abandoned cart
type AbandonedStatus string

const (
	AbandonedStatusPending   AbandonedStatus = "PENDING"
	AbandonedStatusReminded  AbandonedStatus = "REMINDED"
	AbandonedStatusRecovered AbandonedStatus = "RECOVERED"
	AbandonedStatusExpired   AbandonedStatus = "EXPIRED"
	AbandonedStatusIgnored   AbandonedStatus = "IGNORED"
)

// CartItem represents an item in an abandoned cart
type CartItem struct {
	ProductID   uuid.UUID `json:"productId"`
	ProductName string    `json:"productName"`
	SKU         string    `json:"sku"`
	Quantity    int       `json:"quantity"`
	Price       float64   `json:"price"`
	ImageURL    string    `json:"imageUrl,omitempty"`
}

// LoyaltyProgram represents a loyalty program configuration
type LoyaltyProgram struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string          `gorm:"type:varchar(100);not null;unique;index" json:"tenantId"`
	Name            string          `gorm:"type:varchar(255);not null" json:"name"`
	Description     string          `gorm:"type:text" json:"description"`

	// Points configuration
	PointsPerDollar float64         `gorm:"type:decimal(10,2);not null;default:1" json:"pointsPerDollar"`
	MinimumPoints   int             `gorm:"default:0" json:"minimumPoints"`
	PointsExpiry    int             `gorm:"default:365" json:"pointsExpiry"` // Days until points expire

	// Tiers
	Tiers           datatypes.JSON  `gorm:"type:jsonb" json:"tiers,omitempty"`

	// Settings
	IsActive        bool            `gorm:"default:true" json:"isActive"`
	SignupBonus     int             `gorm:"default:0" json:"signupBonus"`
	BirthdayBonus   int             `gorm:"default:0" json:"birthdayBonus"`
	ReferralBonus   int             `gorm:"default:0" json:"referralBonus"`

	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updatedAt"`
}

// LoyaltyTier represents a loyalty tier
type LoyaltyTier struct {
	Name            string  `json:"name"`
	MinimumPoints   int     `json:"minimumPoints"`
	DiscountPercent float64 `json:"discountPercent"`
	BenefitsDesc    string  `json:"benefitsDesc"`
}

// CustomerLoyalty represents a customer's loyalty account
type CustomerLoyalty struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string          `gorm:"type:varchar(100);not null;index:idx_loyalty_tenant" json:"tenantId"`
	CustomerID      uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_loyalty_tenant_customer" json:"customerId"`

	// Points
	TotalPoints     int             `gorm:"default:0" json:"totalPoints"`
	AvailablePoints int             `gorm:"default:0" json:"availablePoints"`
	LifetimePoints  int             `gorm:"default:0" json:"lifetimePoints"`

	// Tier
	CurrentTier     string          `gorm:"type:varchar(100)" json:"currentTier,omitempty"`
	TierSince       *time.Time      `json:"tierSince,omitempty"`

	// Referral
	ReferralCode    string          `gorm:"type:varchar(20);uniqueIndex" json:"referralCode,omitempty"`
	ReferredBy      *uuid.UUID      `gorm:"type:uuid;index:idx_loyalty_referred_by" json:"referredBy,omitempty"`

	// Personal
	DateOfBirth     *time.Time      `gorm:"type:date" json:"dateOfBirth,omitempty"`

	// Metadata
	LastEarned      *time.Time      `json:"lastEarned,omitempty"`
	LastRedeemed    *time.Time      `json:"lastRedeemed,omitempty"`
	JoinedAt        time.Time       `gorm:"not null" json:"joinedAt"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updatedAt"`
}

// LoyaltyTransaction represents a loyalty points transaction
type LoyaltyTransaction struct {
	ID              uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string              `gorm:"type:varchar(100);not null;index:idx_loyalty_txn_tenant" json:"tenantId"`
	CustomerID      uuid.UUID           `gorm:"type:uuid;not null;index:idx_loyalty_txn_customer" json:"customerId"`
	LoyaltyID       uuid.UUID           `gorm:"type:uuid;not null" json:"loyaltyId"`

	// Transaction details
	Type            LoyaltyTxnType      `gorm:"type:varchar(50);not null" json:"type"`
	Points          int                 `gorm:"not null" json:"points"` // Positive for earn, negative for redeem
	Description     string              `gorm:"type:varchar(500)" json:"description"`

	// Reference
	OrderID         *uuid.UUID          `gorm:"type:uuid" json:"orderId,omitempty"`
	ReferenceID     *uuid.UUID          `gorm:"type:uuid" json:"referenceId,omitempty"`
	ReferenceType   string              `gorm:"type:varchar(50)" json:"referenceType,omitempty"`

	// Expiry
	ExpiresAt       *time.Time          `json:"expiresAt,omitempty"`
	ExpiredAt       *time.Time          `json:"expiredAt,omitempty"`

	CreatedAt       time.Time           `gorm:"autoCreateTime" json:"createdAt"`
}

// LoyaltyTxnType represents the type of loyalty transaction
type LoyaltyTxnType string

const (
	LoyaltyTxnEarn      LoyaltyTxnType = "EARN"
	LoyaltyTxnRedeem    LoyaltyTxnType = "REDEEM"
	LoyaltyTxnBonus     LoyaltyTxnType = "BONUS"
	LoyaltyTxnReferral  LoyaltyTxnType = "REFERRAL"
	LoyaltyTxnExpired   LoyaltyTxnType = "EXPIRED"
	LoyaltyTxnAdjust    LoyaltyTxnType = "ADJUSTMENT"
)

// Referral represents a referral relationship between customers
type Referral struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID              string     `gorm:"type:varchar(100);not null;index:idx_referrals_tenant" json:"tenantId"`

	// Referrer (the person who referred)
	ReferrerID            uuid.UUID  `gorm:"type:uuid;not null;index:idx_referrals_referrer" json:"referrerId"`
	ReferrerLoyaltyID     uuid.UUID  `gorm:"type:uuid;not null" json:"referrerLoyaltyId"`

	// Referred (the new customer)
	ReferredID            uuid.UUID  `gorm:"type:uuid;not null;index:idx_referrals_referred" json:"referredId"`
	ReferredLoyaltyID     uuid.UUID  `gorm:"type:uuid;not null" json:"referredLoyaltyId"`

	// Referral details
	ReferralCode          string     `gorm:"type:varchar(20);not null;index:idx_referrals_code" json:"referralCode"`
	Status                ReferralStatus `gorm:"type:varchar(50);not null;default:'PENDING'" json:"status"`

	// Bonus tracking
	ReferrerBonusPoints   int        `gorm:"default:0" json:"referrerBonusPoints"`
	ReferredBonusPoints   int        `gorm:"default:0" json:"referredBonusPoints"`
	ReferrerBonusAwardedAt *time.Time `json:"referrerBonusAwardedAt,omitempty"`
	ReferredBonusAwardedAt *time.Time `json:"referredBonusAwardedAt,omitempty"`

	// Metadata
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt             time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

// ReferralStatus represents the status of a referral
type ReferralStatus string

const (
	ReferralStatusPending   ReferralStatus = "PENDING"
	ReferralStatusCompleted ReferralStatus = "COMPLETED"
	ReferralStatusExpired   ReferralStatus = "EXPIRED"
)

// CouponCode represents an enhanced coupon/discount code
type CouponCode struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string          `gorm:"type:varchar(100);not null;index:idx_coupons_tenant" json:"tenantId"`
	Code            string          `gorm:"type:varchar(50);not null;uniqueIndex:idx_coupons_code" json:"code"`
	Name            string          `gorm:"type:varchar(255);not null" json:"name"`
	Description     string          `gorm:"type:text" json:"description"`

	// Discount configuration
	Type            CouponType      `gorm:"type:varchar(50);not null" json:"type"`
	DiscountValue   float64         `gorm:"type:decimal(15,2);not null" json:"discountValue"`
	MaxDiscount     float64         `gorm:"type:decimal(15,2)" json:"maxDiscount,omitempty"`

	// Conditions
	MinOrderAmount  float64         `gorm:"type:decimal(15,2)" json:"minOrderAmount,omitempty"`
	MaxOrderAmount  float64         `gorm:"type:decimal(15,2)" json:"maxOrderAmount,omitempty"`
	ApplicableProducts datatypes.JSON `gorm:"type:jsonb" json:"applicableProducts,omitempty"`
	ApplicableCategories datatypes.JSON `gorm:"type:jsonb" json:"applicableCategories,omitempty"`
	ExcludedProducts datatypes.JSON `gorm:"type:jsonb" json:"excludedProducts,omitempty"`

	// Usage limits
	MaxUsage        int             `gorm:"default:0" json:"maxUsage"` // 0 = unlimited
	UsagePerCustomer int            `gorm:"default:1" json:"usagePerCustomer"`
	CurrentUsage    int             `gorm:"default:0" json:"currentUsage"`

	// Validity
	ValidFrom       time.Time       `gorm:"not null" json:"validFrom"`
	ValidUntil      time.Time       `gorm:"not null" json:"validUntil"`

	// Settings
	IsActive        bool            `gorm:"default:true" json:"isActive"`
	IsPublic        bool            `gorm:"default:true" json:"isPublic"`
	CampaignID      *uuid.UUID      `gorm:"type:uuid" json:"campaignId,omitempty"`

	// Metadata
	CreatedBy       uuid.UUID       `gorm:"type:uuid" json:"createdBy"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt  `gorm:"index" json:"deletedAt,omitempty"`
}

// CouponType represents the type of coupon discount
type CouponType string

const (
	CouponTypePercentage    CouponType = "PERCENTAGE"
	CouponTypeFixedAmount   CouponType = "FIXED_AMOUNT"
	CouponTypeFreeShipping  CouponType = "FREE_SHIPPING"
	CouponTypeBuyXGetY      CouponType = "BUY_X_GET_Y"
)

// CouponUsage represents a record of coupon usage
type CouponUsage struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string          `gorm:"type:varchar(100);not null;index" json:"tenantId"`
	CouponID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"couponId"`
	CustomerID      uuid.UUID       `gorm:"type:uuid;not null;index" json:"customerId"`
	OrderID         uuid.UUID       `gorm:"type:uuid;not null;unique" json:"orderId"`

	DiscountAmount  float64         `gorm:"type:decimal(15,2);not null" json:"discountAmount"`
	OrderTotal      float64         `gorm:"type:decimal(15,2);not null" json:"orderTotal"`

	UsedAt          time.Time       `gorm:"not null" json:"usedAt"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"createdAt"`
}

// CampaignRecipient represents a recipient in a campaign
type CampaignRecipient struct {
	ID              uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CampaignID      uuid.UUID           `gorm:"type:uuid;not null;index:idx_recipients_campaign" json:"campaignId"`
	CustomerID      uuid.UUID           `gorm:"type:uuid;not null;index:idx_recipients_customer" json:"customerId"`

	Status          RecipientStatus     `gorm:"type:varchar(50);not null;default:'PENDING'" json:"status"`
	SentAt          *time.Time          `json:"sentAt,omitempty"`
	DeliveredAt     *time.Time          `json:"deliveredAt,omitempty"`
	OpenedAt        *time.Time          `json:"openedAt,omitempty"`
	ClickedAt       *time.Time          `json:"clickedAt,omitempty"`
	ConvertedAt     *time.Time          `json:"convertedAt,omitempty"`
	UnsubscribedAt  *time.Time          `json:"unsubscribedAt,omitempty"`

	ErrorMessage    string              `gorm:"type:text" json:"errorMessage,omitempty"`

	CreatedAt       time.Time           `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time           `gorm:"autoUpdateTime" json:"updatedAt"`
}

// RecipientStatus represents the delivery status of a campaign recipient
type RecipientStatus string

const (
	RecipientStatusPending      RecipientStatus = "PENDING"
	RecipientStatusSent         RecipientStatus = "SENT"
	RecipientStatusDelivered    RecipientStatus = "DELIVERED"
	RecipientStatusOpened       RecipientStatus = "OPENED"
	RecipientStatusClicked      RecipientStatus = "CLICKED"
	RecipientStatusConverted    RecipientStatus = "CONVERTED"
	RecipientStatusFailed       RecipientStatus = "FAILED"
	RecipientStatusUnsubscribed RecipientStatus = "UNSUBSCRIBED"
)

// CampaignFilter represents filters for campaign queries
type CampaignFilter struct {
	TenantID    string
	Type        *CampaignType
	Channel     *CampaignChannel
	Status      *CampaignStatus
	SearchQuery string
	Limit       int
	Offset      int
}

// SegmentFilter represents filters for segment queries
type SegmentFilter struct {
	TenantID    string
	Type        *SegmentType
	IsActive    *bool
	SearchQuery string
	Limit       int
	Offset      int
}

// AbandonedCartFilter represents filters for abandoned cart queries
type AbandonedCartFilter struct {
	TenantID     string
	CustomerID   *uuid.UUID
	Status       *AbandonedStatus
	MinAmount    *float64
	MaxAmount    *float64
	FromDate     *time.Time
	ToDate       *time.Time
	Limit        int
	Offset       int
}
