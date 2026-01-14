package services

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/models"
	"customers-service/internal/repository"
)

// SegmentRule represents a single rule in a dynamic segment
type SegmentRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// SegmentRules represents the rules structure for a dynamic segment
type SegmentRules struct {
	LogicalOperator string        `json:"logicalOperator"` // AND or OR
	Rules           []SegmentRule `json:"rules"`
}

// SegmentEvaluator handles dynamic segment evaluation
type SegmentEvaluator struct {
	customerRepo *repository.CustomerRepository
	segmentRepo  *repository.SegmentRepository
}

// NewSegmentEvaluator creates a new segment evaluator
func NewSegmentEvaluator(customerRepo *repository.CustomerRepository, segmentRepo *repository.SegmentRepository) *SegmentEvaluator {
	return &SegmentEvaluator{
		customerRepo: customerRepo,
		segmentRepo:  segmentRepo,
	}
}

// EvaluateCustomerSegments evaluates all dynamic segments for a specific customer
func (e *SegmentEvaluator) EvaluateCustomerSegments(ctx context.Context, customer *models.Customer) error {
	// Get all dynamic segments for this tenant
	segments, err := e.segmentRepo.ListSegments(ctx, customer.TenantID)
	if err != nil {
		return err
	}

	for _, segment := range segments {
		if !segment.IsDynamic {
			continue
		}

		matches, err := e.evaluateRules(customer, segment.Rules)
		if err != nil {
			log.Printf("Error evaluating segment %s for customer %s: %v", segment.ID, customer.ID, err)
			continue
		}

		isMember, err := e.segmentRepo.IsCustomerInSegment(ctx, segment.ID, customer.ID)
		if err != nil {
			log.Printf("Error checking segment membership: %v", err)
			continue
		}

		if matches && !isMember {
			// Add customer to segment
			if err := e.segmentRepo.AddCustomersToSegment(ctx, segment.ID, []uuid.UUID{customer.ID}, true); err != nil {
				log.Printf("Error adding customer to segment: %v", err)
			} else {
				log.Printf("Added customer %s to dynamic segment %s", customer.ID, segment.Name)
			}
		} else if !matches && isMember {
			// Remove customer from segment (only if added automatically)
			if err := e.segmentRepo.RemoveAutoAddedCustomerFromSegment(ctx, segment.ID, customer.ID); err != nil {
				log.Printf("Error removing customer from segment: %v", err)
			} else {
				log.Printf("Removed customer %s from dynamic segment %s", customer.ID, segment.Name)
			}
		}
	}

	return nil
}

// EvaluateSegmentMembers evaluates a dynamic segment against all customers
func (e *SegmentEvaluator) EvaluateSegmentMembers(ctx context.Context, segment *models.CustomerSegment) error {
	if !segment.IsDynamic {
		return nil
	}

	// Get all customers for this tenant
	filter := repository.ListFilter{
		TenantID: segment.TenantID,
		Limit:    10000, // Large limit to get all customers
	}
	customers, _, err := e.customerRepo.List(ctx, filter)
	if err != nil {
		return err
	}

	for _, customer := range customers {
		matches, err := e.evaluateRules(&customer, segment.Rules)
		if err != nil {
			log.Printf("Error evaluating rules for customer %s: %v", customer.ID, err)
			continue
		}

		isMember, err := e.segmentRepo.IsCustomerInSegment(ctx, segment.ID, customer.ID)
		if err != nil {
			log.Printf("Error checking segment membership: %v", err)
			continue
		}

		if matches && !isMember {
			if err := e.segmentRepo.AddCustomersToSegment(ctx, segment.ID, []uuid.UUID{customer.ID}, true); err != nil {
				log.Printf("Error adding customer to segment: %v", err)
			}
		} else if !matches && isMember {
			if err := e.segmentRepo.RemoveAutoAddedCustomerFromSegment(ctx, segment.ID, customer.ID); err != nil {
				log.Printf("Error removing customer from segment: %v", err)
			}
		}
	}

	return nil
}

