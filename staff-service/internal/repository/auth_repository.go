package repository

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"staff-service/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Common errors
var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrAccountLocked            = errors.New("account is locked")
	ErrAccountSuspended         = errors.New("account is suspended")
	ErrAccountDeactivated       = errors.New("account is deactivated")
	ErrAccountPendingActivation = errors.New("account is pending activation")
	ErrInvalidToken             = errors.New("invalid or expired token")
	ErrSessionNotFound          = errors.New("session not found")
	ErrSessionExpired           = errors.New("session expired")
	ErrSessionRevoked           = errors.New("session has been revoked")
	ErrPasswordMismatch         = errors.New("passwords do not match")
	ErrPasswordReused           = errors.New("password has been used recently")
	ErrInvitationExpired        = errors.New("invitation has expired")
	ErrInvitationAlreadyUsed    = errors.New("invitation has already been used")
	ErrMaxSessionsReached       = errors.New("maximum sessions reached")
	ErrSSONotEnabled            = errors.New("SSO is not enabled for this tenant")
	ErrSSOProviderNotLinked     = errors.New("SSO provider not linked to account")
)

// AuthRepository defines authentication operations
type AuthRepository interface {
	// Password management
	SetPassword(tenantID string, staffID uuid.UUID, password string) error
	VerifyPassword(tenantID string, staffID uuid.UUID, password string) (bool, error)
	GeneratePasswordResetToken(tenantID string, staffID uuid.UUID) (string, error)
	VerifyPasswordResetToken(tenantID string, token string) (*models.Staff, error)
	ResetPasswordWithToken(tenantID string, token string, newPassword string) error
	GetPasswordHistory(staffID uuid.UUID, limit int) ([]string, error)
	AddToPasswordHistory(staffID uuid.UUID, passwordHash string) error
	UpdateMustResetPassword(tenantID string, staffID uuid.UUID, mustReset bool) error

	// Session management
	CreateSession(session *models.StaffSession) error
	GetSessionByID(tenantID string, sessionID uuid.UUID) (*models.StaffSession, error)
	GetSessionByAccessToken(accessToken string) (*models.StaffSession, error)
	GetSessionByRefreshToken(refreshToken string) (*models.StaffSession, error)
	GetActiveSessions(tenantID string, staffID uuid.UUID) ([]models.StaffSession, error)
	UpdateSessionActivity(sessionID uuid.UUID) error
	RevokeSession(sessionID uuid.UUID, reason string) error
	RevokeAllSessions(tenantID string, staffID uuid.UUID, exceptSessionID *uuid.UUID, reason string) error
	CleanupExpiredSessions() (int64, error)
	CountActiveSessions(tenantID string, staffID uuid.UUID) (int64, error)

	// OAuth provider management
	LinkOAuthProvider(provider *models.StaffOAuthProvider) error
	GetOAuthProvider(tenantID string, staffID uuid.UUID, providerName string) (*models.StaffOAuthProvider, error)
	GetOAuthProviderByProviderID(tenantID string, providerName, providerUserID string) (*models.StaffOAuthProvider, error)
	GetLinkedProviders(tenantID string, staffID uuid.UUID) ([]models.StaffOAuthProvider, error)
	UpdateOAuthTokens(providerID uuid.UUID, accessToken, refreshToken *string, expiresAt *time.Time) error
	UnlinkOAuthProvider(tenantID string, staffID uuid.UUID, providerName string) error
	UpdateOAuthProviderLastUsed(providerID uuid.UUID) error

	// Invitation management
	CreateInvitation(invitation *models.StaffInvitation) error
	GetInvitationByToken(token string) (*models.StaffInvitation, error)
	GetInvitationByStaffID(tenantID string, staffID uuid.UUID) (*models.StaffInvitation, error)
	GetPendingInvitations(tenantID string) ([]models.StaffInvitation, error)
	UpdateInvitationStatus(invitationID uuid.UUID, status models.InvitationStatus) error
	MarkInvitationSent(invitationID uuid.UUID) error
	MarkInvitationOpened(invitationID uuid.UUID) error
	MarkInvitationAccepted(invitationID uuid.UUID) error
	RevokeInvitation(invitationID uuid.UUID) error
	ResendInvitation(invitationID uuid.UUID) error
	CleanupExpiredInvitations() (int64, error)

	// Activation management
	GenerateActivationToken(tenantID string, staffID uuid.UUID) (string, error)
	VerifyActivationToken(token string) (*models.Staff, error)
	ActivateAccount(tenantID string, staffID uuid.UUID, authMethod models.StaffAuthMethod) error
	UpdateAccountStatus(tenantID string, staffID uuid.UUID, status models.StaffAccountStatus) error

	// Login audit
	LogLoginAttempt(audit *models.StaffLoginAudit) error
	GetRecentLoginAttempts(tenantID string, staffID uuid.UUID, since time.Time) ([]models.StaffLoginAudit, error)
	GetFailedLoginCount(tenantID string, staffID uuid.UUID, since time.Time) (int64, error)
	GetLoginAuditHistory(tenantID string, staffID *uuid.UUID, page, limit int) ([]models.StaffLoginAudit, *models.PaginationInfo, error)

	// Account locking
	LockAccount(tenantID string, staffID uuid.UUID, until *time.Time) error
	UnlockAccount(tenantID string, staffID uuid.UUID) error
	IsAccountLocked(tenantID string, staffID uuid.UUID) (bool, *time.Time, error)
	UnlockExpiredAccounts() (int64, error)

	// Tenant SSO configuration
	GetSSOConfig(tenantID string) (*models.TenantSSOConfig, error)
	CreateSSOConfig(config *models.TenantSSOConfig) error
	UpdateSSOConfig(tenantID string, updates *models.SSOConfigUpdateRequest) error
	DeleteSSOConfig(tenantID string) error

	// Token generation utilities
	GenerateSecureToken(length int) (string, error)
	HashToken(token string) string

	// Cleanup operations
	CleanupStaffRecords(tenantID string, staffID uuid.UUID) error
}

