package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/services"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// SyncServicer interface for sync operations
type SyncServicer interface {
	TriggerManualSync(ctx context.Context, userID int) (*services.SyncResponse, error)
	GetQueueStatus(ctx context.Context) (map[string]interface{}, error)
}

// SyncHandler handles manual sync HTTP requests
type SyncHandler struct {
	syncService SyncServicer
	logger      *logger.Logger
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(syncService SyncServicer, logger *logger.Logger) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
		logger:      logger.WithContext("component", "sync_handler"),
	}
}

// TriggerManualSync handles POST /api/sync requests
func (h *SyncHandler) TriggerManualSync(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Info("Manual sync API request received",
		"user_id", userID,
		"has_user_id", ok,
		"client_ip", clientIP,
		"method", r.Method,
		"user_agent", r.Header.Get("User-Agent"))

	if !ok {
		h.logger.Warn("TriggerManualSync called without valid user context",
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Trigger manual sync
	h.logger.Debug("Calling SyncService.TriggerManualSync",
		"user_id", userID)

	response, err := h.syncService.TriggerManualSync(r.Context(), userID)
	if err != nil {
		// Handle different types of sync errors
		if syncErr, ok := err.(*services.SyncError); ok {
			h.logger.Warn("SyncService returned error",
				"error_type", syncErr.Type,
				"error_message", syncErr.Message,
				"user_id", userID,
				"client_ip", clientIP)

			// Map service errors to HTTP status codes
			statusCode := h.getStatusCodeForSyncError(syncErr.Type)
			h.writeErrorResponse(w, statusCode, syncErr.Type, syncErr.Message)
			return
		}

		// Unexpected error
		h.logger.Error("Unexpected error in TriggerManualSync",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
		return
	}

	// Success response (202 Accepted)
	h.logger.Info("Manual sync triggered successfully",
		"user_id", userID,
		"client_ip", clientIP,
		"estimated_completion_seconds", response.EstimatedCompletionSeconds)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode manual sync response",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
	}
}

// GetQueueStatus handles GET /api/sync/status requests (for debugging)
func (h *SyncHandler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Debug("Queue status API request received",
		"user_id", userID,
		"has_user_id", ok,
		"client_ip", clientIP,
		"method", r.Method)

	if !ok {
		h.logger.Warn("GetQueueStatus called without valid user context",
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
		return
	}

	// Get queue status
	status, err := h.syncService.GetQueueStatus(r.Context())
	if err != nil {
		if syncErr, ok := err.(*services.SyncError); ok {
			h.logger.Warn("SyncService returned error for queue status",
				"error_type", syncErr.Type,
				"error_message", syncErr.Message,
				"user_id", userID,
				"client_ip", clientIP)

			statusCode := h.getStatusCodeForSyncError(syncErr.Type)
			h.writeErrorResponse(w, statusCode, syncErr.Type, syncErr.Message)
			return
		}

		h.logger.Error("Unexpected error in GetQueueStatus",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
		return
	}

	// Success response
	h.logger.Debug("Queue status retrieved successfully",
		"user_id", userID,
		"client_ip", clientIP,
		"queue_length", status["queue_length"])

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.Error("Failed to encode queue status response",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
	}
}

// getStatusCodeForSyncError maps sync error types to HTTP status codes
func (h *SyncHandler) getStatusCodeForSyncError(errorType string) int {
	switch errorType {
	case services.SyncErrorUserNotFound:
		return http.StatusNotFound
	case services.SyncErrorUserNotConfigured:
		return http.StatusBadRequest
	case services.SyncErrorValidation:
		return http.StatusBadRequest
	case services.SyncErrorQueueUnavailable:
		return http.StatusServiceUnavailable
	case services.SyncErrorInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// writeErrorResponse writes a standardized error response
func (h *SyncHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := ErrorResponse{
		Error:   errorCode,
		Message: message,
		Type:    errorCode,
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		h.logger.Error("Failed to encode error response",
			"error", err,
			"status_code", statusCode,
			"error_code", errorCode)
	}
}