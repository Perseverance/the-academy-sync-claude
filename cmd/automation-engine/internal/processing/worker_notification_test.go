package processing

import (
	"context"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Test prepareNotificationData with various scenarios
func TestPrepareNotificationData(t *testing.T) {
	logger := logger.New("test")
	worker := &Worker{logger: logger}
	location, _ := time.LoadLocation("Europe/Sofia")

	config := &automation.ProcessingConfig{
		UserID:        123,
		Email:         "test@example.com",
		SpreadsheetID: "test-spreadsheet-id",
	}

	t.Run("filter out successful rest days", func(t *testing.T) {
		// Create day results with only rest days
		dayResults := []*DayProcessingResult{
			{
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, location),
				Processed:      true,
				ActivitiesFound: 0,
				PlanEntry:      &TrainingPlanEntry{RPE: 1},
				SkippedReason:  SkipReasonRestDayNoActivity,
			},
			{
				Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, location),
				Processed:      true,
				ActivitiesFound: 0,
				PlanEntry:      &TrainingPlanEntry{RPE: 1},
				SkippedReason:  SkipReasonRestDayNoActivity,
			},
		}

		result := worker.prepareNotificationData(config, dayResults, location)
		assert.Nil(t, result, "Should return nil when only successful rest days")
	})

	t.Run("mixed results with activities", func(t *testing.T) {
		dayResults := []*DayProcessingResult{
			{
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, location),
				Processed:      true,
				ActivitiesFound: 1,
				TotalDistance:  5000,
				TotalTime:      1800,
			},
			{
				Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, location),
				Processed:      true,
				ActivitiesFound: 0,
				PlanEntry:      &TrainingPlanEntry{RPE: 1},
				SkippedReason:  SkipReasonRestDayNoActivity,
			},
			{
				Date:           time.Date(2024, 1, 17, 0, 0, 0, 0, location),
				Processed:      true,
				ActivitiesFound: 0,
				PlanEntry:      &TrainingPlanEntry{ActivityType: "Бягане", RPE: 5},
			},
		}

		result := worker.prepareNotificationData(config, dayResults, location)
		assert.NotNil(t, result, "Should return notification data for mixed results")
		assert.Equal(t, 123, result["user_id"])
		assert.Equal(t, "test@example.com", result["user_email"])
		assert.Equal(t, "test@example.com", result["user_name"]) // Using email as name temporarily
		assert.Equal(t, "test-spreadsheet-id", result["spreadsheet_id"])

		logs := result["logs"].([]map[string]interface{})
		assert.Len(t, logs, 2, "Should exclude rest days")

		// Check first log (activity)
		assert.Equal(t, "2024-01-15", logs[0]["date"])
		assert.Equal(t, "success", logs[0]["status"])
		assert.Contains(t, logs[0]["summary_message"], "1 activity logged")
		assert.Contains(t, logs[0]["summary_message"], "5.0km")

		// Check second log (missed workout)
		assert.Equal(t, "2024-01-17", logs[1]["date"])
		assert.Equal(t, "success", logs[1]["status"])
		assert.Equal(t, "No activities found", logs[1]["summary_message"])
	})

	t.Run("failed processing", func(t *testing.T) {
		dayResults := []*DayProcessingResult{
			{
				Date:  time.Date(2024, 1, 15, 0, 0, 0, 0, location),
				Error: assert.AnError,
			},
		}

		result := worker.prepareNotificationData(config, dayResults, location)
		assert.NotNil(t, result, "Should return notification data for failed processing")

		logs := result["logs"].([]map[string]interface{})
		assert.Len(t, logs, 1)
		assert.Equal(t, "failed", logs[0]["status"])
		assert.Contains(t, logs[0]["summary_message"], "Failed to process")
		assert.NotEmpty(t, logs[0]["error"])
	})

	t.Run("empty results", func(t *testing.T) {
		result := worker.prepareNotificationData(config, []*DayProcessingResult{}, location)
		assert.Nil(t, result, "Should return nil for empty results")
	})

	t.Run("all already processed days", func(t *testing.T) {
		dayResults := []*DayProcessingResult{
			{
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, location),
				Processed:      false,
				SkippedReason:  SkipReasonAlreadyProcessed,
			},
			{
				Date:           time.Date(2024, 1, 16, 0, 0, 0, 0, location),
				Processed:      false,
				SkippedReason:  SkipReasonAlreadyProcessed,
			},
		}

		result := worker.prepareNotificationData(config, dayResults, location)
		assert.Nil(t, result, "Should return nil when all days were already processed")
	})

	t.Run("mixed already processed and new", func(t *testing.T) {
		dayResults := []*DayProcessingResult{
			{
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, location),
				Processed:      false,
				SkippedReason:  SkipReasonAlreadyProcessed,
			},
			{
				Date:            time.Date(2024, 1, 16, 0, 0, 0, 0, location),
				Processed:       true,
				ActivitiesFound: 1,
				TotalDistance:   5000,
				TotalTime:       1800,
			},
		}

		result := worker.prepareNotificationData(config, dayResults, location)
		assert.NotNil(t, result, "Should return notification data when there are newly processed days")
		
		logs := result["logs"].([]map[string]interface{})
		assert.Len(t, logs, 1, "Should only include newly processed day")
		assert.Equal(t, "2024-01-16", logs[0]["date"])
		assert.Contains(t, logs[0]["summary_message"], "1 activity logged")
	})

	t.Run("filter out days with no activities and no scheduled run", func(t *testing.T) {
		dayResults := []*DayProcessingResult{
			{
				Date:           time.Date(2024, 1, 15, 0, 0, 0, 0, location),
				Processed:      false,
				SkippedReason:  SkipReasonNoActivities, // No scheduled run, no activities
				ActivitiesFound: 0,
			},
			{
				Date:            time.Date(2024, 1, 16, 0, 0, 0, 0, location),
				Processed:       true,
				ActivitiesFound: 0, // Scheduled run but no activities (missed workout)
				PlanEntry:       &TrainingPlanEntry{ActivityType: "Бягане", RPE: 5},
			},
			{
				Date:            time.Date(2024, 1, 17, 0, 0, 0, 0, location),
				Processed:       true,
				ActivitiesFound: 1, // Unscheduled activity
				TotalDistance:   5000,
				TotalTime:       1800,
			},
			{
				Date:            time.Date(2024, 1, 18, 0, 0, 0, 0, location),
				Processed:       true,
				ActivitiesFound: 1, // Scheduled run with activity
				TotalDistance:   10000,
				TotalTime:       3600,
				PlanEntry:       &TrainingPlanEntry{ActivityType: "Бягане", RPE: 6},
			},
		}

		result := worker.prepareNotificationData(config, dayResults, location)
		assert.NotNil(t, result, "Should return notification data")
		
		logs := result["logs"].([]map[string]interface{})
		assert.Len(t, logs, 3, "Should include all days except those with no activities and no scheduled run")
		assert.Equal(t, "2024-01-16", logs[0]["date"], "Should include missed workout")
		assert.Equal(t, "2024-01-17", logs[1]["date"], "Should include unscheduled activity")
		assert.Equal(t, "2024-01-18", logs[2]["date"], "Should include completed workout")
	})
}