type authRepository struct {
	db *gorm.DB
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

// ===========================================
// Password Management
// ===========================================

func (r *authRepository) SetPassword(tenantID string, staffID uuid.UUID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	hashStr := string(hash)

	// Add current password to history before updating
	var currentHash string
	r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Pluck("password_hash", &currentHash)

	if currentHash != "" {
		if err := r.AddToPasswordHistory(staffID, currentHash); err != nil {
			return err
		}
	}

	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(map[string]interface{}{
			"password_hash":        hashStr,
			"last_password_change": time.Now(),
			"must_reset_password":  false,
			"updated_at":           time.Now(),
		}).Error
}

func (r *authRepository) VerifyPassword(tenantID string, staffID uuid.UUID, password string) (bool, error) {
	var staff models.Staff
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, staffID).First(&staff).Error; err != nil {
		return false, err
	}

	if staff.PasswordHash == nil {
		return false, ErrInvalidCredentials
	}

	err := bcrypt.CompareHashAndPassword([]byte(*staff.PasswordHash), []byte(password))
	return err == nil, nil
}

func (r *authRepository) GeneratePasswordResetToken(tenantID string, staffID uuid.UUID) (string, error) {
	token, err := r.GenerateSecureToken(32)
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(24 * time.Hour) // 24 hours validity

	err = r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(map[string]interface{}{
			"password_reset_token":            r.HashToken(token),
			"password_reset_token_expires_at": expiresAt,
			"updated_at":                      time.Now(),
		}).Error

	if err != nil {
		return "", err
	}

	return token, nil
}

func (r *authRepository) VerifyPasswordResetToken(tenantID string, token string) (*models.Staff, error) {
	hashedToken := r.HashToken(token)

	var staff models.Staff
	err := r.db.Where("tenant_id = ? AND password_reset_token = ? AND password_reset_token_expires_at > ?",
		tenantID, hashedToken, time.Now()).First(&staff).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	return &staff, nil
}

func (r *authRepository) ResetPasswordWithToken(tenantID string, token string, newPassword string) error {
	staff, err := r.VerifyPasswordResetToken(tenantID, token)
	if err != nil {
		return err
	}

	// Check password history
	history, _ := r.GetPasswordHistory(staff.ID, 5)
	newHash, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)

	for _, oldHash := range history {
		if bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(newPassword)) == nil {
			return ErrPasswordReused
		}
	}

	// Set new password and clear reset token
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staff.ID).
		Updates(map[string]interface{}{
			"password_hash":                   string(newHash),
			"password_reset_token":            nil,
			"password_reset_token_expires_at": nil,
			"last_password_change":            time.Now(),
			"must_reset_password":             false,
			"updated_at":                      time.Now(),
		}).Error
}

