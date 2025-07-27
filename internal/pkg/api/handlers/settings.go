package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/services"
)

// SettingsHandler handles user settings endpoints
type SettingsHandler struct {
	settingsService *services.SettingsService
	userRepository  *database.UserRepository
	logger          *logger.Logger
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(settingsService *services.SettingsService, userRepository *database.UserRepository, logger *logger.Logger) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
		userRepository:  userRepository,
		logger:          logger,
	}
}

// UpdateSettingsRequest represents the request body for updating settings
type UpdateSettingsRequest struct {
	AutomationEnabled         bool   `json:"automation_enabled"`
	EmailNotificationsEnabled bool   `json:"email_notifications_enabled"`
	LanguagePreference        string `json:"language_preference"`
}

// UpdateSettings handles PUT /api/settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		h.logger.Error("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.logger.Info("Updating user settings",
		"user_id", userID,
		"automation_enabled", req.AutomationEnabled,
		"email_notifications_enabled", req.EmailNotificationsEnabled,
		"language_preference", req.LanguagePreference)

	// Update settings with validation
	if err := h.settingsService.UpdateUserSettings(r.Context(), userID, req.AutomationEnabled, req.EmailNotificationsEnabled, req.LanguagePreference); err != nil {
		// Check if it's a validation error using error type assertions
		if errors.Is(err, services.ErrStravaConnectionRequired) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, services.ErrSpreadsheetRequired) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, services.ErrInvalidLanguagePreference) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		h.logger.Error("Failed to update user settings", "error", err, "user_id", userID)
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	// Fetch updated user data to return
	user, err := h.userRepository.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to fetch updated user data", "error", err, "user_id", userID)
		http.Error(w, "Failed to fetch updated data", http.StatusInternalServerError)
		return
	}

	// Return updated public user data
	publicUser := user.ToPublicUser()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(publicUser); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully updated user settings", "user_id", userID)
}