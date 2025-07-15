package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock SchedulingService
type MockSchedulingService struct {
	mock.Mock
}

func (m *MockSchedulingService) ProcessScheduledRun(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func TestSchedulerHandler_InvokeScheduler(t *testing.T) {
	log := logger.New("test")

	tests := []struct {
		name               string
		requestBody        string
		setupMock          func(*MockSchedulingService)
		expectedStatus     int
		expectedResponse   map[string]interface{}
		expectError        bool
	}{
		{
			name:        "successful invocation with multiple jobs",
			requestBody: "{}",
			setupMock: func(mockService *MockSchedulingService) {
				mockService.On("ProcessScheduledRun", mock.Anything).Return(5, nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedResponse: map[string]interface{}{
				"status": "accepted",
				"message": "Scheduled processing initiated",
				"jobs_enqueued": float64(5),
			},
			expectError: false,
		},
		{
			name:        "successful invocation with no jobs",
			requestBody: "{}",
			setupMock: func(mockService *MockSchedulingService) {
				mockService.On("ProcessScheduledRun", mock.Anything).Return(0, nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedResponse: map[string]interface{}{
				"status": "accepted",
				"message": "Scheduled processing initiated",
				"jobs_enqueued": float64(0),
			},
			expectError: false,
		},
		{
			name:        "service error",
			requestBody: "{}",
			setupMock: func(mockService *MockSchedulingService) {
				mockService.On("ProcessScheduledRun", mock.Anything).Return(0, errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedResponse: map[string]interface{}{
				"error": "Internal Server Error",
				"message": "Failed to process scheduled run",
			},
			expectError: true,
		},
		{
			name:        "invalid json body",
			requestBody: "{invalid json",
			setupMock: func(mockService *MockSchedulingService) {
				// The handler doesn't parse JSON, so it will still call the service
				mockService.On("ProcessScheduledRun", mock.Anything).Return(3, nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedResponse: map[string]interface{}{
				"status": "accepted",
				"message": "Scheduled processing initiated",
				"jobs_enqueued": float64(3),
			},
			expectError: false,
		},
		{
			name:        "empty body",
			requestBody: "",
			setupMock: func(mockService *MockSchedulingService) {
				mockService.On("ProcessScheduledRun", mock.Anything).Return(3, nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedResponse: map[string]interface{}{
				"status": "accepted",
				"message": "Scheduled processing initiated",
				"jobs_enqueued": float64(3),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock service
			mockService := new(MockSchedulingService)
			tt.setupMock(mockService)

			// Create handler
			handler := &SchedulerHandler{
				schedulingService: mockService,
				logger:           log,
			}

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/tasks/invoke-scheduler", 
				bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.InvokeScheduler(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Parse response
			var response map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Check response
			assert.Equal(t, tt.expectedResponse, response)

			// Verify mock expectations were met
			mockService.AssertExpectations(t)
		})
	}
}

func TestSchedulerHandler_HTTPMethods(t *testing.T) {
	log := logger.New("test")
	mockService := new(MockSchedulingService)
	
	handler := &SchedulerHandler{
		schedulingService: mockService,
		logger:           log,
	}

	// Test non-POST methods should be rejected by the router
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	
	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/tasks/invoke-scheduler", nil)
			rr := httptest.NewRecorder()
			
			// In a real scenario, the router would reject these methods
			// Here we're just documenting the expected behavior
			// The handler should only be called for POST requests
			_ = handler
			_ = req
			_ = rr
		})
	}
}

func TestSchedulerHandler_ConcurrentRequests(t *testing.T) {
	log := logger.New("test")
	mockService := new(MockSchedulingService)
	
	// Setup mock to handle multiple concurrent calls
	mockService.On("ProcessScheduledRun", mock.Anything).Return(2, nil).Times(3)
	
	handler := &SchedulerHandler{
		schedulingService: mockService,
		logger:           log,
	}
	
	// Run multiple requests concurrently
	done := make(chan bool, 3)
	
	for i := 0; i < 3; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/tasks/invoke-scheduler", 
				bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			
			handler.InvokeScheduler(rr, req)
			
			assert.Equal(t, http.StatusAccepted, rr.Code)
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}
	
	// Verify all mock calls were made
	mockService.AssertExpectations(t)
}

func TestSchedulerHandler_ResponseHeaders(t *testing.T) {
	log := logger.New("test")
	mockService := new(MockSchedulingService)
	mockService.On("ProcessScheduledRun", mock.Anything).Return(1, nil)
	
	handler := &SchedulerHandler{
		schedulingService: mockService,
		logger:           log,
	}
	
	req := httptest.NewRequest(http.MethodPost, "/tasks/invoke-scheduler", 
		bytes.NewBufferString("{}"))
	rr := httptest.NewRecorder()
	
	handler.InvokeScheduler(rr, req)
	
	// Check that Content-Type is set
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}