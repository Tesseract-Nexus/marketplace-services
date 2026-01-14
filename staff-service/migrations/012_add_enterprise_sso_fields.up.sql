-- Migration: Add Enterprise SSO and SCIM 2.0 Fields
-- Extends tenant_sso_config with Okta support and SCIM provisioning
-- Adds KeyCloak Identity Provider federation tracking

-- ============================================================================
-- ADD OKTA SSO CONFIGURATION FIELDS
-- ============================================================================

-- Okta basic configuration
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_enabled BOOLEAN DEFAULT false;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_domain VARCHAR(255);
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_client_id VARCHAR(255);
-- Secret stored in GCP Secret Manager: projects/{project}/secrets/sso-{tenant_id}-okta-client-secret
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_client_secret_ref VARCHAR(500);
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_protocol VARCHAR(20) DEFAULT 'oidc'; -- 'oidc' or 'saml'

-- Okta SAML-specific fields
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_saml_metadata_url TEXT;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_saml_entity_id VARCHAR(500);
-- SAML certificate stored in GCP Secret Manager: projects/{project}/secrets/sso-{tenant_id}-okta-saml-cert
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_saml_certificate_ref VARCHAR(500);

-- Okta group/scope restrictions
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS okta_allowed_groups JSONB;

-- ============================================================================
-- ADD SCIM 2.0 PROVISIONING FIELDS
-- ============================================================================

ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_enabled BOOLEAN DEFAULT false;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_endpoint VARCHAR(500);
-- SCIM bearer token stored in GCP Secret Manager: projects/{project}/secrets/scim-{tenant_id}-bearer-token
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_bearer_token_ref VARCHAR(500);
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_sync_interval_minutes INTEGER DEFAULT 60;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_last_sync_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_auto_create_users BOOLEAN DEFAULT true;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_auto_deactivate_users BOOLEAN DEFAULT true;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_token_created_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS scim_token_last_used_at TIMESTAMP WITH TIME ZONE;

-- ============================================================================
-- MIGRATE EXISTING SECRET COLUMNS TO USE GCP SECRET MANAGER REFERENCES
-- ============================================================================

-- Google OAuth - add reference columns for GCP Secret Manager
-- Secret stored in: projects/{project}/secrets/sso-{tenant_id}-google-client-secret
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS google_client_secret_ref VARCHAR(500);

-- Microsoft Entra - add reference column for GCP Secret Manager
-- Secret stored in: projects/{project}/secrets/sso-{tenant_id}-microsoft-client-secret
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS microsoft_client_secret_ref VARCHAR(500);

-- ============================================================================
-- ADD TENANT SECRETS REGISTRY TABLE
-- Tracks all GCP Secret Manager secrets per tenant for management and cleanup
-- ============================================================================

CREATE TABLE IF NOT EXISTS tenant_secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Secret identification
    secret_name VARCHAR(255) NOT NULL,           -- Unique name within tenant (e.g., 'okta-client-secret')
    secret_type VARCHAR(50) NOT NULL,            -- 'sso', 'scim', 'api_key', 'certificate'
    provider VARCHAR(50),                        -- 'google', 'microsoft', 'okta', 'scim'

    -- GCP Secret Manager reference
    gcp_secret_id VARCHAR(500) NOT NULL,         -- Full GCP secret ID: projects/{project}/secrets/{secret_name}
    gcp_secret_version VARCHAR(50) DEFAULT 'latest',

    -- Metadata
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,         -- Optional expiration

    -- Audit
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),
    last_rotated_at TIMESTAMP WITH TIME ZONE,
    rotation_count INTEGER DEFAULT 0,

    -- Unique constraint: one secret per name per tenant
    UNIQUE(tenant_id, secret_name)
);

