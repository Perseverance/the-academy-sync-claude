// Package logger provides a structured logging utility for all backend components.
// It uses Go's standard log/slog package to provide JSON-formatted logging with
// configurable log levels and service identification.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the available log levels.
type LogLevel string

const (
	// LevelDebug provides detailed information for diagnosing problems.
	LevelDebug LogLevel = "DEBUG"
	// LevelInfo provides general information about system operation.
	LevelInfo LogLevel = "INFO"
	// LevelWarning provides warning messages for potential issues.
	LevelWarning LogLevel = "WARNING"
	// LevelError provides error messages for failures that don't stop execution.
	LevelError LogLevel = "ERROR"
	// LevelCritical provides critical errors that may stop system operation.
	LevelCritical LogLevel = "CRITICAL"
)

// Logger wraps slog.Logger with additional functionality for our application.
type Logger struct {
	*slog.Logger
	serviceName string
}

// New creates a new structured logger instance with JSON output to stdout.
// It reads the log level from the LOG_LEVEL environment variable and sets
// a default service name attribute for all log entries.
//
// Parameters:
//   - serviceName: The name of the service using this logger (e.g., "backend-api")
//
// The logger outputs JSON-formatted logs to stdout/stderr as required by the MVP.
// Log levels are parsed from the LOG_LEVEL environment variable, defaulting to INFO.
//
// Example usage:
//
//	logger := logger.New("backend-api")
//	logger.Info("Service starting", "port", 8080, "environment", "production")
func New(serviceName string) *Logger {
	// Parse log level from environment variable
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))

	// Create JSON handler that writes to stdout
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	// Create logger with service name attribute
	slogger := slog.New(handler).With("service", serviceName)

	return &Logger{
		Logger:      slogger,
		serviceName: serviceName,
	}
}

// parseLogLevel converts a string log level to slog.Level.
// Returns slog.LevelInfo as default for invalid or empty input.
func parseLogLevel(levelStr string) slog.Level {
	levelStr = strings.ToUpper(strings.TrimSpace(levelStr))
	
	switch LogLevel(levelStr) {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarning:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelCritical:
		// Map CRITICAL to Error level in slog (highest available)
		return slog.LevelError
	default:
		// Default to INFO level for unknown values
		return slog.LevelInfo
	}
}

// Critical logs a critical error message. These are severe errors that may
// stop system operation. Maps to slog.LevelError internally.
func (l *Logger) Critical(msg string, args ...any) {
	// Add critical indicator to help distinguish from regular errors
	args = append([]any{"severity", "critical"}, args...)
	l.Logger.Error(msg, args...)
}

// ServiceName returns the service name associated with this logger.
func (l *Logger) ServiceName() string {
	return l.serviceName
}

// WithContext creates a new logger with additional context fields.
// This is useful for adding request-specific information like trace IDs.
func (l *Logger) WithContext(args ...any) *Logger {
	return &Logger{
		Logger:      l.Logger.With(args...),
		serviceName: l.serviceName,
	}
}