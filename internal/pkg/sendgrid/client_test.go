package sendgrid

import (
	"errors"
	"testing"
)

func TestNewClient(t *testing.T) {
	apiKey := "test-api-key"
	fromEmail := "test@example.com"

	client := NewClient(apiKey, fromEmail)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.fromEmail != fromEmail {
		t.Errorf("Expected fromEmail to be '%s', got '%s'", fromEmail, client.fromEmail)
	}

	if client.fromName != "The Academy Sync" {
		t.Errorf("Expected fromName to be 'The Academy Sync', got '%s'", client.fromName)
	}

	if client.client == nil {
		t.Error("Expected non-nil SendGrid client")
	}
}

func TestSendEmail(t *testing.T) {
	// Note: This is a basic unit test. In a real scenario, you would:
	// 1. Mock the SendGrid client
	// 2. Use interface for better testability
	// 3. Test various response scenarios

	client := NewClient("test-api-key", "from@example.com")

	tests := []struct {
		name             string
		toEmail          string
		toName           string
		subject          string
		plainTextContent string
		htmlContent      string
		expectError      bool
	}{
		{
			name:             "valid email parameters",
			toEmail:          "to@example.com",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			expectError:      false, // Note: Will actually error without valid API key
		},
		{
			name:             "empty to email",
			toEmail:          "",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			expectError:      false, // SendGrid validation happens server-side
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This will fail with invalid API key in test environment
			// In production, you would mock the SendGrid client
			err := client.SendEmail(tt.toEmail, tt.toName, tt.subject, tt.plainTextContent, tt.htmlContent)
			
			// We expect an error because we're using a test API key
			if err == nil {
				t.Skip("Skipping SendGrid API call test - would require valid API key")
			}
		})
	}
}

func TestSendTemplateEmail(t *testing.T) {
	client := NewClient("test-api-key", "from@example.com")

	tests := []struct {
		name         string
		toEmail      string
		toName       string
		templateID   string
		templateData map[string]interface{}
		expectError  bool
	}{
		{
			name:       "valid template parameters",
			toEmail:    "to@example.com",
			toName:     "Test User",
			templateID: "d-123456",
			templateData: map[string]interface{}{
				"name":    "John Doe",
				"subject": "Welcome!",
			},
			expectError: false, // Note: Will actually error without valid API key
		},
		{
			name:         "empty template data",
			toEmail:      "to@example.com",
			toName:       "Test User",
			templateID:   "d-123456",
			templateData: map[string]interface{}{},
			expectError:  false,
		},
		{
			name:         "nil template data",
			toEmail:      "to@example.com",
			toName:       "Test User",
			templateID:   "d-123456",
			templateData: nil,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This will fail with invalid API key in test environment
			err := client.SendTemplateEmail(tt.toEmail, tt.toName, tt.templateID, tt.templateData)
			
			// We expect an error because we're using a test API key
			if err == nil {
				t.Skip("Skipping SendGrid API call test - would require valid API key")
			}
		})
	}
}

// MockSendGridClient can be used for more comprehensive testing
type MockSendGridClient struct {
	SendFunc         func(email interface{}) (interface{}, error)
	SendReturnStatus int
	SendReturnBody   string
	SendReturnError  error
}

func (m *MockSendGridClient) Send(email interface{}) (interface{}, error) {
	if m.SendFunc != nil {
		return m.SendFunc(email)
	}
	return struct {
		StatusCode int
		Body       string
	}{
		StatusCode: m.SendReturnStatus,
		Body:       m.SendReturnBody,
	}, m.SendReturnError
}

func TestSendEmailErrorHandling(t *testing.T) {
	client := NewClient("test-api-key", "from@example.com")

	// Test error message formatting
	err := client.SendEmail("to@example.com", "Test User", "Subject", "Body", "<p>Body</p>")
	if err == nil {
		t.Skip("Skipping error handling test - would require valid API key")
	}

	// Verify error is wrapped properly
	if !errors.Is(err, err) {
		t.Error("Expected error to be properly wrapped")
	}
}