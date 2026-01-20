package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"staff-service/internal/clients"
	"staff-service/internal/models"
	"staff-service/internal/repository"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	staffRepo          repository.StaffRepository
	authRepo           repository.AuthRepository
	jwtSecret          []byte
	notificationClient *clients.NotificationClient
	keycloakClient     KeycloakClient // Interface for Keycloak operations
	tenantClient       *clients.TenantClient
}

// KeycloakClient interface for Keycloak admin operations
// This allows for easier testing and decoupling
type KeycloakClient interface {
	CreateUser(ctx context.Context, user auth.UserRepresentation) (string, error)
	SetUserPassword(ctx context.Context, userID string, password string, temporary bool) error
	GetUserByEmail(ctx context.Context, email string) (*auth.UserRepresentation, error)
	UpdateUserAttributes(ctx context.Context, userID string, attributes map[string][]string) error
}

// JWTClaims represents JWT claims
type JWTClaims struct {
	StaffID   string `json:"staff_id"`
	TenantID  string `json:"tenant_id"`
	VendorID  string `json:"vendor_id,omitempty"`
	Email     string `json:"email"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// GoogleTokenInfo represents Google token info response
type GoogleTokenInfo struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"`
	Azp           string `json:"azp"`
	Aud           string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Locale        string `json:"locale"`
	Hd            string `json:"hd"` // Hosted domain (for Google Workspace)
}

// MicrosoftTokenInfo represents Microsoft token info
type MicrosoftTokenInfo struct {
	Oid               string   `json:"oid"`
	Sub               string   `json:"sub"`
	Name              string   `json:"name"`
	Email             string   `json:"email"`
	PreferredUsername string   `json:"preferred_username"`
	Tid               string   `json:"tid"` // Tenant ID
	Groups            []string `json:"groups,omitempty"`
}

// NewAuthHandler creates a new auth handler (without Keycloak - legacy)
func NewAuthHandler(staffRepo repository.StaffRepository, authRepo repository.AuthRepository, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		staffRepo:          staffRepo,
		authRepo:           authRepo,
		jwtSecret:          []byte(jwtSecret),
		notificationClient: clients.NewNotificationClient(),
		keycloakClient:     nil,
		tenantClient:       clients.NewTenantClient(),
	}
}

// NewAuthHandlerWithKeycloak creates a new auth handler with Keycloak integration
func NewAuthHandlerWithKeycloak(staffRepo repository.StaffRepository, authRepo repository.AuthRepository, jwtSecret string, keycloakClient KeycloakClient) *AuthHandler {
	return &AuthHandler{
		staffRepo:          staffRepo,
		authRepo:           authRepo,
		jwtSecret:          []byte(jwtSecret),
		notificationClient: clients.NewNotificationClient(),
		keycloakClient:     keycloakClient,
		tenantClient:       clients.NewTenantClient(),
	}
}

// ===========================================
// Password Authentication
// ===========================================

// Login authenticates a staff member with email and password
func (h *AuthHandler) Login(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_TENANT",
				Message: "Tenant ID is required",
			},
		})
		return
	}

	var req models.StaffLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Success: false,
		Error: models.Error{
			Code:    "KEYCLOAK_REQUIRED",
			Message: "Password login is handled by Keycloak. Please use the Keycloak login flow.",
		},
	})
}

// ===========================================
// SSO Authentication
// ===========================================

// SSOLogin handles SSO login (Google/Microsoft)
func (h *AuthHandler) SSOLogin(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_TENANT",
				Message: "Tenant ID is required",
			},
		})
		return
	}

	var req models.StaffSSOLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Get SSO config
	ssoConfig, err := h.authRepo.GetSSOConfig(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SSO_CONFIG_ERROR",
				Message: "Failed to retrieve SSO configuration",
			},
		})
		return
	}

	var staff *models.Staff
	var providerUserID, providerEmail, providerName, providerAvatar string

	switch strings.ToLower(req.Provider) {
	case "google":
		if !ssoConfig.GoogleEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "SSO_DISABLED",
					Message: "Google SSO is not enabled for this tenant",
				},
			})
			return
		}
		staff, providerUserID, providerEmail, providerName, providerAvatar, err = h.verifyGoogleToken(c, tenantID, req.IDToken, ssoConfig)

	case "microsoft":
		if !ssoConfig.MicrosoftEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "SSO_DISABLED",
					Message: "Microsoft SSO is not enabled for this tenant",
				},
			})
			return
		}
		staff, providerUserID, providerEmail, providerName, providerAvatar, err = h.verifyMicrosoftToken(c, tenantID, req.IDToken, ssoConfig)

	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_PROVIDER",
				Message: "Invalid SSO provider. Supported: google, microsoft",
			},
		})
		return
	}

	if err != nil {
		h.logFailedLogin(c, tenantID, nil, &providerEmail, err.Error())
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SSO_AUTH_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	// Check account status
	if err := h.checkAccountStatus(tenantID, staff); err != nil {
		h.logFailedLogin(c, tenantID, &staff.ID, &providerEmail, err.Error())
		h.handleAccountStatusError(c, err)
		return
	}

	// Update or create OAuth provider link
	existingProvider, _ := h.authRepo.GetOAuthProvider(tenantID, staff.ID, req.Provider)
	if existingProvider == nil {
		provider := &models.StaffOAuthProvider{
			TenantID:       tenantID,
			StaffID:        staff.ID,
			Provider:       strings.ToLower(req.Provider),
			ProviderUserID: providerUserID,
			ProviderEmail:  &providerEmail,
			ProviderName:   &providerName,
			ProviderAvatar: &providerAvatar,
			IsPrimary:      true,
		}
		if req.AccessToken != nil {
			provider.AccessToken = req.AccessToken
		}
		_ = h.authRepo.LinkOAuthProvider(provider)
	} else {
		now := time.Now()
		_ = h.authRepo.UpdateOAuthProviderLastUsed(existingProvider.ID)
		if req.AccessToken != nil {
			_ = h.authRepo.UpdateOAuthTokens(existingProvider.ID, req.AccessToken, nil, &now)
		}
	}

	// Create session
	response, err := h.createSessionAndTokens(c, tenantID, staff, ssoConfig, req.DeviceFingerprint, req.DeviceName, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SESSION_CREATE_FAILED",
				Message: "Failed to create session",
			},
		})
		return
	}

	// Log successful login
	h.logSuccessfulLogin(c, tenantID, staff.ID, req.Provider)

	// Update last login
	_ = h.staffRepo.UpdateLoginInfo(tenantID, staff.ID, time.Now(), c.ClientIP())

	c.JSON(http.StatusOK, response)
}

// GetSSOConfig returns SSO configuration for a tenant (public info only)
func (h *AuthHandler) GetSSOConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	config, err := h.authRepo.GetSSOConfig(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CONFIG_ERROR",
				Message: "Failed to retrieve SSO configuration",
			},
		})
		return
	}

	// Return only public info
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"googleEnabled":       config.GoogleEnabled,
			"microsoftEnabled":    config.MicrosoftEnabled,
			"passwordAuthEnabled": config.AllowPasswordAuth,
			"enforceSSO":          config.EnforceSSO,
		},
	})
}

