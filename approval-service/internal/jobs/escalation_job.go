package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"approval-service/internal/models"
	"approval-service/internal/repository"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// EscalationJob handles automatic escalation of pending approval requests
type EscalationJob struct {
	repo      *repository.ApprovalRepository
	publisher *events.Publisher
	logger    *logrus.Logger
	interval  time.Duration
	stopCh    chan struct{}
}

// NewEscalationJob creates a new escalation job
func NewEscalationJob(repo *repository.ApprovalRepository, publisher *events.Publisher, logger *logrus.Logger) *EscalationJob {
	return &EscalationJob{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		interval:  15 * time.Minute, // Check every 15 minutes
		stopCh:    make(chan struct{}),
	}
}

// Start begins the escalation job
func (j *EscalationJob) Start(ctx context.Context) {
	j.logger.Info("Escalation job started")

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Run immediately on start
	j.runEscalationCheck(ctx)

	for {
		select {
		case <-ticker.C:
			j.runEscalationCheck(ctx)
		case <-j.stopCh:
			j.logger.Info("Escalation job stopped")
			return
		case <-ctx.Done():
			j.logger.Info("Escalation job context cancelled")
			return
		}
	}
}

// Stop signals the job to stop
func (j *EscalationJob) Stop() {
	close(j.stopCh)
}

// runEscalationCheck finds and escalates pending requests
func (j *EscalationJob) runEscalationCheck(ctx context.Context) {
	j.logger.Debug("Running escalation check...")

	// Find pending requests that need escalation
	requests, err := j.repo.FindRequestsNeedingEscalation(ctx)
	if err != nil {
		j.logger.Errorf("Failed to find requests needing escalation: %v", err)
		return
	}

	if len(requests) == 0 {
		j.logger.Debug("No requests need escalation")
		return
	}

	j.logger.Infof("Found %d requests needing escalation", len(requests))

	for _, request := range requests {
		if err := j.escalateRequest(ctx, &request); err != nil {
			j.logger.Errorf("Failed to escalate request %s: %v", request.ID, err)
			continue
		}
		j.logger.Infof("Escalated request %s to level %d", request.ID, request.EscalationLevel+1)
	}

	// Also check for expired requests
	j.expireTimedOutRequests(ctx)
}

// escalateRequest escalates a single request to the next level
// Uses database-level locking to prevent concurrent escalation in multi-pod deployments
func (j *EscalationJob) escalateRequest(ctx context.Context, request *models.ApprovalRequest) error {
	// Get the workflow to find escalation config
	workflow, err := j.repo.GetWorkflowByID(ctx, request.WorkflowID)
	if err != nil {
		return err
	}

	// Parse escalation config
	var escalationConfig models.EscalationConfig
	if err := json.Unmarshal(workflow.EscalationConfig, &escalationConfig); err != nil {
		return err
	}

	if !escalationConfig.Enabled || len(escalationConfig.Levels) == 0 {
		return nil
	}

	// Find the next escalation level
	nextLevel := request.EscalationLevel + 1
	if nextLevel > len(escalationConfig.Levels) {
		// No more escalation levels, let it expire naturally
		return nil
	}

	// Get the escalation level config
	levelConfig := escalationConfig.Levels[nextLevel-1]

	// Update the request with new escalation level and approver role
	// Use locking to prevent concurrent escalation in multi-pod deployments
	now := time.Now()
	previousApproverID := request.CurrentApproverID
	previousApproverRole := request.CurrentApproverRole

	escalated, err := j.repo.EscalateRequestWithLock(ctx, request.ID, request.EscalationLevel, repository.EscalationUpdate{
		EscalationLevel:     nextLevel,
		EscalatedAt:         &now,
		EscalatedFromID:     previousApproverID,
		CurrentApproverRole: levelConfig.EscalateToRole,
	})
	if err != nil {
		return err
	}

	// If not escalated, another instance already processed this request
	if !escalated {
		j.logger.Debugf("Request %s already escalated by another instance, skipping", request.ID)
		return nil
	}

	// Create audit log entry
	j.createEscalationAuditLog(ctx, request, previousApproverRole, levelConfig.EscalateToRole, nextLevel)

	// Publish escalation event
	j.publishEscalationEvent(ctx, request, levelConfig.EscalateToRole, nextLevel)

	return nil
}

// expireTimedOutRequests marks requests as expired if they've exceeded their timeout
func (j *EscalationJob) expireTimedOutRequests(ctx context.Context) {
	expired, err := j.repo.ExpireTimedOutRequests(ctx)
	if err != nil {
		j.logger.Errorf("Failed to expire timed out requests: %v", err)
		return
	}

	if expired > 0 {
		j.logger.Infof("Expired %d timed out approval requests", expired)
	}
}

// createEscalationAuditLog creates an audit log entry for the escalation
func (j *EscalationJob) createEscalationAuditLog(ctx context.Context, request *models.ApprovalRequest, fromRole, toRole string, level int) {
	metadata := map[string]interface{}{
		"from_role":        fromRole,
		"to_role":          toRole,
		"escalation_level": level,
	}
	metadataJSON, _ := json.Marshal(metadata)

	auditLog := &models.ApprovalAuditLog{
		RequestID: request.ID,
		TenantID:  request.TenantID,
		EventType: models.AuditEventEscalated,
		Metadata:  metadataJSON,
		CreatedAt: time.Now(),
	}

	if err := j.repo.CreateAuditLog(ctx, auditLog); err != nil {
		j.logger.Errorf("Failed to create escalation audit log: %v", err)
	}
}

// publishEscalationEvent publishes an escalation event
func (j *EscalationJob) publishEscalationEvent(ctx context.Context, request *models.ApprovalRequest, newRole string, level int) {
	if j.publisher == nil {
		return
	}

	event := events.NewApprovalEvent(events.ApprovalEscalated, request.TenantID)
	event.ApprovalRequestID = request.ID.String()
	event.WorkflowID = request.WorkflowID.String()
	event.RequesterID = request.RequesterID.String()
	event.ActionType = request.ActionType
	event.Status = request.Status
	event.EscalatedTo = newRole
	event.EscalationLevel = level
	event.EscalationReason = "timeout"

	if err := j.publisher.PublishApproval(ctx, event); err != nil {
		j.logger.Errorf("Failed to publish escalation event: %v", err)
	}
}