func (r *authRepository) GetPasswordHistory(staffID uuid.UUID, limit int) ([]string, error) {
	var history []models.StaffPasswordHistory
	err := r.db.Where("staff_id = ?", staffID).
		Order("created_at DESC").
		Limit(limit).
		Find(&history).Error

	if err != nil {
		return nil, err
	}

	hashes := make([]string, len(history))
	for i, h := range history {
		hashes[i] = h.PasswordHash
	}

	return hashes, nil
}

func (r *authRepository) AddToPasswordHistory(staffID uuid.UUID, passwordHash string) error {
	history := &models.StaffPasswordHistory{
		ID:           uuid.New(),
		StaffID:      staffID,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}

	return r.db.Create(history).Error
}

func (r *authRepository) UpdateMustResetPassword(tenantID string, staffID uuid.UUID, mustReset bool) error {
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Update("must_reset_password", mustReset).Error
}

// ===========================================
// Session Management
// ===========================================

func (r *authRepository) CreateSession(session *models.StaffSession) error {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	session.CreatedAt = time.Now()
	session.LastActivityAt = time.Now()

	return r.db.Create(session).Error
}

func (r *authRepository) GetSessionByID(tenantID string, sessionID uuid.UUID) (*models.StaffSession, error) {
	var session models.StaffSession
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, sessionID).
		Preload("Staff").
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if !session.IsActive {
		return nil, ErrSessionRevoked
	}

	if time.Now().After(session.AccessTokenExpiresAt) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

func (r *authRepository) GetSessionByAccessToken(accessToken string) (*models.StaffSession, error) {
	var session models.StaffSession
	err := r.db.Where("access_token = ?", accessToken).
		Preload("Staff").
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if !session.IsActive {
		return nil, ErrSessionRevoked
	}

	if time.Now().After(session.AccessTokenExpiresAt) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

func (r *authRepository) GetSessionByRefreshToken(refreshToken string) (*models.StaffSession, error) {
	var session models.StaffSession
	err := r.db.Where("refresh_token = ?", refreshToken).
		Preload("Staff").
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if !session.IsActive {
		return nil, ErrSessionRevoked
	}

	if session.RefreshTokenExpiresAt != nil && time.Now().After(*session.RefreshTokenExpiresAt) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

func (r *authRepository) GetActiveSessions(tenantID string, staffID uuid.UUID) ([]models.StaffSession, error) {
	var sessions []models.StaffSession
	err := r.db.Where("tenant_id = ? AND staff_id = ? AND is_active = ?", tenantID, staffID, true).
		Where("access_token_expires_at > ?", time.Now()).
		Order("last_activity_at DESC").
		Find(&sessions).Error

	return sessions, err
}

func (r *authRepository) UpdateSessionActivity(sessionID uuid.UUID) error {
	return r.db.Model(&models.StaffSession{}).
		Where("id = ?", sessionID).
		Update("last_activity_at", time.Now()).Error
}

func (r *authRepository) RevokeSession(sessionID uuid.UUID, reason string) error {
	now := time.Now()
	return r.db.Model(&models.StaffSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"is_active":      false,
			"revoked_at":     now,
			"revoked_reason": reason,
		}).Error
}

func (r *authRepository) RevokeAllSessions(tenantID string, staffID uuid.UUID, exceptSessionID *uuid.UUID, reason string) error {
	now := time.Now()
	query := r.db.Model(&models.StaffSession{}).
		Where("tenant_id = ? AND staff_id = ? AND is_active = ?", tenantID, staffID, true)

	if exceptSessionID != nil {
		query = query.Where("id != ?", *exceptSessionID)
	}

	return query.Updates(map[string]interface{}{
		"is_active":      false,
		"revoked_at":     now,
		"revoked_reason": reason,
	}).Error
}

func (r *authRepository) CleanupExpiredSessions() (int64, error) {
	result := r.db.Where("access_token_expires_at < ? AND is_active = ?", time.Now(), true).
		Delete(&models.StaffSession{})

	return result.RowsAffected, result.Error
}

func (r *authRepository) CountActiveSessions(tenantID string, staffID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.StaffSession{}).
		Where("tenant_id = ? AND staff_id = ? AND is_active = ?", tenantID, staffID, true).
		Where("access_token_expires_at > ?", time.Now()).
		Count(&count).Error

	return count, err
}

// ===========================================
// OAuth Provider Management
// ===========================================