// UpdateSSOConfig updates SSO configuration (admin only)
func (h *AuthHandler) UpdateSSOConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	updatedBy := c.GetString("staff_id")

	var req models.SSOConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Get existing config
	existing, _ := h.authRepo.GetSSOConfig(tenantID)
	if existing == nil || existing.ID == uuid.Nil {
		// Create new config
		config := &models.TenantSSOConfig{
			TenantID:  tenantID,
			CreatedBy: &updatedBy,
			UpdatedBy: &updatedBy,
		}
		if err := h.authRepo.CreateSSOConfig(config); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "CREATE_FAILED",
					Message: "Failed to create SSO configuration",
				},
			})
			return
		}
	}

	if err := h.authRepo.UpdateSSOConfig(tenantID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update SSO configuration",
			},
		})
		return
	}

	// Get updated config
	config, _ := h.authRepo.GetSSOConfig(tenantID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// ===========================================
// Token Management
// ===========================================

// RefreshToken refreshes access token using refresh token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req models.TokenRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Get session by refresh token
	session, err := h.authRepo.GetSessionByRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_TOKEN",
				Message: "Invalid or expired refresh token",
			},
		})
		return
	}

	// Get staff
	staff, err := h.staffRepo.GetByID(session.TenantID, session.StaffID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "STAFF_NOT_FOUND",
				Message: "Staff member not found",
			},
		})
		return
	}

	// Check account status
	if err := h.checkAccountStatus(session.TenantID, staff); err != nil {
		_ = h.authRepo.RevokeSession(session.ID, "account_status_invalid")
		h.handleAccountStatusError(c, err)
		return
	}

	// Get SSO config for token duration
	ssoConfig, _ := h.authRepo.GetSSOConfig(session.TenantID)

	// Generate new tokens
	accessToken, refreshToken, accessExp, refreshExp, err := h.generateTokens(session.TenantID, staff, session.ID.String(), ssoConfig, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TOKEN_GENERATION_FAILED",
				Message: "Failed to generate new tokens",
			},
		})
		return
	}

	// Update session with new tokens
	session.AccessToken = accessToken
	session.RefreshToken = &refreshToken
	session.AccessTokenExpiresAt = accessExp
	session.RefreshTokenExpiresAt = &refreshExp
	session.LastActivityAt = time.Now()

	// Update in database (simplified - in production, use a proper update method)
	_ = h.authRepo.UpdateSessionActivity(session.ID)

	c.JSON(http.StatusOK, models.TokenRefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExp,
		TokenType:    "Bearer",
	})
}

// Logout revokes the current session
func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_SESSION",
				Message: "Session ID is required",
			},
		})
		return
	}

	sid, err := uuid.Parse(sessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_SESSION",
				Message: "Invalid session ID",
			},
		})
		return
	}

	if err := h.authRepo.RevokeSession(sid, "user_logout"); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LOGOUT_FAILED",
				Message: "Failed to logout",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully logged out",
	})
}

// LogoutAll revokes all sessions except current
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.GetString("staff_id")
	sessionID := c.GetString("session_id")

	sid, err := uuid.Parse(staffID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID",
			},
		})
		return
	}

	var exceptSession *uuid.UUID
	if sessionID != "" {
		if parsed, err := uuid.Parse(sessionID); err == nil {
			exceptSession = &parsed
		}
	}

	if err := h.authRepo.RevokeAllSessions(tenantID, sid, exceptSession, "logout_all"); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LOGOUT_FAILED",
				Message: "Failed to logout from all devices",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully logged out from all devices",
	})
}

// ===========================================
// Session Management
// ===========================================

// GetSessions returns all active sessions for a staff member
func (h *AuthHandler) GetSessions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.GetString("staff_id")

	sid, err := uuid.Parse(staffID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID",
			},
		})
		return
	}

	sessions, err := h.authRepo.GetActiveSessions(tenantID, sid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch sessions",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.StaffSessionListResponse{
		Success: true,
		Data:    sessions,
		Total:   len(sessions),
	})
}

// RevokeSession revokes a specific session
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.GetString("staff_id")
	sessionIDStr := c.Param("sessionId")

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_SESSION_ID",
				Message: "Invalid session ID",
			},
		})
		return
	}

	// Verify session belongs to the staff member
	session, err := h.authRepo.GetSessionByID(tenantID, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SESSION_NOT_FOUND",
				Message: "Session not found",
			},
		})
		return
	}

	if session.StaffID.String() != staffID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UNAUTHORIZED",
				Message: "You can only revoke your own sessions",
			},
		})
		return
	}

	if err := h.authRepo.RevokeSession(sessionID, "user_revoked"); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REVOKE_FAILED",
				Message: "Failed to revoke session",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session revoked successfully",
	})
}

// ===========================================
// Password Management
// ===========================================

// RequestPasswordReset initiates password reset flow
func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Success: false,
		Error: models.Error{
			Code:    "KEYCLOAK_REQUIRED",
			Message: "Password reset is handled by Keycloak. Please use the Keycloak reset flow.",
		},
	})
}

// ResetPassword resets password using reset token
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Success: false,
		Error: models.Error{
			Code:    "KEYCLOAK_REQUIRED",
			Message: "Password reset is handled by Keycloak. Please use the Keycloak reset flow.",
		},
	})
}

// ChangePassword changes password for authenticated user
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Success: false,
		Error: models.Error{
			Code:    "KEYCLOAK_REQUIRED",
			Message: "Password changes are handled by Keycloak. Please use the Keycloak account settings.",
		},
	})
}

// ===========================================
// Invitation Management
// ===========================================

