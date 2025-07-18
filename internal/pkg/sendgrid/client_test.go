package sendgrid

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
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
	tests := []struct {
		name             string
		toEmail          string
		toName           string
		subject          string
		plainTextContent string
		htmlContent      string
		mockStatus       int
		mockBody         string
		mockError        error
		expectError      bool
		expectedErrMsg   string
	}{
		{
			name:             "successful email send",
			toEmail:          "to@example.com",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			mockStatus:       202,
			mockBody:         "Accepted",
			expectError:      false,
		},
		{
			name:             "sendgrid returns 400 error",
			toEmail:          "to@example.com",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			mockStatus:       400,
			mockBody:         "Bad Request: Invalid email format",
			expectError:      true,
			expectedErrMsg:   "sendgrid returned error status 400: Bad Request: Invalid email format",
		},
		{
			name:             "sendgrid returns 401 unauthorized",
			toEmail:          "to@example.com",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			mockStatus:       401,
			mockBody:         "Unauthorized",
			expectError:      true,
			expectedErrMsg:   "sendgrid returned error status 401: Unauthorized",
		},
		{
			name:             "network error",
			toEmail:          "to@example.com",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			mockError:        errors.New("network timeout"),
			expectError:      true,
			expectedErrMsg:   "failed to send email: network timeout",
		},
		{
			name:             "empty to email",
			toEmail:          "",
			toName:           "Test User",
			subject:          "Test Subject",
			plainTextContent: "Test plain text",
			htmlContent:      "<p>Test HTML</p>",
			mockStatus:       202,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &MockSendGridClient{
				SendReturnStatus: tt.mockStatus,
				SendReturnBody:   tt.mockBody,
				SendReturnError:  tt.mockError,
			}

			// Optionally verify the email content
			if tt.name == "successful email send" {
				mockClient.SendFunc = func(email *mail.SGMailV3) (*rest.Response, error) {
					// Verify email structure
					if email == nil {
						t.Error("Expected non-nil email")
					}
					if email.From == nil || email.From.Address != "from@example.com" {
						t.Errorf("Expected from email to be 'from@example.com', got %v", email.From)
					}
					if email.Subject != tt.subject {
						t.Errorf("Expected subject '%s', got '%s'", tt.subject, email.Subject)
					}

					return &rest.Response{
						StatusCode: tt.mockStatus,
						Body:       tt.mockBody,
					}, nil
				}
			}

			// Create client with mock
			client := NewClientWithInterface(mockClient, "from@example.com")

			// Test send email
			err := client.SendEmail(tt.toEmail, tt.toName, tt.subject, tt.plainTextContent, tt.htmlContent)

			// Verify results
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && tt.expectedErrMsg != "" && err != nil {
				if err.Error() != tt.expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
			}
		})
	}
}

func TestSendTemplateEmail(t *testing.T) {
	tests := []struct {
		name           string
		toEmail        string
		toName         string
		templateID     string
		templateData   map[string]interface{}
		mockStatus     int
		mockBody       string
		mockError      error
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:       "successful template email send",
			toEmail:    "to@example.com",
			toName:     "Test User",
			templateID: "d-123456",
			templateData: map[string]interface{}{
				"name":    "John Doe",
				"subject": "Welcome!",
			},
			mockStatus:  202,
			mockBody:    "Accepted",
			expectError: false,
		},
		{
			name:         "empty template data",
			toEmail:      "to@example.com",
			toName:       "Test User",
			templateID:   "d-123456",
			templateData: map[string]interface{}{},
			mockStatus:   202,
			mockBody:     "Accepted",
			expectError:  false,
		},
		{
			name:         "nil template data",
			toEmail:      "to@example.com",
			toName:       "Test User",
			templateID:   "d-123456",
			templateData: nil,
			mockStatus:   202,
			mockBody:     "Accepted",
			expectError:  false,
		},
		{
			name:       "invalid template ID",
			toEmail:    "to@example.com",
			toName:     "Test User",
			templateID: "invalid-template",
			templateData: map[string]interface{}{
				"name": "John Doe",
			},
			mockStatus:     400,
			mockBody:       "Invalid template ID",
			expectError:    true,
			expectedErrMsg: "sendgrid returned error status 400: Invalid template ID",
		},
		{
			name:           "sendgrid service error",
			toEmail:        "to@example.com",
			toName:         "Test User",
			templateID:     "d-123456",
			templateData:   nil,
			mockError:      errors.New("service unavailable"),
			expectError:    true,
			expectedErrMsg: "failed to send template email: service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &MockSendGridClient{
				SendReturnStatus: tt.mockStatus,
				SendReturnBody:   tt.mockBody,
				SendReturnError:  tt.mockError,
			}

			// Verify template email structure for successful case
			if tt.name == "successful template email send" {
				mockClient.SendFunc = func(email *mail.SGMailV3) (*rest.Response, error) {
					// Verify email structure
					if email == nil {
						t.Error("Expected non-nil email")
					}
					if email.TemplateID != tt.templateID {
						t.Errorf("Expected template ID '%s', got '%s'", tt.templateID, email.TemplateID)
					}
					if email.From == nil || email.From.Address != "from@example.com" {
						t.Errorf("Expected from email to be 'from@example.com', got %v", email.From)
					}

					// Verify personalization
					if len(email.Personalizations) == 0 {
						t.Error("Expected at least one personalization")
					} else {
						p := email.Personalizations[0]
						if len(p.To) == 0 || p.To[0].Address != tt.toEmail {
							t.Errorf("Expected to email '%s', got %v", tt.toEmail, p.To)
						}

						// Verify dynamic template data
						if tt.templateData != nil {
							for key, expectedValue := range tt.templateData {
								if actualValue, ok := p.DynamicTemplateData[key]; !ok {
									t.Errorf("Missing template data key '%s'", key)
								} else if actualValue != expectedValue {
									t.Errorf("Template data key '%s': expected '%v', got '%v'", key, expectedValue, actualValue)
								}
							}
						}
					}

					return &rest.Response{
						StatusCode: tt.mockStatus,
						Body:       tt.mockBody,
					}, nil
				}
			}

			// Create client with mock
			client := NewClientWithInterface(mockClient, "from@example.com")

			// Test send template email
			err := client.SendTemplateEmail(tt.toEmail, tt.toName, tt.templateID, tt.templateData)

			// Verify results
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && tt.expectedErrMsg != "" && err != nil {
				if err.Error() != tt.expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
			}
		})
	}
}

