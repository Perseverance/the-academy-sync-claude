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
CREATE INDEX idx_activity_logs_user_id ON activity_logs(user_id);                     -- Index on user_id for user queries
CREATE INDEX idx_activity_logs_processing_date ON activity_logs(processing_date);     -- Index on date for date range queries
CREATE INDEX idx_activity_logs_status ON activity_logs(status);                       -- Index on status for filtering
CREATE INDEX idx_activity_logs_created_at ON activity_logs(created_at DESC);          -- Index on created_at for ordering