// CreateInvitation creates a staff invitation
func (h *AuthHandler) CreateInvitation(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	senderID := c.GetString("staff_id")

	var req models.StaffInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Get the staff member to invite
	staff, err := h.staffRepo.GetByID(tenantID, req.StaffID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "STAFF_NOT_FOUND",
				Message: "Staff member not found",
			},
		})
		return
	}

	// Check if already activated
	if staff.AccountStatus != nil && *staff.AccountStatus == models.AccountStatusActive {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ALREADY_ACTIVATED",
				Message: "Staff member is already activated",
			},
		})
		return
	}

	// Set expiration
	expiresIn := 72 * time.Hour
	if req.ExpiresInHours != nil {
		expiresIn = time.Duration(*req.ExpiresInHours) * time.Hour
	}

	// Get SSO config to determine available auth methods
	ssoConfig, _ := h.authRepo.GetSSOConfig(tenantID)
	authOptions := make([]models.StaffAuthMethod, 0)
	if len(req.AuthMethodOptions) > 0 {
		authOptions = req.AuthMethodOptions
	} else {
		// Default based on SSO config
		if ssoConfig != nil && ssoConfig.AllowPasswordAuth {
			authOptions = append(authOptions, models.AuthMethodPassword)
		}
		if ssoConfig != nil && ssoConfig.GoogleEnabled {
			authOptions = append(authOptions, models.AuthMethodGoogleSSO)
		}
		if ssoConfig != nil && ssoConfig.MicrosoftEnabled {
			authOptions = append(authOptions, models.AuthMethodMicrosoftSSO)
		}
	}

	// Convert auth options to JSONArray
	authOptionsJSONArray := make(models.JSONArray, len(authOptions))
	for i, opt := range authOptions {
		authOptionsJSONArray[i] = string(opt)
	}

	senderUUID, _ := uuid.Parse(senderID)
	invitation := &models.StaffInvitation{
		TenantID:          tenantID,
		StaffID:           req.StaffID,
		InvitationType:    "email",
		AuthMethodOptions: &authOptionsJSONArray,
		SentToEmail:       &staff.Email,
		ExpiresAt:         time.Now().Add(expiresIn),
		SentBy:            &senderUUID,
		CustomMessage:     req.CustomMessage,
	}

	if err := h.authRepo.CreateInvitation(invitation); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATE_FAILED",
				Message: "Failed to create invitation",
			},
		})
		return
	}

	// Generate activation token
	token, err := h.authRepo.GenerateActivationToken(tenantID, req.StaffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TOKEN_GENERATION_FAILED",
				Message: "Failed to generate activation token",
			},
		})
		return
	}

	// Update staff status
	_ = h.authRepo.UpdateAccountStatus(tenantID, req.StaffID, models.AccountStatusPendingActivation)

	// Mark invitation as sent and send email
	if req.SendEmail {
		_ = h.authRepo.MarkInvitationSent(invitation.ID)

		// Send invitation email via notification service
		if h.notificationClient != nil {
			go func() {
				// Build activation link - use request URL if provided, otherwise fall back to env or default
				var activationBaseURL string
				if req.ActivationBaseURL != nil && *req.ActivationBaseURL != "" {
					activationBaseURL = *req.ActivationBaseURL
				} else if envHost := os.Getenv("ADMIN_HOST"); envHost != "" {
					activationBaseURL = envHost
				} else {
					// This should not happen in production - frontend should always send the URL
					log.Printf("[AUTH] Warning: No activation base URL provided, using default")
					activationBaseURL = "https://admin.tesserix.app"
				}
				activationLink := fmt.Sprintf("%s/activate?token=%s", activationBaseURL, token)

				// Get sender info for inviter name
				inviterName := "Your administrator"
				if senderUUID, err := uuid.Parse(senderID); err == nil {
					if sender, err := h.staffRepo.GetByID(tenantID, senderUUID); err == nil {
						inviterName = fmt.Sprintf("%s %s", sender.FirstName, sender.LastName)
					}
				}

				// Get business name from request, required for proper email content
				businessName := "Your Store"
				if req.BusinessName != nil && *req.BusinessName != "" {
					businessName = *req.BusinessName
				}

				notification := &clients.StaffInvitationNotification{
					TenantID:       tenantID,
					StaffID:        req.StaffID.String(),
					StaffEmail:     staff.Email,
					StaffName:      fmt.Sprintf("%s %s", staff.FirstName, staff.LastName),
					Role:           string(staff.Role),
					InviterName:    inviterName,
					InviterID:      senderID,
					BusinessName:   businessName,
					ActivationLink: activationLink,
				}
				if err := h.notificationClient.SendStaffInvitation(context.Background(), notification); err != nil {
					log.Printf("[AUTH] Failed to send invitation email to %s: %v", staff.Email, err)
				}
			}()
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"invitationId":    invitation.ID,
			"invitationToken": invitation.InvitationToken,
			"activationToken": token,
			"expiresAt":       invitation.ExpiresAt,
		},
	})
}

// VerifyInvitation verifies an invitation token (supports both invitation_token and activation_token)
func (h *AuthHandler) VerifyInvitation(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_TOKEN",
				Message: "Token is required",
			},
		})
		return
	}

	// Try to find by invitation_token first
	invitation, err := h.authRepo.GetInvitationByToken(token)
	if err != nil {
		// If not found by invitation_token, try to find staff by activation_token
		staff, staffErr := h.authRepo.VerifyActivationToken(token)
		if staffErr != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_TOKEN",
					Message: "Invalid or expired token",
				},
			})
			return
		}

		// Found staff by activation_token, get their invitation for auth method options
		staffInvitation, _ := h.authRepo.GetInvitationByStaffID(staff.TenantID, staff.ID)

		// Get SSO config
		ssoConfig, _ := h.authRepo.GetSSOConfig(staff.TenantID)

		// Parse auth method options from invitation if available
		var authOptions []models.StaffAuthMethod
		if staffInvitation != nil && staffInvitation.AuthMethodOptions != nil {
			for _, opt := range *staffInvitation.AuthMethodOptions {
				if optStr, ok := opt.(string); ok {
					authOptions = append(authOptions, models.StaffAuthMethod(optStr))
				}
			}

			// Mark invitation as opened
			_ = h.authRepo.MarkInvitationOpened(staffInvitation.ID)
		}

		// If no auth options from invitation, provide defaults based on SSO config
		if len(authOptions) == 0 {
			if ssoConfig == nil || ssoConfig.AllowPasswordAuth {
				authOptions = append(authOptions, models.AuthMethodPassword)
			}
			if ssoConfig != nil && ssoConfig.GoogleEnabled {
				authOptions = append(authOptions, models.AuthMethodGoogleSSO)
			}
			if ssoConfig != nil && ssoConfig.MicrosoftEnabled {
				authOptions = append(authOptions, models.AuthMethodMicrosoftSSO)
			}
		}

		c.JSON(http.StatusOK, models.InvitationVerifyResponse{
			Valid:               true,
			Staff:               staff,
			ActivationToken:     staff.ActivationToken, // Return token for use in activation
			AuthMethodOptions:   authOptions,
			ExpiresAt:           staff.ActivationTokenExpiresAt,
			GoogleEnabled:       ssoConfig != nil && ssoConfig.GoogleEnabled,
			MicrosoftEnabled:    ssoConfig != nil && ssoConfig.MicrosoftEnabled,
			PasswordAuthEnabled: ssoConfig == nil || ssoConfig.AllowPasswordAuth,
		})
		return
	}

	// Mark as opened
	_ = h.authRepo.MarkInvitationOpened(invitation.ID)

	// Get SSO config
	ssoConfig, _ := h.authRepo.GetSSOConfig(invitation.TenantID)

	// Parse auth method options from JSONArray
	var authOptions []models.StaffAuthMethod
	if invitation.AuthMethodOptions != nil {
		for _, opt := range *invitation.AuthMethodOptions {
			if optStr, ok := opt.(string); ok {
				authOptions = append(authOptions, models.StaffAuthMethod(optStr))
			}
		}
	}

	c.JSON(http.StatusOK, models.InvitationVerifyResponse{
		Valid:               true,
		Staff:               invitation.Staff,
		ActivationToken:     invitation.Staff.ActivationToken, // Return token for use in activation
		AuthMethodOptions:   authOptions,
		ExpiresAt:           &invitation.ExpiresAt,
		GoogleEnabled:       ssoConfig != nil && ssoConfig.GoogleEnabled,
		MicrosoftEnabled:    ssoConfig != nil && ssoConfig.MicrosoftEnabled,
		PasswordAuthEnabled: ssoConfig == nil || ssoConfig.AllowPasswordAuth,
	})
}

