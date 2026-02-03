package seeders

import (
	"log"

	"approval-service/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedSystemWorkflows creates or updates system-level approval workflows.
// These workflows use tenant_id 'system' and are available to all tenants as fallback.
func SeedSystemWorkflows(db *gorm.DB) error {
	workflows := []models.ApprovalWorkflow{
		// Product Creation/Publication Approval
		{
			TenantID:    "system",
			Name:        "product_creation",
			DisplayName: "Product Publication Approval",
			Description: "Approval workflow for publishing products from draft state",
			TriggerType: "always",
			TriggerConfig: datatypes.JSON(`{}`),
			ApproverConfig: datatypes.JSON(`{
				"approver_role": "manager",
				"require_different_user": false,
				"require_active_staff": true
			}`),
			TimeoutHours: 48,
			EscalationConfig: datatypes.JSON(`{
				"enabled": true,
				"levels": [
					{"after_hours": 24, "escalate_to_role": "admin"},
					{"after_hours": 48, "escalate_to_role": "owner"}
				]
			}`),
			IsSystem: true,
			IsActive: true,
		},
		// Category Creation/Publication Approval
		{
			TenantID:    "system",
			Name:        "category_creation",
			DisplayName: "Category Publication Approval",
			Description: "Approval workflow for publishing categories from draft state",
			TriggerType: "always",
			TriggerConfig: datatypes.JSON(`{}`),
			ApproverConfig: datatypes.JSON(`{
				"approver_role": "manager",
				"require_different_user": false,
				"require_active_staff": true
			}`),
			TimeoutHours: 48,
			EscalationConfig: datatypes.JSON(`{
				"enabled": true,
				"levels": [
					{"after_hours": 24, "escalate_to_role": "admin"},
					{"after_hours": 48, "escalate_to_role": "owner"}
				]
			}`),
			IsSystem: true,
			IsActive: true,
		},
	}

	for _, workflow := range workflows {
		// Use upsert (ON CONFLICT DO UPDATE) to create or update
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"display_name", "description", "trigger_config", "approver_config", "escalation_config", "updated_at"}),
		}).Create(&workflow)

		if result.Error != nil {
			log.Printf("Failed to seed workflow %s: %v", workflow.Name, result.Error)
			return result.Error
		}
		log.Printf("Seeded workflow: %s (tenant: %s)", workflow.Name, workflow.TenantID)
	}

	return nil
}
