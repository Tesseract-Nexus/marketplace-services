package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/Tesseract-Nexus/go-shared/secrets"
	"gorm.io/gorm"

	"staff-service/internal/models"
)

// SSOService handles enterprise SSO configuration and management
type SSOService struct {
	db            *gorm.DB
	secretsClient *secrets.GCPSecretManagerClient
	keycloakAdmin *auth.KeycloakAdminClient
	gcpProjectID  string
}

// NewSSOService creates a new SSO service
func NewSSOService(
	db *gorm.DB,
	secretsClient *secrets.GCPSecretManagerClient,
	keycloakAdmin *auth.KeycloakAdminClient,
	gcpProjectID string,
) *SSOService {
	return &SSOService{
		db:            db,
		secretsClient: secretsClient,
		keycloakAdmin: keycloakAdmin,
		gcpProjectID:  gcpProjectID,
	}
}

// GetSSOConfig retrieves SSO configuration for a tenant
func (s *SSOService) GetSSOConfig(ctx context.Context, tenantID string) (*models.TenantSSOConfig, error) {
	var config models.TenantSSOConfig
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default config for new tenants
			return &models.TenantSSOConfig{
				TenantID:          tenantID,
				AllowPasswordAuth: true,
			}, nil
		}
		return nil, fmt.Errorf("failed to get SSO config: %w", err)
	}
	return &config, nil
}

// GetSSOStatus returns the status of all SSO providers for a tenant
func (s *SSOService) GetSSOStatus(ctx context.Context, tenantID string) (*models.SSOStatusResponse, error) {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	status := &models.SSOStatusResponse{
		GoogleConfigured:    config.GoogleEnabled && config.GoogleClientID != nil,
		MicrosoftConfigured: config.MicrosoftEnabled && config.MicrosoftClientID != nil,
		OktaConfigured:      config.OktaEnabled && config.OktaClientID != nil,
		SCIMEnabled:         config.SCIMEnabled,
		EnforceSSO:          config.EnforceSSO,
		AllowPasswordAuth:   config.AllowPasswordAuth,
		ProviderDetails:     []models.SSOProviderStatus{},
	}

	// Add Google status
	if config.GoogleEnabled {
		status.ProviderDetails = append(status.ProviderDetails, models.SSOProviderStatus{
			Provider: "google",
			Enabled:  true,
			Protocol: "oidc",
			IdPAlias: config.KeycloakGoogleIdPAlias,
		})
	}

	// Add Microsoft status
	if config.MicrosoftEnabled {
		status.ProviderDetails = append(status.ProviderDetails, models.SSOProviderStatus{
			Provider:   "microsoft",
			Enabled:    true,
			Protocol:   "oidc",
			IdPAlias:   config.KeycloakMicrosoftIdPAlias,
			LastSyncAt: config.KeycloakLastSyncAt,
		})
	}

	// Add Okta status
	if config.OktaEnabled {
		protocol := "oidc"
		if config.OktaProtocol != nil {
			protocol = *config.OktaProtocol
		}
		status.ProviderDetails = append(status.ProviderDetails, models.SSOProviderStatus{
			Provider:   "okta",
			Enabled:    true,
			Protocol:   protocol,
			IdPAlias:   config.KeycloakOktaIdPAlias,
			LastSyncAt: config.KeycloakLastSyncAt,
		})
	}

	return status, nil
}

