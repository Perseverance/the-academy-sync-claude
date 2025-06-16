package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/services"
)

// SyncServiceInterface defines the interface for sync operations
type SyncServiceInterface interface {
	TriggerManualSync(ctx context.Context, userID int) error
	GetUserSyncStatus(ctx context.Context, userID int) (bool, string, error)
}

// SyncHandler handles sync-related API endpoints
type SyncHandler struct {
	syncService SyncServiceInterface
	logger      *logger.Logger
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(syncService *services.SyncService, logger *logger.Logger) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
		logger:      logger.WithContext("component", "sync_handler"),
	}
}

// TriggerManualSync handles POST /api/sync
// It enqueues a manual sync job for the authenticated user
func (h *SyncHandler) TriggerManualSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user from context (set by auth middleware)
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		h.logger.Error("Failed to extract user ID from context")
		h.sendJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	email, _ := middleware.GetEmailFromContext(ctx)
	clientIP := middleware.GetClientIP(r)
	h.logger.Debug("Manual sync requested",
		"user_id", userID,
		"email", email,
		"client_ip", clientIP,
		"method", r.Method,
		"path", r.URL.Path)

	// Trigger the sync
	err := h.syncService.TriggerManualSync(ctx, userID)
	if err != nil {
		h.logger.Warn("Manual sync trigger failed",
			"error", err,
			"user_id", userID,
			"email", email,
			"client_ip", clientIP)

		// Determine appropriate error response
		switch {
		case errors.Is(err, services.ErrUserNotFound):
			h.sendJSONError(w, "User not found", http.StatusNotFound)
		case errors.Is(err, services.ErrStravaConnectionRequired):
			h.sendJSONError(w, "Strava connection required. Please connect your Strava account first.", http.StatusBadRequest)
		case errors.Is(err, services.ErrSpreadsheetRequired):
			h.sendJSONError(w, "Spreadsheet configuration required. Please configure your Google Spreadsheet first.", http.StatusBadRequest)
		case errors.Is(err, services.ErrSyncAlreadyInProgress):
			h.sendJSONError(w, "A sync is already in progress for your account. Please wait for it to complete.", http.StatusConflict)
		default:
			// Check if it's a connection error
			if queue.IsConnectionError(err) {
				h.logger.Error("Redis connection error during manual sync",
					"error", err,
					"user_id", userID)
				h.sendJSONError(w, "Service temporarily unavailable. Please try again later.", http.StatusServiceUnavailable)
			} else {
				h.sendJSONError(w, "Failed to trigger sync. Please try again.", http.StatusInternalServerError)
			}
		}
		return
	}

	h.logger.Info("Manual sync triggered successfully",
		"user_id", userID,
		"email", email,
		"client_ip", clientIP)

	// Return 202 Accepted with simple success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	
	response := map[string]interface{}{
		"status": "accepted",
		"message": "Sync request has been queued for processing",
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response",
			"error", err,
			"user_id", userID)
	}
}

// GetSyncStatus handles GET /api/sync/status
// It returns whether the user is eligible for sync
func (h *SyncHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		h.logger.Error("Failed to extract user ID from context")
		h.sendJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	email, _ := middleware.GetEmailFromContext(ctx)
	h.logger.Debug("Sync status requested",
		"user_id", userID,
		"email", email)

	// Check sync eligibility
	eligible, reason, err := h.syncService.GetUserSyncStatus(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get sync status",
			"error", err,
			"user_id", userID)
		h.sendJSONError(w, "Failed to get sync status", http.StatusInternalServerError)
		return
	}

	// Return status
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"eligible": eligible,
		"reason":   reason,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response",
			"error", err,
			"user_id", userID)
	}
}

// sendJSONError sends a JSON error response
func (h *SyncHandler) sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorResponse := map[string]interface{}{
		"error":  message,
		"status": statusCode,
	}
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		h.logger.Error("Failed to encode error response", "error", err)
		// Fallback to plain text error
		http.Error(w, message, statusCode)
	}
}