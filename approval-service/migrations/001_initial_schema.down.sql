-- Rollback: Drop approval workflow tables

DROP TABLE IF EXISTS approval_delegations;
DROP TABLE IF EXISTS approval_scheduled_events;
DROP TABLE IF EXISTS approval_audit_log;
DROP TABLE IF EXISTS approval_decisions;
DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS approval_workflows;
