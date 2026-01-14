-- Insert sample staff data for development and testing

-- Insert super admin
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, department_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Admin',
    'User',
    'admin@tesseract.com',
    'EMP001',
    'super_admin',
    'full_time',
    NOW() - INTERVAL '365 days',
    'IT',
    true,
    true,
    'UTC',
    'en',
    '["System Administration", "User Management", "Security"]'::jsonb,
    'system'
);

-- Insert manager
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, department_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'John',
    'Manager',
    'john.manager@tesseract.com',
    'EMP002',
    'manager',
    'full_time',
    NOW() - INTERVAL '200 days',
    'Operations',
    true,
    true,
    'America/New_York',
    'en',
    '["Team Management", "Project Planning", "Budget Management"]'::jsonb,
    'system'
);

-- Insert senior employees
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, department_id, manager_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES 
(
    '00000000-0000-0000-0000-000000000001',
    'Sarah',
    'Developer',
    'sarah.developer@tesseract.com',
    'EMP003',
    'senior_employee',
    'full_time',
    NOW() - INTERVAL '150 days',
    'Engineering',
    (SELECT id FROM staff WHERE email = 'john.manager@tesseract.com'),
    true,
    true,
    'Europe/London',
    'en',
    '["Go", "PostgreSQL", "Docker", "Kubernetes", "Microservices"]'::jsonb,
    'system'
),
(
    '00000000-0000-0000-0000-000000000001',
    'Mike',
    'DevOps',
    'mike.devops@tesseract.com',
    'EMP004',
    'senior_employee',
    'full_time',
    NOW() - INTERVAL '120 days',
    'Engineering',
    (SELECT id FROM staff WHERE email = 'john.manager@tesseract.com'),
    true,
    true,
    'Australia/Sydney',
    'en',
    '["AWS", "Docker", "Jenkins", "Terraform", "Monitoring"]'::jsonb,
    'system'
);

-- Insert regular employees
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, department_id, manager_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES 
(
    '00000000-0000-0000-0000-000000000001',
    'Alice',
    'Frontend',
    'alice.frontend@tesseract.com',
    'EMP005',
    'employee',
    'full_time',
    NOW() - INTERVAL '90 days',
    'Engineering',
    (SELECT id FROM staff WHERE email = 'sarah.developer@tesseract.com'),
    true,
    true,
    'America/Los_Angeles',
    'en',
    '["React", "TypeScript", "CSS", "UI/UX", "Testing"]'::jsonb,
    'system'
),
(
    '00000000-0000-0000-0000-000000000001',
    'Bob',
    'Backend',
    'bob.backend@tesseract.com',
    'EMP006',
    'employee',
    'full_time',
    NOW() - INTERVAL '60 days',
    'Engineering',
    (SELECT id FROM staff WHERE email = 'sarah.developer@tesseract.com'),
    true,
    true,
    'Asia/Tokyo',
    'en',
    '["Node.js", "Python", "REST APIs", "GraphQL", "Database Design"]'::jsonb,
    'system'
);

-- Insert intern
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, end_date, department_id, manager_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Emma',
    'Student',
    'emma.student@tesseract.com',
    'INT001',
    'intern',
    'intern',
    NOW() - INTERVAL '30 days',
    NOW() + INTERVAL '90 days',
    'Engineering',
    (SELECT id FROM staff WHERE email = 'alice.frontend@tesseract.com'),
    true,
    true,
    'America/Chicago',
    'en',
    '["JavaScript", "HTML", "CSS", "Learning"]'::jsonb,
    'system'
);

-- Insert contractor
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, end_date, department_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'David',
    'Consultant',
    'david.consultant@tesseract.com',
    'CON001',
    'contractor',
    'contract',
    NOW() - INTERVAL '45 days',
    NOW() + INTERVAL '120 days',
    'Consulting',
    true,
    true,
    'Europe/Berlin',
    'en',
    '["Business Analysis", "Process Optimization", "Strategy", "Data Analysis"]'::jsonb,
    'system'
);

-- Insert inactive employee
INSERT INTO staff (
    tenant_id, first_name, last_name, email, employee_id, role, employment_type,
    start_date, end_date, department_id, is_active, is_verified, timezone, locale,
    skills, created_by
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Former',
    'Employee',
    'former.employee@tesseract.com',
    'EMP999',
    'employee',
    'full_time',
    NOW() - INTERVAL '300 days',
    NOW() - INTERVAL '30 days',
    'Sales',
    false,
    true,
    'UTC',
    'en',
    '["Sales", "Customer Relations", "CRM"]'::jsonb,
    'system'
);