// ActivateAccount activates a staff account
func (h *AuthHandler) ActivateAccount(c *gin.Context) {
	var req models.StaffActivationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Verify activation token
	staff, err := h.authRepo.VerifyActivationToken(req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_TOKEN",
				Message: "Invalid or expired activation token",
			},
		})
		return
	}

	tenantID := staff.TenantID

	// Get SSO config
	ssoConfig, _ := h.authRepo.GetSSOConfig(tenantID)

	// Track keycloak user ID for activation - will be set based on auth method
	var keycloakUserID string

	// FIX-HIGH-005: Validate auth method against tenant SSO policy
	// Previously, activation didn't check if the auth method was allowed
	switch req.AuthMethod {
	case models.AuthMethodPassword:
		if ssoConfig != nil && !ssoConfig.AllowPasswordAuth {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "AUTH_METHOD_DISABLED",
					Message: "Password authentication is not enabled for this organization. Please use SSO.",
				},
			})
			return
		}
	case models.AuthMethodGoogleSSO:
		if ssoConfig == nil || !ssoConfig.GoogleEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "AUTH_METHOD_DISABLED",
					Message: "Google SSO is not enabled for this organization.",
				},
			})
			return
		}
	case models.AuthMethodMicrosoftSSO:
		if ssoConfig == nil || !ssoConfig.MicrosoftEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "AUTH_METHOD_DISABLED",
					Message: "Microsoft SSO is not enabled for this organization.",
				},
			})
			return
		}
	}

	// Process based on auth method
	switch req.AuthMethod {
	case models.AuthMethodPassword:
		if req.Password == nil || req.ConfirmPassword == nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "MISSING_PASSWORD",
					Message: "Password is required for password authentication",
				},
			})
			return
		}
		if *req.Password != *req.ConfirmPassword {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "PASSWORD_MISMATCH",
					Message: "Passwords do not match",
				},
			})
			return
		}
		if err := validatePasswordStrength(*req.Password); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "WEAK_PASSWORD",
					Message: err.Error(),
				},
			})
			return
		}
		// Set password in Keycloak - Keycloak is required for password auth
		if h.keycloakClient == nil {
			log.Printf("[ActivateAccount] ERROR: Keycloak not configured - cannot set password for %s", staff.Email)
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "KEYCLOAK_UNAVAILABLE",
					Message: "Identity provider not configured. Please contact your administrator.",
				},
			})
			return
		}

		// Get or create Keycloak user
		if staff.KeycloakUserID != nil && *staff.KeycloakUserID != "" {
			keycloakUserID = *staff.KeycloakUserID
		} else {
			// First check if user already exists in Keycloak
			existingUser, err := h.keycloakClient.GetUserByEmail(c.Request.Context(), staff.Email)
			if err != nil {
				log.Printf("[ActivateAccount] Error checking for existing Keycloak user: %v", err)
			}

			if existingUser != nil && existingUser.ID != "" {
				// User already exists in Keycloak, use their ID
				keycloakUserID = existingUser.ID
				log.Printf("[ActivateAccount] Found existing Keycloak user for %s: %s", staff.Email, keycloakUserID)
			} else {
				// Create Keycloak user
				userRep := auth.UserRepresentation{
					Username:      staff.Email,
					Email:         staff.Email,
					EmailVerified: true,
					Enabled:       true,
					FirstName:     staff.FirstName,
					LastName:      staff.LastName,
				}
				newUserID, err := h.keycloakClient.CreateUser(c.Request.Context(), userRep)
				if err != nil {
					log.Printf("[ActivateAccount] Failed to create Keycloak user for %s: %v", staff.Email, err)
					c.JSON(http.StatusInternalServerError, models.ErrorResponse{
						Success: false,
						Error: models.Error{
							Code:    "USER_CREATE_FAILED",
							Message: "Failed to create user in identity provider",
						},
					})
					return
				}
				keycloakUserID = newUserID
				log.Printf("[ActivateAccount] Created Keycloak user for %s: %s", staff.Email, keycloakUserID)
			}
			// keycloak_user_id will be stored atomically in ActivateAccount below
		}

		// Set user attributes in Keycloak for JWT claims (staff_id, tenant_id, vendor_id)
		// These attributes are extracted by Istio and forwarded to backend services
		keycloakAttrs := map[string][]string{
			"staff_id":  {staff.ID.String()},
			"tenant_id": {tenantID},
		}
		if staff.VendorID != nil && *staff.VendorID != "" {
			keycloakAttrs["vendor_id"] = []string{*staff.VendorID}
		}
		if err := h.keycloakClient.UpdateUserAttributes(c.Request.Context(), keycloakUserID, keycloakAttrs); err != nil {
			log.Printf("[ActivateAccount] Warning: Failed to set Keycloak user attributes for %s: %v", staff.Email, err)
			// Don't fail activation - attributes can be synced later via backfill
		} else {
			log.Printf("[ActivateAccount] Keycloak user attributes set for %s (staff_id=%s, tenant_id=%s)", staff.Email, staff.ID, tenantID)
		}

		// Set password in Keycloak
		if err := h.keycloakClient.SetUserPassword(c.Request.Context(), keycloakUserID, *req.Password, false); err != nil {
			log.Printf("[ActivateAccount] Failed to set Keycloak password for %s: %v", staff.Email, err)
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "PASSWORD_SET_FAILED",
					Message: "Failed to set password in identity provider",
				},
			})
			return
		}
		log.Printf("[ActivateAccount] Password set in Keycloak for user %s", staff.Email)

	case models.AuthMethodGoogleSSO:
		if req.GoogleIDToken == nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "MISSING_TOKEN",
					Message: "Google ID token is required",
				},
			})
			return
		}
		// Verify Google token and link account
		_, providerUserID, providerEmail, providerName, providerAvatar, err := h.verifyGoogleToken(c, tenantID, *req.GoogleIDToken, ssoConfig)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "SSO_VERIFICATION_FAILED",
					Message: err.Error(),
				},
			})
			return
		}
		// Link OAuth provider
		provider := &models.StaffOAuthProvider{
			TenantID:       tenantID,
			StaffID:        staff.ID,
			Provider:       "google",
			ProviderUserID: providerUserID,
			ProviderEmail:  &providerEmail,
			ProviderName:   &providerName,
			ProviderAvatar: &providerAvatar,
			IsPrimary:      true,
		}
		_ = h.authRepo.LinkOAuthProvider(provider)

		// Create/update Keycloak user for SSO users to enable JWT claims extraction by Istio
		if h.keycloakClient != nil {
			kcUserID, err := h.ensureKeycloakUserWithAttributes(c.Request.Context(), tenantID, staff)
			if err != nil {
				log.Printf("[ActivateAccount] Warning: Failed to sync Keycloak user for Google SSO %s: %v", staff.Email, err)
			} else if kcUserID != "" {
				keycloakUserID = kcUserID
			}
		}

	case models.AuthMethodMicrosoftSSO:
		if req.MicrosoftIDToken == nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "MISSING_TOKEN",
					Message: "Microsoft ID token is required",
				},
			})
			return
		}
		// Verify Microsoft token and link account
		_, providerUserID, providerEmail, providerName, providerAvatar, err := h.verifyMicrosoftToken(c, tenantID, *req.MicrosoftIDToken, ssoConfig)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "SSO_VERIFICATION_FAILED",
					Message: err.Error(),
				},
			})
			return
		}
		// Link OAuth provider
		provider := &models.StaffOAuthProvider{
			TenantID:       tenantID,
			StaffID:        staff.ID,
			Provider:       "microsoft",
			ProviderUserID: providerUserID,
			ProviderEmail:  &providerEmail,
			ProviderName:   &providerName,
			ProviderAvatar: &providerAvatar,
			IsPrimary:      true,
		}
		_ = h.authRepo.LinkOAuthProvider(provider)

		// Create/update Keycloak user for SSO users to enable JWT claims extraction by Istio
		if h.keycloakClient != nil {
			kcUserID, err := h.ensureKeycloakUserWithAttributes(c.Request.Context(), tenantID, staff)
			if err != nil {
				log.Printf("[ActivateAccount] Warning: Failed to sync Keycloak user for Microsoft SSO %s: %v", staff.Email, err)
			} else if kcUserID != "" {
				keycloakUserID = kcUserID
			}
		}

	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_AUTH_METHOD",
				Message: "Invalid authentication method",
			},
		})
		return
	}

	// Activate account (includes storing keycloak_user_id atomically)
	var keycloakUserIDPtr *string
	if keycloakUserID != "" {
		keycloakUserIDPtr = &keycloakUserID
	}
	if err := h.authRepo.ActivateAccount(tenantID, staff.ID, req.AuthMethod, keycloakUserIDPtr); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACTIVATION_FAILED",
				Message: "Failed to activate account",
			},
		})
		return
	}

	// Mark invitation as accepted
	invitation, _ := h.authRepo.GetInvitationByStaffID(tenantID, staff.ID)
	if invitation != nil {
		_ = h.authRepo.MarkInvitationAccepted(invitation.ID)
	}

	// Refresh staff data
	staff, _ = h.staffRepo.GetByID(tenantID, staff.ID)

	// Create session and generate tokens
	response, err := h.createSessionAndTokens(c, tenantID, staff, ssoConfig, req.DeviceFingerprint, req.DeviceName, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SESSION_CREATE_FAILED",
				Message: "Failed to create session",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.StaffActivationResponse{
		Success:      true,
		Staff:        staff,
		AccessToken:  &response.AccessToken,
		RefreshToken: &response.RefreshToken,
		ExpiresAt:    &response.ExpiresAt,
		Message:      "Account activated successfully",
	})
}