// evaluateRules evaluates segment rules against a customer
func (e *SegmentEvaluator) evaluateRules(customer *models.Customer, rulesJSON models.JSONB) (bool, error) {
	if len(rulesJSON) == 0 {
		return false, nil
	}

	var rules SegmentRules
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return false, err
	}

	if len(rules.Rules) == 0 {
		return false, nil
	}

	results := make([]bool, len(rules.Rules))
	for i, rule := range rules.Rules {
		results[i] = e.evaluateRule(customer, rule)
	}

	// Apply logical operator
	if strings.ToUpper(rules.LogicalOperator) == "OR" {
		for _, r := range results {
			if r {
				return true, nil
			}
		}
		return false, nil
	}

	// Default to AND
	for _, r := range results {
		if !r {
			return false, nil
		}
	}
	return true, nil
}

// evaluateRule evaluates a single rule against a customer
func (e *SegmentEvaluator) evaluateRule(customer *models.Customer, rule SegmentRule) bool {
	fieldValue := e.getCustomerFieldValue(customer, rule.Field)
	return e.compareValues(fieldValue, rule.Operator, rule.Value)
}

// getCustomerFieldValue gets the value of a customer field by name
func (e *SegmentEvaluator) getCustomerFieldValue(customer *models.Customer, field string) interface{} {
	switch strings.ToLower(field) {
	case "email":
		return customer.Email
	case "firstname", "first_name":
		return customer.FirstName
	case "lastname", "last_name":
		return customer.LastName
	case "phone":
		return customer.Phone
	case "status":
		return string(customer.Status)
	case "customertype", "customer_type":
		return string(customer.CustomerType)
	case "totalorders", "total_orders":
		return customer.TotalOrders
	case "totalspent", "total_spent":
		return customer.TotalSpent
	case "averageordervalue", "average_order_value":
		return customer.AverageOrderValue
	case "lifetimevalue", "lifetime_value":
		return customer.LifetimeValue
	case "marketingoptin", "marketing_opt_in":
		return customer.MarketingOptIn
	case "emailverified", "email_verified":
		return customer.EmailVerified
	case "daysSinceCreated", "days_since_created":
		return int(time.Since(customer.CreatedAt).Hours() / 24)
	case "daysSinceLastOrder", "days_since_last_order":
		if customer.LastOrderDate != nil {
			return int(time.Since(*customer.LastOrderDate).Hours() / 24)
		}
		return -1 // No last order
	default:
		return nil
	}
}

// compareValues compares values using the specified operator
func (e *SegmentEvaluator) compareValues(fieldValue interface{}, operator, ruleValue string) bool {
	if fieldValue == nil {
		return operator == "isEmpty" || operator == "isNull"
	}

	switch operator {
	case "equals", "eq":
		return e.toString(fieldValue) == ruleValue
	case "notEquals", "neq":
		return e.toString(fieldValue) != ruleValue
	case "contains":
		return strings.Contains(strings.ToLower(e.toString(fieldValue)), strings.ToLower(ruleValue))
	case "notContains":
		return !strings.Contains(strings.ToLower(e.toString(fieldValue)), strings.ToLower(ruleValue))
	case "startsWith":
		return strings.HasPrefix(strings.ToLower(e.toString(fieldValue)), strings.ToLower(ruleValue))
	case "endsWith":
		return strings.HasSuffix(strings.ToLower(e.toString(fieldValue)), strings.ToLower(ruleValue))
	case "greaterThan", "gt":
		return e.toFloat(fieldValue) > e.toFloat(ruleValue)
	case "greaterThanOrEqual", "gte":
		return e.toFloat(fieldValue) >= e.toFloat(ruleValue)
	case "lessThan", "lt":
		return e.toFloat(fieldValue) < e.toFloat(ruleValue)
	case "lessThanOrEqual", "lte":
		return e.toFloat(fieldValue) <= e.toFloat(ruleValue)
	case "isEmpty":
		return e.toString(fieldValue) == ""
	case "isNotEmpty":
		return e.toString(fieldValue) != ""
	case "isTrue":
		return e.toBool(fieldValue) == true
	case "isFalse":
		return e.toBool(fieldValue) == false
	default:
		return false
	}
}

func (e *SegmentEvaluator) toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return ""
	}
}

func (e *SegmentEvaluator) toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

func (e *SegmentEvaluator) toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.ToLower(val) == "true" || val == "1"
	case int:
		return val != 0
	default:
		return false
	}
}
