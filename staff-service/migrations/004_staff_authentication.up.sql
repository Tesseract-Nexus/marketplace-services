-- Migration: Staff Authentication System
-- Adds comprehensive authentication support for staff members
-- Supports: Password auth, Google SSO, Microsoft Entra SSO, Invitation links

-- Authentication method enum
DO $$ BEGIN
    CREATE TYPE staff_auth_method AS ENUM (
        'password',           -- Traditional username/password
        'google_sso',         -- Google OAuth
        'microsoft_sso',      -- Microsoft Entra (Azure AD)
        'invitation_pending', -- Awaiting activation via link
        'sso_pending'         -- Awaiting first SSO login
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Account status enum
DO $$ BEGIN
    CREATE TYPE staff_account_status AS ENUM (
        'pending_activation',  -- Invitation sent, not yet activated
        'pending_password',    -- Needs to set/reset password
        'active',              -- Fully active
        'suspended',           -- Temporarily suspended
        'locked',              -- Locked due to failed attempts
        'deactivated'          -- Permanently deactivated
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Add authentication columns to staff table
ALTER TABLE staff ADD COLUMN IF NOT EXISTS auth_method staff_auth_method DEFAULT 'invitation_pending';
ALTER TABLE staff ADD COLUMN IF NOT EXISTS account_status staff_account_status DEFAULT 'pending_activation';
ALTER TABLE staff ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS must_reset_password BOOLEAN DEFAULT false;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS password_reset_token VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS password_reset_token_expires_at TIMESTAMP WITH TIME ZONE;

-- Activation/Invitation
ALTER TABLE staff ADD COLUMN IF NOT EXISTS activation_token VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS activation_token_expires_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS is_email_verified BOOLEAN DEFAULT false;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS invited_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS invitation_accepted_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES staff(id);

-- SSO Provider IDs
ALTER TABLE staff ADD COLUMN IF NOT EXISTS google_id VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS microsoft_id VARCHAR(255);
ALTER TABLE staff ADD COLUMN IF NOT EXISTS sso_profile_data JSONB;

-- Security audit fields
ALTER TABLE staff ADD COLUMN IF NOT EXISTS last_password_change TIMESTAMP WITH TIME ZONE;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS password_history JSONB; -- Last N password hashes to prevent reuse
ALTER TABLE staff ADD COLUMN IF NOT EXISTS login_history JSONB;    -- Recent login attempts

-- Create indexes for auth queries
CREATE INDEX IF NOT EXISTS idx_staff_auth_method ON staff(auth_method);
CREATE INDEX IF NOT EXISTS idx_staff_account_status ON staff(account_status);
CREATE INDEX IF NOT EXISTS idx_staff_activation_token ON staff(activation_token) WHERE activation_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_staff_password_reset_token ON staff(password_reset_token) WHERE password_reset_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_staff_google_id ON staff(google_id) WHERE google_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_staff_microsoft_id ON staff(microsoft_id) WHERE microsoft_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_tenant_google_id ON staff(tenant_id, google_id) WHERE google_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_tenant_microsoft_id ON staff(tenant_id, microsoft_id) WHERE microsoft_id IS NOT NULL;

-- Staff Sessions table for managing active sessions
CREATE TABLE IF NOT EXISTS staff_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255),
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,

    -- Session tokens
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    access_token_expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    refresh_token_expires_at TIMESTAMP WITH TIME ZONE,

    -- Device/Client info
    device_fingerprint VARCHAR(255),
    device_name VARCHAR(255),
    device_type VARCHAR(50), -- desktop, mobile, tablet
    os_name VARCHAR(100),
    os_version VARCHAR(50),
    browser_name VARCHAR(100),
    browser_version VARCHAR(50),
    ip_address VARCHAR(50),
    location VARCHAR(255),
    user_agent TEXT,

    -- Session state
    is_active BOOLEAN DEFAULT true,
    is_trusted BOOLEAN DEFAULT false,
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_reason VARCHAR(255),

    -- Indexes
    CONSTRAINT fk_staff_sessions_staff FOREIGN KEY (staff_id) REFERENCES staff(id)
);