// ConfigureEntra configures Microsoft Entra SSO for a tenant
func (s *SSOService) ConfigureEntra(ctx context.Context, tenantID string, req models.EntraConfigRequest, updatedBy string) error {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	// Store client secret in GCP Secret Manager
	secretMetadata := secrets.SecretMetadata{
		TenantID:   tenantID,
		SecretType: secrets.SecretTypeSSO,
		Provider:   secrets.ProviderMicrosoft,
		SecretName: "client-secret",
	}

	storedSecret, err := s.secretsClient.CreateSecret(ctx, secretMetadata, req.ClientSecret)
	if err != nil {
		return fmt.Errorf("failed to store client secret: %w", err)
	}

	// Build KeyCloak IdP alias
	idpAlias := auth.BuildIdPAlias(tenantID, "microsoft")

	// Create or update KeyCloak IdP
	idpConfig := auth.BuildMicrosoftEntraConfig(
		req.TenantID,
		idpAlias,
		"Microsoft Entra",
		req.ClientID,
		req.ClientSecret, // KeyCloak needs the actual secret
	)

	existingIdP, err := s.keycloakAdmin.GetIdentityProvider(ctx, idpAlias)
	if err != nil {
		return fmt.Errorf("failed to check existing IdP: %w", err)
	}

	if existingIdP != nil {
		if err := s.keycloakAdmin.UpdateIdentityProvider(ctx, idpAlias, idpConfig); err != nil {
			return fmt.Errorf("failed to update KeyCloak IdP: %w", err)
		}
	} else {
		if err := s.keycloakAdmin.CreateIdentityProvider(ctx, idpConfig); err != nil {
			return fmt.Errorf("failed to create KeyCloak IdP: %w", err)
		}
	}

	// Update database
	config.MicrosoftEnabled = req.Enabled
	config.MicrosoftTenantID = &req.TenantID
	config.MicrosoftClientID = &req.ClientID
	config.MicrosoftClientSecretRef = &storedSecret.GCPSecretID
	config.KeycloakMicrosoftIdPAlias = &idpAlias
	config.KeycloakFederationEnabled = true
	now := time.Now()
	config.KeycloakLastSyncAt = &now
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if config.ID == uuid.Nil {
		config.ID = uuid.New()
		config.TenantID = tenantID
		config.CreatedAt = now
		config.CreatedBy = &updatedBy
		if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
			return fmt.Errorf("failed to create SSO config: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
			return fmt.Errorf("failed to update SSO config: %w", err)
		}
	}

	// Record the secret in tenant_secrets registry
	if err := s.recordSecret(ctx, tenantID, storedSecret, "Microsoft Entra client secret", updatedBy); err != nil {
		// Log but don't fail - main operation succeeded
		log.Printf("[SSOService] Warning: failed to record secret in registry: %v", err)
	}

	return nil
}

// ConfigureOkta configures Okta SSO for a tenant
func (s *SSOService) ConfigureOkta(ctx context.Context, tenantID string, req models.OktaConfigRequest, updatedBy string) error {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	// Store client secret in GCP Secret Manager
	secretMetadata := secrets.SecretMetadata{
		TenantID:   tenantID,
		SecretType: secrets.SecretTypeSSO,
		Provider:   secrets.ProviderOkta,
		SecretName: "client-secret",
	}

	storedSecret, err := s.secretsClient.CreateSecret(ctx, secretMetadata, req.ClientSecret)
	if err != nil {
		return fmt.Errorf("failed to store client secret: %w", err)
	}

	// Build KeyCloak IdP alias
	idpAlias := auth.BuildIdPAlias(tenantID, "okta")

	// Create KeyCloak IdP config based on protocol
	var idpConfig auth.IdentityProviderConfig
	if req.Protocol == "saml" {
		// Store SAML certificate if provided
		if req.SAMLCertificate != nil && *req.SAMLCertificate != "" {
			certMetadata := secrets.SecretMetadata{
				TenantID:   tenantID,
				SecretType: secrets.SecretTypeCertificate,
				Provider:   secrets.ProviderOkta,
				SecretName: "saml-certificate",
			}
			certSecret, err := s.secretsClient.CreateSecret(ctx, certMetadata, *req.SAMLCertificate)
			if err != nil {
				return fmt.Errorf("failed to store SAML certificate: %w", err)
			}
			config.OktaSAMLCertificateRef = &certSecret.GCPSecretID
		}

		entityID := ""
		if req.SAMLEntityID != nil {
			entityID = *req.SAMLEntityID
		}
		metadataURL := ""
		if req.SAMLMetadataURL != nil {
			metadataURL = *req.SAMLMetadataURL
		}
		idpConfig = auth.BuildOktaSAMLConfig(req.Domain, idpAlias, "Okta (SAML)", entityID, metadataURL)
	} else {
		idpConfig = auth.BuildOktaOIDCConfig(req.Domain, idpAlias, "Okta (OIDC)", req.ClientID, req.ClientSecret)
	}

	// Create or update KeyCloak IdP
	existingIdP, err := s.keycloakAdmin.GetIdentityProvider(ctx, idpAlias)
	if err != nil {
		return fmt.Errorf("failed to check existing IdP: %w", err)
	}

	if existingIdP != nil {
		if err := s.keycloakAdmin.UpdateIdentityProvider(ctx, idpAlias, idpConfig); err != nil {
			return fmt.Errorf("failed to update KeyCloak IdP: %w", err)
		}
	} else {
		if err := s.keycloakAdmin.CreateIdentityProvider(ctx, idpConfig); err != nil {
			return fmt.Errorf("failed to create KeyCloak IdP: %w", err)
		}
	}

	// Update database
	config.OktaEnabled = req.Enabled
	config.OktaDomain = &req.Domain
	config.OktaClientID = &req.ClientID
	config.OktaClientSecretRef = &storedSecret.GCPSecretID
	config.OktaProtocol = &req.Protocol
	config.OktaSAMLMetadataURL = req.SAMLMetadataURL
	config.OktaSAMLEntityID = req.SAMLEntityID
	config.KeycloakOktaIdPAlias = &idpAlias
	config.KeycloakFederationEnabled = true
	now := time.Now()
	config.KeycloakLastSyncAt = &now
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if config.ID == uuid.Nil {
		config.ID = uuid.New()
		config.TenantID = tenantID
		config.CreatedAt = now
		config.CreatedBy = &updatedBy
		if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
			return fmt.Errorf("failed to create SSO config: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
			return fmt.Errorf("failed to update SSO config: %w", err)
		}
	}

	// Record the secret in tenant_secrets registry
	if err := s.recordSecret(ctx, tenantID, storedSecret, "Okta client secret", updatedBy); err != nil {
		log.Printf("[SSOService] Warning: failed to record secret in registry: %v", err)
	}

	return nil
}

