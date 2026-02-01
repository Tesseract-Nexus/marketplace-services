-- Add password_and_google to staff_auth_method enum
-- This represents staff who activated with password and later linked Google SSO
DO $$
BEGIN
    ALTER TYPE staff_auth_method ADD VALUE IF NOT EXISTS 'password_and_google';
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
