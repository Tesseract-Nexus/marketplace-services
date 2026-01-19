package models

import (
	"time"

	"github.com/google/uuid"
)

// Note: StaffAuthMethod and StaffAccountStatus are defined in staff.go

// ===========================================
// Authentication Enums
// ===========================================

// InvitationStatus represents the invitation status
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusSent     InvitationStatus = "sent"
	InvitationStatusOpened   InvitationStatus = "opened"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
	InvitationStatusRevoked  InvitationStatus = "revoked"
)

// Note: Staff authentication fields are now part of the Staff model in staff.go

// ===========================================
// Staff Session Model
// ===========================================

// StaffSession represents an active staff session
type StaffSession struct {
	ID                    uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID              string     `json:"tenantId" gorm:"not null"`
	VendorID              *string    `json:"vendorId,omitempty"`
	StaffID               uuid.UUID  `json:"staffId" gorm:"not null"`
	AccessToken           string     `json:"-" gorm:"not null"`
	RefreshToken          *string    `json:"-"`
	AccessTokenExpiresAt  time.Time  `json:"accessTokenExpiresAt" gorm:"not null"`
	RefreshTokenExpiresAt *time.Time `json:"refreshTokenExpiresAt,omitempty"`
	DeviceFingerprint     *string    `json:"deviceFingerprint,omitempty"`
	DeviceName            *string    `json:"deviceName,omitempty"`
	DeviceType            *string    `json:"deviceType,omitempty"`
	OSName                *string    `json:"osName,omitempty" gorm:"column:os_name"`
	OSVersion             *string    `json:"osVersion,omitempty" gorm:"column:os_version"`
	BrowserName           *string    `json:"browserName,omitempty"`
	BrowserVersion        *string    `json:"browserVersion,omitempty"`
	IPAddress             *string    `json:"ipAddress,omitempty"`
	Location              *string    `json:"location,omitempty"`
	UserAgent             *string    `json:"-"`
	IsActive              bool       `json:"isActive" gorm:"default:true"`
	IsTrusted             bool       `json:"isTrusted" gorm:"default:false"`
	LastActivityAt        time.Time  `json:"lastActivityAt" gorm:"default:now()"`
	CreatedAt             time.Time  `json:"createdAt"`
	RevokedAt             *time.Time `json:"revokedAt,omitempty"`
	RevokedReason         *string    `json:"revokedReason,omitempty"`

	// Relationships
	Staff *Staff `json:"staff,omitempty" gorm:"foreignKey:StaffID"`
}

func (StaffSession) TableName() string {
	return "staff_sessions"
}

// ===========================================
// Staff OAuth Provider Model
// ===========================================

