-- Remove sample vendor data
DELETE FROM vendor_payments WHERE vendor_id IN (
    'f47ac10b-58cc-4372-a567-0e02b2c3d479',
    'f47ac10b-58cc-4372-a567-0e02b2c3d480',
    'f47ac10b-58cc-4372-a567-0e02b2c3d481',
    'f47ac10b-58cc-4372-a567-0e02b2c3d482',
    'f47ac10b-58cc-4372-a567-0e02b2c3d483'
);

DELETE FROM vendor_addresses WHERE vendor_id IN (
    'f47ac10b-58cc-4372-a567-0e02b2c3d479',
    'f47ac10b-58cc-4372-a567-0e02b2c3d480',
    'f47ac10b-58cc-4372-a567-0e02b2c3d481',
    'f47ac10b-58cc-4372-a567-0e02b2c3d482',
    'f47ac10b-58cc-4372-a567-0e02b2c3d483'
);

DELETE FROM vendors WHERE id IN (
    'f47ac10b-58cc-4372-a567-0e02b2c3d479',
    'f47ac10b-58cc-4372-a567-0e02b2c3d480',
    'f47ac10b-58cc-4372-a567-0e02b2c3d481',
    'f47ac10b-58cc-4372-a567-0e02b2c3d482',
    'f47ac10b-58cc-4372-a567-0e02b2c3d483'
);