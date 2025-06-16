package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/services"
)

type mockSyncService struct {
	triggerError error
	eligible     bool
	reason       string
	statusError  error
}

func (m *mockSyncService) TriggerManualSync(ctx context.Context, userID int) error {
	return m.triggerError
}

func (m *mockSyncService) GetUserSyncStatus(ctx context.Context, userID int) (bool, string, error) {
	return m.eligible, m.reason, m.statusError
}

func setupSyncHandlerTest() (*SyncHandler, *mockSyncService) {
	mockService := &mockSyncService{}
	log := logger.New("test")
	// Create handler directly with interface since we're testing
	handler := &SyncHandler{
		syncService: mockService,
		logger:      log.WithContext("component", "sync_handler"),
	}
	return handler, mockService
}

func addUserToContext(r *http.Request, userID int, email string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.EmailKey, email)
	return r.WithContext(ctx)
}

func TestTriggerManualSync(t *testing.T) {
	t.Run("successful sync trigger", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.triggerError = nil

		req := httptest.NewRequest("POST", "/api/sync", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "accepted", response["status"])
		assert.Contains(t, response["message"], "queued for processing")
	})

	t.Run("no user in context", func(t *testing.T) {
		handler, _ := setupSyncHandlerTest()

		req := httptest.NewRequest("POST", "/api/sync", nil)
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("user not found", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.triggerError = services.ErrUserNotFound

		req := httptest.NewRequest("POST", "/api/sync", nil)
		req = addUserToContext(req, 999, "test@example.com")
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "User not found", response["error"])
	})

	t.Run("strava connection required", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.triggerError = services.ErrStravaConnectionRequired

		req := httptest.NewRequest("POST", "/api/sync", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Strava connection required")
	})

	t.Run("spreadsheet configuration required", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.triggerError = services.ErrSpreadsheetRequired

		req := httptest.NewRequest("POST", "/api/sync", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Spreadsheet configuration required")
	})

	t.Run("redis connection error", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		// Simulate a Redis connection error
		mockService.triggerError = fmt.Errorf("dial tcp: connect: connection refused")

		req := httptest.NewRequest("POST", "/api/sync", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Service temporarily unavailable")
	})

	t.Run("generic error", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.triggerError = fmt.Errorf("unexpected error")

		req := httptest.NewRequest("POST", "/api/sync", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.TriggerManualSync(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "Failed to trigger sync")
	})
}

func TestGetSyncStatus(t *testing.T) {
	t.Run("eligible user", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.eligible = true
		mockService.reason = ""
		mockService.statusError = nil

		req := httptest.NewRequest("GET", "/api/sync/status", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.GetSyncStatus(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["eligible"].(bool))
		assert.Empty(t, response["reason"])
	})

	t.Run("ineligible user", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.eligible = false
		mockService.reason = "strava connection required"
		mockService.statusError = nil

		req := httptest.NewRequest("GET", "/api/sync/status", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.GetSyncStatus(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		assert.False(t, response["eligible"].(bool))
		assert.Equal(t, "strava connection required", response["reason"])
	})

	t.Run("no user in context", func(t *testing.T) {
		handler, _ := setupSyncHandlerTest()

		req := httptest.NewRequest("GET", "/api/sync/status", nil)
		rr := httptest.NewRecorder()

		handler.GetSyncStatus(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		handler, mockService := setupSyncHandlerTest()
		mockService.statusError = fmt.Errorf("database error")

		req := httptest.NewRequest("GET", "/api/sync/status", nil)
		req = addUserToContext(req, 123, "test@example.com")
		rr := httptest.NewRecorder()

		handler.GetSyncStatus(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "Failed to get sync status", response["error"])
	})
}