// StaffOAuthProvider represents a linked OAuth provider
type StaffOAuthProvider struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID       string     `json:"tenantId" gorm:"not null"`
	VendorID       *string    `json:"vendorId,omitempty"`
	StaffID        uuid.UUID  `json:"staffId" gorm:"not null"`
	Provider       string     `json:"provider" gorm:"not null"` // google, microsoft, etc.
	ProviderUserID string     `json:"providerUserId" gorm:"not null"`
	ProviderEmail  *string    `json:"providerEmail,omitempty"`
	ProviderName   *string    `json:"providerName,omitempty"`
	ProviderAvatar *string    `json:"providerAvatar,omitempty" gorm:"column:provider_avatar_url"`
	AccessToken    *string    `json:"-"`
	RefreshToken   *string    `json:"-"`
	TokenExpiresAt *time.Time `json:"-"`
	ProfileData    *JSON      `json:"profileData,omitempty" gorm:"type:jsonb"`
	IsPrimary      bool       `json:"isPrimary" gorm:"default:false"`
	LastUsedAt     *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`

	// Relationships
	Staff *Staff `json:"staff,omitempty" gorm:"foreignKey:StaffID"`
}

func (StaffOAuthProvider) TableName() string {
	return "staff_oauth_providers"
}

// ===========================================
// Staff Invitation Model
// ===========================================

// StaffInvitation represents a staff invitation
type StaffInvitation struct {
	ID                uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID          string           `json:"tenantId" gorm:"not null"`
	VendorID          *string          `json:"vendorId,omitempty"`
	StaffID           uuid.UUID        `json:"staffId" gorm:"not null"`
	InvitationToken   string           `json:"-" gorm:"not null;unique"`
	InvitationType    string           `json:"invitationType" gorm:"not null"` // email, sms, link
	AuthMethodOptions *JSONArray       `json:"authMethodOptions,omitempty" gorm:"type:jsonb"`
	SentToEmail       *string          `json:"sentToEmail,omitempty"`
	SentToPhone       *string          `json:"sentToPhone,omitempty"`
	Status            InvitationStatus `json:"status" gorm:"default:'pending'"`
	SentAt            *time.Time       `json:"sentAt,omitempty"`
	OpenedAt          *time.Time       `json:"openedAt,omitempty"`
	AcceptedAt        *time.Time       `json:"acceptedAt,omitempty"`
	ExpiresAt         time.Time        `json:"expiresAt" gorm:"not null"`
	SentBy            *uuid.UUID       `json:"sentBy,omitempty"`
	SendCount         int              `json:"sendCount" gorm:"default:0"`
	LastSentAt        *time.Time       `json:"lastSentAt,omitempty"`
	CustomMessage     *string          `json:"customMessage,omitempty"`
	Metadata          *JSON            `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`

	// Relationships
	Staff  *Staff `json:"staff,omitempty" gorm:"foreignKey:StaffID"`
	Sender *Staff `json:"sender,omitempty" gorm:"foreignKey:SentBy"`
}

func (StaffInvitation) TableName() string {
	return "staff_invitations"
}

// ===========================================
// Login Audit Model
// ===========================================

// StaffLoginAudit represents a login attempt audit record
type StaffLoginAudit struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID          string     `json:"tenantId" gorm:"not null"`
	VendorID          *string    `json:"vendorId,omitempty"`
	StaffID           *uuid.UUID `json:"staffId,omitempty"`
	Email             *string    `json:"email,omitempty"`
	AuthMethod        *string    `json:"authMethod,omitempty"`
	Success           bool       `json:"success" gorm:"not null"`
	FailureReason     *string    `json:"failureReason,omitempty"`
	IPAddress         *string    `json:"ipAddress,omitempty"`
	UserAgent         *string    `json:"userAgent,omitempty"`
	DeviceFingerprint *string    `json:"deviceFingerprint,omitempty"`
	Location          *string    `json:"location,omitempty"`
	RiskScore         *int       `json:"riskScore,omitempty"`
	RiskFactors       *JSON      `json:"riskFactors,omitempty" gorm:"type:jsonb"`
	AttemptedAt       time.Time  `json:"attemptedAt" gorm:"default:now()"`
}

func (StaffLoginAudit) TableName() string {
	return "staff_login_audit"
}

// ===========================================
// Tenant SSO Configuration Model
// ===========================================

// TenantSSOConfig represents SSO configuration for a tenant
type TenantSSOConfig struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID string    `json:"tenantId" gorm:"not null;unique"`

	// Google OAuth configuration
	GoogleEnabled         bool       `json:"googleEnabled" gorm:"default:false"`
	GoogleClientID        *string    `json:"googleClientId,omitempty"`
	GoogleClientSecret    *string    `json:"-"`                                        // Legacy: direct storage
	GoogleClientSecretRef *string    `json:"-" gorm:"column:google_client_secret_ref"` // GCP Secret Manager reference
	GoogleAllowedDomains  *JSONArray `json:"googleAllowedDomains,omitempty" gorm:"type:jsonb"`

	// Microsoft Entra (Azure AD) configuration
	MicrosoftEnabled         bool       `json:"microsoftEnabled" gorm:"default:false"`
	MicrosoftTenantID        *string    `json:"microsoftTenantId,omitempty"`
	MicrosoftClientID        *string    `json:"microsoftClientId,omitempty"`
	MicrosoftClientSecret    *string    `json:"-"`                                           // Legacy: direct storage
	MicrosoftClientSecretRef *string    `json:"-" gorm:"column:microsoft_client_secret_ref"` // GCP Secret Manager reference
	MicrosoftAllowedGroups   *JSONArray `json:"microsoftAllowedGroups,omitempty" gorm:"type:jsonb"`

	// Okta configuration
	OktaEnabled            bool       `json:"oktaEnabled" gorm:"default:false"`
	OktaDomain             *string    `json:"oktaDomain,omitempty"` // e.g., "company.okta.com"
	OktaClientID           *string    `json:"oktaClientId,omitempty"`
	OktaClientSecretRef    *string    `json:"-" gorm:"column:okta_client_secret_ref"`       // GCP Secret Manager reference
	OktaProtocol           *string    `json:"oktaProtocol,omitempty" gorm:"default:'oidc'"` // "oidc" or "saml"
	OktaSAMLMetadataURL    *string    `json:"oktaSamlMetadataUrl,omitempty" gorm:"column:okta_saml_metadata_url"`
	OktaSAMLEntityID       *string    `json:"oktaSamlEntityId,omitempty" gorm:"column:okta_saml_entity_id"`
	OktaSAMLCertificateRef *string    `json:"-" gorm:"column:okta_saml_certificate_ref"` // GCP Secret Manager reference
	OktaAllowedGroups      *JSONArray `json:"oktaAllowedGroups,omitempty" gorm:"type:jsonb"`

	// SCIM 2.0 Provisioning configuration
	SCIMEnabled             bool       `json:"scimEnabled" gorm:"default:false"`
	SCIMEndpoint            *string    `json:"scimEndpoint,omitempty"`
	SCIMBearerTokenRef      *string    `json:"-" gorm:"column:scim_bearer_token_ref"` // GCP Secret Manager reference
	SCIMSyncIntervalMinutes int        `json:"scimSyncIntervalMinutes" gorm:"default:60"`
	SCIMLastSyncAt          *time.Time `json:"scimLastSyncAt,omitempty"`
	SCIMAutoCreateUsers     bool       `json:"scimAutoCreateUsers" gorm:"default:true"`
	SCIMAutoDeactivateUsers bool       `json:"scimAutoDeactivateUsers" gorm:"default:true"`
	SCIMTokenCreatedAt      *time.Time `json:"scimTokenCreatedAt,omitempty"`
	SCIMTokenLastUsedAt     *time.Time `json:"scimTokenLastUsedAt,omitempty"`

	// KeyCloak Identity Provider federation
	KeycloakMicrosoftIdPAlias *string    `json:"keycloakMicrosoftIdpAlias,omitempty" gorm:"column:keycloak_microsoft_idp_alias"`
	KeycloakOktaIdPAlias      *string    `json:"keycloakOktaIdpAlias,omitempty" gorm:"column:keycloak_okta_idp_alias"`
	KeycloakGoogleIdPAlias    *string    `json:"keycloakGoogleIdpAlias,omitempty" gorm:"column:keycloak_google_idp_alias"`
	KeycloakFederationEnabled bool       `json:"keycloakFederationEnabled" gorm:"default:false"`
	KeycloakLastSyncAt        *time.Time `json:"keycloakLastSyncAt,omitempty"`

	// General SSO settings
	AllowPasswordAuth  bool       `json:"allowPasswordAuth" gorm:"default:true"`
	EnforceSSO         bool       `json:"enforceSSO" gorm:"default:false"`
	AutoProvisionUsers bool       `json:"autoProvisionUsers" gorm:"default:false"`
	DefaultRoleID      *uuid.UUID `json:"defaultRoleId,omitempty"`

	// Security settings
	SessionDurationHours int  `json:"sessionDurationHours" gorm:"default:8"`
	RefreshTokenDays     int  `json:"refreshTokenDays" gorm:"default:30"`
	MaxSessionsPerUser   int  `json:"maxSessionsPerUser" gorm:"default:5"`
	RequireMFA           bool `json:"requireMFA" gorm:"default:false"`

	// Audit
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy *string   `json:"createdBy,omitempty"`
	UpdatedBy *string   `json:"updatedBy,omitempty"`
}

func (TenantSSOConfig) TableName() string {
	return "tenant_sso_config"
}

// ===========================================
// SCIM Sync Log Model
// ===========================================

// SCIMSyncLog represents a SCIM provisioning operation audit record
type SCIMSyncLog struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID        string    `json:"tenantId" gorm:"not null"`
	Operation       string    `json:"operation" gorm:"not null"`    // create, update, delete, sync
	ResourceType    string    `json:"resourceType" gorm:"not null"` // User, Group
	ResourceID      *string   `json:"resourceId,omitempty"`
	ExternalID      *string   `json:"externalId,omitempty"` // ID from enterprise IdP
	Success         bool      `json:"success" gorm:"not null"`
	ErrorMessage    *string   `json:"errorMessage,omitempty"`
	UserEmail       *string   `json:"userEmail,omitempty"`
	UserDisplayName *string   `json:"userDisplayName,omitempty"`
	RequestID       *string   `json:"requestId,omitempty"`
	SourceIP        *string   `json:"sourceIp,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (SCIMSyncLog) TableName() string {
	return "scim_sync_log"
}

