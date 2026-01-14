-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types
DO $$ BEGIN
    CREATE TYPE staff_role AS ENUM (
        'super_admin',
        'admin', 
        'manager',
        'senior_employee',
        'employee',
        'intern',
        'contractor',
        'guest',
        'readonly'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE employment_type AS ENUM (
        'full_time',
        'part_time',
        'contract',
        'temporary',
        'intern',
        'consultant',
        'volunteer'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE two_factor_method AS ENUM (
        'none',
        'sms',
        'email',
        'authenticator_app',
        'hardware_token',
        'biometric'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create staff table
CREATE TABLE staff (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    application_id VARCHAR(255),
    vendor_id VARCHAR(255),
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    middle_name VARCHAR(255),
    display_name VARCHAR(255),
    email VARCHAR(255) NOT NULL,
    alternate_email VARCHAR(255),
    phone_number VARCHAR(50),
    mobile_number VARCHAR(50),
    employee_id VARCHAR(100),
    role staff_role NOT NULL,
    employment_type employment_type NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE,
    end_date TIMESTAMP WITH TIME ZONE,
    probation_end_date TIMESTAMP WITH TIME ZONE,
    department_id VARCHAR(255),
    team_id VARCHAR(255),
    manager_id UUID REFERENCES staff(id),
    location_id VARCHAR(255),
    cost_center VARCHAR(255),
    profile_photo_url TEXT,
    timezone VARCHAR(100),
    locale VARCHAR(10),
    skills JSONB,
    certifications JSONB,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_verified BOOLEAN DEFAULT false,
    last_login_at TIMESTAMP WITH TIME ZONE,
    last_activity_at TIMESTAMP WITH TIME ZONE,
    failed_login_attempts INTEGER DEFAULT 0,
    account_locked_until TIMESTAMP WITH TIME ZONE,
    password_last_changed_at TIMESTAMP WITH TIME ZONE,
    password_expires_at TIMESTAMP WITH TIME ZONE,
    two_factor_enabled BOOLEAN DEFAULT false,
    two_factor_method two_factor_method DEFAULT 'none',
    allowed_ip_ranges JSONB,
    trusted_devices JSONB,
    custom_fields JSONB,
    tags JSONB,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Create indexes for performance
CREATE INDEX idx_staff_tenant_id ON staff(tenant_id);
CREATE INDEX idx_staff_email ON staff(email);
CREATE INDEX idx_staff_employee_id ON staff(employee_id);
CREATE INDEX idx_staff_role ON staff(role);
CREATE INDEX idx_staff_employment_type ON staff(employment_type);
CREATE INDEX idx_staff_department_id ON staff(department_id);
CREATE INDEX idx_staff_manager_id ON staff(manager_id);
CREATE INDEX idx_staff_is_active ON staff(is_active);
CREATE INDEX idx_staff_created_at ON staff(created_at);
CREATE INDEX idx_staff_deleted_at ON staff(deleted_at);

-- Create unique constraints
CREATE UNIQUE INDEX idx_tenant_email ON staff(tenant_id, email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_tenant_employee_id ON staff(tenant_id, employee_id) WHERE deleted_at IS NULL AND employee_id IS NOT NULL;

-- Create GIN indexes for JSONB fields
CREATE INDEX idx_staff_skills_gin ON staff USING GIN(skills);
CREATE INDEX idx_staff_certifications_gin ON staff USING GIN(certifications);
CREATE INDEX idx_staff_custom_fields_gin ON staff USING GIN(custom_fields);
CREATE INDEX idx_staff_tags_gin ON staff USING GIN(tags);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_staff_updated_at 
    BEFORE UPDATE ON staff 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();