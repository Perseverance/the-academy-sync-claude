package internal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/sendgrid"
)

// NotificationService handles the core business logic for processing notifications
type NotificationService struct {
	sendgridClient *sendgrid.Client
	logger         *logger.Logger
}

// NewNotificationService creates a new notification service
func NewNotificationService(sendgridClient *sendgrid.Client, logger *logger.Logger) *NotificationService {
	return &NotificationService{
		sendgridClient: sendgridClient,
		logger:         logger.WithContext("component", "notification_service"),
	}
}

// ProcessNotification processes a notification job
func (s *NotificationService) ProcessNotification(ctx context.Context, notification *NotificationJob) error {
	s.logger.Debug("Processing notification",
		"user_id", notification.UserID,
		"email", notification.UserEmail,
		"log_count", len(notification.Logs))

	// Apply US038 - "No Email on Uneventful Rest Days" filter
	if !s.ShouldSendEmail(notification.Logs) {
		s.logger.Info("Skipping notification - all days are uneventful rest days",
			"user_id", notification.UserID,
			"email", notification.UserEmail)
		return nil
	}

	// Parse run date
	runDate, err := time.Parse(time.RFC3339, notification.RunDate)
	if err != nil {
		s.logger.Warn("Failed to parse run date, using current time",
			"error", err,
			"run_date", notification.RunDate)
		runDate = time.Now()
	}

	// Construct email body (US041)
	subject := fmt.Sprintf("Daily Sync Summary - %s", runDate.Format("Jan 2, 2006"))
	plainTextBody := s.ConstructEmailBody(notification, runDate)
	
	// For now, we're sending plain text only
	// HTML body will be implemented when we integrate the template
	htmlBody := ""

	// Send email via SendGrid
	if s.sendgridClient != nil {
		err := s.sendgridClient.SendEmail(
			notification.UserEmail,
			notification.UserName,
			subject,
			plainTextBody,
			htmlBody,
		)
		if err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}

		s.logger.Info("Successfully sent notification email",
			"user_id", notification.UserID,
			"email", notification.UserEmail,
			"subject", subject)
	} else {
		s.logger.Warn("SendGrid client not configured, skipping email send",
			"user_id", notification.UserID,
			"email", notification.UserEmail)
	}

	return nil
}

// ShouldSendEmail implements US038 - determines if an email should be sent
// Returns false if ALL logs are for uneventful rest days
// Note: The automation engine already filters out "already processed" days,
// so this function only receives logs for newly processed days
func (s *NotificationService) ShouldSendEmail(logs []ProcessingLog) bool {
	if len(logs) == 0 {
		return false
	}

	// Check if all logs are uneventful rest days
	// Note: We're using a hardcoded string here because the notification service
	// shouldn't depend on the automation engine's internal enums
	restDayMessage := "Rest day, no activity"
	for _, log := range logs {
		// If any log is NOT an uneventful rest day, we should send the email
		if log.SummaryMessage != restDayMessage {
			return true
		}
	}

	// All logs are uneventful rest days
	return false
}

// ConstructEmailBody constructs the plain-text email body (US041)
func (s *NotificationService) ConstructEmailBody(notification *NotificationJob, runDate time.Time) string {
	var builder strings.Builder

	// Header
	builder.WriteString(fmt.Sprintf("Daily Sync Summary for %s:\n\n", runDate.Format("January 2, 2006")))

	// Process each log
	for _, log := range notification.Logs {
		// Determine status emoji
		emoji := s.getStatusEmoji(log.Status)

		// Parse and format date
		logDate, err := time.Parse("2006-01-02", log.Date)
		if err != nil {
			s.logger.Warn("Failed to parse log date",
				"error", err,
				"date", log.Date)
			// Use original date string if parsing fails
			builder.WriteString(fmt.Sprintf("%s %s: %s\n", emoji, log.Date, log.SummaryMessage))
		} else {
			// Format date nicely
			formattedDate := logDate.Format("Mon, Jan 2")
			builder.WriteString(fmt.Sprintf("%s %s: %s\n", emoji, formattedDate, log.SummaryMessage))
		}

		// Add error details if present
		if log.Error != "" && log.Status == "failed" {
			builder.WriteString(fmt.Sprintf("   Error: %s\n", log.Error))
		}
	}

	// Add footer
	builder.WriteString("\n---\n")
	builder.WriteString("The Academy Sync - Automated Training Log\n")

	return builder.String()
}

// getStatusEmoji returns the appropriate emoji for a status
func (s *NotificationService) getStatusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "partial":
		return "⚠️"
	case "failed":
		return "❌"
	case "skipped":
		return "⏭️"
	default:
		return "•"
	}
}