// MockSendGridClient implements the SendGridClient interface for testing
type MockSendGridClient struct {
	SendFunc         func(email *mail.SGMailV3) (*rest.Response, error)
	SendReturnStatus int
	SendReturnBody   string
	SendReturnError  error
}

// Send implements the SendGridClient interface
func (m *MockSendGridClient) Send(email *mail.SGMailV3) (*rest.Response, error) {
	if m.SendFunc != nil {
		return m.SendFunc(email)
	}
	return &rest.Response{
		StatusCode: m.SendReturnStatus,
		Body:       m.SendReturnBody,
	}, m.SendReturnError
}

// Define custom error types for testing
type NetworkError struct {
	msg string
}

func (e NetworkError) Error() string {
	return e.msg
}

type TimeoutError struct {
	msg string
}

func (e TimeoutError) Error() string {
	return e.msg
}

func TestSendEmailErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		mockError      error
		expectedErrMsg string
		checkWrapping  bool
		wrappedError   error
	}{
		{
			name:           "network error with wrapping",
			mockError:      NetworkError{msg: "connection refused"},
			expectedErrMsg: "failed to send email: connection refused",
			checkWrapping:  true,
			wrappedError:   NetworkError{msg: "connection refused"},
		},
		{
			name:           "timeout error with wrapping",
			mockError:      TimeoutError{msg: "context deadline exceeded"},
			expectedErrMsg: "failed to send email: context deadline exceeded",
			checkWrapping:  true,
			wrappedError:   TimeoutError{msg: "context deadline exceeded"},
		},
		{
			name:           "generic error",
			mockError:      fmt.Errorf("service unavailable"),
			expectedErrMsg: "failed to send email: service unavailable",
			checkWrapping:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client that returns an error
			mockClient := &MockSendGridClient{
				SendReturnError: tt.mockError,
			}

			// Create client with mock
			client := NewClientWithInterface(mockClient, "from@example.com")

			// Test error handling
			err := client.SendEmail("to@example.com", "Test User", "Subject", "Body", "<p>Body</p>")

			if err == nil {
				t.Error("Expected error but got none")
			}

			if err != nil && err.Error() != tt.expectedErrMsg {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, err.Error())
			}

			// Check error wrapping if specified
			if tt.checkWrapping && err != nil {
				// Verify that the error wraps the original error type
				var networkErr NetworkError
				if errors.As(err, &networkErr) {
					if _, ok := tt.wrappedError.(NetworkError); !ok {
						t.Errorf("Expected error to wrap NetworkError, but it doesn't")
					}
				}

				var timeoutErr TimeoutError
				if errors.As(err, &timeoutErr) {
					if _, ok := tt.wrappedError.(TimeoutError); !ok {
						t.Errorf("Expected error to wrap TimeoutError, but it doesn't")
					}
				}

				// Also verify using errors.Is
				if !errors.Is(err, tt.mockError) {
					t.Errorf("Expected error to wrap the original error using errors.Is")
				}
			}
		})
	}
}

func TestSendTemplateEmailErrorHandling(t *testing.T) {
	// Define a specific error for testing
	baseError := NetworkError{msg: "network unreachable"}
	
	// Create mock client that returns an error
	mockClient := &MockSendGridClient{
		SendReturnError: baseError,
	}

	// Create client with mock
	client := NewClientWithInterface(mockClient, "from@example.com")

	// Test send template email error handling
	err := client.SendTemplateEmail("to@example.com", "Test User", "d-123456", nil)

	if err == nil {
		t.Fatal("Expected error but got none")
	}

	expectedMsg := "failed to send template email: network unreachable"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Verify error wrapping
	var networkErr NetworkError
	if !errors.As(err, &networkErr) {
		t.Error("Expected error to wrap NetworkError type")
	}

	// Verify with errors.Is
	if !errors.Is(err, baseError) {
		t.Error("Expected error to wrap the original error using errors.Is")
	}
}

func TestNewClientWithInterface(t *testing.T) {
	// Create a mock client
	mockClient := &MockSendGridClient{}
	fromEmail := "test@example.com"

	// Create client with interface
	client := NewClientWithInterface(mockClient, fromEmail)

	// Verify client properties
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.fromEmail != fromEmail {
		t.Errorf("Expected fromEmail to be '%s', got '%s'", fromEmail, client.fromEmail)
	}

	if client.fromName != "The Academy Sync" {
		t.Errorf("Expected fromName to be 'The Academy Sync', got '%s'", client.fromName)
	}

	if client.client != mockClient {
		t.Error("Expected client to use the provided mock client")
	}
}

