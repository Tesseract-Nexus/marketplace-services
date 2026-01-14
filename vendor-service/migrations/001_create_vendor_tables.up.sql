-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enum types
DO $$ BEGIN
    CREATE TYPE vendor_status AS ENUM (
        'PENDING',
        'ACTIVE',
        'INACTIVE',
        'SUSPENDED',
        'TERMINATED'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE validation_status AS ENUM (
        'NOT_STARTED',
        'IN_PROGRESS',
        'COMPLETED',
        'FAILED',
        'EXPIRED'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE address_type AS ENUM (
        'BUSINESS',
        'WAREHOUSE',
        'RETURNS'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE payment_method AS ENUM (
        'BANK_TRANSFER',
        'WIRE',
        'CHECK',
        'ACH'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create vendors table
CREATE TABLE vendors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    details TEXT,
    status vendor_status NOT NULL DEFAULT 'PENDING',
    location VARCHAR(255),
    primary_contact VARCHAR(255) NOT NULL,
    secondary_contact VARCHAR(255),
    email VARCHAR(255) NOT NULL,
    validation_status validation_status NOT NULL DEFAULT 'NOT_STARTED',
    commission_rate DECIMAL(5,2) NOT NULL DEFAULT 0.0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    
    -- Business Information
    business_registration_number VARCHAR(255),
    tax_identification_number VARCHAR(255),
    website VARCHAR(255),
    business_type VARCHAR(255),
    founded_year INTEGER,
    employee_count INTEGER,
    annual_revenue DECIMAL(15,2),
    
    -- Contract Information
    contract_start_date TIMESTAMP WITH TIME ZONE,
    contract_end_date TIMESTAMP WITH TIME ZONE,
    contract_renewal_date TIMESTAMP WITH TIME ZONE,
    contract_value DECIMAL(15,2),
    payment_terms VARCHAR(255),
    service_level VARCHAR(255),
    
    -- Compliance and Certifications
    certifications JSONB,
    compliance_documents JSONB,
    insurance_info JSONB,
    
    -- Performance Metrics
    performance_rating DECIMAL(3,2),
    last_review_date TIMESTAMP WITH TIME ZONE,
    next_review_date TIMESTAMP WITH TIME ZONE,
    
    -- Flexible Fields
    custom_fields JSONB,
    tags JSONB,
    notes TEXT,
    
    -- Audit Fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- Create vendor_addresses table
CREATE TABLE vendor_addresses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    vendor_id UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    address_type address_type NOT NULL,
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255),
    city VARCHAR(255) NOT NULL,
    state VARCHAR(255) NOT NULL,
    postal_code VARCHAR(50) NOT NULL,
    country VARCHAR(255) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create vendor_payments table
CREATE TABLE vendor_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    vendor_id UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    account_holder_name VARCHAR(255) NOT NULL,
    bank_name VARCHAR(255) NOT NULL,
    account_number VARCHAR(255) NOT NULL,
    routing_number VARCHAR(255),
    swift_code VARCHAR(255),
    tax_identifier VARCHAR(255),
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    payment_method payment_method NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for performance
-- Vendors table indexes
CREATE INDEX idx_vendors_tenant_id ON vendors(tenant_id);
CREATE INDEX idx_vendors_email ON vendors(email);
CREATE INDEX idx_vendors_status ON vendors(status);
CREATE INDEX idx_vendors_validation_status ON vendors(validation_status);
CREATE INDEX idx_vendors_location ON vendors(location);
CREATE INDEX idx_vendors_business_type ON vendors(business_type);
CREATE INDEX idx_vendors_is_active ON vendors(is_active);
CREATE INDEX idx_vendors_created_at ON vendors(created_at);
CREATE INDEX idx_vendors_deleted_at ON vendors(deleted_at);
CREATE INDEX idx_vendors_contract_end_date ON vendors(contract_end_date);
CREATE INDEX idx_vendors_performance_rating ON vendors(performance_rating);

-- Vendor addresses table indexes
CREATE INDEX idx_vendor_addresses_vendor_id ON vendor_addresses(vendor_id);
CREATE INDEX idx_vendor_addresses_type ON vendor_addresses(address_type);
CREATE INDEX idx_vendor_addresses_is_default ON vendor_addresses(is_default);

-- Vendor payments table indexes
CREATE INDEX idx_vendor_payments_vendor_id ON vendor_payments(vendor_id);
CREATE INDEX idx_vendor_payments_method ON vendor_payments(payment_method);
CREATE INDEX idx_vendor_payments_is_default ON vendor_payments(is_default);
CREATE INDEX idx_vendor_payments_is_verified ON vendor_payments(is_verified);

-- Create unique constraints
CREATE UNIQUE INDEX idx_vendors_tenant_email ON vendors(tenant_id, email) WHERE deleted_at IS NULL;

-- Ensure only one default address per vendor per type
CREATE UNIQUE INDEX idx_vendor_addresses_default ON vendor_addresses(vendor_id, address_type) 
WHERE is_default = true;

-- Ensure only one default payment per vendor
CREATE UNIQUE INDEX idx_vendor_payments_default ON vendor_payments(vendor_id) 
WHERE is_default = true;

-- Create GIN indexes for JSONB fields
CREATE INDEX idx_vendors_certifications_gin ON vendors USING GIN(certifications);
CREATE INDEX idx_vendors_compliance_documents_gin ON vendors USING GIN(compliance_documents);
CREATE INDEX idx_vendors_insurance_info_gin ON vendors USING GIN(insurance_info);
CREATE INDEX idx_vendors_custom_fields_gin ON vendors USING GIN(custom_fields);
CREATE INDEX idx_vendors_tags_gin ON vendors USING GIN(tags);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
CREATE TRIGGER update_vendors_updated_at 
    BEFORE UPDATE ON vendors 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vendor_addresses_updated_at 
    BEFORE UPDATE ON vendor_addresses 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vendor_payments_updated_at 
    BEFORE UPDATE ON vendor_payments 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create function to ensure only one default address per type
CREATE OR REPLACE FUNCTION ensure_single_default_address()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_default = true THEN
        UPDATE vendor_addresses 
        SET is_default = false 
        WHERE vendor_id = NEW.vendor_id 
          AND address_type = NEW.address_type 
          AND id != NEW.id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create function to ensure only one default payment
CREATE OR REPLACE FUNCTION ensure_single_default_payment()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_default = true THEN
        UPDATE vendor_payments 
        SET is_default = false 
        WHERE vendor_id = NEW.vendor_id 
          AND id != NEW.id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for default constraints
CREATE TRIGGER ensure_single_default_address_trigger
    BEFORE INSERT OR UPDATE ON vendor_addresses
    FOR EACH ROW EXECUTE FUNCTION ensure_single_default_address();

CREATE TRIGGER ensure_single_default_payment_trigger
    BEFORE INSERT OR UPDATE ON vendor_payments
    FOR EACH ROW EXECUTE FUNCTION ensure_single_default_payment();