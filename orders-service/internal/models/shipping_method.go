package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// ShippingMethod represents a shipping method available for a tenant
type ShippingMethod struct {
	ID                    uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID              string         `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	Name                  string         `json:"name" gorm:"type:varchar(100);not null"`
	Description           string         `json:"description" gorm:"type:text"`
	EstimatedDaysMin      int            `json:"estimatedDaysMin" gorm:"default:1"`
	EstimatedDaysMax      int            `json:"estimatedDaysMax" gorm:"default:5"`
	BaseRate              float64        `json:"baseRate" gorm:"type:decimal(10,2);not null"`
	FreeShippingThreshold *float64       `json:"freeShippingThreshold" gorm:"type:decimal(10,2)"`
	Countries             pq.StringArray `json:"countries" gorm:"type:text[]"`
	IsActive              bool           `json:"isActive" gorm:"default:true"`
	SortOrder             int            `json:"sortOrder" gorm:"default:0"`
	CreatedAt             time.Time      `json:"createdAt"`
	UpdatedAt             time.Time      `json:"updatedAt"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName specifies the table name for ShippingMethod
func (ShippingMethod) TableName() string {
	return "shipping_methods"
}
