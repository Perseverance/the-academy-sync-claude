package internal

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/sendgrid"
	"github.com/vanng822/go-premailer/premailer"
	"golang.org/x/text/language"
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
	// Localized strings
	Title           string
	Intro           string
	StatusHeader    string
	DateHeader      string
	SummaryHeader   string
	FooterLine1     string
	FooterLine2     string
	DashboardLink   string
	SpreadsheetLink string
}

// NotificationService handles the core business logic for processing notifications
type NotificationService struct {
	sendgridClient *sendgrid.Client
	logger         *logger.Logger
	emailTemplate  *template.Template
	baseURL        string
	frontendURL    string
	i18nBundle     *i18n.Bundle
}

//go:embed templates/summary.html
var emailTemplates embed.FS

//go:embed assets/locales/*.json
var localesFS embed.FS

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

	// Initialize i18n bundle
	bundle := i18n.NewBundle(language.Bulgarian)
	bundle.RegisterUnmarshalFunc("json", i18n.UnmarshalFunc(json.Unmarshal))
	
	// Load Bulgarian translations
	bgData, err := localesFS.ReadFile("assets/locales/bg.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read Bulgarian translations: %w", err)
	}
	bundle.MustParseMessageFileBytes(bgData, "bg.json")
	
	// Load English translations
	enData, err := localesFS.ReadFile("assets/locales/en.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read English translations: %w", err)
	}
	bundle.MustParseMessageFileBytes(enData, "en.json")

	return &NotificationService{
		sendgridClient: sendgridClient,
		logger:         logger.WithContext("component", "notification_service"),
		emailTemplate:  tmpl,
		baseURL:        baseURL,
		frontendURL:    frontendURL,
		i18nBundle:     bundle,
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

	// Get localizer for user's language preference
	lang := notification.LanguagePreference
	if lang == "" {
		lang = "bg" // Default to Bulgarian
	}
	localizer := i18n.NewLocalizer(s.i18nBundle, lang)
	
	// Construct email body (US041)
	subject := localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "email.subject",
		TemplateData: map[string]interface{}{
			"Date": runDate.Format("Jan 2, 2006"),
		},
	})
	plainTextBody := s.ConstructEmailBody(notification, runDate, localizer)
	
	// Render HTML body
	htmlBody, err := s.RenderHTMLEmail(notification, runDate, localizer)
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
	// We need to check for the rest day message in both Bulgarian and English
	// since we don't have access to the user's language preference at this point
	bgLocalizer := i18n.NewLocalizer(s.i18nBundle, "bg")
	enLocalizer := i18n.NewLocalizer(s.i18nBundle, "en")
	
	bgRestDayMessage := bgLocalizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "email.restDayMessage",
	})
	enRestDayMessage := enLocalizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "email.restDayMessage",
	})
	
	for _, log := range logs {
		// If any log is NOT an uneventful rest day, we should send the email
		if log.SummaryMessage != bgRestDayMessage && log.SummaryMessage != enRestDayMessage {
			return true
		}
	}

	// All logs are uneventful rest days
	return false
}

// ConstructEmailBody constructs the plain-text email body (US041)
func (s *NotificationService) ConstructEmailBody(notification *NotificationJob, runDate time.Time, localizer *i18n.Localizer) string {
	var builder strings.Builder

	// Header
	headerText := localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "email.headerText",
		TemplateData: map[string]interface{}{
			"DateLong": runDate.Format("January 2, 2006"),
		},
	})
	builder.WriteString(headerText + "\n")

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
			// Format date nicely using localized format
			dateFormat := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "email.dateFormat",
			})
			formattedDate := logDate.Format(dateFormat)
			builder.WriteString(fmt.Sprintf("%s %s: %s\n", emoji, formattedDate, log.SummaryMessage))
		}

		// Add error details if present
		if log.Error != "" && log.Status == "failed" {
			errorPrefix := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "email.errorPrefix",
				TemplateData: map[string]interface{}{
					"Error": log.Error,
				},
			})
			builder.WriteString("   " + errorPrefix + "\n")
		}
	}

	// Add footer
	builder.WriteString("\n---\n")
	footerLine1 := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.footer.line1"})
	footerLine2 := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.footer.line2"})
	builder.WriteString(footerLine1 + " - " + footerLine2 + "\n")

	return builder.String()
}

// RenderHTMLEmail renders the HTML email body using the template
func (s *NotificationService) RenderHTMLEmail(notification *NotificationJob, runDate time.Time, localizer *i18n.Localizer) (string, error) {
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
		// Localized strings
		Title:           localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.htmlTitle"}),
		Intro:           localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.htmlIntro"}),
		StatusHeader:    localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.tableHeaders.status"}),
		DateHeader:      localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.tableHeaders.date"}),
		SummaryHeader:   localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.tableHeaders.summary"}),
		FooterLine1:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.footer.line1"}),
		FooterLine2:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.footer.line2"}),
		DashboardLink:   localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.footerLinks.dashboard"}),
		SpreadsheetLink: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "email.footerLinks.spreadsheet"}),
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
			// Format date nicely using localized format
			dateFormat := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "email.dateFormat",
			})
			formattedDate := logDate.Format(dateFormat)
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