// ResendInvitation resends a staff invitation
func (h *AuthHandler) ResendInvitation(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	invitationIDStr := c.Param("id")

	invitationID, err := uuid.Parse(invitationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid invitation ID",
			},
		})
		return
	}

	// FIX-HIGH-004: Get the invitation by ID first to get staff info for token regeneration
	invitation, err := h.authRepo.GetInvitationByID(invitationID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVITATION_NOT_FOUND",
				Message: "Invitation not found",
			},
		})
		return
	}

	// Check invitation status - cannot resend accepted or revoked invitations
	if invitation.Status == models.InvitationStatusAccepted {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVITATION_ACCEPTED",
				Message: "Cannot resend an already accepted invitation",
			},
		})
		return
	}
	if invitation.Status == models.InvitationStatusRevoked {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVITATION_REVOKED",
				Message: "Cannot resend a revoked invitation",
			},
		})
		return
	}

	// Update invitation (generates new invitation token and extends expiration)
	if err := h.authRepo.ResendInvitation(invitationID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESEND_FAILED",
				Message: "Failed to resend invitation",
			},
		})
		return
	}

	// FIX-HIGH-004: Regenerate activation token (this was broken - was calling GetInvitationByToken(""))
	activationToken, err := h.authRepo.GenerateActivationToken(tenantID, invitation.StaffID)
	if err != nil {
		log.Printf("[AUTH] Failed to regenerate activation token for staff %s: %v", invitation.StaffID, err)
		// Don't fail the request - invitation was already updated
	}

	// Send the invitation email with the new activation token
	if h.notificationClient != nil && invitation.Staff != nil {
		go func() {
			// Get activation base URL from environment
			var activationBaseURL string
			if envHost := os.Getenv("ADMIN_HOST"); envHost != "" {
				activationBaseURL = envHost
			} else {
				activationBaseURL = "https://admin.tesserix.app"
			}
			activationLink := fmt.Sprintf("%s/activate?token=%s", activationBaseURL, activationToken)

			notification := &clients.StaffInvitationNotification{
				TenantID:       tenantID,
				StaffID:        invitation.StaffID.String(),
				StaffEmail:     invitation.Staff.Email,
				StaffName:      fmt.Sprintf("%s %s", invitation.Staff.FirstName, invitation.Staff.LastName),
				Role:           string(invitation.Staff.Role),
				InviterName:    "Your administrator",
				ActivationLink: activationLink,
			}
			if err := h.notificationClient.SendStaffInvitation(context.Background(), notification); err != nil {
				log.Printf("[AUTH] Failed to send resent invitation email to %s: %v", invitation.Staff.Email, err)
			} else {
				log.Printf("[AUTH] Resent invitation email to %s", invitation.Staff.Email)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Invitation resent successfully",
	})
}

// GetPendingInvitations returns all pending invitations
func (h *AuthHandler) GetPendingInvitations(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	invitations, err := h.authRepo.GetPendingInvitations(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch invitations",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    invitations,
		"total":   len(invitations),
	})
}

// RevokeInvitation revokes a staff invitation
func (h *AuthHandler) RevokeInvitation(c *gin.Context) {
	invitationIDStr := c.Param("id")

	invitationID, err := uuid.Parse(invitationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid invitation ID",
			},
		})
		return
	}

	if err := h.authRepo.RevokeInvitation(invitationID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REVOKE_FAILED",
				Message: "Failed to revoke invitation",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Invitation revoked successfully",
	})
}

// ===========================================
// Login Audit
// ===========================================

// GetLoginAudit returns login audit history
func (h *AuthHandler) GetLoginAudit(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffIDStr := c.Query("staffId")
	page := getIntParam(c, "page", 1)
	limit := getIntParam(c, "limit", 20)

	var staffID *uuid.UUID
	if staffIDStr != "" {
		if parsed, err := uuid.Parse(staffIDStr); err == nil {
			staffID = &parsed
		}
	}

	audits, pagination, err := h.authRepo.GetLoginAuditHistory(tenantID, staffID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch login audit",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       audits,
		"pagination": pagination,
	})
}

// ===========================================
// OAuth Provider Management
// ===========================================

// GetLinkedProviders returns linked OAuth providers for a staff member
func (h *AuthHandler) GetLinkedProviders(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.GetString("staff_id")

	sid, err := uuid.Parse(staffID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID",
			},
		})
		return
	}

	providers, err := h.authRepo.GetLinkedProviders(tenantID, sid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch linked providers",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    providers,
	})
}

// UnlinkProvider unlinks an OAuth provider from a staff account
func (h *AuthHandler) UnlinkProvider(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.GetString("staff_id")
	provider := c.Param("provider")

	sid, err := uuid.Parse(staffID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID",
			},
		})
		return
	}

	// Check if staff has another auth method
	staff, err := h.staffRepo.GetByID(tenantID, sid)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "STAFF_NOT_FOUND",
				Message: "Staff member not found",
			},
		})
		return
	}

	// Ensure they have password or another provider
	if staff.PasswordHash == nil || *staff.PasswordHash == "" {
		providers, _ := h.authRepo.GetLinkedProviders(tenantID, sid)
		if len(providers) <= 1 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "CANNOT_UNLINK",
					Message: "Cannot unlink last authentication method",
				},
			})
			return
		}
	}

	if err := h.authRepo.UnlinkOAuthProvider(tenantID, sid, provider); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UNLINK_FAILED",
				Message: "Failed to unlink provider",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Provider unlinked successfully",
	})
}

