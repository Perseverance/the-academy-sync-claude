package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/services"
	authMiddleware "github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// SyncHandler handles manual sync API endpoints
type SyncHandler struct {
	syncService *services.SyncService
	logger      *logger.Logger
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(syncService *services.SyncService, logger *logger.Logger) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
		logger:      logger,
	}
}

// TriggerManualSync handles POST /api/sync requests to trigger manual sync
func (h *SyncHandler) TriggerManualSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Generate trace ID for this request
	traceID := uuid.New().String()
	
	// Extract user ID from authenticated context
	userID, ok := authMiddleware.GetUserIDFromContext(ctx)
	if !ok {
		h.logger.Error("User ID not found in authenticated context", "trace_id", traceID)
		h.writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "User authentication required")
		return
	}

	h.logger.Info("Manual sync trigger requested",
		"user_id", userID,
		"trace_id", traceID,
		"method", r.Method,
		"path", r.URL.Path)

	// Trigger the manual sync
	response, err := h.syncService.TriggerManualSync(ctx, userID, traceID)
	if err != nil {
		h.handleSyncError(w, err, traceID)
		return
	}

	// Buffer the JSON response to prevent status/body mismatches
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		h.logger.Error("Failed to encode manual sync response", "error", err, "trace_id", traceID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode response")
		return
	}

	// Write response atomically
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write(buf.Bytes())

	h.logger.Info("Manual sync triggered successfully",
		"user_id", userID,
		"job_id", response.JobID,
		"trace_id", traceID)
}

// GetQueueStatus handles GET /api/sync/status requests to get queue status
func (h *SyncHandler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Generate trace ID for this request
	traceID := uuid.New().String()
	
	// Extract user ID from authenticated context (for logging purposes)
	userID, ok := authMiddleware.GetUserIDFromContext(ctx)
	if !ok {
		h.logger.Error("User ID not found in authenticated context", "trace_id", traceID)
		h.writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "User authentication required")
		return
	}

	h.logger.Info("Queue status requested",
		"user_id", userID,
		"trace_id", traceID,
		"method", r.Method,
		"path", r.URL.Path)

	// Get queue status
	status, err := h.syncService.GetQueueStatus(ctx)
	if err != nil {
		h.handleSyncError(w, err, traceID)
		return
	}

	// Buffer the JSON response
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(status); err != nil {
		h.logger.Error("Failed to encode queue status response", "error", err, "trace_id", traceID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to encode response")
		return
	}

	// Write response atomically
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())

	h.logger.Debug("Queue status retrieved successfully",
		"user_id", userID,
		"queue_length", status.QueueLength,
		"health_status", status.HealthStatus,
		"trace_id", traceID)
}

// handleSyncError handles sync service errors and writes appropriate HTTP responses
func (h *SyncHandler) handleSyncError(w http.ResponseWriter, err error, traceID string) {
	syncErr, ok := err.(*services.SyncError)
	if !ok {
		// Generic error handling for non-SyncError types
		h.logger.Error("Unexpected error in sync handler", "error", err, "trace_id", traceID)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
		return
	}

	// Map sync error types to HTTP status codes
	var statusCode int
	var errorCode string

	switch syncErr.Type {
	case services.SyncErrorValidation, services.SyncErrorUserNotConfigured:
		statusCode = http.StatusBadRequest
		errorCode = syncErr.Code
		if errorCode == "" {
			errorCode = "USER_NOT_CONFIGURED"
		}
	case services.SyncErrorServiceUnavailable:
		statusCode = http.StatusServiceUnavailable
		errorCode = "SERVICE_UNAVAILABLE"
	case services.SyncErrorInternal:
		statusCode = http.StatusInternalServerError
		errorCode = "INTERNAL_ERROR"
	default:
		statusCode = http.StatusInternalServerError
		errorCode = "UNKNOWN_ERROR"
	}

	h.logger.Error("Sync operation failed",
		"error_type", syncErr.Type,
		"error_code", errorCode,
		"error_message", syncErr.Message,
		"status_code", statusCode,
		"trace_id", traceID)

	h.writeErrorResponse(w, statusCode, errorCode, syncErr.Message)
}

// writeErrorResponse writes a standardized error response
func (h *SyncHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	errorResponse := map[string]interface{}{
		"error":   errorCode,
		"message": message,
		"type":    errorCode, // Include type for frontend error handling
	}

	// Buffer the error response
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(errorResponse); err != nil {
		// Fallback to plain text if JSON encoding fails
		h.logger.Error("Failed to encode error response", "error", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}

	// Write error response atomically
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(buf.Bytes())
}