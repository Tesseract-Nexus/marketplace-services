-- Migration: 027_grant_store_owner_all_permissions
-- Description: Re-run store_owner permission grant to catch new tenants and new permissions
-- This fixes permission gaps for tenants onboarded after migration 023

-- Grant ALL permissions to store_owner/owner roles for ALL tenants
-- Uses CROSS JOIN to ensure every permission is granted
-- ON CONFLICT ensures idempotency
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
  AND sp.is_active = true
ON CONFLICT (role_id, permission_id) DO NOTHING;