// ===========================================
// Helper Functions
// ===========================================

func (h *AuthHandler) checkAccountStatus(tenantID string, staff *models.Staff) error {
	if staff.AccountStatus == nil {
		return nil
	}

	switch *staff.AccountStatus {
	case models.AccountStatusPendingActivation:
		return repository.ErrAccountPendingActivation
	case models.AccountStatusSuspended:
		return repository.ErrAccountSuspended
	case models.AccountStatusLocked:
		return repository.ErrAccountLocked
	case models.AccountStatusDeactivated:
		return repository.ErrAccountDeactivated
	}

	return nil
}

func (h *AuthHandler) handleAccountStatusError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrAccountPendingActivation):
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACCOUNT_PENDING",
				Message: "Account is pending activation. Please check your email for the invitation link.",
			},
		})
	case errors.Is(err, repository.ErrAccountSuspended):
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACCOUNT_SUSPENDED",
				Message: "Account has been suspended. Please contact your administrator.",
			},
		})
	case errors.Is(err, repository.ErrAccountLocked):
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACCOUNT_LOCKED",
				Message: "Account is locked due to too many failed login attempts.",
			},
		})
	case errors.Is(err, repository.ErrAccountDeactivated):
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACCOUNT_DEACTIVATED",
				Message: "Account has been deactivated.",
			},
		})
	default:
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACCOUNT_ERROR",
				Message: err.Error(),
			},
		})
	}
}

func (h *AuthHandler) handleFailedLogin(c *gin.Context, tenantID string, staff *models.Staff) {
	h.logFailedLogin(c, tenantID, &staff.ID, &staff.Email, "invalid_password")

	// Increment failed attempts and check for lock
	// This would typically be done in the repository
	failedCount, _ := h.authRepo.GetFailedLoginCount(tenantID, staff.ID, time.Now().Add(-1*time.Hour))

	if failedCount >= 5 {
		lockUntil := time.Now().Add(30 * time.Minute)
		_ = h.authRepo.LockAccount(tenantID, staff.ID, &lockUntil)

		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ACCOUNT_LOCKED",
				Message: "Account has been locked due to too many failed login attempts. Try again in 30 minutes.",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, models.ErrorResponse{
		Success: false,
		Error: models.Error{
			Code:    "INVALID_CREDENTIALS",
			Message: "Invalid email or password",
		},
	})
}

func (h *AuthHandler) logFailedLogin(c *gin.Context, tenantID string, staffID *uuid.UUID, email *string, reason string) {
	authMethod := "password"
	audit := &models.StaffLoginAudit{
		TenantID:          tenantID,
		StaffID:           staffID,
		Email:             email,
		AuthMethod:        &authMethod,
		Success:           false,
		FailureReason:     &reason,
		IPAddress:         stringPtr(c.ClientIP()),
		UserAgent:         stringPtr(c.GetHeader("User-Agent")),
		DeviceFingerprint: stringPtr(c.GetHeader("X-Device-Fingerprint")),
	}
	_ = h.authRepo.LogLoginAttempt(audit)
}

func (h *AuthHandler) logSuccessfulLogin(c *gin.Context, tenantID string, staffID uuid.UUID, authMethod string) {
	audit := &models.StaffLoginAudit{
		TenantID:          tenantID,
		StaffID:           &staffID,
		AuthMethod:        &authMethod,
		Success:           true,
		IPAddress:         stringPtr(c.ClientIP()),
		UserAgent:         stringPtr(c.GetHeader("User-Agent")),
		DeviceFingerprint: stringPtr(c.GetHeader("X-Device-Fingerprint")),
	}
	_ = h.authRepo.LogLoginAttempt(audit)
}

func (h *AuthHandler) createSessionAndTokens(c *gin.Context, tenantID string, staff *models.Staff, ssoConfig *models.TenantSSOConfig, deviceFingerprint, deviceName *string, rememberMe bool) (*models.StaffLoginResponse, error) {
	sessionID := uuid.New()

	// Generate tokens
	accessToken, refreshToken, accessExp, refreshExp, err := h.generateTokens(tenantID, staff, sessionID.String(), ssoConfig, rememberMe)
	if err != nil {
		return nil, err
	}

	// Create session
	session := &models.StaffSession{
		ID:                    sessionID,
		TenantID:              tenantID,
		StaffID:               staff.ID,
		AccessToken:           accessToken,
		RefreshToken:          &refreshToken,
		AccessTokenExpiresAt:  accessExp,
		RefreshTokenExpiresAt: &refreshExp,
		DeviceFingerprint:     deviceFingerprint,
		DeviceName:            deviceName,
		IPAddress:             stringPtr(c.ClientIP()),
		UserAgent:             stringPtr(c.GetHeader("User-Agent")),
		IsActive:              true,
	}

	// Parse user agent for device info (simplified)
	userAgent := c.GetHeader("User-Agent")
	if strings.Contains(strings.ToLower(userAgent), "mobile") {
		session.DeviceType = stringPtr("mobile")
	} else if strings.Contains(strings.ToLower(userAgent), "tablet") {
		session.DeviceType = stringPtr("tablet")
	} else {
		session.DeviceType = stringPtr("desktop")
	}

	if err := h.authRepo.CreateSession(session); err != nil {
		return nil, err
	}

	mustReset := staff.MustResetPassword != nil && *staff.MustResetPassword

	return &models.StaffLoginResponse{
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		ExpiresAt:         accessExp,
		TokenType:         "Bearer",
		Staff:             staff,
		MustResetPassword: mustReset,
		SessionID:         sessionID.String(),
	}, nil
}

func (h *AuthHandler) generateTokens(tenantID string, staff *models.Staff, sessionID string, ssoConfig *models.TenantSSOConfig, rememberMe bool) (string, string, time.Time, time.Time, error) {
	// Access token duration
	accessDuration := 8 * time.Hour
	if ssoConfig != nil && ssoConfig.SessionDurationHours > 0 {
		accessDuration = time.Duration(ssoConfig.SessionDurationHours) * time.Hour
	}

	// Refresh token duration
	refreshDuration := 30 * 24 * time.Hour
	if ssoConfig != nil && ssoConfig.RefreshTokenDays > 0 {
		refreshDuration = time.Duration(ssoConfig.RefreshTokenDays) * 24 * time.Hour
	}
	if rememberMe {
		refreshDuration = 90 * 24 * time.Hour // 90 days for remember me
	}

	accessExp := time.Now().Add(accessDuration)
	refreshExp := time.Now().Add(refreshDuration)

	// Generate access token
	accessClaims := JWTClaims{
		StaffID:   staff.ID.String(),
		TenantID:  tenantID,
		Email:     staff.Email,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "staff-service",
			Subject:   staff.ID.String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(h.jwtSecret)
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}

	// Generate refresh token (simple secure random)
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	refreshTokenString := base64.URLEncoding.EncodeToString(refreshBytes)

	return accessTokenString, refreshTokenString, accessExp, refreshExp, nil
}