// ConfigureGoogle configures Google OAuth SSO for a tenant
func (s *SSOService) ConfigureGoogle(ctx context.Context, tenantID string, req models.GoogleConfigRequest, updatedBy string) error {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	// Store client secret in GCP Secret Manager
	secretMetadata := secrets.SecretMetadata{
		TenantID:   tenantID,
		SecretType: secrets.SecretTypeSSO,
		Provider:   secrets.ProviderGoogle,
		SecretName: "client-secret",
	}

	storedSecret, err := s.secretsClient.CreateSecret(ctx, secretMetadata, req.ClientSecret)
	if err != nil {
		return fmt.Errorf("failed to store client secret: %w", err)
	}

	// Build KeyCloak IdP alias
	idpAlias := auth.BuildIdPAlias(tenantID, "google")

	// Create or update KeyCloak IdP
	idpConfig := auth.BuildGoogleConfig(
		idpAlias,
		"Google",
		req.ClientID,
		req.ClientSecret, // KeyCloak needs the actual secret
	)

	existingIdP, err := s.keycloakAdmin.GetIdentityProvider(ctx, idpAlias)
	if err != nil {
		return fmt.Errorf("failed to check existing IdP: %w", err)
	}

	if existingIdP != nil {
		if err := s.keycloakAdmin.UpdateIdentityProvider(ctx, idpAlias, idpConfig); err != nil {
			return fmt.Errorf("failed to update KeyCloak IdP: %w", err)
		}
	} else {
		if err := s.keycloakAdmin.CreateIdentityProvider(ctx, idpConfig); err != nil {
			return fmt.Errorf("failed to create KeyCloak IdP: %w", err)
		}
	}

	// Update database
	config.GoogleEnabled = req.Enabled
	config.GoogleClientID = &req.ClientID
	config.GoogleClientSecretRef = &storedSecret.GCPSecretID
	config.KeycloakGoogleIdPAlias = &idpAlias
	config.KeycloakFederationEnabled = true
	now := time.Now()
	config.KeycloakLastSyncAt = &now
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	// Store allowed domains if provided
	if len(req.AllowedDomains) > 0 {
		// Convert []string to []interface{} for JSONArray
		domains := make(models.JSONArray, len(req.AllowedDomains))
		for i, d := range req.AllowedDomains {
			domains[i] = d
		}
		config.GoogleAllowedDomains = &domains
	}

	if config.ID == uuid.Nil {
		config.ID = uuid.New()
		config.TenantID = tenantID
		config.CreatedAt = now
		config.CreatedBy = &updatedBy
		if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
			return fmt.Errorf("failed to create SSO config: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
			return fmt.Errorf("failed to update SSO config: %w", err)
		}
	}

	// Record the secret in tenant_secrets registry
	if err := s.recordSecret(ctx, tenantID, storedSecret, "Google OAuth client secret", updatedBy); err != nil {
		log.Printf("[SSOService] Warning: failed to record secret in registry: %v", err)
	}

	return nil
}

