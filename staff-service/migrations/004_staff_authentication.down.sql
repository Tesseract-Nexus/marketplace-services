-- Rollback: Staff Authentication System

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_staff_password_change ON staff;

-- Drop functions
DROP FUNCTION IF EXISTS update_password_change_timestamp();
DROP FUNCTION IF EXISTS unlock_expired_staff_accounts();
DROP FUNCTION IF EXISTS check_and_lock_staff_account(UUID, INTEGER, INTEGER);

-- Drop tables
DROP TABLE IF EXISTS tenant_sso_config;
DROP TABLE IF EXISTS staff_invitations;
DROP TABLE IF EXISTS staff_login_audit;
DROP TABLE IF EXISTS staff_password_history;
DROP TABLE IF EXISTS staff_oauth_providers;
DROP TABLE IF EXISTS staff_sessions;

-- Drop indexes on staff table
DROP INDEX IF EXISTS idx_staff_tenant_microsoft_id;
DROP INDEX IF EXISTS idx_staff_tenant_google_id;
DROP INDEX IF EXISTS idx_staff_microsoft_id;
DROP INDEX IF EXISTS idx_staff_google_id;
DROP INDEX IF EXISTS idx_staff_password_reset_token;
DROP INDEX IF EXISTS idx_staff_activation_token;
DROP INDEX IF EXISTS idx_staff_account_status;
DROP INDEX IF EXISTS idx_staff_auth_method;

-- Remove auth columns from staff table
ALTER TABLE staff DROP COLUMN IF EXISTS login_history;
ALTER TABLE staff DROP COLUMN IF EXISTS password_history;
ALTER TABLE staff DROP COLUMN IF EXISTS last_password_change;
ALTER TABLE staff DROP COLUMN IF EXISTS sso_profile_data;
ALTER TABLE staff DROP COLUMN IF EXISTS microsoft_id;
ALTER TABLE staff DROP COLUMN IF EXISTS google_id;
ALTER TABLE staff DROP COLUMN IF EXISTS invited_by;
ALTER TABLE staff DROP COLUMN IF EXISTS invitation_accepted_at;
ALTER TABLE staff DROP COLUMN IF EXISTS invited_at;
ALTER TABLE staff DROP COLUMN IF EXISTS is_email_verified;
ALTER TABLE staff DROP COLUMN IF EXISTS activation_token_expires_at;
ALTER TABLE staff DROP COLUMN IF EXISTS activation_token;
ALTER TABLE staff DROP COLUMN IF EXISTS password_reset_token_expires_at;
ALTER TABLE staff DROP COLUMN IF EXISTS password_reset_token;
ALTER TABLE staff DROP COLUMN IF EXISTS must_reset_password;
ALTER TABLE staff DROP COLUMN IF EXISTS password_hash;
ALTER TABLE staff DROP COLUMN IF EXISTS account_status;
ALTER TABLE staff DROP COLUMN IF EXISTS auth_method;

-- Drop enums
DROP TYPE IF EXISTS staff_account_status;
DROP TYPE IF EXISTS staff_auth_method;
