package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	// Test with default log level (no environment variable)
	os.Unsetenv("LOG_LEVEL")
	logger := New("test-service")
	
	if logger == nil {
		t.Fatal("Expected logger to be created, got nil")
	}
	
	if logger.ServiceName() != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", logger.ServiceName())
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"WARNING", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"error", slog.LevelError},
		{"CRITICAL", slog.LevelError}, // Maps to Error level
		{"critical", slog.LevelError},
		{"", slog.LevelInfo},          // Default
		{"INVALID", slog.LevelInfo},   // Default
		{"  INFO  ", slog.LevelInfo},  // With whitespace
	}
	
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := parseLogLevel(test.input)
			if result != test.expected {
				t.Errorf("parseLogLevel(%q) = %v, expected %v", test.input, result, test.expected)
			}
		})
	}
}

func TestLoggerWithEnvironmentVariable(t *testing.T) {
	tests := []struct {
		envValue string
		logLevel string
		expected bool // whether the log should appear
	}{
		{"DEBUG", "DEBUG", true},
		{"DEBUG", "INFO", true},
		{"DEBUG", "WARN", true},
		{"DEBUG", "ERROR", true},
		{"INFO", "DEBUG", false},
		{"INFO", "INFO", true},
		{"INFO", "WARN", true},
		{"INFO", "ERROR", true},
		{"WARNING", "DEBUG", false},
		{"WARNING", "INFO", false},
		{"WARNING", "WARN", true},
		{"WARNING", "ERROR", true},
		{"ERROR", "DEBUG", false},
		{"ERROR", "INFO", false},
		{"ERROR", "WARN", false},
		{"ERROR", "ERROR", true},
	}
	
	for _, test := range tests {
		t.Run(test.envValue+"_"+test.logLevel, func(t *testing.T) {
			// Set environment variable
			os.Setenv("LOG_LEVEL", test.envValue)
			defer os.Unsetenv("LOG_LEVEL")
			
			// Capture output
			var buf bytes.Buffer
			
			// Create a temporary handler for testing
			handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: parseLogLevel(test.envValue),
			})
			logger := &Logger{
				Logger:      slog.New(handler).With("service", "test"),
				serviceName: "test",
			}
			
			// Log at the specified level
			switch test.logLevel {
			case "DEBUG":
				logger.Debug("test message")
			case "INFO":
				logger.Info("test message")
			case "WARN":
				logger.Warn("test message")
			case "ERROR":
				logger.Error("test message")
			}
			
			output := buf.String()
			hasOutput := strings.TrimSpace(output) != ""
			
			if hasOutput != test.expected {
				t.Errorf("Expected output=%v, got output=%v for env=%s, log=%s. Output: %s", 
					test.expected, hasOutput, test.envValue, test.logLevel, output)
			}
		})
	}
}

func TestLoggerJSONOutput(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	
	// Create logger with JSON handler writing to buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := &Logger{
		Logger:      slog.New(handler).With("service", "test-service"),
		serviceName: "test-service",
	}
	
	// Log a message with additional fields
	logger.Info("test message", "key1", "value1", "key2", 42)
	
	// Parse JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	
	// Verify required fields
	if logEntry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", logEntry["msg"])
	}
	
	if logEntry["level"] != "INFO" {
		t.Errorf("Expected level='INFO', got %v", logEntry["level"])
	}
	
	if logEntry["service"] != "test-service" {
		t.Errorf("Expected service='test-service', got %v", logEntry["service"])
	}
	
	if logEntry["key1"] != "value1" {
		t.Errorf("Expected key1='value1', got %v", logEntry["key1"])
	}
	
	if logEntry["key2"] != float64(42) { // JSON numbers are float64
		t.Errorf("Expected key2=42, got %v", logEntry["key2"])
	}
	
	// Verify time field exists
	if _, exists := logEntry["time"]; !exists {
		t.Error("Expected 'time' field in JSON output")
	}
}

func TestCriticalLogging(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	
	// Create logger with JSON handler writing to buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	logger := &Logger{
		Logger:      slog.New(handler).With("service", "test-service"),
		serviceName: "test-service",
	}
	
	// Log a critical message
	logger.Critical("critical error occurred", "errorCode", "CRIT001")
	
	// Parse JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	
	// Verify it's logged as ERROR level with severity indicator
	if logEntry["level"] != "ERROR" {
		t.Errorf("Expected level='ERROR', got %v", logEntry["level"])
	}
	
	if logEntry["severity"] != "critical" {
		t.Errorf("Expected severity='critical', got %v", logEntry["severity"])
	}
	
	if logEntry["msg"] != "critical error occurred" {
		t.Errorf("Expected msg='critical error occurred', got %v", logEntry["msg"])
	}
	
	if logEntry["errorCode"] != "CRIT001" {
		t.Errorf("Expected errorCode='CRIT001', got %v", logEntry["errorCode"])
	}
}

func TestWithContext(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	
	// Create logger with JSON handler writing to buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := &Logger{
		Logger:      slog.New(handler).With("service", "test-service"),
		serviceName: "test-service",
	}
	
	// Create logger with context
	contextLogger := logger.WithContext("requestId", "req-123", "userId", "user-456")
	
	// Verify service name is preserved
	if contextLogger.ServiceName() != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", contextLogger.ServiceName())
	}
	
	// Log a message
	contextLogger.Info("processing request")
	
	// Parse JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	
	// Verify context fields are included
	if logEntry["requestId"] != "req-123" {
		t.Errorf("Expected requestId='req-123', got %v", logEntry["requestId"])
	}
	
	if logEntry["userId"] != "user-456" {
		t.Errorf("Expected userId='user-456', got %v", logEntry["userId"])
	}
	
	if logEntry["service"] != "test-service" {
		t.Errorf("Expected service='test-service', got %v", logEntry["service"])
	}
}

func TestLogLevelConstants(t *testing.T) {
	// Verify log level constants are correctly defined
	expectedLevels := map[LogLevel]string{
		LevelDebug:    "DEBUG",
		LevelInfo:     "INFO",
		LevelWarning:  "WARNING",
		LevelError:    "ERROR",
		LevelCritical: "CRITICAL",
	}
	
	for level, expected := range expectedLevels {
		if string(level) != expected {
			t.Errorf("Expected %s, got %s", expected, string(level))
		}
	}
}