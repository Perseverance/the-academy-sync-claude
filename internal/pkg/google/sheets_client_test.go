package google

import (
	"testing"
)

func TestConvertTimeToFractionalDay(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		expected  float64
		expectErr bool
	}{
		{
			name:      "zero time",
			timeStr:   "00:00:00",
			expected:  0.0,
			expectErr: false,
		},
		{
			name:      "empty string",
			timeStr:   "",
			expected:  0.0,
			expectErr: false,
		},
		{
			name:      "one hour",
			timeStr:   "01:00:00",
			expected:  1.0 / 24.0, // 0.041666...
			expectErr: false,
		},
		{
			name:      "noon (12 hours)",
			timeStr:   "12:00:00",
			expected:  0.5,
			expectErr: false,
		},
		{
			name:      "1 hour 30 minutes",
			timeStr:   "01:30:00",
			expected:  1.5 / 24.0, // 0.0625
			expectErr: false,
		},
		{
			name:      "1 hour 10 minutes 10 seconds",
			timeStr:   "01:10:10",
			expected:  4210.0 / 86400.0, // ~0.04873
			expectErr: false,
		},
		{
			name:      "23:59:59",
			timeStr:   "23:59:59",
			expected:  86399.0 / 86400.0, // ~0.999988
			expectErr: false,
		},
		{
			name:      "invalid format",
			timeStr:   "invalid",
			expected:  0.0,
			expectErr: true,
		},
		{
			name:      "missing seconds",
			timeStr:   "01:30",
			expected:  0.0,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertTimeToFractionalDay(tt.timeStr)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Allow small floating point differences
			const epsilon = 0.000001
			if diff := result - tt.expected; diff < -epsilon || diff > epsilon {
				t.Errorf("Expected %f, got %f (difference: %f)", tt.expected, result, diff)
			}
		})
	}
}

// Test that common time values convert correctly
func TestConvertTimeToFractionalDay_CommonValues(t *testing.T) {
	testCases := []struct {
		description string
		time        string
		hours       float64
	}{
		{"6 hours", "06:00:00", 6.0},
		{"30 minutes", "00:30:00", 0.5},
		{"1 hour 15 minutes", "01:15:00", 1.25},
		{"2 hours 30 minutes 30 seconds", "02:30:30", 2.5083333},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			expected := tc.hours / 24.0
			result, err := convertTimeToFractionalDay(tc.time)
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			// Verify the conversion is correct
			const epsilon = 0.000001
			if diff := result - expected; diff < -epsilon || diff > epsilon {
				t.Errorf("For %s (%s): expected %f, got %f", tc.description, tc.time, expected, result)
			}
		})
	}
}