// ===========================================
// Tenant Secrets Registry Model
// ===========================================

// TenantSecret represents a secret stored in GCP Secret Manager for a tenant
type TenantSecret struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	TenantID         string     `json:"tenantId" gorm:"not null"`
	SecretName       string     `json:"secretName" gorm:"not null"`  // Unique name within tenant
	SecretType       string     `json:"secretType" gorm:"not null"`  // sso, scim, api_key, certificate
	Provider         *string    `json:"provider,omitempty"`          // google, microsoft, okta, scim
	GCPSecretID      string     `json:"gcpSecretId" gorm:"not null"` // Full GCP secret ID
	GCPSecretVersion string     `json:"gcpSecretVersion" gorm:"default:'latest'"`
	Description      *string    `json:"description,omitempty"`
	IsActive         bool       `json:"isActive" gorm:"default:true"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	CreatedBy        *string    `json:"createdBy,omitempty"`
	LastRotatedAt    *time.Time `json:"lastRotatedAt,omitempty"`
	RotationCount    int        `json:"rotationCount" gorm:"default:0"`
}

func (TenantSecret) TableName() string {
	return "tenant_secrets"
}

// ===========================================
// Request/Response Types
// ===========================================

// StaffLoginRequest represents a login request
type StaffLoginRequest struct {
	Email             string  `json:"email" binding:"required,email"`
	Password          string  `json:"password" binding:"required"`
	DeviceFingerprint *string `json:"deviceFingerprint,omitempty"`
	DeviceName        *string `json:"deviceName,omitempty"`
	RememberMe        bool    `json:"rememberMe,omitempty"`
}

// StaffLoginResponse represents a login response
type StaffLoginResponse struct {
	AccessToken       string    `json:"accessToken"`
	RefreshToken      string    `json:"refreshToken,omitempty"`
	ExpiresAt         time.Time `json:"expiresAt"`
	TokenType         string    `json:"tokenType"`
	Staff             *Staff    `json:"staff"`
	MustResetPassword bool      `json:"mustResetPassword"`
	SessionID         string    `json:"sessionId"`
}

// StaffSSOLoginRequest represents an SSO login request
type StaffSSOLoginRequest struct {
	Provider          string  `json:"provider" binding:"required"` // google, microsoft
	IDToken           string  `json:"idToken" binding:"required"`
	AccessToken       *string `json:"accessToken,omitempty"`
	DeviceFingerprint *string `json:"deviceFingerprint,omitempty"`
	DeviceName        *string `json:"deviceName,omitempty"`
}

// StaffPasswordResetRequest represents a password reset request
type StaffPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// StaffPasswordResetConfirmRequest represents password reset confirmation
type StaffPasswordResetConfirmRequest struct {
	Token           string `json:"token" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