func (r *authRepository) LinkOAuthProvider(provider *models.StaffOAuthProvider) error {
	if provider.ID == uuid.Nil {
		provider.ID = uuid.New()
	}
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()

	return r.db.Create(provider).Error
}

func (r *authRepository) GetOAuthProvider(tenantID string, staffID uuid.UUID, providerName string) (*models.StaffOAuthProvider, error) {
	var provider models.StaffOAuthProvider
	err := r.db.Where("tenant_id = ? AND staff_id = ? AND provider = ?", tenantID, staffID, providerName).
		First(&provider).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &provider, nil
}

func (r *authRepository) GetOAuthProviderByProviderID(tenantID string, providerName, providerUserID string) (*models.StaffOAuthProvider, error) {
	var provider models.StaffOAuthProvider
	err := r.db.Where("tenant_id = ? AND provider = ? AND provider_user_id = ?", tenantID, providerName, providerUserID).
		Preload("Staff").
		First(&provider).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &provider, nil
}

func (r *authRepository) GetLinkedProviders(tenantID string, staffID uuid.UUID) ([]models.StaffOAuthProvider, error) {
	var providers []models.StaffOAuthProvider
	err := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID).
		Order("created_at ASC").
		Find(&providers).Error

	return providers, err
}

func (r *authRepository) UpdateOAuthTokens(providerID uuid.UUID, accessToken, refreshToken *string, expiresAt *time.Time) error {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}

	if accessToken != nil {
		updates["access_token"] = *accessToken
	}
	if refreshToken != nil {
		updates["refresh_token"] = *refreshToken
	}
	if expiresAt != nil {
		updates["token_expires_at"] = *expiresAt
	}

	return r.db.Model(&models.StaffOAuthProvider{}).
		Where("id = ?", providerID).
		Updates(updates).Error
}

func (r *authRepository) UnlinkOAuthProvider(tenantID string, staffID uuid.UUID, providerName string) error {
	return r.db.Where("tenant_id = ? AND staff_id = ? AND provider = ?", tenantID, staffID, providerName).
		Delete(&models.StaffOAuthProvider{}).Error
}

func (r *authRepository) UpdateOAuthProviderLastUsed(providerID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.StaffOAuthProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"last_used_at": now,
			"updated_at":   now,
		}).Error
}

// ===========================================
// Invitation Management
// ===========================================

func (r *authRepository) CreateInvitation(invitation *models.StaffInvitation) error {
	if invitation.ID == uuid.Nil {
		invitation.ID = uuid.New()
	}

	// Generate secure invitation token
	token, err := r.GenerateSecureToken(32)
	if err != nil {
		return err
	}
	invitation.InvitationToken = token

	// Set default expiration (72 hours)
	if invitation.ExpiresAt.IsZero() {
		invitation.ExpiresAt = time.Now().Add(72 * time.Hour)
	}

	invitation.Status = models.InvitationStatusPending
	invitation.CreatedAt = time.Now()
	invitation.UpdatedAt = time.Now()

	return r.db.Create(invitation).Error
}

