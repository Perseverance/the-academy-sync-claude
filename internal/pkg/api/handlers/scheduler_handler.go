package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// SchedulerHandler handles scheduler-related API endpoints
type SchedulerHandler struct {
	schedulingService SchedulingService
	logger            *logger.Logger
}

// NewSchedulerHandler creates a new scheduler handler
func NewSchedulerHandler(schedulingService SchedulingService, logger *logger.Logger) *SchedulerHandler {
	return &SchedulerHandler{
		schedulingService: schedulingService,
		logger:            logger.WithContext("component", "scheduler_handler"),
	}
}

// InvokeScheduler handles POST /tasks/invoke-scheduler
// This endpoint is called by Google Cloud Scheduler to trigger scheduled processing
func (h *SchedulerHandler) InvokeScheduler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Log the request details
	h.logger.Info("Scheduler invocation received",
		"method", r.Method,
		"path", r.URL.Path,
		"client_ip", middleware.GetClientIP(r),
		"user_agent", r.Header.Get("User-Agent"))

	// Process the scheduled run
	jobsEnqueued, err := h.schedulingService.ProcessScheduledRun(ctx)
	if err != nil {
		h.logger.Error("Failed to process scheduled run", "error", err)
		h.sendJSONError(w, "Failed to process scheduled run", http.StatusInternalServerError)
		return
	}

	// Return success response with job count
	response := map[string]interface{}{
		"status":        "accepted",
		"jobs_enqueued": jobsEnqueued,
		"message":       "Scheduled processing initiated",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
	}

	h.logger.Info("Scheduler invocation completed",
		"jobs_enqueued", jobsEnqueued)
}

// sendJSONError sends a JSON error response
func (h *SchedulerHandler) sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]string{
		"error":   http.StatusText(statusCode),
		"message": message,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode error response", "error", err)
	}
}