func (h *AuthHandler) verifyGoogleToken(c *gin.Context, tenantID string, idToken string, ssoConfig *models.TenantSSOConfig) (*models.Staff, string, string, string, string, error) {
	// Verify token with Google
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken))
	if err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to verify Google token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", "", "", fmt.Errorf("invalid Google token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to read Google response")
	}

	var tokenInfo GoogleTokenInfo
	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to parse Google response")
	}

	// Verify audience
	if ssoConfig.GoogleClientID != nil && tokenInfo.Aud != *ssoConfig.GoogleClientID {
		return nil, "", "", "", "", fmt.Errorf("invalid Google client ID")
	}

	// Check allowed domains
	if ssoConfig.GoogleAllowedDomains != nil && len(*ssoConfig.GoogleAllowedDomains) > 0 {
		domainAllowed := false
		for _, domain := range *ssoConfig.GoogleAllowedDomains {
			if domainStr, ok := domain.(string); ok && tokenInfo.Hd == domainStr {
				domainAllowed = true
				break
			}
		}
		if !domainAllowed {
			return nil, "", "", "", "", fmt.Errorf("email domain not allowed")
		}
	}

	// Find or auto-provision staff
	staff, err := h.staffRepo.GetByEmail(tenantID, tokenInfo.Email)
	if err != nil {
		// Check for existing OAuth link
		provider, _ := h.authRepo.GetOAuthProviderByProviderID(tenantID, "google", tokenInfo.Sub)
		if provider != nil && provider.Staff != nil {
			staff = provider.Staff
		} else if ssoConfig.AutoProvisionUsers {
			// Auto-provision new user
			// This would require additional logic to create a new staff member
			return nil, "", "", "", "", fmt.Errorf("user not found and auto-provisioning is not fully implemented")
		} else {
			return nil, "", "", "", "", fmt.Errorf("no account found for this email")
		}
	}

	// SECURITY: Validate staff member is authorized for admin portal access
	// Staff must be either:
	// 1. A store owner (created via tenant onboarding)
	// 2. Explicitly invited by an owner/admin (InvitedBy is set)
	if err := validateStaffAdminAccess(staff); err != nil {
		return nil, "", "", "", "", err
	}

	return staff, tokenInfo.Sub, tokenInfo.Email, tokenInfo.Name, tokenInfo.Picture, nil
}

func (h *AuthHandler) verifyMicrosoftToken(c *gin.Context, tenantID string, idToken string, ssoConfig *models.TenantSSOConfig) (*models.Staff, string, string, string, string, error) {
	// For Microsoft, we need to decode and verify the JWT
	// In production, you would verify the signature against Microsoft's public keys

	// Parse the token (without verification for now - in production, use proper verification)
	token, _, err := new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, "", "", "", "", fmt.Errorf("failed to parse Microsoft token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, "", "", "", "", fmt.Errorf("invalid Microsoft token claims")
	}

	// Extract claims
	oid, _ := claims["oid"].(string)
	email, _ := claims["email"].(string)
	if email == "" {
		email, _ = claims["preferred_username"].(string)
	}
	name, _ := claims["name"].(string)
	tid, _ := claims["tid"].(string)

	// Verify tenant ID
	if ssoConfig.MicrosoftTenantID != nil && tid != *ssoConfig.MicrosoftTenantID {
		return nil, "", "", "", "", fmt.Errorf("invalid Microsoft tenant")
	}

	// Check allowed groups (if configured)
	if ssoConfig.MicrosoftAllowedGroups != nil && len(*ssoConfig.MicrosoftAllowedGroups) > 0 {
		// Would need to check group membership via Microsoft Graph API
		// Group membership verification is typically done by calling Microsoft Graph API
		// with the access token to get user's group memberships
		// For now, skip this check - implement full Graph API integration for production
		_ = ssoConfig.MicrosoftAllowedGroups // Placeholder to avoid unused variable
	}

	// Find or auto-provision staff
	staff, err := h.staffRepo.GetByEmail(tenantID, email)
	if err != nil {
		// Check for existing OAuth link
		provider, _ := h.authRepo.GetOAuthProviderByProviderID(tenantID, "microsoft", oid)
		if provider != nil && provider.Staff != nil {
			staff = provider.Staff
		} else if ssoConfig.AutoProvisionUsers {
			return nil, "", "", "", "", fmt.Errorf("user not found and auto-provisioning is not fully implemented")
		} else {
			return nil, "", "", "", "", fmt.Errorf("no account found for this email")
		}
	}

	// SECURITY: Validate staff member is authorized for admin portal access
	if err := validateStaffAdminAccess(staff); err != nil {
		return nil, "", "", "", "", err
	}

	return staff, oid, email, name, "", nil
}

func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?", char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetStaffTenants returns all tenants a staff member has access to (for login tenant lookup)
// POST /api/v1/auth/tenants
// This is called by tenant-service to include staff in the login tenant lookup
func (h *AuthHandler) GetStaffTenants(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Get all staff records for this email across all tenants
	staffList, err := h.staffRepo.GetAllByEmail(req.Email)
	if err != nil {
		log.Printf("[AUTH] Error getting staff tenants for email %s: %v", req.Email, err)
		// Don't reveal error details - just return empty list
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"tenants": []interface{}{},
				"count":   0,
			},
		})
		return
	}

	// Build tenant list - only include staff who are authorized for admin portal access
	// NOTE: Do NOT call tenant-service here as this endpoint is called BY tenant-service,
	// which would create a circular dependency and cause timeouts.
	// Tenant enrichment (slug, name) is done by tenant-service after receiving this response.
	tenants := make([]gin.H, 0, len(staffList))
	for _, staff := range staffList {
		// SECURITY: Filter out staff who are not authorized for admin portal access
		// This prevents customers from seeing tenants in the admin portal tenant switcher
		if err := validateStaffAdminAccess(&staff); err != nil {
			log.Printf("[AUTH] Filtering out staff %s from tenant list: %v", staff.Email, err)
			continue
		}

		tenants = append(tenants, gin.H{
			"id":           staff.TenantID,
			"staff_id":     staff.ID,
			"role":         staff.Role,
			"vendor_id":    staff.VendorID,
			"display_name": fmt.Sprintf("%s %s", staff.FirstName, staff.LastName),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tenants": tenants,
			"count":   len(tenants),
		},
	})
}