func (r *authRepository) GetInvitationByToken(token string) (*models.StaffInvitation, error) {
	var invitation models.StaffInvitation
	err := r.db.Where("invitation_token = ?", token).
		Preload("Staff").
		First(&invitation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	if time.Now().After(invitation.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	if invitation.Status == models.InvitationStatusAccepted {
		return nil, ErrInvitationAlreadyUsed
	}

	if invitation.Status == models.InvitationStatusRevoked {
		return nil, ErrInvalidToken
	}

	return &invitation, nil
}

func (r *authRepository) GetInvitationByStaffID(tenantID string, staffID uuid.UUID) (*models.StaffInvitation, error) {
	var invitation models.StaffInvitation
	err := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID).
		Order("created_at DESC").
		First(&invitation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &invitation, nil
}

func (r *authRepository) GetPendingInvitations(tenantID string) ([]models.StaffInvitation, error) {
	var invitations []models.StaffInvitation
	err := r.db.Where("tenant_id = ? AND status IN ?", tenantID,
		[]models.InvitationStatus{models.InvitationStatusPending, models.InvitationStatusSent}).
		Where("expires_at > ?", time.Now()).
		Preload("Staff").
		Order("created_at DESC").
		Find(&invitations).Error

	return invitations, err
}

func (r *authRepository) UpdateInvitationStatus(invitationID uuid.UUID, status models.InvitationStatus) error {
	return r.db.Model(&models.StaffInvitation{}).
		Where("id = ?", invitationID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

func (r *authRepository) MarkInvitationSent(invitationID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.StaffInvitation{}).
		Where("id = ?", invitationID).
		Updates(map[string]interface{}{
			"status":       models.InvitationStatusSent,
			"sent_at":      now,
			"last_sent_at": now,
			"send_count":   gorm.Expr("send_count + 1"),
			"updated_at":   now,
		}).Error
}

func (r *authRepository) MarkInvitationOpened(invitationID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.StaffInvitation{}).
		Where("id = ? AND opened_at IS NULL", invitationID).
		Updates(map[string]interface{}{
			"status":     models.InvitationStatusOpened,
			"opened_at":  now,
			"updated_at": now,
		}).Error
}

func (r *authRepository) MarkInvitationAccepted(invitationID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.StaffInvitation{}).
		Where("id = ?", invitationID).
		Updates(map[string]interface{}{
			"status":      models.InvitationStatusAccepted,
			"accepted_at": now,
			"updated_at":  now,
		}).Error
}

func (r *authRepository) RevokeInvitation(invitationID uuid.UUID) error {
	return r.UpdateInvitationStatus(invitationID, models.InvitationStatusRevoked)
}

func (r *authRepository) ResendInvitation(invitationID uuid.UUID) error {
	// Generate new token and extend expiration
	token, err := r.GenerateSecureToken(32)
	if err != nil {
		return err
	}

	now := time.Now()
	return r.db.Model(&models.StaffInvitation{}).
		Where("id = ?", invitationID).
		Updates(map[string]interface{}{
			"invitation_token": token,
			"status":           models.InvitationStatusPending,
			"expires_at":       now.Add(72 * time.Hour),
			"send_count":       gorm.Expr("send_count + 1"),
			"last_sent_at":     now,
			"updated_at":       now,
		}).Error
}

func (r *authRepository) CleanupExpiredInvitations() (int64, error) {
	result := r.db.Model(&models.StaffInvitation{}).
		Where("expires_at < ? AND status IN ?", time.Now(),
			[]models.InvitationStatus{models.InvitationStatusPending, models.InvitationStatusSent}).
		Update("status", models.InvitationStatusExpired)

	return result.RowsAffected, result.Error
}

// ===========================================
// Activation Management
// ===========================================

func (r *authRepository) GenerateActivationToken(tenantID string, staffID uuid.UUID) (string, error) {
	token, err := r.GenerateSecureToken(32)
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(72 * time.Hour) // 72 hours validity

	err = r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(map[string]interface{}{
			"activation_token":            token,
			"activation_token_expires_at": expiresAt,
			"updated_at":                  time.Now(),
		}).Error

	if err != nil {
		return "", err
	}

	return token, nil
}

func (r *authRepository) VerifyActivationToken(token string) (*models.Staff, error) {
	var staff models.Staff
	err := r.db.Where("activation_token = ? AND activation_token_expires_at > ?", token, time.Now()).
		First(&staff).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	return &staff, nil
}

func (r *authRepository) ActivateAccount(tenantID string, staffID uuid.UUID, authMethod models.StaffAuthMethod) error {
	now := time.Now()
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(map[string]interface{}{
			"account_status":              models.AccountStatusActive,
			"auth_method":                 authMethod,
			"activation_token":            nil,
			"activation_token_expires_at": nil,
			"is_email_verified":           true,
			"is_active":                   true, // Set IsActive=true on activation
			"invitation_accepted_at":      now,
			"updated_at":                  now,
		}).Error
}

func (r *authRepository) UpdateAccountStatus(tenantID string, staffID uuid.UUID, status models.StaffAccountStatus) error {
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(map[string]interface{}{
			"account_status": status,
			"updated_at":     time.Now(),
		}).Error
}

// ===========================================
// Login Audit
// ===========================================

func (r *authRepository) LogLoginAttempt(audit *models.StaffLoginAudit) error {
	if audit.ID == uuid.Nil {
		audit.ID = uuid.New()
	}
	audit.AttemptedAt = time.Now()

	return r.db.Create(audit).Error
}

func (r *authRepository) GetRecentLoginAttempts(tenantID string, staffID uuid.UUID, since time.Time) ([]models.StaffLoginAudit, error) {
	var audits []models.StaffLoginAudit
	err := r.db.Where("tenant_id = ? AND staff_id = ? AND attempted_at > ?", tenantID, staffID, since).
		Order("attempted_at DESC").
		Limit(100).
		Find(&audits).Error

	return audits, err
}

func (r *authRepository) GetFailedLoginCount(tenantID string, staffID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.StaffLoginAudit{}).
		Where("tenant_id = ? AND staff_id = ? AND success = ? AND attempted_at > ?",
			tenantID, staffID, false, since).
		Count(&count).Error

	return count, err
}