CREATE INDEX IF NOT EXISTS idx_staff_sessions_staff_id ON staff_sessions(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_sessions_tenant ON staff_sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_sessions_access_token ON staff_sessions(access_token);
CREATE INDEX IF NOT EXISTS idx_staff_sessions_refresh_token ON staff_sessions(refresh_token) WHERE refresh_token IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_staff_sessions_active ON staff_sessions(staff_id, is_active) WHERE is_active = true;

-- Staff OAuth Providers table for linking SSO accounts
CREATE TABLE IF NOT EXISTS staff_oauth_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255),
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,

    -- Provider info
    provider VARCHAR(50) NOT NULL, -- 'google', 'microsoft', 'apple', etc.
    provider_user_id VARCHAR(255) NOT NULL,
    provider_email VARCHAR(255),
    provider_name VARCHAR(255),
    provider_avatar_url TEXT,

    -- OAuth tokens
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMP WITH TIME ZONE,

    -- Profile data from provider
    profile_data JSONB,

    -- Metadata
    is_primary BOOLEAN DEFAULT false,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Unique constraint per tenant/provider/provider_user_id
    UNIQUE(tenant_id, provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_staff_oauth_staff_id ON staff_oauth_providers(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_oauth_provider ON staff_oauth_providers(provider, provider_user_id);
CREATE INDEX IF NOT EXISTS idx_staff_oauth_tenant_provider ON staff_oauth_providers(tenant_id, provider);

-- Staff Password History (for preventing password reuse)
CREATE TABLE IF NOT EXISTS staff_password_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_staff_password_history_staff ON staff_password_history(staff_id);

-- Staff Login Audit Log
CREATE TABLE IF NOT EXISTS staff_login_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255),
    staff_id UUID REFERENCES staff(id) ON DELETE SET NULL,

    -- Login attempt details
    email VARCHAR(255),
    auth_method VARCHAR(50),
    success BOOLEAN NOT NULL,
    failure_reason VARCHAR(255),

    -- Client info
    ip_address VARCHAR(50),
    user_agent TEXT,
    device_fingerprint VARCHAR(255),
    location VARCHAR(255),

    -- Risk assessment
    risk_score INTEGER, -- 0-100
    risk_factors JSONB,

    -- Timestamps
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_staff_login_audit_tenant ON staff_login_audit(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_login_audit_staff ON staff_login_audit(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_login_audit_email ON staff_login_audit(email);
CREATE INDEX IF NOT EXISTS idx_staff_login_audit_time ON staff_login_audit(attempted_at DESC);
CREATE INDEX IF NOT EXISTS idx_staff_login_audit_failed ON staff_login_audit(staff_id, success, attempted_at) WHERE success = false;

-- Staff Invitations table for tracking sent invitations
CREATE TABLE IF NOT EXISTS staff_invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255),
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,

    -- Invitation details
    invitation_token VARCHAR(255) NOT NULL UNIQUE,
    invitation_type VARCHAR(50) NOT NULL, -- 'email', 'sms', 'link'
    auth_method_options JSONB, -- Available auth methods for invitee

    -- Sending details
    sent_to_email VARCHAR(255),
    sent_to_phone VARCHAR(50),

    -- State
    status VARCHAR(50) DEFAULT 'pending', -- pending, sent, opened, accepted, expired, revoked
    sent_at TIMESTAMP WITH TIME ZONE,
    opened_at TIMESTAMP WITH TIME ZONE,
    accepted_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Tracking
    sent_by UUID REFERENCES staff(id),
    send_count INTEGER DEFAULT 0,
    last_sent_at TIMESTAMP WITH TIME ZONE,

    -- Metadata
    custom_message TEXT,
    metadata JSONB,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_staff_invitations_token ON staff_invitations(invitation_token);
CREATE INDEX IF NOT EXISTS idx_staff_invitations_staff ON staff_invitations(staff_id);
CREATE INDEX IF NOT EXISTS idx_staff_invitations_tenant ON staff_invitations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_invitations_status ON staff_invitations(status, expires_at);

-- Tenant SSO Configuration table
CREATE TABLE IF NOT EXISTS tenant_sso_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,

    -- Google OAuth config
    google_enabled BOOLEAN DEFAULT false,
    google_client_id VARCHAR(255),
    google_client_secret VARCHAR(500),
    google_allowed_domains JSONB, -- Restrict to specific domains

    -- Microsoft Entra (Azure AD) config
    microsoft_enabled BOOLEAN DEFAULT false,
    microsoft_tenant_id VARCHAR(255),
    microsoft_client_id VARCHAR(255),
    microsoft_client_secret VARCHAR(500),
    microsoft_allowed_groups JSONB, -- Restrict to specific AD groups

    -- General SSO settings
    allow_password_auth BOOLEAN DEFAULT true,
    enforce_sso BOOLEAN DEFAULT false,
    auto_provision_users BOOLEAN DEFAULT false,
    default_role_id UUID,

    -- Security settings
    session_duration_hours INTEGER DEFAULT 8,
    refresh_token_days INTEGER DEFAULT 30,
    max_sessions_per_user INTEGER DEFAULT 5,
    require_mfa BOOLEAN DEFAULT false,

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255),

    UNIQUE(tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_sso_config_tenant ON tenant_sso_config(tenant_id);

-- Insert default SSO config for existing tenants (optional)
-- This is a placeholder - in production, run a separate script to populate

-- Function to check and lock account after failed attempts
CREATE OR REPLACE FUNCTION check_and_lock_staff_account(
    p_staff_id UUID,
    p_max_attempts INTEGER DEFAULT 5,
    p_lockout_minutes INTEGER DEFAULT 30
) RETURNS BOOLEAN AS $$
DECLARE
    v_failed_count INTEGER;
    v_last_failed TIMESTAMP WITH TIME ZONE;
BEGIN
    -- Count recent failed attempts
    SELECT COUNT(*), MAX(attempted_at)
    INTO v_failed_count, v_last_failed
    FROM staff_login_audit
    WHERE staff_id = p_staff_id
      AND success = false
      AND attempted_at > NOW() - INTERVAL '1 hour';

    -- If exceeded max attempts, lock the account
    IF v_failed_count >= p_max_attempts THEN
        UPDATE staff
        SET account_status = 'locked',
            account_locked_until = NOW() + (p_lockout_minutes || ' minutes')::INTERVAL
        WHERE id = p_staff_id;
        RETURN true; -- Account was locked
    END IF;

    RETURN false; -- Account not locked
END;
$$ LANGUAGE plpgsql;

-- Function to unlock expired locks
CREATE OR REPLACE FUNCTION unlock_expired_staff_accounts() RETURNS INTEGER AS $$
DECLARE
    v_count INTEGER;
BEGIN
    UPDATE staff
    SET account_status = 'active',
        account_locked_until = NULL,
        failed_login_attempts = 0
    WHERE account_status = 'locked'
      AND account_locked_until IS NOT NULL
      AND account_locked_until < NOW();

    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update last_password_change
CREATE OR REPLACE FUNCTION update_password_change_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.password_hash IS DISTINCT FROM OLD.password_hash THEN
        NEW.last_password_change = NOW();
        NEW.must_reset_password = false;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_staff_password_change ON staff;
CREATE TRIGGER trigger_staff_password_change
    BEFORE UPDATE ON staff
    FOR EACH ROW
    EXECUTE FUNCTION update_password_change_timestamp();

-- Comments for documentation
COMMENT ON TABLE staff_sessions IS 'Active authentication sessions for staff members';
COMMENT ON TABLE staff_oauth_providers IS 'Linked OAuth/SSO providers for staff accounts';
COMMENT ON TABLE staff_password_history IS 'Password history to prevent reuse';
COMMENT ON TABLE staff_login_audit IS 'Audit log of all login attempts';
COMMENT ON TABLE staff_invitations IS 'Staff invitations tracking';
COMMENT ON TABLE tenant_sso_config IS 'SSO configuration per tenant';
COMMENT ON COLUMN staff.auth_method IS 'Primary authentication method for staff member';
COMMENT ON COLUMN staff.account_status IS 'Current account activation/lock status';
COMMENT ON COLUMN staff.must_reset_password IS 'If true, staff must reset password on next login';
