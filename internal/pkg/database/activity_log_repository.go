package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// ActivityLogRepository handles activity log persistence
type ActivityLogRepository struct {
	db     *sqlx.DB
	logger *logger.Logger
}

// NewActivityLogRepository creates a new activity log repository
func NewActivityLogRepository(db *sqlx.DB, logger *logger.Logger) *ActivityLogRepository {
	return &ActivityLogRepository{
		db:     db,
		logger: logger.WithContext("component", "activity_log_repository"),
	}
}

// CreateActivityLog creates a new activity log entry
func (r *ActivityLogRepository) CreateActivityLog(ctx context.Context, log *ActivityLog) error {

	query := `
		INSERT INTO activity_logs (
			user_id,
			processing_date,
			processing_type,
			processing_scope,
			status,
			activities_found,
			activities_processed,
			total_distance_meters,
			total_duration_seconds,
			spreadsheet_row,
			spreadsheet_updated,
			description_generated,
			error_message,
			warning_messages,
			processing_started_at,
			processing_completed_at,
			processing_duration_ms,
			metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		) RETURNING id, created_at`

	// Convert metadata to JSON if present
	var metadataJSON interface{}
	if log.Metadata != nil && len(log.Metadata) > 0 {
		var err error
		jsonBytes, err := json.Marshal(log.Metadata)
		if err != nil {
			r.logger.Error("Failed to marshal metadata", "error", err)
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = jsonBytes
	} else {
		// Use SQL NULL instead of empty string for metadata
		metadataJSON = nil
	}

	// Execute the insert
	err := r.db.QueryRowContext(
		ctx,
		query,
		log.UserID,
		log.ProcessingDate,
		log.ProcessingType,
		log.ProcessingScope,
		log.Status,
		log.ActivitiesFound,
		log.ActivitiesProcessed,
		log.TotalDistanceMeters,
		log.TotalDurationSeconds,
		log.SpreadsheetRow,
		log.SpreadsheetUpdated,
		log.DescriptionGenerated,
		log.ErrorMessage,
		pq.Array(log.WarningMessages),
		log.ProcessingStartedAt,
		log.ProcessingCompletedAt,
		log.ProcessingDurationMs,
		metadataJSON,
	).Scan(&log.ID, &log.CreatedAt)

	if err != nil {
		r.logger.Error("Failed to create activity log",
			"error", err,
			"user_id", log.UserID,
			"processing_date", log.ProcessingDate,
			"status", log.Status)
		return fmt.Errorf("failed to create activity log: %w", err)
	}

	r.logger.Debug("Activity log created successfully",
		"id", log.ID,
		"user_id", log.UserID,
		"processing_date", log.ProcessingDate,
		"status", log.Status)

	return nil
}

// GetRecentLogs retrieves recent activity logs for a user
func (r *ActivityLogRepository) GetRecentLogs(ctx context.Context, userID int, limit int) ([]ActivityLog, error) {
	if limit <= 0 {
		limit = 30 // Default to 30 days
	}

	query := `
		SELECT 
			id,
			user_id,
			processing_date,
			processing_type,
			processing_scope,
			status,
			activities_found,
			activities_processed,
			total_distance_meters,
			total_duration_seconds,
			spreadsheet_row,
			spreadsheet_updated,
			description_generated,
			error_message,
			warning_messages,
			processing_started_at,
			processing_completed_at,
			processing_duration_ms,
			metadata,
			created_at
		FROM activity_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	var logs []ActivityLog
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		r.logger.Error("Failed to query activity logs",
			"error", err,
			"user_id", userID,
			"limit", limit)
		return nil, fmt.Errorf("failed to query activity logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var log ActivityLog
		var metadataJSON sql.NullString
		var warningMessages pq.StringArray

		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.ProcessingDate,
			&log.ProcessingType,
			&log.ProcessingScope,
			&log.Status,
			&log.ActivitiesFound,
			&log.ActivitiesProcessed,
			&log.TotalDistanceMeters,
			&log.TotalDurationSeconds,
			&log.SpreadsheetRow,
			&log.SpreadsheetUpdated,
			&log.DescriptionGenerated,
			&log.ErrorMessage,
			&warningMessages,
			&log.ProcessingStartedAt,
			&log.ProcessingCompletedAt,
			&log.ProcessingDurationMs,
			&metadataJSON,
			&log.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan activity log row", "error", err)
			return nil, fmt.Errorf("failed to scan activity log: %w", err)
		}

		// Convert warning messages
		log.WarningMessages = []string(warningMessages)

		// Unmarshal metadata if present
		if metadataJSON.Valid && len(metadataJSON.String) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				r.logger.Warn("Failed to unmarshal metadata", "error", err, "log_id", log.ID)
				// Continue without metadata rather than failing
			} else {
				log.Metadata = metadata
			}
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating activity log rows", "error", err)
		return nil, fmt.Errorf("error iterating activity logs: %w", err)
	}

	r.logger.Debug("Retrieved activity logs",
		"user_id", userID,
		"count", len(logs))

	return logs, nil
}

// GetActivityLogByID retrieves a single activity log by ID
func (r *ActivityLogRepository) GetActivityLogByID(ctx context.Context, logID int) (*ActivityLog, error) {
	query := `
		SELECT 
			id,
			user_id,
			processing_date,
			processing_type,
			processing_scope,
			status,
			activities_found,
			activities_processed,
			total_distance_meters,
			total_duration_seconds,
			spreadsheet_row,
			spreadsheet_updated,
			description_generated,
			error_message,
			warning_messages,
			processing_started_at,
			processing_completed_at,
			processing_duration_ms,
			metadata,
			created_at
		FROM activity_logs
		WHERE id = $1`

	var log ActivityLog
	var metadataJSON sql.NullString
	var warningMessages pq.StringArray

	err := r.db.QueryRowContext(ctx, query, logID).Scan(
		&log.ID,
		&log.UserID,
		&log.ProcessingDate,
		&log.ProcessingType,
		&log.ProcessingScope,
		&log.Status,
		&log.ActivitiesFound,
		&log.ActivitiesProcessed,
		&log.TotalDistanceMeters,
		&log.TotalDurationSeconds,
		&log.SpreadsheetRow,
		&log.SpreadsheetUpdated,
		&log.DescriptionGenerated,
		&log.ErrorMessage,
		&warningMessages,
		&log.ProcessingStartedAt,
		&log.ProcessingCompletedAt,
		&log.ProcessingDurationMs,
		&metadataJSON,
		&log.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get activity log by ID", "error", err, "log_id", logID)
		return nil, fmt.Errorf("failed to get activity log: %w", err)
	}

	// Convert warning messages
	log.WarningMessages = []string(warningMessages)

	// Unmarshal metadata if present
	if metadataJSON.Valid && len(metadataJSON.String) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
			r.logger.Warn("Failed to unmarshal metadata", "error", err, "log_id", log.ID)
		} else {
			log.Metadata = metadata
		}
	}

	return &log, nil
}

// GetRecentLogsByDateRange retrieves activity logs within a date range
func (r *ActivityLogRepository) GetRecentLogsByDateRange(ctx context.Context, userID int, startDate, endDate time.Time) ([]ActivityLog, error) {
	query := `
		SELECT 
			id,
			user_id,
			processing_date,
			processing_type,
			processing_scope,
			status,
			activities_found,
			activities_processed,
			total_distance_meters,
			total_duration_seconds,
			spreadsheet_row,
			spreadsheet_updated,
			description_generated,
			error_message,
			warning_messages,
			processing_started_at,
			processing_completed_at,
			processing_duration_ms,
			metadata,
			created_at
		FROM activity_logs
		WHERE user_id = $1 
		  AND processing_date >= $2 
		  AND processing_date <= $3
		ORDER BY processing_date DESC, created_at DESC`

	var logs []ActivityLog
	rows, err := r.db.QueryContext(ctx, query, userID, startDate, endDate)
	if err != nil {
		r.logger.Error("Failed to query activity logs by date range",
			"error", err,
			"user_id", userID,
			"start_date", startDate,
			"end_date", endDate)
		return nil, fmt.Errorf("failed to query activity logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var log ActivityLog
		var metadataJSON sql.NullString
		var warningMessages pq.StringArray

		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.ProcessingDate,
			&log.ProcessingType,
			&log.ProcessingScope,
			&log.Status,
			&log.ActivitiesFound,
			&log.ActivitiesProcessed,
			&log.TotalDistanceMeters,
			&log.TotalDurationSeconds,
			&log.SpreadsheetRow,
			&log.SpreadsheetUpdated,
			&log.DescriptionGenerated,
			&log.ErrorMessage,
			&warningMessages,
			&log.ProcessingStartedAt,
			&log.ProcessingCompletedAt,
			&log.ProcessingDurationMs,
			&metadataJSON,
			&log.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan activity log row", "error", err)
			return nil, fmt.Errorf("failed to scan activity log: %w", err)
		}

		// Convert warning messages
		log.WarningMessages = []string(warningMessages)

		// Unmarshal metadata if present
		if metadataJSON.Valid && len(metadataJSON.String) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				r.logger.Warn("Failed to unmarshal metadata", "error", err, "log_id", log.ID)
			} else {
				log.Metadata = metadata
			}
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating activity log rows", "error", err)
		return nil, fmt.Errorf("error iterating activity logs: %w", err)
	}

	r.logger.Debug("Retrieved activity logs by date range",
		"user_id", userID,
		"count", len(logs),
		"start_date", startDate,
		"end_date", endDate)

	return logs, nil
}

// GetSuccessfulLogForDate retrieves a successful activity log for a specific date with zero activities
func (r *ActivityLogRepository) GetSuccessfulLogForDate(ctx context.Context, userID int, processingDate time.Time, processingType string) (*ActivityLog, error) {
	query := `
		SELECT 
			id,
			user_id,
			processing_date,
			processing_type,
			processing_scope,
			status,
			activities_found,
			activities_processed,
			total_distance_meters,
			total_duration_seconds,
			spreadsheet_row,
			spreadsheet_updated,
			description_generated,
			error_message,
			warning_messages,
			processing_started_at,
			processing_completed_at,
			processing_duration_ms,
			metadata,
			created_at
		FROM activity_logs
		WHERE user_id = $1 
		  AND processing_date = $2
		  AND processing_type = $3
		  AND status = 'success'
		  AND activities_found = 0
		ORDER BY created_at DESC
		LIMIT 1`

	var log ActivityLog
	var metadataJSON sql.NullString
	var warningMessages pq.StringArray

	err := r.db.QueryRowContext(ctx, query, userID, processingDate, processingType).Scan(
		&log.ID,
		&log.UserID,
		&log.ProcessingDate,
		&log.ProcessingType,
		&log.ProcessingScope,
		&log.Status,
		&log.ActivitiesFound,
		&log.ActivitiesProcessed,
		&log.TotalDistanceMeters,
		&log.TotalDurationSeconds,
		&log.SpreadsheetRow,
		&log.SpreadsheetUpdated,
		&log.DescriptionGenerated,
		&log.ErrorMessage,
		&warningMessages,
		&log.ProcessingStartedAt,
		&log.ProcessingCompletedAt,
		&log.ProcessingDurationMs,
		&metadataJSON,
		&log.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get successful log for date", 
			"error", err, 
			"user_id", userID,
			"processing_date", processingDate,
			"processing_type", processingType)
		return nil, fmt.Errorf("failed to get successful log for date: %w", err)
	}

	// Convert warning messages
	log.WarningMessages = []string(warningMessages)

	// Unmarshal metadata if present
	if metadataJSON.Valid && len(metadataJSON.String) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
			r.logger.Warn("Failed to unmarshal metadata", "error", err, "log_id", log.ID)
		} else {
			log.Metadata = metadata
		}
	}

	r.logger.Debug("Found existing successful log for date",
		"user_id", userID,
		"processing_date", processingDate,
		"processing_type", processingType,
		"log_id", log.ID)

	return &log, nil
}