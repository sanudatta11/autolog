-- Drop indexes
DROP INDEX IF EXISTS idx_log_analyses_severity;
DROP INDEX IF EXISTS idx_log_analyses_incident_id;
DROP INDEX IF EXISTS idx_log_analyses_log_file_id;
DROP INDEX IF EXISTS idx_log_entries_level;
DROP INDEX IF EXISTS idx_log_entries_timestamp;
DROP INDEX IF EXISTS idx_log_entries_log_file_id;
DROP INDEX IF EXISTS idx_log_files_created_at;
DROP INDEX IF EXISTS idx_log_files_status;
DROP INDEX IF EXISTS idx_log_files_uploaded_by;

-- Drop tables
DROP TABLE IF EXISTS log_analyses;
DROP TABLE IF EXISTS log_entries;
DROP TABLE IF EXISTS log_files; 