// BackfillKeycloakAttributes syncs staff_id, tenant_id, and vendor_id attributes
// to Keycloak for all activated staff members. This is an admin-only endpoint
// used for one-time migration when enabling Istio JWT claim extraction.
// POST /api/v1/auth/admin/backfill-keycloak
func (h *AuthHandler) BackfillKeycloakAttributes(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_TENANT",
				Message: "Tenant ID is required",
			},
		})
		return
	}

	if h.keycloakClient == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "KEYCLOAK_UNAVAILABLE",
				Message: "Keycloak client not configured",
			},
		})
		return
	}

	// Get all activated staff for this tenant
	staffList, err := h.staffRepo.GetActivatedStaff(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch staff list",
			},
		})
		return
	}

	var synced, failed, skipped int
	var errors []string

	for _, staff := range staffList {
		// Skip if no Keycloak user ID
		if staff.KeycloakUserID == nil || *staff.KeycloakUserID == "" {
			skipped++
			continue
		}

		// Set attributes in Keycloak
		keycloakAttrs := map[string][]string{
			"staff_id":  {staff.ID.String()},
			"tenant_id": {tenantID},
		}
		if staff.VendorID != nil && *staff.VendorID != "" {
			keycloakAttrs["vendor_id"] = []string{*staff.VendorID}
		}

		if err := h.keycloakClient.UpdateUserAttributes(c.Request.Context(), *staff.KeycloakUserID, keycloakAttrs); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %v", staff.Email, err))
			log.Printf("[BackfillKeycloakAttributes] Failed to sync %s: %v", staff.Email, err)
		} else {
			synced++
			log.Printf("[BackfillKeycloakAttributes] Synced %s (staff_id=%s)", staff.Email, staff.ID)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":   len(staffList),
			"synced":  synced,
			"failed":  failed,
			"skipped": skipped,
			"errors":  errors,
		},
		"message": fmt.Sprintf("Backfill completed: %d synced, %d failed, %d skipped (no Keycloak ID)", synced, failed, skipped),
	})
}

// ValidateStaffCredentials - DEPRECATED
// POST /api/v1/auth/validate
// This endpoint is deprecated - staff authentication should go through Keycloak.
// Returns 410 Gone to prevent fallback to DB password validation.
func (h *AuthHandler) ValidateStaffCredentials(c *gin.Context) {
	log.Printf("[ValidateStaffCredentials] DEPRECATED: This endpoint should not be called. Staff auth must use Keycloak.")
	c.JSON(http.StatusGone, gin.H{
		"success": false,
		"error":   "This authentication endpoint has been deprecated.",
		"message": "Staff credentials are stored in Keycloak. Use the standard Keycloak authentication flow.",
		"code":    "ENDPOINT_DEPRECATED",
	})
}

// validateStaffAdminAccess validates that a staff member is authorized to access the admin portal.
// Access is granted only if:
// 1. Account status is active
// 2. Staff is marked as active (isActive = true)
// 3. Staff is either:
//   - A store owner (created via tenant onboarding)
//   - Explicitly invited by an owner/admin (InvitedBy is set and invitation was accepted)
//
// This prevents customers who registered on the storefront from accessing the admin portal
// even if they happen to have a staff record with matching email.
func validateStaffAdminAccess(staff *models.Staff) error {
	if staff == nil {
		return fmt.Errorf("no staff account found")
	}

	// Check account status is active
	if staff.AccountStatus != nil && *staff.AccountStatus != models.AccountStatusActive {
		return fmt.Errorf("account is not active (status: %s)", *staff.AccountStatus)
	}

	// Check staff is active
	if !staff.IsActive {
		return fmt.Errorf("staff account is deactivated")
	}

	// Valid admin portal roles (excludes customer-like roles)
	validAdminRoles := map[models.StaffRole]bool{
		models.RoleStoreOwner:       true,
		models.RoleStoreAdmin:       true,
		models.RoleStoreManager:     true,
		models.RoleInventoryManager: true,
		models.RoleMarketingManager: true,
		models.RoleOrderManager:     true,
		models.RoleCustomerSupport:  true,
		models.RoleSuperAdmin:       true,
		models.RoleAdmin:            true,
		models.RoleManager:          true,
		models.RoleSeniorEmployee:   true,
		models.RoleEmployee:         true,
		models.RoleViewer:           true,
	}

	// Verify staff has a valid admin role
	if !validAdminRoles[staff.Role] {
		return fmt.Errorf("insufficient permissions: role '%s' is not authorized for admin portal access", staff.Role)
	}

	// For store owners, they are created during tenant onboarding - always allowed
	if staff.Role == models.RoleStoreOwner || staff.Role == models.RoleSuperAdmin {
		return nil
	}

	// For other staff, they must have been explicitly invited by an owner/admin
	// This ensures customers can't access admin just because their email matches
	if staff.InvitedBy == nil {
		return fmt.Errorf("staff member was not properly invited - admin portal access denied")
	}

	// Optionally check invitation was accepted (for extra security)
	// If InvitationAcceptedAt is nil but InvitedBy is set, the staff was invited but hasn't completed onboarding
	// We allow this for SSO as the SSO login itself can serve as acceptance

	return nil
}

func getIntParam(c *gin.Context, key string, defaultVal int) int {
	if val := c.Query(key); val != "" {
		if intVal, err := parseInt(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// ensureKeycloakUserWithAttributes creates or updates a Keycloak user with staff attributes.
// This is necessary for Istio JWT claim extraction to work properly - Istio extracts claims
// from the JWT and forwards them as headers (x-jwt-claim-staff-id, x-jwt-claim-vendor-id, etc.)
// to backend services for RBAC authorization.
// Returns the keycloak user ID so it can be stored atomically during activation.
func (h *AuthHandler) ensureKeycloakUserWithAttributes(ctx context.Context, tenantID string, staff *models.Staff) (string, error) {
	if h.keycloakClient == nil {
		return "", fmt.Errorf("keycloak client not configured")
	}

	var keycloakUserID string

	// Check if staff already has a Keycloak user ID
	if staff.KeycloakUserID != nil && *staff.KeycloakUserID != "" {
		keycloakUserID = *staff.KeycloakUserID
	} else {
		// Check if user exists in Keycloak by email
		existingUser, err := h.keycloakClient.GetUserByEmail(ctx, staff.Email)
		if err == nil && existingUser != nil && existingUser.ID != "" {
			keycloakUserID = existingUser.ID
			log.Printf("[ensureKeycloakUserWithAttributes] Found existing Keycloak user for %s: %s", staff.Email, keycloakUserID)
		} else {
			// Create new Keycloak user
			userRep := auth.UserRepresentation{
				Username:      staff.Email,
				Email:         staff.Email,
				EmailVerified: true,
				Enabled:       true,
				FirstName:     staff.FirstName,
				LastName:      staff.LastName,
			}
			newUserID, err := h.keycloakClient.CreateUser(ctx, userRep)
			if err != nil {
				return "", fmt.Errorf("failed to create Keycloak user: %w", err)
			}
			keycloakUserID = newUserID
			log.Printf("[ensureKeycloakUserWithAttributes] Created Keycloak user for %s: %s", staff.Email, keycloakUserID)
		}
		// Note: keycloak_user_id will be stored atomically in ActivateAccount
	}

	// Set user attributes in Keycloak for JWT claims
	// These are extracted by Istio RequestAuthentication and forwarded as headers
	keycloakAttrs := map[string][]string{
		"staff_id":  {staff.ID.String()},
		"tenant_id": {tenantID},
	}
	if staff.VendorID != nil && *staff.VendorID != "" {
		keycloakAttrs["vendor_id"] = []string{*staff.VendorID}
	}

	if err := h.keycloakClient.UpdateUserAttributes(ctx, keycloakUserID, keycloakAttrs); err != nil {
		return "", fmt.Errorf("failed to update Keycloak user attributes: %w", err)
	}

	log.Printf("[ensureKeycloakUserWithAttributes] Keycloak attributes set for %s (staff_id=%s, tenant_id=%s)", staff.Email, staff.ID, tenantID)
	return keycloakUserID, nil
}
