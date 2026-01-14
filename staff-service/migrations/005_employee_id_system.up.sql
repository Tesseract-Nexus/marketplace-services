-- Migration: Employee ID Auto-Generation System
-- Adds atomic employee ID generation with format: {BUSINESS_CODE}-{7_DIGIT_SEQUENCE}
-- Sequence is per-vendor within each tenant

-- ============================================================================
-- EMPLOYEE ID SEQUENCES TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS employee_id_sequences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255), -- NULL means tenant-level, otherwise vendor-specific sequence
    current_sequence INTEGER NOT NULL DEFAULT 0,
    prefix VARCHAR(10), -- Business code (e.g., 'DEMST')
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Unique constraint for tenant+vendor combination (using empty string for NULL vendor_id)
CREATE UNIQUE INDEX IF NOT EXISTS idx_employee_id_sequences_tenant_vendor
    ON employee_id_sequences(tenant_id, COALESCE(vendor_id, ''));

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_employee_id_sequences_tenant
    ON employee_id_sequences(tenant_id);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_employee_id_sequences_updated_at ON employee_id_sequences;
CREATE TRIGGER update_employee_id_sequences_updated_at
    BEFORE UPDATE ON employee_id_sequences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- ADD SLUG COLUMNS TO DEPARTMENTS AND TEAMS
-- ============================================================================

-- Add slug column to departments for auto-generated codes
ALTER TABLE departments ADD COLUMN IF NOT EXISTS slug VARCHAR(100);

-- Add slug column to teams for auto-generated codes
ALTER TABLE teams ADD COLUMN IF NOT EXISTS slug VARCHAR(100);

-- Create indexes for slug lookups
CREATE INDEX IF NOT EXISTS idx_departments_slug ON departments(tenant_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_teams_slug ON teams(tenant_id, slug) WHERE deleted_at IS NULL;

-- ============================================================================
-- ADD NEW COLUMNS TO STAFF TABLE
-- ============================================================================

-- Add profile_photo_document_id to link to document-service
ALTER TABLE staff ADD COLUMN IF NOT EXISTS profile_photo_document_id UUID;

-- Add job_title column for staff position
ALTER TABLE staff ADD COLUMN IF NOT EXISTS job_title VARCHAR(255);

-- ============================================================================
-- FUNCTION: GET NEXT EMPLOYEE ID (ATOMIC)
-- ============================================================================

-- This function atomically generates the next employee ID for a tenant/vendor
-- Uses INSERT ON CONFLICT (upsert) to ensure thread-safety
CREATE OR REPLACE FUNCTION get_next_employee_id(
    p_tenant_id VARCHAR(255),
    p_vendor_id VARCHAR(255),
    p_business_code VARCHAR(10)
) RETURNS VARCHAR(20) LANGUAGE plpgsql AS $$
DECLARE
    v_sequence INTEGER;
    v_vendor_id VARCHAR(255);
    v_prefix VARCHAR(10);
BEGIN
    -- Normalize NULL vendor_id to empty string for constraint matching
    v_vendor_id := NULLIF(COALESCE(p_vendor_id, ''), '');
    v_prefix := COALESCE(NULLIF(p_business_code, ''), 'EMP');

    -- Atomically insert or update the sequence
    INSERT INTO employee_id_sequences (tenant_id, vendor_id, current_sequence, prefix)
    VALUES (p_tenant_id, v_vendor_id, 1, v_prefix)
    ON CONFLICT (tenant_id, COALESCE(vendor_id, ''))
    DO UPDATE SET
        current_sequence = employee_id_sequences.current_sequence + 1,
        prefix = COALESCE(NULLIF(p_business_code, ''), employee_id_sequences.prefix),
        updated_at = NOW()
    RETURNING current_sequence INTO v_sequence;

    -- Return formatted employee ID: PREFIX-0000001
    RETURN CONCAT(v_prefix, '-', LPAD(v_sequence::TEXT, 7, '0'));
END;
$$;

-- ============================================================================
-- FUNCTION: GENERATE SLUG FROM NAME
-- ============================================================================

-- Utility function to generate slug from name (lowercase, replace spaces with hyphens)
CREATE OR REPLACE FUNCTION generate_slug(
    p_name VARCHAR(255)
) RETURNS VARCHAR(100) LANGUAGE plpgsql AS $$
DECLARE
    v_slug VARCHAR(100);
BEGIN
    -- Convert to lowercase, replace spaces and special chars with hyphens
    v_slug := LOWER(TRIM(p_name));
    v_slug := REGEXP_REPLACE(v_slug, '[^a-z0-9]+', '-', 'g');
    v_slug := REGEXP_REPLACE(v_slug, '^-|-$', '', 'g'); -- Remove leading/trailing hyphens
    v_slug := SUBSTRING(v_slug FROM 1 FOR 100); -- Limit to 100 chars

    RETURN v_slug;
END;
$$;

-- ============================================================================
-- FUNCTION: GET NEXT EMPLOYEE ID WITH VALIDATION
-- ============================================================================

-- Higher-level function that validates input and calls get_next_employee_id
CREATE OR REPLACE FUNCTION generate_employee_id(
    p_tenant_id VARCHAR(255),
    p_vendor_id VARCHAR(255),
    p_business_code VARCHAR(10)
) RETURNS VARCHAR(20) LANGUAGE plpgsql AS $$
DECLARE
    v_employee_id VARCHAR(20);
    v_code_length INTEGER;
BEGIN
    -- Validate tenant_id is provided
    IF p_tenant_id IS NULL OR TRIM(p_tenant_id) = '' THEN
        RAISE EXCEPTION 'tenant_id is required for employee ID generation';
    END IF;

    -- Validate business_code length (3-5 characters) if provided
    IF p_business_code IS NOT NULL AND TRIM(p_business_code) != '' THEN
        v_code_length := LENGTH(TRIM(p_business_code));
        IF v_code_length < 3 OR v_code_length > 5 THEN
            RAISE EXCEPTION 'business_code must be between 3 and 5 characters';
        END IF;
    END IF;

    -- Generate the employee ID
    v_employee_id := get_next_employee_id(
        TRIM(p_tenant_id),
        TRIM(p_vendor_id),
        UPPER(TRIM(p_business_code))
    );

    RETURN v_employee_id;
END;
$$;

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================

COMMENT ON TABLE employee_id_sequences IS 'Tracks auto-increment sequences for employee IDs per tenant/vendor';
COMMENT ON COLUMN employee_id_sequences.prefix IS 'Business code prefix (3-5 chars) for employee IDs';
COMMENT ON COLUMN employee_id_sequences.current_sequence IS 'Current sequence number (last used)';

COMMENT ON FUNCTION get_next_employee_id(VARCHAR, VARCHAR, VARCHAR) IS 'Atomically generates the next employee ID in format PREFIX-0000001';
COMMENT ON FUNCTION generate_employee_id(VARCHAR, VARCHAR, VARCHAR) IS 'Validates input and generates employee ID with proper business code';
COMMENT ON FUNCTION generate_slug(VARCHAR) IS 'Generates URL-safe slug from a name string';

COMMENT ON COLUMN departments.slug IS 'URL-safe auto-generated code from department name';
COMMENT ON COLUMN teams.slug IS 'URL-safe auto-generated code from team name';
COMMENT ON COLUMN staff.profile_photo_document_id IS 'Reference to profile photo in document-service';
