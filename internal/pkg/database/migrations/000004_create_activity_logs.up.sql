-- Create activity_logs table to track processing outcomes for automation engine
CREATE TABLE activity_logs (
    id SERIAL PRIMARY KEY,                                    -- Auto-incrementing primary key
    user_id INTEGER NOT NULL,                                 -- Foreign key to users table
    
    -- Processing metadata
    processing_date DATE NOT NULL,                            -- The calendar date being processed
    processing_type VARCHAR(50) NOT NULL,                     -- Type of processing: 'previous_day', 'today_so_far', 'lookback_period'
    processing_scope VARCHAR(50) NOT NULL,                    -- Scope identifier: 'manual_sync', 'scheduled_daily', etc.
    
    -- Processing outcomes
    status VARCHAR(20) NOT NULL,                              -- Status: 'success', 'partial', 'failed', 'skipped'
    activities_found INTEGER DEFAULT 0,                       -- Number of Strava activities found for the day
    activities_processed INTEGER DEFAULT 0,                   -- Number of activities successfully processed
    total_distance_meters DECIMAL(10, 2),                     -- Total distance in meters
    total_duration_seconds INTEGER,                           -- Total duration in seconds
    
    -- Spreadsheet update info
    spreadsheet_row INTEGER,                                  -- Row number in spreadsheet that was updated
    spreadsheet_updated BOOLEAN DEFAULT false,                -- Whether spreadsheet was successfully updated
    description_generated TEXT,                               -- The generated description
    
    -- Error tracking
    error_message TEXT,                                       -- Error message if processing failed
    warning_messages TEXT[],                                  -- Array of warning messages
    
    -- Timing information
    processing_started_at TIMESTAMPTZ NOT NULL,               -- When processing started
    processing_completed_at TIMESTAMPTZ,                      -- When processing completed
    processing_duration_ms INTEGER,                           -- Processing duration in milliseconds
    
    -- Additional context
    metadata JSONB,                                           -- Additional metadata (e.g., activity IDs, special cases)
    
    -- Audit timestamps
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,         -- When log entry was created
    
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
CREATE INDEX idx_activity_logs_created_at ON activity_logs(created_at);                     -- Query by creation time
CREATE INDEX idx_activity_logs_user_date ON activity_logs(user_id, processing_date);        -- Composite index for user+date queries

-- Add comments for documentation
COMMENT ON TABLE activity_logs IS 'Tracks processing outcomes from the automation engine for audit and debugging purposes';
COMMENT ON COLUMN activity_logs.processing_type IS 'Type of processing: previous_day (US025), today_so_far (US028), lookback_period (US026/027)';
COMMENT ON COLUMN activity_logs.status IS 'Processing outcome: success (all good), partial (some issues), failed (error), skipped (no action needed)';
COMMENT ON COLUMN activity_logs.metadata IS 'JSON data including activity_ids, special_cases (e.g., rest_day_with_activity), and other context';