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
			name:        "request body ignored",
			requestBody: "any content here is ignored",
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

	tests := []struct {
		method         string
		expectSuccess  bool
		expectedStatus int
	}{
		{http.MethodPost, true, http.StatusAccepted},
		{http.MethodGet, false, http.StatusMethodNotAllowed},
		{http.MethodPut, false, http.StatusMethodNotAllowed},
		{http.MethodDelete, false, http.StatusMethodNotAllowed},
		{http.MethodPatch, false, http.StatusMethodNotAllowed},
		{http.MethodHead, false, http.StatusMethodNotAllowed},
		{http.MethodOptions, false, http.StatusMethodNotAllowed},
	}
	
	for _, tt := range tests {
		t.Run("method_"+tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/tasks/invoke-scheduler", nil)
			rr := httptest.NewRecorder()
			
			// Setup mock expectation only for POST
			if tt.expectSuccess {
				mockService.On("ProcessScheduledRun", mock.Anything).Return(2, nil).Once()
			}
			
			// Call the handler
			handler.InvokeScheduler(rr, req)
			
			// Check status code
			assert.Equal(t, tt.expectedStatus, rr.Code, "Expected status %d for method %s", tt.expectedStatus, tt.method)
			
			// For non-POST methods, verify error response
			if !tt.expectSuccess {
				var response map[string]string
				err := json.NewDecoder(rr.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, "Method Not Allowed", response["error"])
				assert.Equal(t, "Method not allowed", response["message"])
			}
			
			// Verify mock expectations
			mockService.AssertExpectations(t)
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