CREATE INDEX IF NOT EXISTS idx_tenant_secrets_tenant ON tenant_secrets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_secrets_type ON tenant_secrets(tenant_id, secret_type);
CREATE INDEX IF NOT EXISTS idx_tenant_secrets_provider ON tenant_secrets(tenant_id, provider);
CREATE INDEX IF NOT EXISTS idx_tenant_secrets_active ON tenant_secrets(tenant_id, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_tenant_secrets_expiring ON tenant_secrets(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================================================
-- ADD KEYCLOAK IDENTITY PROVIDER TRACKING
-- ============================================================================

-- Track the KeyCloak IdP alias for each provider
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS keycloak_microsoft_idp_alias VARCHAR(255);
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS keycloak_okta_idp_alias VARCHAR(255);
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS keycloak_google_idp_alias VARCHAR(255);

-- IdP federation status
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS keycloak_federation_enabled BOOLEAN DEFAULT false;
ALTER TABLE tenant_sso_config ADD COLUMN IF NOT EXISTS keycloak_last_sync_at TIMESTAMP WITH TIME ZONE;

-- ============================================================================
-- ADD SCIM ACTIVITY LOG TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS scim_sync_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Sync details
    operation VARCHAR(50) NOT NULL, -- 'create', 'update', 'delete', 'sync'
    resource_type VARCHAR(50) NOT NULL, -- 'User', 'Group'
    resource_id VARCHAR(255),
    external_id VARCHAR(255), -- ID from enterprise IdP

    -- Result
    success BOOLEAN NOT NULL,
    error_message TEXT,

    -- User details for audit
    user_email VARCHAR(255),
    user_display_name VARCHAR(255),

    -- Request tracking
    request_id VARCHAR(255),
    source_ip VARCHAR(50),

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scim_sync_log_tenant ON scim_sync_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_scim_sync_log_resource ON scim_sync_log(tenant_id, resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_scim_sync_log_time ON scim_sync_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scim_sync_log_failures ON scim_sync_log(tenant_id, success) WHERE success = false;

-- ============================================================================
-- ADD ENTERPRISE SSO PERMISSIONS
-- ============================================================================

-- Insert SSO integration permissions (owner-only access)
INSERT INTO staff_permissions (id, name, display_name, description, category, is_system)
VALUES
    (uuid_generate_v4(), 'integrations:sso:view', 'View SSO Configuration', 'View enterprise SSO configuration and status', 'integrations', true),
    (uuid_generate_v4(), 'integrations:sso:manage', 'Manage SSO Configuration', 'Configure and manage enterprise SSO providers', 'integrations', true),
    (uuid_generate_v4(), 'integrations:scim:view', 'View SCIM Configuration', 'View SCIM provisioning configuration and logs', 'integrations', true),
    (uuid_generate_v4(), 'integrations:scim:manage', 'Manage SCIM Configuration', 'Configure and manage SCIM user provisioning', 'integrations', true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- GRANT SSO PERMISSIONS TO OWNER ROLES ONLY
-- ============================================================================

-- Only store owners can manage SSO settings
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'integrations:sso:view',
    'integrations:sso:manage',
    'integrations:scim:view',
    'integrations:scim:manage'
)
AND sr.name IN ('store_owner', 'owner')
ON CONFLICT DO NOTHING;

-- ============================================================================
-- ADD EXTENDED AUTH METHOD ENUM VALUE FOR OKTA
-- ============================================================================

-- Add okta_sso to staff_auth_method enum if not exists
DO $$
BEGIN
    ALTER TYPE staff_auth_method ADD VALUE IF NOT EXISTS 'okta_sso';
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Add enterprise_sso for KeyCloak brokered auth
DO $$
BEGIN
    ALTER TYPE staff_auth_method ADD VALUE IF NOT EXISTS 'enterprise_sso';
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- ============================================================================
-- ADD OKTA ID TO STAFF TABLE
-- ============================================================================

ALTER TABLE staff ADD COLUMN IF NOT EXISTS okta_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_staff_okta_id ON staff(okta_id) WHERE okta_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_tenant_okta_id ON staff(tenant_id, okta_id) WHERE okta_id IS NOT NULL;

-- ============================================================================
-- ADD KEYCLOAK USER ID TO STAFF TABLE
-- ============================================================================

ALTER TABLE staff ADD COLUMN IF NOT EXISTS keycloak_user_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_staff_keycloak_id ON staff(keycloak_user_id) WHERE keycloak_user_id IS NOT NULL;

-- ============================================================================
-- DOCUMENTATION
-- ============================================================================

COMMENT ON COLUMN tenant_sso_config.okta_enabled IS 'Whether Okta SSO is enabled for this tenant';
COMMENT ON COLUMN tenant_sso_config.okta_domain IS 'Okta organization domain (e.g., company.okta.com)';
COMMENT ON COLUMN tenant_sso_config.okta_protocol IS 'Authentication protocol: oidc or saml';
COMMENT ON COLUMN tenant_sso_config.okta_client_secret_ref IS 'GCP Secret Manager reference for Okta client secret';
COMMENT ON COLUMN tenant_sso_config.okta_saml_certificate_ref IS 'GCP Secret Manager reference for Okta SAML certificate';
COMMENT ON COLUMN tenant_sso_config.scim_enabled IS 'Whether SCIM 2.0 user provisioning is enabled';
COMMENT ON COLUMN tenant_sso_config.scim_bearer_token_ref IS 'GCP Secret Manager reference for SCIM bearer token';
COMMENT ON COLUMN tenant_sso_config.google_client_secret_ref IS 'GCP Secret Manager reference for Google OAuth client secret';
COMMENT ON COLUMN tenant_sso_config.microsoft_client_secret_ref IS 'GCP Secret Manager reference for Microsoft Entra client secret';
COMMENT ON COLUMN tenant_sso_config.keycloak_microsoft_idp_alias IS 'KeyCloak IdP alias for Microsoft Entra federation';
COMMENT ON COLUMN tenant_sso_config.keycloak_okta_idp_alias IS 'KeyCloak IdP alias for Okta federation';
COMMENT ON TABLE scim_sync_log IS 'Audit log for SCIM user provisioning operations';
COMMENT ON TABLE tenant_secrets IS 'Registry of GCP Secret Manager secrets per tenant for SSO/SCIM integrations';

-- ============================================================================
-- GCP SECRET MANAGER NAMING CONVENTION
-- ============================================================================
-- Secrets are stored in GCP Secret Manager with the following naming pattern:
--
-- Pattern: {type}-{tenant_id}-{provider}-{secret_name}
--
-- Examples:
--   sso-tenant123-okta-client-secret
--   sso-tenant123-microsoft-client-secret
--   sso-tenant123-google-client-secret
--   scim-tenant123-bearer-token
--   sso-tenant123-okta-saml-certificate
--
-- The service account for the staff-service pod must have:
--   - roles/secretmanager.secretAccessor - to read secrets
--   - roles/secretmanager.secretVersionManager - to create new versions (rotation)
--
-- Secret Manager IAM is scoped to:
--   - Only the staff-service pod service account can access these secrets
--   - Secrets are encrypted at rest with Google-managed encryption keys
-- ============================================================================
