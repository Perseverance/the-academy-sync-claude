package internal

import (
	"context"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/sendgrid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSendGridClient for testing
type MockSendGridClient struct {
	mock.Mock
}

func (m *MockSendGridClient) SendEmail(to, toName, subject, plainTextContent, htmlContent string) error {
	args := m.Called(to, toName, subject, plainTextContent, htmlContent)
	return args.Error(0)
}

// Test ShouldSendEmail with various scenarios
func TestShouldSendEmail(t *testing.T) {
	logger := logger.New("test")
	service := NewNotificationService(nil, logger)

	t.Run("all uneventful rest days", func(t *testing.T) {
		logs := []ProcessingLog{
			{
				Date:           "2024-01-15",
				Status:         "success",
				SummaryMessage: "Rest day, no activity",
			},
			{
				Date:           "2024-01-16",
				Status:         "success",
				SummaryMessage: "Rest day, no activity",
			},
			{
				Date:           "2024-01-17",
				Status:         "success",
				SummaryMessage: "Rest day, no activity",
			},
		}

		result := service.ShouldSendEmail(logs)
		assert.False(t, result, "Should return false for all uneventful rest days")
	})

	t.Run("mixed with activities", func(t *testing.T) {
		logs := []ProcessingLog{
			{
				Date:           "2024-01-15",
				Status:         "success",
				SummaryMessage: "1 activity logged (5.0km in 00:30:00)",
			},
			{
				Date:           "2024-01-16",
				Status:         "success",
				SummaryMessage: "Rest day, no activity",
			},
		}

		result := service.ShouldSendEmail(logs)
		assert.True(t, result, "Should return true when activities are present")
	})

	t.Run("with errors", func(t *testing.T) {
		logs := []ProcessingLog{
			{
				Date:           "2024-01-15",
				Status:         "failed",
				SummaryMessage: "Failed to process: connection error",
				Error:          "timeout connecting to Strava API",
			},
			{
				Date:           "2024-01-16",
				Status:         "success",
				SummaryMessage: "Rest day, no activity",
			},
		}

		result := service.ShouldSendEmail(logs)
		assert.True(t, result, "Should return true when errors are present")
	})

	t.Run("only new activities", func(t *testing.T) {
		logs := []ProcessingLog{
			{
				Date:           "2024-01-15",
				Status:         "success",
				SummaryMessage: "2 activities logged (10.5km in 01:15:00)",
			},
			{
				Date:           "2024-01-16",
				Status:         "success",
				SummaryMessage: "1 activity logged (5.0km in 00:30:00)",
			},
		}

		result := service.ShouldSendEmail(logs)
		assert.True(t, result, "Should return true when new activities are present")
	})

	t.Run("no activities found", func(t *testing.T) {
		logs := []ProcessingLog{
			{
				Date:           "2024-01-15",
				Status:         "success",
				SummaryMessage: "No activities found",
			},
			{
				Date:           "2024-01-16",
				Status:         "success",
				SummaryMessage: "Rest day, no activity",
			},
		}

		result := service.ShouldSendEmail(logs)
		assert.True(t, result, "Should return true for 'No activities found' message")
	})

	t.Run("empty logs", func(t *testing.T) {
		logs := []ProcessingLog{}

		result := service.ShouldSendEmail(logs)
		assert.False(t, result, "Should return false for empty logs")
	})

	t.Run("nil logs", func(t *testing.T) {
		result := service.ShouldSendEmail(nil)
		assert.False(t, result, "Should return false for nil logs")
	})
}

// Test ConstructEmailBody formatting
func TestConstructEmailBody(t *testing.T) {
	logger := logger.New("test")
	service := NewNotificationService(nil, logger)

	runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)

	t.Run("basic formatting", func(t *testing.T) {
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "1 activity logged (5.0km in 00:30:00)",
				},
				{
					Date:           "2024-01-16",
					Status:         "success",
					SummaryMessage: "Rest day, no activity",
				},
			},
		}

		body := service.ConstructEmailBody(notification, runDate)

		assert.Contains(t, body, "Daily Sync Summary for January 20, 2024:")
		assert.Contains(t, body, "✅ Mon, Jan 15: 1 activity logged (5.0km in 00:30:00)")
		assert.Contains(t, body, "✅ Tue, Jan 16: Rest day, no activity")
		assert.Contains(t, body, "The Academy Sync - Automated Training Log")
	})

	t.Run("with errors", func(t *testing.T) {
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "failed",
					SummaryMessage: "Failed to process",
					Error:          "API rate limit exceeded",
				},
			},
		}

		body := service.ConstructEmailBody(notification, runDate)

		assert.Contains(t, body, "❌ Mon, Jan 15: Failed to process")
		assert.Contains(t, body, "Error: API rate limit exceeded")
	})

	t.Run("various statuses", func(t *testing.T) {
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "2 activities logged",
				},
				{
					Date:           "2024-01-16",
					Status:         "partial",
					SummaryMessage: "Partial sync completed",
				},
				{
					Date:           "2024-01-17",
					Status:         "failed",
					SummaryMessage: "Sync failed",
				},
				{
					Date:           "2024-01-18",
					Status:         "skipped",
					SummaryMessage: "Already processed",
				},
			},
		}

		body := service.ConstructEmailBody(notification, runDate)

		assert.Contains(t, body, "✅ Mon, Jan 15: 2 activities logged")
		assert.Contains(t, body, "⚠️ Tue, Jan 16: Partial sync completed")
		assert.Contains(t, body, "❌ Wed, Jan 17: Sync failed")
		assert.Contains(t, body, "⏭️ Thu, Jan 18: Already processed")
	})

	t.Run("invalid date format", func(t *testing.T) {
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "invalid-date",
					Status:         "success",
					SummaryMessage: "Test message",
				},
			},
		}

		body := service.ConstructEmailBody(notification, runDate)

		// Should use original date string when parsing fails
		assert.Contains(t, body, "✅ invalid-date: Test message")
	})
}