// StaffChangePasswordRequest represents a change password request
type StaffChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

// StaffInviteRequest represents a staff invitation request
type StaffInviteRequest struct {
	StaffID           uuid.UUID         `json:"staffId" binding:"required"`
	AuthMethodOptions []StaffAuthMethod `json:"authMethodOptions,omitempty"` // Available auth methods
	CustomMessage     *string           `json:"customMessage,omitempty"`
	SendEmail         bool              `json:"sendEmail" binding:"required"`
	ExpiresInHours    *int              `json:"expiresInHours,omitempty"`    // Default 72 hours
	ActivationBaseURL *string           `json:"activationBaseUrl,omitempty"` // Base URL for activation link (e.g., https://store-admin.tesserix.app)
	BusinessName      *string           `json:"businessName,omitempty"`      // Store/business name for email
}

// StaffActivationRequest represents account activation request
type StaffActivationRequest struct {
	Token             string          `json:"token" binding:"required"`
	AuthMethod        StaffAuthMethod `json:"authMethod" binding:"required"` // password or sso
	Password          *string         `json:"password,omitempty"`            // Required if authMethod is password
	ConfirmPassword   *string         `json:"confirmPassword,omitempty"`
	GoogleIDToken     *string         `json:"googleIdToken,omitempty"`    // Required if authMethod is google_sso
	MicrosoftIDToken  *string         `json:"microsoftIdToken,omitempty"` // Required if authMethod is microsoft_sso
	DeviceFingerprint *string         `json:"deviceFingerprint,omitempty"`
	DeviceName        *string         `json:"deviceName,omitempty"`
}

