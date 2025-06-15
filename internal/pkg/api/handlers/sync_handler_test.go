package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/services"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// MockSyncService implements sync service interface for testing
type MockSyncService struct {
	triggerResult *services.SyncResponse
	triggerError  error
	statusResult  map[string]interface{}
	statusError   error
}

func (m *MockSyncService) TriggerManualSync(ctx context.Context, userID int) (*services.SyncResponse, error) {
	return m.triggerResult, m.triggerError
}

func (m *MockSyncService) GetQueueStatus(ctx context.Context) (map[string]interface{}, error) {
	return m.statusResult, m.statusError
}

func TestSyncHandler_TriggerManualSync(t *testing.T) {
	log := logger.New("test")

	t.Run("SuccessfulSync", func(t *testing.T) {
		mockService := &MockSyncService{
			triggerResult: &services.SyncResponse{
				Success:                   true,
				Message:                   "Manual sync triggered successfully",
				EstimatedCompletionSeconds: 60,
			},
		}

		handler := NewSyncHandler(mockService, log)

		req := httptest.NewRequest("POST", "/api/sync", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		
		// Add user ID to context (simulating auth middleware)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.TriggerManualSync(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected status 202, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}
	})

	t.Run("UnauthorizedRequest", func(t *testing.T) {
		mockService := &MockSyncService{}
		handler := NewSyncHandler(mockService, log)

		req := httptest.NewRequest("POST", "/api/sync", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		// No user ID in context

		w := httptest.NewRecorder()
		handler.TriggerManualSync(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("UserNotConfigured", func(t *testing.T) {
		mockService := &MockSyncService{
			triggerError: &services.SyncError{
				Type:    services.SyncErrorUserNotConfigured,
				Message: "User is not fully configured",
			},
		}

		handler := NewSyncHandler(mockService, log)

		req := httptest.NewRequest("POST", "/api/sync", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		
		// Add user ID to context
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.TriggerManualSync(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestSyncHandler_GetQueueStatus(t *testing.T) {
	log := logger.New("test")

	t.Run("SuccessfulStatus", func(t *testing.T) {
		mockService := &MockSyncService{
			statusResult: map[string]interface{}{
				"queue_length":  int64(5),
				"queue_name":    "jobs_queue",
				"health_status": "healthy",
			},
		}

		handler := NewSyncHandler(mockService, log)

		req := httptest.NewRequest("GET", "/api/sync/status", nil)
		
		// Add user ID to context
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, 123)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.GetQueueStatus(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}
	})

	t.Run("UnauthorizedStatus", func(t *testing.T) {
		mockService := &MockSyncService{}
		handler := NewSyncHandler(mockService, log)

		req := httptest.NewRequest("GET", "/api/sync/status", nil)
		// No user ID in context

		w := httptest.NewRecorder()
		handler.GetQueueStatus(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func TestSyncHandler_GetStatusCodeForSyncError(t *testing.T) {
	log := logger.New("test")
	handler := NewSyncHandler(nil, log)

	testCases := []struct {
		errorType      string
		expectedStatus int
	}{
		{services.SyncErrorUserNotFound, http.StatusNotFound},
		{services.SyncErrorUserNotConfigured, http.StatusBadRequest},
		{services.SyncErrorValidation, http.StatusBadRequest},
		{services.SyncErrorQueueUnavailable, http.StatusServiceUnavailable},
		{services.SyncErrorInternal, http.StatusInternalServerError},
		{"UNKNOWN_ERROR", http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.errorType, func(t *testing.T) {
			status := handler.getStatusCodeForSyncError(tc.errorType)
			if status != tc.expectedStatus {
				t.Errorf("Expected status %d for error type %s, got %d", 
					tc.expectedStatus, tc.errorType, status)
			}
		})
	}
}