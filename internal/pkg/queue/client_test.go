package queue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func TestJob_JSONSerialization(t *testing.T) {
	// Test that Job struct can be properly serialized/deserialized
	original := &Job{
		UserID:         123,
		TraceID:        "test-trace-id",
		TriggerType:    "manual",
		CreatedAt:      time.Now().Truncate(time.Second), // Truncate for comparison
		TimeoutSeconds: 300,
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	// Deserialize from JSON
	var restored Job
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	// Compare fields
	if restored.UserID != original.UserID {
		t.Errorf("UserID mismatch: expected %d, got %d", original.UserID, restored.UserID)
	}

	if restored.TraceID != original.TraceID {
		t.Errorf("TraceID mismatch: expected %s, got %s", original.TraceID, restored.TraceID)
	}

	if restored.TriggerType != original.TriggerType {
		t.Errorf("TriggerType mismatch: expected %s, got %s", original.TriggerType, restored.TriggerType)
	}

	if !restored.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch: expected %v, got %v", original.CreatedAt, restored.CreatedAt)
	}

	if restored.TimeoutSeconds != original.TimeoutSeconds {
		t.Errorf("TimeoutSeconds mismatch: expected %d, got %d", original.TimeoutSeconds, restored.TimeoutSeconds)
	}
}

func TestGenerateTraceID(t *testing.T) {
	// Generate multiple trace IDs and ensure they're unique
	ids := make(map[string]bool)
	
	for i := 0; i < 100; i++ {
		id := GenerateTraceID()
		
		if id == "" {
			t.Error("Generated empty trace ID")
		}
		
		if ids[id] {
			t.Errorf("Duplicate trace ID generated: %s", id)
		}
		
		ids[id] = true
		
		// Basic UUID format check (simplified)
		if len(id) != 36 {
			t.Errorf("Invalid trace ID format: %s (length %d)", id, len(id))
		}
	}
}

func TestNewClient_InvalidURL(t *testing.T) {
	log := logger.New("test")
	
	// Test with invalid Redis URL
	client, err := NewClient("invalid-url", log)
	
	if err == nil {
		t.Error("Expected error for invalid Redis URL")
	}
	
	if client != nil {
		t.Error("Expected nil client for invalid URL")
	}
}

func TestConstants(t *testing.T) {
	// Test that constants are set to expected values
	if JobsQueueName != "jobs_queue" {
		t.Errorf("Expected JobsQueueName to be 'jobs_queue', got '%s'", JobsQueueName)
	}
	
	if JobTimeout != 30*time.Second {
		t.Errorf("Expected JobTimeout to be 30s, got %v", JobTimeout)
	}
}

// Note: Integration tests with actual Redis would require a test Redis instance
// These tests focus on the logic that doesn't require external dependencies