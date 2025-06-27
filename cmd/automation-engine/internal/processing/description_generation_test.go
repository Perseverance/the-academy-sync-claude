package processing

import (
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// Test data helpers
func createTestActivity(distance float64, movingTime int, startTime time.Time) strava.Activity {
	return strava.Activity{
		ID:         12345,
		Name:       "Morning Run",
		Type:       "Run",
		Distance:   distance,
		MovingTime: movingTime,
		StartDate:  startTime,
	}
}

func createTestProcessedActivity(distance float64, movingTime int, formattedDuration string) ProcessedActivity {
	return ProcessedActivity{
		StravaActivity: createTestActivity(distance, movingTime, time.Now()),
		ProcessedData: struct {
			Distance float64
			Duration string
		}{
			Distance: distance / 1000, // Convert to km
			Duration: formattedDuration,
		},
	}
}

// Test prepareDescriptionUpdate for different RPE values
func TestPrepareDescriptionUpdate_SingleActivity(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	testCases := []struct {
		name             string
		rpe              int
		activity         ProcessedActivity
		originalDesc     string
		expectedContains []string
	}{
		{
			name:         "RPE 2 - Rest day with activity",
			rpe:          2,
			activity:     createTestProcessedActivity(5000, 1500, "00:25:00"),
			originalDesc: "Почивка",
			expectedContains: []string{
				"Лека разходка",
				"5.00км",
				"00:25:00",
				"5:00",
			},
		},
		{
			name:         "RPE 4 - Progressive run",
			rpe:          4,
			activity:     createTestProcessedActivity(10000, 3000, "00:50:00"),
			originalDesc: "Прогресивно бягане",
			expectedContains: []string{
				"Прогресивно бягане",
				"10.00км",
				"00:50:00",
			},
		},
		{
			name:         "RPE 6 - Steady state run",
			rpe:          6,
			activity:     createTestProcessedActivity(10000, 3000, "00:50:00"),
			originalDesc: "Равномерно бягане",
			expectedContains: []string{
				"Равномерно бягане",
				"10.00км",
				"00:50:00",
				"5:00 темпо",
			},
		},
		{
			name:         "RPE 7 - Tempo run",
			rpe:          7,
			activity:     createTestProcessedActivity(8000, 2280, "00:38:00"),
			originalDesc: "Темпово бягане",
			expectedContains: []string{
				"Темпово бягане",
				"8.00км",
				"00:38:00",
				"4:45",
			},
		},
		{
			name:         "RPE 8 - Interval workout",
			rpe:          8,
			activity:     createTestProcessedActivity(10000, 3600, "01:00:00"),
			originalDesc: "интервали: 5x1000м @ 4:00/км",
			expectedContains: []string{
				"интервали: 5x1000м @ 4:00/км",
				"изпълнено:",
				"10.00км",
				"01:00:00",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			activities := []ProcessedActivity{tc.activity}
			result := service.prepareDescriptionUpdate(tc.originalDesc, activities, tc.rpe)

			for _, expected := range tc.expectedContains {
				if !contains(result, expected) {
					t.Errorf("Expected description to contain '%s', got: %s", expected, result)
				}
			}
		})
	}
}

// Test multiple activities handling
func TestPrepareDescriptionUpdate_MultipleActivities(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	activities := []ProcessedActivity{
		createTestProcessedActivity(5000, 1500, "00:25:00"),
		createTestProcessedActivity(3000, 900, "00:15:00"),
	}

	result := service.prepareDescriptionUpdate("Бягане", activities, 5)

	expectedContains := []string{
		"2 бягания",
		"общо 8.00км",
		"00:40:00",
	}

	for _, expected := range expectedContains {
		if !contains(result, expected) {
			t.Errorf("Expected description to contain '%s', got: %s", expected, result)
		}
	}
}

// Test empty activities
func TestPrepareDescriptionUpdate_NoActivities(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	result := service.prepareDescriptionUpdate("Original description", []ProcessedActivity{}, 5)

	if result != "Original description" {
		t.Errorf("Expected original description for no activities, got: %s", result)
	}
}

// Test pace calculation
func TestCalculatePacePerKm(t *testing.T) {
	testCases := []struct {
		seconds     int
		distanceKm  float64
		expected    string
	}{
		{300, 1.0, "5:00"},    // 5 minutes for 1 km
		{600, 2.0, "5:00"},    // 10 minutes for 2 km
		{1800, 5.0, "6:00"},   // 30 minutes for 5 km
		{3600, 10.0, "6:00"},  // 60 minutes for 10 km
		{2160, 8.0, "4:30"},   // 36 minutes for 8 km
		{0, 10.0, "0:00"},     // Zero time
		{3600, 0.0, "0:00"},   // Zero distance
	}

	for _, tc := range testCases {
		result := calculatePacePerKm(tc.seconds, tc.distanceKm)
		if result != tc.expected {
			t.Errorf("For %d seconds and %.1f km, expected pace %s, got %s",
				tc.seconds, tc.distanceKm, tc.expected, result)
		}
	}
}

// Test progressive run description
func TestGenerateProgressiveRunDescription(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	activity := createTestProcessedActivity(10000, 3000, "00:50:00")
	result := service.generateProgressiveRunDescription(activity)

	expectedContains := []string{
		"Прогресивно бягане",
		"10.00км",
		"00:50:00",
		"средно",
		"5:00",
		"темпо",
	}

	for _, expected := range expectedContains {
		if !contains(result, expected) {
			t.Errorf("Expected progressive description to contain '%s', got: %s", expected, result)
		}
	}
}

// Test tempo/interval description with original description
func TestGenerateTempoIntervalDescription(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	activity := createTestProcessedActivity(10000, 3600, "01:00:00")

	// Test with interval description (lowercase check)
	intervalDesc := "интервали: 5x1000м @ 4:00/км с 90с почивка"
	result := service.generateTempoIntervalDescription(activity, intervalDesc)

	if !contains(result, "интервали: 5x1000м") {
		t.Errorf("Expected to preserve interval description, got: %s", result)
	}

	if !contains(result, "изпълнено:") {
		t.Errorf("Expected 'изпълнено:' in description, got: %s", result)
	}

	// Test without interval description
	simpleDesc := "Бързо бягане"
	result2 := service.generateTempoIntervalDescription(activity, simpleDesc)

	if !contains(result2, "Темпово бягане") {
		t.Errorf("Expected 'Темпово бягане' for non-interval description, got: %s", result2)
	}
}

// Helper function to check if string contains substring
func contains(str, substr string) bool {
	return len(substr) > 0 && len(str) >= len(substr) && 
		(str == substr || str[:len(substr)] == substr || str[len(str)-len(substr):] == substr ||
		 findSubstring(str, substr))
}

func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}