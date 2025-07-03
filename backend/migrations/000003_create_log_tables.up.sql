-- Create log_files table
CREATE TABLE log_files (
    id SERIAL PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    uploaded_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'pending',
    entry_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    warning_count INTEGER DEFAULT 0,
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create log_entries table
CREATE TABLE log_entries (
    id SERIAL PRIMARY KEY,
    log_file_id INTEGER NOT NULL REFERENCES log_files(id) ON DELETE CASCADE,
    timestamp TIMESTAMP,
    level VARCHAR(20),
    message TEXT,
    raw_data TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create log_analyses table
CREATE TABLE log_analyses (
    id SERIAL PRIMARY KEY,
    log_file_id INTEGER NOT NULL REFERENCES log_files(id) ON DELETE CASCADE,
    incident_id INTEGER REFERENCES incidents(id) ON DELETE SET NULL,
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    summary TEXT,
    severity VARCHAR(20),
    error_count INTEGER DEFAULT 0,
    warning_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX idx_log_files_uploaded_by ON log_files(uploaded_by);
CREATE INDEX idx_log_files_status ON log_files(status);
CREATE INDEX idx_log_files_created_at ON log_files(created_at);
CREATE INDEX idx_log_entries_log_file_id ON log_entries(log_file_id);
CREATE INDEX idx_log_entries_timestamp ON log_entries(timestamp);
CREATE INDEX idx_log_entries_level ON log_entries(level);
CREATE INDEX idx_log_analyses_log_file_id ON log_analyses(log_file_id);
CREATE INDEX idx_log_analyses_incident_id ON log_analyses(incident_id);
CREATE INDEX idx_log_analyses_severity ON log_analyses(severity); 