-- Migration Rollback: Remove Enterprise SSO and SCIM 2.0 Fields
-- Reverses migration 012

-- ============================================================================
-- REMOVE SSO PERMISSIONS FROM ROLES
-- ============================================================================

DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name IN (
        'integrations:sso:view',
        'integrations:sso:manage',
        'integrations:scim:view',
        'integrations:scim:manage'
    )
);

-- ============================================================================
-- REMOVE SSO PERMISSIONS
-- ============================================================================

DELETE FROM staff_permissions
WHERE name IN (
    'integrations:sso:view',
    'integrations:sso:manage',
    'integrations:scim:view',
    'integrations:scim:manage'
);

-- ============================================================================
-- DROP TABLES
-- ============================================================================

DROP TABLE IF EXISTS scim_sync_log;
DROP TABLE IF EXISTS tenant_secrets;

-- ============================================================================
-- REMOVE OKTA COLUMNS FROM tenant_sso_config
-- ============================================================================

ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_enabled;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_domain;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_client_id;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_client_secret_ref;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_protocol;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_saml_metadata_url;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_saml_entity_id;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_saml_certificate_ref;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS okta_allowed_groups;

-- ============================================================================
-- REMOVE SCIM COLUMNS FROM tenant_sso_config
-- ============================================================================

ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_enabled;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_endpoint;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_bearer_token_ref;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_sync_interval_minutes;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_last_sync_at;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_auto_create_users;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_auto_deactivate_users;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_token_created_at;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS scim_token_last_used_at;

-- ============================================================================
-- REMOVE GCP SECRET MANAGER REFERENCE COLUMNS
-- ============================================================================

ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS google_client_secret_ref;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS microsoft_client_secret_ref;

-- ============================================================================
-- REMOVE KEYCLOAK FEDERATION COLUMNS
-- ============================================================================

ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS keycloak_microsoft_idp_alias;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS keycloak_okta_idp_alias;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS keycloak_google_idp_alias;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS keycloak_federation_enabled;
ALTER TABLE tenant_sso_config DROP COLUMN IF EXISTS keycloak_last_sync_at;

-- ============================================================================
-- REMOVE OKTA/KEYCLOAK ID FROM STAFF TABLE
-- ============================================================================

DROP INDEX IF EXISTS idx_staff_tenant_okta_id;
DROP INDEX IF EXISTS idx_staff_okta_id;
ALTER TABLE staff DROP COLUMN IF EXISTS okta_id;

DROP INDEX IF EXISTS idx_staff_keycloak_id;
ALTER TABLE staff DROP COLUMN IF EXISTS keycloak_user_id;

-- ============================================================================
-- NOTE: Cannot remove enum values in PostgreSQL easily
-- The okta_sso and enterprise_sso auth method values will remain but be unused
-- ============================================================================