// Test getStatusEmoji
func TestGetStatusEmoji(t *testing.T) {
	logger := logger.New("test")
	service := NewNotificationService(nil, logger)

	tests := []struct {
		status   string
		expected string
	}{
		{"success", "✅"},
		{"partial", "⚠️"},
		{"failed", "❌"},
		{"skipped", "⏭️"},
		{"unknown", "•"},
		{"", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := service.getStatusEmoji(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test ProcessNotification
func TestProcessNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("successful email send", func(t *testing.T) {
		logger := logger.New("test")
		
		// For now, we'll test with nil client
		service := NewNotificationService(nil, logger)

		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   time.Now().Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "1 activity logged",
					ActivitiesFound: 1,
				},
			},
		}

		// With nil client, should log warning but not error
		err := service.ProcessNotification(ctx, notification)
		assert.NoError(t, err)
	})

	t.Run("skip uneventful rest days", func(t *testing.T) {
		logger := logger.New("test")
		service := NewNotificationService(nil, logger)

		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   time.Now().Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "Rest day, no activity",
				},
			},
		}

		err := service.ProcessNotification(ctx, notification)
		assert.NoError(t, err, "Should not error when skipping notification")
	})

	t.Run("invalid run date", func(t *testing.T) {
		logger := logger.New("test")
		service := NewNotificationService(nil, logger)

		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   "invalid-date",
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "1 activity logged",
				},
			},
		}

		// Should handle gracefully and use current time
		err := service.ProcessNotification(ctx, notification)
		assert.NoError(t, err)
	})
}

// Integration test with actual SendGrid client (if needed)
func TestProcessNotificationWithSendGrid(t *testing.T) {
	t.Skip("Skipping integration test - requires SendGrid API key")

	// This test would use an actual SendGrid client
	// Only run in integration test environments
	apiKey := "test-api-key"
	fromEmail := "test@example.com"
	
	sendgridClient := sendgrid.NewClient(apiKey, fromEmail)
	logger := logger.New("test")
	service := NewNotificationService(sendgridClient, logger)

	notification := &NotificationJob{
		UserID:    123,
		UserEmail: "test@example.com",
		UserName:  "Test User",
		RunDate:   time.Now().Format(time.RFC3339),
		Logs: []ProcessingLog{
			{
				Date:           "2024-01-15",
				Status:         "success",
				SummaryMessage: "1 activity logged",
			},
		},
	}

	err := service.ProcessNotification(context.Background(), notification)
	// Would fail with invalid API key
	assert.Error(t, err)
}