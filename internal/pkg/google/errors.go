package google

import (
	"errors"
	"fmt"
	"strings"
)

// ErrReauthRequired is returned when the refresh token is invalid and user re-authorization is needed
// This error type implements the US024 requirement for recognizable errors that can trigger re-authorization flags
var ErrReauthRequired = &AuthError{
	Type:    "REAUTH_REQUIRED",
	Message: "Google connection requires re-authorization",
}

// AuthError represents authentication-related errors in Google API interactions
type AuthError struct {
	Type    string
	Message string
	Cause   error
}

func (e *AuthError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("google auth error (%s): %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("google auth error (%s): %s", e.Type, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Cause
}

// IsReauthRequired checks if an error indicates that user re-authorization is required
func IsReauthRequired(err error) bool {
	if err == nil {
		return false
	}
	
	// First check if this is our specific error type (supports wrapped errors)
	if errors.Is(err, ErrReauthRequired) {
		return true
	}
	
	// Check for our specific error type
	if authErr, ok := err.(*AuthError); ok {
		return authErr.Type == "REAUTH_REQUIRED"
	}
	
	// Check for common OAuth error patterns that indicate invalid refresh tokens
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "invalid_grant") ||
		   strings.Contains(errStr, "invalid refresh token") ||
		   strings.Contains(errStr, "authorization_revoked") ||
		   strings.Contains(errStr, "token_revoked") ||
		   strings.Contains(errStr, "refresh token is invalid")
}

// APIError represents general Google API errors
type APIError struct {
	StatusCode int
	Message    string
	Type       string
	Cause      error
}

func (e *APIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("google api error (status %d, type %s): %s (caused by: %v)", 
			e.StatusCode, e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("google api error (status %d, type %s): %s", 
		e.StatusCode, e.Type, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Cause
}

// NetworkError represents network-related errors during API calls
type NetworkError struct {
	Operation string
	Message   string
	Cause     error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("google network error during %s: %s (caused by: %v)", 
			e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("google network error during %s: %s", e.Operation, e.Message)
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// ValidationError represents data validation errors for Google APIs
type ValidationError struct {
	Field   string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("google validation error for %s: %s (caused by: %v)", 
			e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("google validation error for %s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// SheetsError represents Google Sheets-specific errors
type SheetsError struct {
	SpreadsheetID string
	Type          string
	Message       string
	Cause         error
}

func (e *SheetsError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("google sheets error (spreadsheet %s, type %s): %s (caused by: %v)", 
			e.SpreadsheetID, e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("google sheets error (spreadsheet %s, type %s): %s", 
		e.SpreadsheetID, e.Type, e.Message)
}

func (e *SheetsError) Unwrap() error {
	return e.Cause
}