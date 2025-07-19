package internal

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/sendgrid"
	"github.com/vanng822/go-premailer/premailer"
)

// ProcessedDay represents a single day's processing result for the email template
type ProcessedDay struct {
	StatusIcon     string
	ProcessedDate  string
	SummaryMessage string
}

// TemplateData represents the data structure for the HTML email template
type TemplateData struct {
	RunDate        string
	ProcessedDays  []ProcessedDay
	DashboardURL   string
	SpreadsheetURL string
}

// NotificationService handles the core business logic for processing notifications
type NotificationService struct {
	sendgridClient *sendgrid.Client
	logger         *logger.Logger
	emailTemplate  *template.Template
	baseURL        string
	frontendURL    string
}

//go:embed templates/summary.html
var emailTemplates embed.FS

// NewNotificationService creates a new notification service
func NewNotificationService(sendgridClient *sendgrid.Client, logger *logger.Logger, baseURL string, frontendURL string) (*NotificationService, error) {
	// Load the email template
	tmplContent, err := emailTemplates.ReadFile("templates/summary.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read email template: %w", err)
	}

	// Parse the template
	tmpl, err := template.New("email").Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email template: %w", err)
	}

	return &NotificationService{
		sendgridClient: sendgridClient,
		logger:         logger.WithContext("component", "notification_service"),
		emailTemplate:  tmpl,
		baseURL:        baseURL,
		frontendURL:    frontendURL,
	}, nil
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
	
	// Render HTML body
	htmlBody, err := s.RenderHTMLEmail(notification, runDate)
	if err != nil {
		s.logger.Warn("Failed to render HTML email, falling back to plain text",
			"error", err,
			"user_id", notification.UserID)
		htmlBody = "" // Fall back to plain text only
	}

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

// RenderHTMLEmail renders the HTML email body using the template
func (s *NotificationService) RenderHTMLEmail(notification *NotificationJob, runDate time.Time) (string, error) {
	// Prepare template data
	// Build the Google Sheets URL from the spreadsheet ID
	spreadsheetURL := ""
	if notification.SpreadsheetID != "" {
		spreadsheetURL = fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s", notification.SpreadsheetID)
	}
	
	templateData := TemplateData{
		RunDate:        runDate.Format("January 2, 2006"),
		ProcessedDays:  make([]ProcessedDay, 0, len(notification.Logs)),
		DashboardURL:   s.frontendURL,  // Points to the frontend
		SpreadsheetURL: spreadsheetURL,
	}

	// Convert logs to template format
	for _, log := range notification.Logs {
		// Parse and format date
		logDate, err := time.Parse("2006-01-02", log.Date)
		if err != nil {
			s.logger.Warn("Failed to parse log date for HTML template",
				"error", err,
				"date", log.Date)
			// Use original date string if parsing fails
			templateData.ProcessedDays = append(templateData.ProcessedDays, ProcessedDay{
				StatusIcon:     s.getStatusEmoji(log.Status),
				ProcessedDate:  log.Date,
				SummaryMessage: log.SummaryMessage,
			})
		} else {
			// Format date nicely
			formattedDate := logDate.Format("Mon, Jan 2")
			templateData.ProcessedDays = append(templateData.ProcessedDays, ProcessedDay{
				StatusIcon:     s.getStatusEmoji(log.Status),
				ProcessedDate:  formattedDate,
				SummaryMessage: log.SummaryMessage,
			})
		}
	}

	// Render the template
	var buf bytes.Buffer
	if err := s.emailTemplate.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute email template: %w", err)
	}

	// Apply CSS inlining for email client compatibility
	prem, err := premailer.NewPremailerFromString(buf.String(), premailer.NewOptions())
	if err != nil {
		return "", fmt.Errorf("failed to create premailer: %w", err)
	}

	htmlResult, err := prem.Transform()
	if err != nil {
		return "", fmt.Errorf("failed to inline CSS: %w", err)
	}

	return htmlResult, nil
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