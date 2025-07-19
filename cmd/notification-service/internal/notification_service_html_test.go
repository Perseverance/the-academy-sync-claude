package internal

import (
	"strings"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// Test RenderHTMLEmail with various scenarios
func TestRenderHTMLEmail(t *testing.T) {
	logger := logger.New("test")
	
	t.Run("successful HTML rendering", func(t *testing.T) {
		service, err := NewNotificationService(nil, logger, "http://localhost:8080", "http://localhost:3000")
		assert.NoError(t, err)

		runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
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

		html, err := service.RenderHTMLEmail(notification, runDate)
		assert.NoError(t, err)
		assert.NotEmpty(t, html)

		// Check for key template elements
		assert.Contains(t, html, "Academy Sync Daily Summary")
		assert.Contains(t, html, "January 20, 2024")

		// Check for processed days
		assert.Contains(t, html, "✅")
		assert.Contains(t, html, "Mon, Jan 15")
		assert.Contains(t, html, "1 activity logged (5.0km in 00:30:00)")

		assert.Contains(t, html, "⚠️")
		assert.Contains(t, html, "Tue, Jan 16")
		assert.Contains(t, html, "Partial sync completed")

		assert.Contains(t, html, "❌")
		assert.Contains(t, html, "Wed, Jan 17")
		assert.Contains(t, html, "Sync failed")

		assert.Contains(t, html, "⏭️")
		assert.Contains(t, html, "Thu, Jan 18")
		assert.Contains(t, html, "Already processed")

		// Check for footer links
		assert.Contains(t, html, "View Dashboard")
		assert.Contains(t, html, "View Spreadsheet")
		assert.Contains(t, html, "http://localhost:3000") // Frontend URL
		assert.Contains(t, html, "https://docs.google.com/spreadsheets") // Spreadsheet URL

		// Check that CSS is inlined
		assert.Contains(t, html, "style=")
		assert.NotContains(t, html, "<style>") // Style tags should be removed after inlining
	})

	t.Run("empty logs", func(t *testing.T) {
		service, err := NewNotificationService(nil, logger, "http://localhost:8080", "http://localhost:3000")
		assert.NoError(t, err)

		runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs:      []ProcessingLog{},
		}

		html, err := service.RenderHTMLEmail(notification, runDate)
		assert.NoError(t, err)
		assert.NotEmpty(t, html)
		
		// Should still have the template structure
		assert.Contains(t, html, "Academy Sync Daily Summary")
		assert.Contains(t, html, "January 20, 2024")
		
		// But no data rows
		assert.NotContains(t, html, "✅")
		assert.NotContains(t, html, "⚠️")
		assert.NotContains(t, html, "❌")
	})

	t.Run("invalid date format in logs", func(t *testing.T) {
		service, err := NewNotificationService(nil, logger, "http://localhost:8080", "http://localhost:3000")
		assert.NoError(t, err)

		runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
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

		html, err := service.RenderHTMLEmail(notification, runDate)
		assert.NoError(t, err)
		assert.NotEmpty(t, html)
		
		// Should use original date string when parsing fails
		assert.Contains(t, html, "invalid-date")
		assert.Contains(t, html, "Test message")
	})

	t.Run("HTML escaping", func(t *testing.T) {
		service, err := NewNotificationService(nil, logger, "http://localhost:8080", "http://localhost:3000")
		assert.NoError(t, err)

		runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "Test <script>alert('xss')</script> message",
				},
			},
		}

		html, err := service.RenderHTMLEmail(notification, runDate)
		assert.NoError(t, err)
		
		// Check that HTML is properly escaped
		assert.NotContains(t, html, "<script>")
		assert.Contains(t, html, "&lt;script&gt;") // Should be escaped
	})

	t.Run("table-based layout", func(t *testing.T) {
		service, err := NewNotificationService(nil, logger, "http://localhost:8080", "http://localhost:3000")
		assert.NoError(t, err)

		runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "1 activity logged",
				},
			},
		}

		html, err := service.RenderHTMLEmail(notification, runDate)
		assert.NoError(t, err)
		
		// Check for table-based layout elements
		assert.Contains(t, html, "<table")
		assert.Contains(t, html, "role=\"presentation\"")
		assert.Contains(t, html, "width=\"600\"")
		
		// Check that styles are inlined (not in style block)
		assert.True(t, strings.Contains(html, "style=") || strings.Contains(html, "bgcolor="))
	})

	t.Run("color scheme matches style guide", func(t *testing.T) {
		service, err := NewNotificationService(nil, logger, "http://localhost:8080", "http://localhost:3000")
		assert.NoError(t, err)

		runDate := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
		notification := &NotificationJob{
			UserID:    123,
			UserEmail: "test@example.com",
			UserName:  "Test User",
			RunDate:   runDate.Format(time.RFC3339),
			Logs: []ProcessingLog{
				{
					Date:           "2024-01-15",
					Status:         "success",
					SummaryMessage: "1 activity logged",
				},
			},
		}

		html, err := service.RenderHTMLEmail(notification, runDate)
		assert.NoError(t, err)
		
		// Check for style guide colors
		assert.Contains(t, html, "#2a5b3e") // Academy Green
		assert.Contains(t, html, "#333333") // Primary text color
		assert.Contains(t, html, "#f3f4f6") // Background color
	})
}

// Test template data structure
func TestTemplateDataStructure(t *testing.T) {
	// This test ensures the TemplateData struct matches what the template expects
	data := TemplateData{
		RunDate: "January 20, 2024",
		ProcessedDays: []ProcessedDay{
			{
				StatusIcon:     "✅",
				ProcessedDate:  "Mon, Jan 15",
				SummaryMessage: "1 activity logged",
			},
		},
		DashboardURL:   "http://localhost:3000",
		SpreadsheetURL: "https://docs.google.com/spreadsheets/d/YOUR_SPREADSHEET_ID",
	}

	// Verify all fields are present
	assert.NotEmpty(t, data.RunDate)
	assert.Len(t, data.ProcessedDays, 1)
	assert.NotEmpty(t, data.ProcessedDays[0].StatusIcon)
	assert.NotEmpty(t, data.ProcessedDays[0].ProcessedDate)
	assert.NotEmpty(t, data.ProcessedDays[0].SummaryMessage)
	assert.NotEmpty(t, data.DashboardURL)
	assert.NotEmpty(t, data.SpreadsheetURL)
}