// RemoveProvider removes an SSO provider configuration
func (s *SSOService) RemoveProvider(ctx context.Context, tenantID, provider, updatedBy string) error {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	var idpAlias *string
	var secretRef *string

	switch provider {
	case "microsoft":
		idpAlias = config.KeycloakMicrosoftIdPAlias
		secretRef = config.MicrosoftClientSecretRef
		config.MicrosoftEnabled = false
		config.MicrosoftTenantID = nil
		config.MicrosoftClientID = nil
		config.MicrosoftClientSecretRef = nil
		config.KeycloakMicrosoftIdPAlias = nil
	case "okta":
		idpAlias = config.KeycloakOktaIdPAlias
		secretRef = config.OktaClientSecretRef
		config.OktaEnabled = false
		config.OktaDomain = nil
		config.OktaClientID = nil
		config.OktaClientSecretRef = nil
		config.OktaProtocol = nil
		config.OktaSAMLMetadataURL = nil
		config.OktaSAMLEntityID = nil
		config.OktaSAMLCertificateRef = nil
		config.KeycloakOktaIdPAlias = nil
	case "google":
		idpAlias = config.KeycloakGoogleIdPAlias
		secretRef = config.GoogleClientSecretRef
		config.GoogleEnabled = false
		config.GoogleClientID = nil
		config.GoogleClientSecretRef = nil
		config.KeycloakGoogleIdPAlias = nil
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}

	// Delete KeyCloak IdP
	if idpAlias != nil && *idpAlias != "" {
		if err := s.keycloakAdmin.DeleteIdentityProvider(ctx, *idpAlias); err != nil {
			return fmt.Errorf("failed to delete KeyCloak IdP: %w", err)
		}
	}

	// Delete secret from GCP Secret Manager
	if secretRef != nil && *secretRef != "" {
		if err := s.secretsClient.DeleteSecret(ctx, *secretRef); err != nil {
			log.Printf("[SSOService] Warning: failed to delete secret: %v", err)
		}
	}

	// Update database
	now := time.Now()
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
		return fmt.Errorf("failed to update SSO config: %w", err)
	}

	return nil
}

// TestProvider tests connectivity to an SSO provider
func (s *SSOService) TestProvider(ctx context.Context, tenantID, provider string) (*models.IdPTestResult, error) {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var idpAlias *string
	switch provider {
	case "microsoft":
		idpAlias = config.KeycloakMicrosoftIdPAlias
	case "okta":
		idpAlias = config.KeycloakOktaIdPAlias
	case "google":
		idpAlias = config.KeycloakGoogleIdPAlias
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	if idpAlias == nil || *idpAlias == "" {
		return &models.IdPTestResult{
			Success:  false,
			Message:  "Provider not configured",
			TestedAt: time.Now(),
		}, nil
	}

	result, err := s.keycloakAdmin.TestIdentityProvider(ctx, *idpAlias)
	if err != nil {
		return &models.IdPTestResult{
			Success:  false,
			Message:  fmt.Sprintf("Test failed: %v", err),
			TestedAt: time.Now(),
		}, nil
	}

	return &models.IdPTestResult{
		Success:      result.Success,
		Message:      result.Message,
		TestedAt:     time.Now(),
		ResponseTime: int64(result.ResponseTime.Milliseconds()),
	}, nil
}

// UpdateSecuritySettings updates SSO security settings
func (s *SSOService) UpdateSecuritySettings(ctx context.Context, tenantID string, req models.SSOConfigUpdateRequest, updatedBy string) error {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	// Update fields if provided
	if req.AllowPasswordAuth != nil {
		config.AllowPasswordAuth = *req.AllowPasswordAuth
	}
	if req.EnforceSSO != nil {
		config.EnforceSSO = *req.EnforceSSO
	}
	if req.RequireMFA != nil {
		config.RequireMFA = *req.RequireMFA
	}
	if req.SessionDurationHours != nil {
		config.SessionDurationHours = *req.SessionDurationHours
	}
	if req.MaxSessionsPerUser != nil {
		config.MaxSessionsPerUser = *req.MaxSessionsPerUser
	}
	if req.AutoProvisionUsers != nil {
		config.AutoProvisionUsers = *req.AutoProvisionUsers
	}
	if req.DefaultRoleID != nil {
		config.DefaultRoleID = req.DefaultRoleID
	}

	now := time.Now()
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if config.ID == uuid.Nil {
		config.ID = uuid.New()
		config.TenantID = tenantID
		config.CreatedAt = now
		config.CreatedBy = &updatedBy
		if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
			return fmt.Errorf("failed to create SSO config: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
			return fmt.Errorf("failed to update SSO config: %w", err)
		}
	}

	return nil
}

// EnableSCIM enables SCIM provisioning for a tenant
func (s *SSOService) EnableSCIM(ctx context.Context, tenantID, updatedBy string) (*models.SCIMTokenResponse, error) {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Generate SCIM bearer token
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SCIM token: %w", err)
	}

	// Store token in GCP Secret Manager
	secretMetadata := secrets.SecretMetadata{
		TenantID:   tenantID,
		SecretType: secrets.SecretTypeSCIM,
		Provider:   secrets.ProviderSCIM,
		SecretName: "bearer-token",
	}

	storedSecret, err := s.secretsClient.CreateSecret(ctx, secretMetadata, token)
	if err != nil {
		return nil, fmt.Errorf("failed to store SCIM token: %w", err)
	}

	// Generate SCIM endpoint using configurable base URL
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "https://api.tesserix.app"
	}
	scimEndpoint := fmt.Sprintf("%s/scim/v2/%s", apiBaseURL, tenantID)

	// Update config
	config.SCIMEnabled = true
	config.SCIMEndpoint = &scimEndpoint
	config.SCIMBearerTokenRef = &storedSecret.GCPSecretID
	now := time.Now()
	config.SCIMTokenCreatedAt = &now
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if config.ID == uuid.Nil {
		config.ID = uuid.New()
		config.TenantID = tenantID
		config.CreatedAt = now
		config.CreatedBy = &updatedBy
		if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
			return nil, fmt.Errorf("failed to create SSO config: %w", err)
		}
	} else {
		if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
			return nil, fmt.Errorf("failed to update SSO config: %w", err)
		}
	}

	// Record the secret
	if err := s.recordSecret(ctx, tenantID, storedSecret, "SCIM bearer token", updatedBy); err != nil {
		log.Printf("[SSOService] Warning: failed to record secret in registry: %v", err)
	}

	return &models.SCIMTokenResponse{
		Endpoint:  scimEndpoint,
		Token:     token, // Only returned once on creation
		CreatedAt: now,
	}, nil
}