// Test createSummaryMessage
func TestCreateSummaryMessage(t *testing.T) {
	logger := logger.New("test")
	worker := &Worker{logger: logger}

	t.Run("error case", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Error: assert.AnError,
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Contains(t, msg, "Failed to process")
	})

	t.Run("not processed with reason", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Processed:     false,
			SkippedReason: SkipReasonAlreadyProcessed,
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Equal(t, SkipReasonAlreadyProcessed.String(), msg)
	})

	t.Run("not processed without reason", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Processed: false,
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Equal(t, "No scheduled training for this day", msg)
	})

	t.Run("rest day no activity", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Processed:       true,
			ActivitiesFound: 0,
			PlanEntry:       &TrainingPlanEntry{RPE: 1},
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Equal(t, SkipReasonRestDayNoActivity.String(), msg)
	})

	t.Run("no activities found", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Processed:       true,
			ActivitiesFound: 0,
			PlanEntry:       &TrainingPlanEntry{RPE: 5},
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Equal(t, "No activities found", msg)
	})

	t.Run("single activity", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Processed:       true,
			ActivitiesFound: 1,
			TotalDistance:   5432.1,
			TotalTime:       1823,
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Equal(t, "1 activity logged (5.4km in 00:30:23)", msg)
	})

	t.Run("multiple activities", func(t *testing.T) {
		dayResult := &DayProcessingResult{
			Processed:       true,
			ActivitiesFound: 3,
			TotalDistance:   15678.9,
			TotalTime:       5432,
		}
		msg := worker.createSummaryMessage(dayResult)
		assert.Equal(t, "3 activities logged (total: 15.7km in 01:30:32)", msg)
	})
}

// Test formatDuration
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "00:00:00"},
		{"seconds only", 45 * time.Second, "00:00:45"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "00:02:30"},
		{"hours minutes seconds", 1*time.Hour + 15*time.Minute + 45*time.Second, "01:15:45"},
		{"over 24 hours", 25*time.Hour + 30*time.Minute, "25:30:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test enqueueNotificationJob
func TestEnqueueNotificationJob(t *testing.T) {
	t.Run("successful enqueue", func(t *testing.T) {
		mockQueueClient := new(MockQueueClient)
		logger := logger.New("test")
		worker := &Worker{
			notificationQueueClient: mockQueueClient,
			logger:                  logger,
		}

		jobData := map[string]interface{}{
			"user_id":    123,
			"user_email": "test@example.com",
		}

		expectedJob := &queue.Job{
			ID:     "job-123",
			Type:   "notification",
			UserID: 123,
			Data:   jobData,
		}

		mockQueueClient.On("EnqueueJob", mock.Anything, queue.JobType("notification"), 123, jobData).
			Return(expectedJob, nil)

		err := worker.enqueueNotificationJob(context.Background(), 123, jobData)
		assert.NoError(t, err)
		mockQueueClient.AssertExpectations(t)
	})

	t.Run("no queue client", func(t *testing.T) {
		logger := logger.New("test")
		worker := &Worker{
			notificationQueueClient: nil,
			logger:                  logger,
		}

		err := worker.enqueueNotificationJob(context.Background(), 123, map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "queue client not configured")
	})

	t.Run("enqueue error", func(t *testing.T) {
		mockQueueClient := new(MockQueueClient)
		logger := logger.New("test")
		worker := &Worker{
			notificationQueueClient: mockQueueClient,
			logger:                  logger,
		}

		jobData := map[string]interface{}{
			"user_id": 123,
		}

		mockQueueClient.On("EnqueueJob", mock.Anything, queue.JobType("notification"), 123, jobData).
			Return(nil, assert.AnError)

		err := worker.enqueueNotificationJob(context.Background(), 123, jobData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to enqueue notification job")
		mockQueueClient.AssertExpectations(t)
	})
}