func (r *authRepository) GetLoginAuditHistory(tenantID string, staffID *uuid.UUID, page, limit int) ([]models.StaffLoginAudit, *models.PaginationInfo, error) {
	var audits []models.StaffLoginAudit
	var total int64

	query := r.db.Model(&models.StaffLoginAudit{}).Where("tenant_id = ?", tenantID)

	if staffID != nil {
		query = query.Where("staff_id = ?", *staffID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Order("attempted_at DESC").
		Find(&audits).Error; err != nil {
		return nil, nil, err
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return audits, pagination, nil
}

// ===========================================
// Account Locking
// ===========================================

func (r *authRepository) LockAccount(tenantID string, staffID uuid.UUID, until *time.Time) error {
	updates := map[string]interface{}{
		"account_status": models.AccountStatusLocked,
		"updated_at":     time.Now(),
	}

	if until != nil {
		updates["account_locked_until"] = *until
	}

	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(updates).Error
}

func (r *authRepository) UnlockAccount(tenantID string, staffID uuid.UUID) error {
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Updates(map[string]interface{}{
			"account_status":        models.AccountStatusActive,
			"account_locked_until":  nil,
			"failed_login_attempts": 0,
			"updated_at":            time.Now(),
		}).Error
}

func (r *authRepository) IsAccountLocked(tenantID string, staffID uuid.UUID) (bool, *time.Time, error) {
	var staff struct {
		AccountStatus      models.StaffAccountStatus
		AccountLockedUntil *time.Time
	}

	err := r.db.Model(&models.Staff{}).
		Select("account_status", "account_locked_until").
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		First(&staff).Error

	if err != nil {
		return false, nil, err
	}

	if staff.AccountStatus == models.AccountStatusLocked {
		// Check if lock has expired
		if staff.AccountLockedUntil != nil && time.Now().After(*staff.AccountLockedUntil) {
			// Auto-unlock
			_ = r.UnlockAccount(tenantID, staffID)
			return false, nil, nil
		}
		return true, staff.AccountLockedUntil, nil
	}

	return false, nil, nil
}

func (r *authRepository) UnlockExpiredAccounts() (int64, error) {
	result := r.db.Model(&models.Staff{}).
		Where("account_status = ? AND account_locked_until IS NOT NULL AND account_locked_until < ?",
			models.AccountStatusLocked, time.Now()).
		Updates(map[string]interface{}{
			"account_status":        models.AccountStatusActive,
			"account_locked_until":  nil,
			"failed_login_attempts": 0,
			"updated_at":            time.Now(),
		})

	return result.RowsAffected, result.Error
}

// ===========================================
// Tenant SSO Configuration
// ===========================================

func (r *authRepository) GetSSOConfig(tenantID string) (*models.TenantSSOConfig, error) {
	var config models.TenantSSOConfig
	err := r.db.Where("tenant_id = ?", tenantID).First(&config).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default config
			return &models.TenantSSOConfig{
				TenantID:             tenantID,
				AllowPasswordAuth:    true,
				SessionDurationHours: 8,
				RefreshTokenDays:     30,
				MaxSessionsPerUser:   5,
			}, nil
		}
		return nil, err
	}

	return &config, nil
}

func (r *authRepository) CreateSSOConfig(config *models.TenantSSOConfig) error {
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	return r.db.Create(config).Error
}

func (r *authRepository) UpdateSSOConfig(tenantID string, updates *models.SSOConfigUpdateRequest) error {
	updateMap := make(map[string]interface{})
	updateMap["updated_at"] = time.Now()

	if updates.GoogleEnabled != nil {
		updateMap["google_enabled"] = *updates.GoogleEnabled
	}
	if updates.GoogleClientID != nil {
		updateMap["google_client_id"] = *updates.GoogleClientID
	}
	if updates.GoogleClientSecret != nil {
		updateMap["google_client_secret"] = *updates.GoogleClientSecret
	}
	if updates.GoogleAllowedDomains != nil {
		updateMap["google_allowed_domains"] = updates.GoogleAllowedDomains
	}
	if updates.MicrosoftEnabled != nil {
		updateMap["microsoft_enabled"] = *updates.MicrosoftEnabled
	}
	if updates.MicrosoftTenantID != nil {
		updateMap["microsoft_tenant_id"] = *updates.MicrosoftTenantID
	}
	if updates.MicrosoftClientID != nil {
		updateMap["microsoft_client_id"] = *updates.MicrosoftClientID
	}
	if updates.MicrosoftClientSecret != nil {
		updateMap["microsoft_client_secret"] = *updates.MicrosoftClientSecret
	}
	if updates.MicrosoftAllowedGroups != nil {
		updateMap["microsoft_allowed_groups"] = updates.MicrosoftAllowedGroups
	}
	if updates.AllowPasswordAuth != nil {
		updateMap["allow_password_auth"] = *updates.AllowPasswordAuth
	}
	if updates.EnforceSSO != nil {
		updateMap["enforce_sso"] = *updates.EnforceSSO
	}
	if updates.AutoProvisionUsers != nil {
		updateMap["auto_provision_users"] = *updates.AutoProvisionUsers
	}
	if updates.DefaultRoleID != nil {
		updateMap["default_role_id"] = *updates.DefaultRoleID
	}
	if updates.SessionDurationHours != nil {
		updateMap["session_duration_hours"] = *updates.SessionDurationHours
	}
	if updates.RefreshTokenDays != nil {
		updateMap["refresh_token_days"] = *updates.RefreshTokenDays
	}
	if updates.MaxSessionsPerUser != nil {
		updateMap["max_sessions_per_user"] = *updates.MaxSessionsPerUser
	}
	if updates.RequireMFA != nil {
		updateMap["require_mfa"] = *updates.RequireMFA
	}

	result := r.db.Model(&models.TenantSSOConfig{}).
		Where("tenant_id = ?", tenantID).
		Updates(updateMap)

	if result.RowsAffected == 0 {
		// Create new config if doesn't exist
		config := &models.TenantSSOConfig{
			ID:       uuid.New(),
			TenantID: tenantID,
		}
		// Apply updates to new config
		return r.db.Create(config).Error
	}

	return result.Error
}

func (r *authRepository) DeleteSSOConfig(tenantID string) error {
	return r.db.Where("tenant_id = ?", tenantID).Delete(&models.TenantSSOConfig{}).Error
}

// ===========================================
// Token Generation Utilities
// ===========================================

func (r *authRepository) GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Use URL-safe base64 encoding
	token := base64.URLEncoding.EncodeToString(bytes)
	// Remove padding
	token = strings.TrimRight(token, "=")

	return token, nil
}

