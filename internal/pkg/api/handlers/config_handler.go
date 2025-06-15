package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/services"
)

// ConfigHandler handles configuration-related HTTP requests
type ConfigHandler struct {
	configService *services.ConfigService
	logger        *logger.Logger
}

// NewConfigHandler creates a new configuration handler
func NewConfigHandler(configService *services.ConfigService, logger *logger.Logger) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
		logger:        logger.WithContext("component", "config_handler"),
	}
}

// SetSpreadsheetRequest represents the request body for setting a spreadsheet URL
type SetSpreadsheetRequest struct {
	URL string `json:"url"`
}

// SetSpreadsheetResponse represents the response for spreadsheet configuration
type SetSpreadsheetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// SetSpreadsheet handles POST /api/config/spreadsheet requests
func (h *ConfigHandler) SetSpreadsheet(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Info("SetSpreadsheet API request received",
		"user_id", userID,
		"has_user_id", ok,
		"client_ip", clientIP,
		"method", r.Method,
		"user_agent", r.Header.Get("User-Agent"))

	if !ok {
		h.logger.Warn("SetSpreadsheet called without valid user context",
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", "")
		return
	}

	// Parse request body
	var req SetSpreadsheetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid JSON in SetSpreadsheet request",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body", "")
		return
	}

	h.logger.Debug("Parsed SetSpreadsheet request",
		"user_id", userID,
		"url_provided", req.URL != "",
		"url_length", len(req.URL))

	// Validate request
	if strings.TrimSpace(req.URL) == "" {
		h.logger.Warn("Empty URL provided in SetSpreadsheet request",
			"user_id", userID,
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusBadRequest, "EMPTY_URL", "Spreadsheet URL cannot be empty", "")
		return
	}

	// Call service to set spreadsheet
	h.logger.Debug("Calling ConfigService.SetSpreadsheetURL",
		"user_id", userID)

	err := h.configService.SetSpreadsheetURL(r.Context(), userID, req.URL)
	if err != nil {
		// Handle different types of configuration errors
		if configErr, ok := err.(*services.ConfigError); ok {
			h.logger.Warn("ConfigService returned error",
				"error_type", configErr.Type,
				"error_message", configErr.Message,
				"user_id", userID,
				"client_ip", clientIP)

			// Map service errors to HTTP status codes
			statusCode := h.getStatusCodeForConfigError(configErr.Type)
			h.writeErrorResponse(w, statusCode, configErr.Type, configErr.Message, configErr.Type)
			return
		}

		// Unexpected error
		h.logger.Error("Unexpected error in SetSpreadsheet",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", "")
		return
	}

	// Success response
	h.logger.Info("SetSpreadsheet completed successfully",
		"user_id", userID,
		"client_ip", clientIP)

	response := SetSpreadsheetResponse{
		Success: true,
		Message: "Spreadsheet configuration saved successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode SetSpreadsheet response",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
	}
}

// ClearSpreadsheet handles DELETE /api/config/spreadsheet requests
func (h *ConfigHandler) ClearSpreadsheet(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	clientIP := middleware.GetClientIP(r)
	
	h.logger.Info("ClearSpreadsheet API request received",
		"user_id", userID,
		"has_user_id", ok,
		"client_ip", clientIP,
		"method", r.Method)

	if !ok {
		h.logger.Warn("ClearSpreadsheet called without valid user context",
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated", "")
		return
	}

	// Call service to clear spreadsheet
	h.logger.Debug("Calling ConfigService.ClearSpreadsheetURL",
		"user_id", userID)

	err := h.configService.ClearSpreadsheetURL(r.Context(), userID)
	if err != nil {
		// Handle configuration errors
		if configErr, ok := err.(*services.ConfigError); ok {
			h.logger.Warn("ConfigService returned error during clear",
				"error_type", configErr.Type,
				"error_message", configErr.Message,
				"user_id", userID,
				"client_ip", clientIP)

			statusCode := h.getStatusCodeForConfigError(configErr.Type)
			h.writeErrorResponse(w, statusCode, configErr.Type, configErr.Message, configErr.Type)
			return
		}

		// Unexpected error
		h.logger.Error("Unexpected error in ClearSpreadsheet",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
		h.writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", "")
		return
	}

	// Success response
	h.logger.Info("ClearSpreadsheet completed successfully",
		"user_id", userID,
		"client_ip", clientIP)

	response := SetSpreadsheetResponse{
		Success: true,
		Message: "Spreadsheet configuration cleared successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode ClearSpreadsheet response",
			"error", err,
			"user_id", userID,
			"client_ip", clientIP)
	}
}

// getStatusCodeForConfigError maps configuration error types to HTTP status codes
func (h *ConfigHandler) getStatusCodeForConfigError(errorType string) int {
	switch errorType {
	case services.ConfigErrorInvalidURL:
		return http.StatusBadRequest
	case services.ConfigErrorPermission:
		return http.StatusForbidden
	case services.ConfigErrorNotFound:
		return http.StatusNotFound
	case services.ConfigErrorDatabase:
		return http.StatusInternalServerError
	case services.ConfigErrorValidation:
		return http.StatusBadRequest
	case services.ConfigErrorNetwork:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// writeErrorResponse writes a standardized error response
func (h *ConfigHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message, errorType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := ErrorResponse{
		Error:   errorCode,
		Message: message,
	}

	if errorType != "" {
		errorResponse.Type = errorType
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		h.logger.Error("Failed to encode error response",
			"error", err,
			"status_code", statusCode,
			"error_code", errorCode)
	}
}