// RotateSCIMToken rotates the SCIM bearer token
func (s *SSOService) RotateSCIMToken(ctx context.Context, tenantID, updatedBy string) (*models.SCIMTokenResponse, error) {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if !config.SCIMEnabled {
		return nil, fmt.Errorf("SCIM is not enabled for this tenant")
	}

	// Generate new token
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new SCIM token: %w", err)
	}

	// Get secret name from ref
	secretName := secrets.BuildSecretName(tenantID, secrets.SecretTypeSCIM, secrets.ProviderSCIM, "bearer-token")

	// Rotate the secret
	_, err = s.secretsClient.RotateSecret(ctx, secretName, token)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate SCIM token: %w", err)
	}

	now := time.Now()
	config.SCIMTokenCreatedAt = &now
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
		return nil, fmt.Errorf("failed to update SSO config: %w", err)
	}

	return &models.SCIMTokenResponse{
		Endpoint:  *config.SCIMEndpoint,
		Token:     token,
		CreatedAt: now,
	}, nil
}

// DisableSCIM disables SCIM provisioning
func (s *SSOService) DisableSCIM(ctx context.Context, tenantID, updatedBy string) error {
	config, err := s.GetSSOConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	// Delete SCIM token from Secret Manager
	if config.SCIMBearerTokenRef != nil {
		secretName := secrets.BuildSecretName(tenantID, secrets.SecretTypeSCIM, secrets.ProviderSCIM, "bearer-token")
		if err := s.secretsClient.DeleteSecret(ctx, secretName); err != nil {
			log.Printf("[SSOService] Warning: failed to delete SCIM token: %v", err)
		}
	}

	config.SCIMEnabled = false
	config.SCIMEndpoint = nil
	config.SCIMBearerTokenRef = nil
	config.SCIMTokenCreatedAt = nil
	now := time.Now()
	config.UpdatedBy = &updatedBy
	config.UpdatedAt = now

	if err := s.db.WithContext(ctx).Save(config).Error; err != nil {
		return fmt.Errorf("failed to update SSO config: %w", err)
	}

	return nil
}

// recordSecret records a secret in the tenant_secrets registry
func (s *SSOService) recordSecret(ctx context.Context, tenantID string, metadata *secrets.SecretMetadata, description, createdBy string) error {
	secret := models.TenantSecret{
		ID:          uuid.New(),
		TenantID:    tenantID,
		SecretName:  metadata.SecretName,
		SecretType:  string(metadata.SecretType),
		Provider:    stringPtr(string(metadata.Provider)),
		GCPSecretID: metadata.GCPSecretID,
		Description: &description,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   &createdBy,
	}

	return s.db.WithContext(ctx).Create(&secret).Error
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func stringPtr(s string) *string {
	return &s
}
