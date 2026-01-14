#!/bin/bash

# Staff Service API Test Script
# This script tests all the endpoints to ensure end-to-end functionality

BASE_URL="http://localhost:8080/api/v1"
TENANT_ID="test-tenant"

echo "🧪 Testing Staff Service API Endpoints"
echo "========================================"

# Health Check
echo "1. Testing Health Check..."
curl -s -X GET "$BASE_URL/health" | jq '.'
echo ""

# Create Staff Member
echo "2. Testing Create Staff..."
STAFF_ID=$(curl -s -X POST "$BASE_URL/staff" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "phone_number": "+1234567890",
    "employee_id": "EMP001",
    "role": "employee",
    "employment_type": "full_time",
    "department_id": "IT",
    "is_active": true
  }' | jq -r '.data.id')

echo "Created staff with ID: $STAFF_ID"
echo ""

# Get Staff by ID
echo "3. Testing Get Staff by ID..."
curl -s -X GET "$BASE_URL/staff/$STAFF_ID" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Update Staff
echo "4. Testing Update Staff..."
curl -s -X PUT "$BASE_URL/staff/$STAFF_ID" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "first_name": "Jane",
    "role": "manager"
  }' | jq '.'
echo ""

# Get Staff List
echo "5. Testing Get Staff List..."
curl -s -X GET "$BASE_URL/staff?page=1&limit=10" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Search Staff
echo "6. Testing Search Staff..."
curl -s -X GET "$BASE_URL/staff?search=Jane" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Filter Staff by Role
echo "7. Testing Filter by Role..."
curl -s -X GET "$BASE_URL/staff?role=manager" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Create Second Staff for Bulk Operations
echo "8. Testing Bulk Create..."
curl -s -X POST "$BASE_URL/staff/bulk" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '[
    {
      "first_name": "Alice",
      "last_name": "Smith",
      "email": "alice.smith@example.com",
      "employee_id": "EMP002",
      "role": "employee",
      "employment_type": "full_time"
    },
    {
      "first_name": "Bob",
      "last_name": "Johnson",
      "email": "bob.johnson@example.com",
      "employee_id": "EMP003",
      "role": "intern",
      "employment_type": "intern"
    }
  ]' | jq '.'
echo ""

# Get Analytics
echo "9. Testing Analytics..."
curl -s -X GET "$BASE_URL/staff/analytics" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Get Hierarchy
echo "10. Testing Hierarchy..."
curl -s -X GET "$BASE_URL/staff/hierarchy" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Export Staff
echo "11. Testing Export..."
curl -s -X POST "$BASE_URL/staff/export?format=json" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Test Error Cases
echo "12. Testing Error Cases..."

# Invalid ID format
echo "12a. Invalid ID format:"
curl -s -X GET "$BASE_URL/staff/invalid-id" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

# Missing tenant ID
echo "12b. Missing tenant ID:"
curl -s -X GET "$BASE_URL/staff" | jq '.'
echo ""

# Duplicate email
echo "12c. Duplicate email:"
curl -s -X POST "$BASE_URL/staff" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "first_name": "Duplicate",
    "last_name": "User",
    "email": "john.doe@example.com",
    "role": "employee",
    "employment_type": "full_time"
  }' | jq '.'
echo ""

# Clean up - Delete Staff (soft delete)
echo "13. Testing Delete Staff..."
curl -s -X DELETE "$BASE_URL/staff/$STAFF_ID" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '.'
echo ""

echo "✅ All endpoint tests completed!"
echo "Check the responses above to verify functionality."