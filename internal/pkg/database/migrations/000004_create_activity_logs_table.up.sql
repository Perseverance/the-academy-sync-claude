-- Create activity_logs table to track activity processing history
-- Create ENUM types for constrained values
CREATE TYPE processing_type_enum AS ENUM ('daily', 'manual', 'backfill');
CREATE TYPE processing_scope_enum AS ENUM ('single_day', 'date_range');
CREATE TYPE processing_status_enum AS ENUM ('success', 'failed', 'skipped', 'in_progress');

CREATE TABLE activity_logs (
    id BIGSERIAL PRIMARY KEY,                                   -- Auto-incrementing primary key (using BIGSERIAL for larger range)
    user_id INTEGER NOT NULL,                                   -- Foreign key to users table
    
    -- Processing information
    processing_date DATE NOT NULL,                              -- Date being processed (e.g., 2024-01-15)
    processing_type processing_type_enum NOT NULL,              -- Type of processing (constrained to enum values)
    processing_scope processing_scope_enum NOT NULL,            -- Scope of processing (constrained to enum values)
    status processing_status_enum NOT NULL,                     -- Processing status (constrained to enum values)
    
    -- Activity statistics
    activities_found INTEGER DEFAULT 0,                         -- Number of activities found from Strava
    activities_processed INTEGER DEFAULT 0,                     -- Number of activities successfully processed
    total_distance_meters NUMERIC(12, 2),                       -- Total distance in meters
    total_duration_seconds INTEGER,                             -- Total duration in seconds
    
    -- Spreadsheet integration results
    spreadsheet_row INTEGER,                                    -- Row number in spreadsheet
    spreadsheet_updated BOOLEAN DEFAULT false,                  -- Whether spreadsheet was updated
    description_generated TEXT,                                 -- Generated description for the day
    
    -- Error handling
    error_message TEXT,                                         -- Error message if processing failed
    warning_messages TEXT[],                                    -- Array of warning messages
    
    -- Processing metadata
    processing_started_at TIMESTAMPTZ NOT NULL,                 -- When processing started
    processing_completed_at TIMESTAMPTZ,                        -- When processing completed
    processing_duration_ms INTEGER,                             -- Processing duration in milliseconds
    
    -- Additional metadata as JSONB
    metadata JSONB DEFAULT '{}'::jsonb,                         -- Flexible field for additional data
    
    -- Audit timestamp
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,           -- When log entry was created
    
    -- Foreign key constraint
    CONSTRAINT fk_activity_logs_user_id 
        FOREIGN KEY (user_id) 
        REFERENCES users(id) 
        ON DELETE CASCADE
);

-- Create indexes for efficient queries
CREATE INDEX idx_activity_logs_user_id ON activity_logs(user_id);                           -- Query logs by user
CREATE INDEX idx_activity_logs_processing_date ON activity_logs(processing_date);           -- Query by date
CREATE INDEX idx_activity_logs_status ON activity_logs(status);                             -- Query by status
CREATE INDEX idx_activity_logs_created_at ON activity_logs(created_at DESC);                -- Query by creation time
CREATE INDEX idx_activity_logs_user_date ON activity_logs(user_id, processing_date);        -- Composite index for user+date queries

-- Add comments for documentation
COMMENT ON TABLE activity_logs IS 'Tracks processing outcomes from the automation engine for audit and debugging purposes';
COMMENT ON COLUMN activity_logs.processing_type IS 'Type of processing: daily (US025), manual (US028), backfill (US026/027)';
COMMENT ON COLUMN activity_logs.processing_scope IS 'Scope of processing: single_day (one specific date), date_range (multiple dates)';
COMMENT ON COLUMN activity_logs.status IS 'Processing outcome: success (all good), failed (error), skipped (no action needed), in_progress (currently processing)';
COMMENT ON COLUMN activity_logs.metadata IS 'JSON data including activity_ids, special_cases (e.g., rest_day_with_activity), and other context';