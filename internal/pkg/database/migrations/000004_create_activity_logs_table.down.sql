-- Drop activity_logs table and its indexes
DROP TABLE IF EXISTS activity_logs CASCADE;

-- Drop ENUM types
DROP TYPE IF EXISTS processing_type_enum CASCADE;
DROP TYPE IF EXISTS processing_scope_enum CASCADE;
DROP TYPE IF EXISTS processing_status_enum CASCADE;