func (r *authRepository) HashToken(token string) string {
	// Use constant-time comparison for security
	// Simple hash for storage - in production, consider using SHA-256
	hash := make([]byte, len(token))
	for i := 0; i < len(token); i++ {
		hash[i] = token[i] ^ 0x5A
	}
	return base64.StdEncoding.EncodeToString(hash)
}

// VerifyTokenHash verifies a token against its hash using constant-time comparison
func VerifyTokenHash(token, hash string) bool {
	expectedHash := (&authRepository{}).HashToken(token)
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(hash)) == 1
}

// Note: StaffPasswordHistory is defined in models/auth.go

// ===========================================
// Cleanup Operations
// ===========================================

// CleanupStaffRecords removes all related records when a staff member is deleted
func (r *authRepository) CleanupStaffRecords(tenantID string, staffID uuid.UUID) error {
	// Delete invitations
	if err := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID).
		Delete(&models.StaffInvitation{}).Error; err != nil {
		return fmt.Errorf("failed to delete invitations: %w", err)
	}

	// Delete sessions
	if err := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID).
		Delete(&models.StaffSession{}).Error; err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}

	// Delete login audit records
	if err := r.db.Where("tenant_id = ? AND staff_id = ?", tenantID, staffID).
		Delete(&models.StaffLoginAudit{}).Error; err != nil {
		return fmt.Errorf("failed to delete login audit: %w", err)
	}

	// Delete password history
	if err := r.db.Where("staff_id = ?", staffID).
		Delete(&models.StaffPasswordHistory{}).Error; err != nil {
		return fmt.Errorf("failed to delete password history: %w", err)
	}

	return nil
}
