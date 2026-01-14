-- Drop triggers
DROP TRIGGER IF EXISTS ensure_single_default_payment_trigger ON vendor_payments;
DROP TRIGGER IF EXISTS ensure_single_default_address_trigger ON vendor_addresses;
DROP TRIGGER IF EXISTS update_vendor_payments_updated_at ON vendor_payments;
DROP TRIGGER IF EXISTS update_vendor_addresses_updated_at ON vendor_addresses;
DROP TRIGGER IF EXISTS update_vendors_updated_at ON vendors;

-- Drop functions
DROP FUNCTION IF EXISTS ensure_single_default_payment();
DROP FUNCTION IF EXISTS ensure_single_default_address();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_vendors_tags_gin;
DROP INDEX IF EXISTS idx_vendors_custom_fields_gin;
DROP INDEX IF EXISTS idx_vendors_insurance_info_gin;
DROP INDEX IF EXISTS idx_vendors_compliance_documents_gin;
DROP INDEX IF EXISTS idx_vendors_certifications_gin;
DROP INDEX IF EXISTS idx_vendor_payments_default;
DROP INDEX IF EXISTS idx_vendor_addresses_default;
DROP INDEX IF EXISTS idx_vendors_tenant_email;
DROP INDEX IF EXISTS idx_vendor_payments_is_verified;
DROP INDEX IF EXISTS idx_vendor_payments_is_default;
DROP INDEX IF EXISTS idx_vendor_payments_method;
DROP INDEX IF EXISTS idx_vendor_payments_vendor_id;
DROP INDEX IF EXISTS idx_vendor_addresses_is_default;
DROP INDEX IF EXISTS idx_vendor_addresses_type;
DROP INDEX IF EXISTS idx_vendor_addresses_vendor_id;
DROP INDEX IF EXISTS idx_vendors_performance_rating;
DROP INDEX IF EXISTS idx_vendors_contract_end_date;
DROP INDEX IF EXISTS idx_vendors_deleted_at;
DROP INDEX IF EXISTS idx_vendors_created_at;
DROP INDEX IF EXISTS idx_vendors_is_active;
DROP INDEX IF EXISTS idx_vendors_business_type;
DROP INDEX IF EXISTS idx_vendors_location;
DROP INDEX IF EXISTS idx_vendors_validation_status;
DROP INDEX IF EXISTS idx_vendors_status;
DROP INDEX IF EXISTS idx_vendors_email;
DROP INDEX IF EXISTS idx_vendors_tenant_id;

-- Drop tables
DROP TABLE IF EXISTS vendor_payments;
DROP TABLE IF EXISTS vendor_addresses;
DROP TABLE IF EXISTS vendors;

-- Drop enum types
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS address_type;
DROP TYPE IF EXISTS validation_status;
DROP TYPE IF EXISTS vendor_status;