// StaffActivationResponse represents account activation response
type StaffActivationResponse struct {
	Success      bool       `json:"success"`
	Staff        *Staff     `json:"staff,omitempty"`
	AccessToken  *string    `json:"accessToken,omitempty"`
	RefreshToken *string    `json:"refreshToken,omitempty"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	Message      string     `json:"message"`
}

// SSOConfigUpdateRequest represents SSO config update request
type SSOConfigUpdateRequest struct {
	// Google OAuth
	GoogleEnabled        *bool      `json:"googleEnabled,omitempty"`
	GoogleClientID       *string    `json:"googleClientId,omitempty"`
	GoogleClientSecret   *string    `json:"googleClientSecret,omitempty"`
	GoogleAllowedDomains *JSONArray `json:"googleAllowedDomains,omitempty"`

	// Microsoft Entra
	MicrosoftEnabled       *bool      `json:"microsoftEnabled,omitempty"`
	MicrosoftTenantID      *string    `json:"microsoftTenantId,omitempty"`
	MicrosoftClientID      *string    `json:"microsoftClientId,omitempty"`
	MicrosoftClientSecret  *string    `json:"microsoftClientSecret,omitempty"`
	MicrosoftAllowedGroups *JSONArray `json:"microsoftAllowedGroups,omitempty"`

	// Okta
	OktaEnabled         *bool      `json:"oktaEnabled,omitempty"`
	OktaDomain          *string    `json:"oktaDomain,omitempty"`
	OktaClientID        *string    `json:"oktaClientId,omitempty"`
	OktaClientSecret    *string    `json:"oktaClientSecret,omitempty"`
	OktaProtocol        *string    `json:"oktaProtocol,omitempty"` // "oidc" or "saml"
	OktaSAMLMetadataURL *string    `json:"oktaSamlMetadataUrl,omitempty"`
	OktaSAMLEntityID    *string    `json:"oktaSamlEntityId,omitempty"`
	OktaSAMLCertificate *string    `json:"oktaSamlCertificate,omitempty"`
	OktaAllowedGroups   *JSONArray `json:"oktaAllowedGroups,omitempty"`

	// SCIM
	SCIMEnabled             *bool `json:"scimEnabled,omitempty"`
	SCIMSyncIntervalMinutes *int  `json:"scimSyncIntervalMinutes,omitempty"`
	SCIMAutoCreateUsers     *bool `json:"scimAutoCreateUsers,omitempty"`
	SCIMAutoDeactivateUsers *bool `json:"scimAutoDeactivateUsers,omitempty"`

	// General SSO settings
	AllowPasswordAuth  *bool      `json:"allowPasswordAuth,omitempty"`
	EnforceSSO         *bool      `json:"enforceSSO,omitempty"`
	AutoProvisionUsers *bool      `json:"autoProvisionUsers,omitempty"`
	DefaultRoleID      *uuid.UUID `json:"defaultRoleId,omitempty"`

	// Security settings
	SessionDurationHours *int  `json:"sessionDurationHours,omitempty"`
	RefreshTokenDays     *int  `json:"refreshTokenDays,omitempty"`
	MaxSessionsPerUser   *int  `json:"maxSessionsPerUser,omitempty"`
	RequireMFA           *bool `json:"requireMFA,omitempty"`
}

// ===========================================
// Enterprise SSO Request/Response Types
// ===========================================

// OktaConfigRequest represents Okta SSO configuration request
type OktaConfigRequest struct {
	Enabled         bool     `json:"enabled"`
	Domain          string   `json:"domain" binding:"required"` // e.g., "company.okta.com"
	ClientID        string   `json:"clientId" binding:"required"`
	ClientSecret    string   `json:"clientSecret" binding:"required"`
	Protocol        string   `json:"protocol" binding:"required,oneof=oidc saml"`
	SAMLMetadataURL *string  `json:"samlMetadataUrl,omitempty"`
	SAMLEntityID    *string  `json:"samlEntityId,omitempty"`
	SAMLCertificate *string  `json:"samlCertificate,omitempty"`
	AllowedGroups   []string `json:"allowedGroups,omitempty"`
}

// EntraConfigRequest represents Microsoft Entra SSO configuration request
type EntraConfigRequest struct {
	Enabled       bool     `json:"enabled"`
	TenantID      string   `json:"tenantId" binding:"required"`
	ClientID      string   `json:"clientId" binding:"required"`
	ClientSecret  string   `json:"clientSecret" binding:"required"`
	AllowedGroups []string `json:"allowedGroups,omitempty"`
}

// GoogleConfigRequest represents Google OAuth SSO configuration request
type GoogleConfigRequest struct {
	Enabled        bool     `json:"enabled"`
	ClientID       string   `json:"clientId" binding:"required"`
	ClientSecret   string   `json:"clientSecret" binding:"required"`
	AllowedDomains []string `json:"allowedDomains,omitempty"` // Optional domain restrictions (e.g., ["company.com"])
}

// SCIMConfigRequest represents SCIM provisioning configuration request
type SCIMConfigRequest struct {
	Enabled             bool  `json:"enabled"`
	SyncIntervalMinutes *int  `json:"syncIntervalMinutes,omitempty"`
	AutoCreateUsers     *bool `json:"autoCreateUsers,omitempty"`
	AutoDeactivateUsers *bool `json:"autoDeactivateUsers,omitempty"`
}

// SCIMTokenResponse represents SCIM token generation response
type SCIMTokenResponse struct {
	Endpoint  string    `json:"endpoint"`
	Token     string    `json:"token"` // Only shown once on creation
	CreatedAt time.Time `json:"createdAt"`
}

// SSOStatusResponse represents SSO status response
type SSOStatusResponse struct {
	GoogleConfigured    bool                `json:"googleConfigured"`
	MicrosoftConfigured bool                `json:"microsoftConfigured"`
	OktaConfigured      bool                `json:"oktaConfigured"`
	SCIMEnabled         bool                `json:"scimEnabled"`
	EnforceSSO          bool                `json:"enforceSSO"`
	AllowPasswordAuth   bool                `json:"allowPasswordAuth"`
	ProviderDetails     []SSOProviderStatus `json:"providerDetails"`
}

// SSOProviderStatus represents status of a single SSO provider
type SSOProviderStatus struct {
	Provider       string     `json:"provider"` // google, microsoft, okta
	Enabled        bool       `json:"enabled"`
	Protocol       string     `json:"protocol"` // oidc, saml
	LastSyncAt     *time.Time `json:"lastSyncAt,omitempty"`
	IdPAlias       *string    `json:"idpAlias,omitempty"` // KeyCloak IdP alias
	ConnectionTest *bool      `json:"connectionTest,omitempty"`
}

// IdPTestResult represents identity provider connection test result
type IdPTestResult struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	TestedAt     time.Time `json:"testedAt"`
	ResponseTime int64     `json:"responseTimeMs"`
	Details      *JSON     `json:"details,omitempty"`
}

// TokenRefreshRequest represents a token refresh request
type TokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// TokenRefreshResponse represents a token refresh response
type TokenRefreshResponse struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	TokenType    string    `json:"tokenType"`
}

// StaffSessionListResponse represents sessions list response
type StaffSessionListResponse struct {
	Success bool           `json:"success"`
	Data    []StaffSession `json:"data"`
	Total   int            `json:"total"`
}

// InvitationVerifyResponse represents invitation verification response
type InvitationVerifyResponse struct {
	Valid               bool              `json:"valid"`
	Staff               *Staff            `json:"staff,omitempty"`
	ActivationToken     string            `json:"activationToken,omitempty"` // Token to use for activation
	AuthMethodOptions   []StaffAuthMethod `json:"authMethodOptions,omitempty"`
	ExpiresAt           *time.Time        `json:"expiresAt,omitempty"`
	Message             string            `json:"message,omitempty"`
	GoogleEnabled       bool              `json:"googleEnabled"`
	MicrosoftEnabled    bool              `json:"microsoftEnabled"`
	PasswordAuthEnabled bool              `json:"passwordAuthEnabled"`
}

// ===========================================
// Password History Model
// ===========================================

// StaffPasswordHistory represents password history entry
type StaffPasswordHistory struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	StaffID      uuid.UUID `json:"staffId" gorm:"not null"`
	PasswordHash string    `json:"-" gorm:"not null"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (StaffPasswordHistory) TableName() string {
	return "staff_password_history"
}
