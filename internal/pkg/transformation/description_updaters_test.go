package transformation

import (
	"testing"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

func TestUpdateIntervalWorkoutDescription(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		laps     []strava.Lap
		expected string
	}{
		{
			name: "6x1000 with duration only",
			desc: "загрявка, 6 x 1000 / 600 (3:45 - 3:50 мин), разпускане",
			laps: []strava.Lap{
				{MovingTime: 600, Distance: 2000},  // warmup
				{MovingTime: 225, Distance: 1000},  // work 1 (3:45)
				{MovingTime: 90, Distance: 600},    // recovery 1
				{MovingTime: 230, Distance: 1000},  // work 2 (3:50)
				{MovingTime: 90, Distance: 600},    // recovery 2
				{MovingTime: 227, Distance: 1000},  // work 3 (3:47)
				{MovingTime: 90, Distance: 600},    // recovery 3
				{MovingTime: 228, Distance: 1000},  // work 4 (3:48)
				{MovingTime: 90, Distance: 600},    // recovery 4
				{MovingTime: 226, Distance: 1000},  // work 5 (3:46)
				{MovingTime: 90, Distance: 600},    // recovery 5
				{MovingTime: 229, Distance: 1000},  // work 6 (3:49)
				{MovingTime: 90, Distance: 600},    // recovery 6
				{MovingTime: 600, Distance: 2000},  // cooldown
			},
			expected: "загрявка, 6 x 1000 / 600 (3:47 мин), разпускане",
		},
		{
			name: "5x1.6km with duration and pace",
			desc: "загрявка, 5 х 1.6 км / 400 м (6:10 - 6:15 мин, ~3:55 мин/км), разпускане",
			laps: []strava.Lap{
				{MovingTime: 600, Distance: 2000},  // warmup
				{MovingTime: 370, Distance: 1600},  // work 1 (6:10, pace 3:51)
				{MovingTime: 120, Distance: 400},   // recovery 1
				{MovingTime: 375, Distance: 1600},  // work 2 (6:15, pace 3:54)
				{MovingTime: 120, Distance: 400},   // recovery 2
				{MovingTime: 372, Distance: 1600},  // work 3 (6:12, pace 3:52)
				{MovingTime: 120, Distance: 400},   // recovery 3
				{MovingTime: 374, Distance: 1600},  // work 4 (6:14, pace 3:53)
				{MovingTime: 120, Distance: 400},   // recovery 4
				{MovingTime: 371, Distance: 1600},  // work 5 (6:11, pace 3:52)
				{MovingTime: 120, Distance: 400},   // recovery 5
				{MovingTime: 600, Distance: 2000},  // cooldown
			},
			expected: "загрявка, 5 х 1.6 км / 400 м (6:12 мин, 3:52 мин/км), разпускане",
		},
		{
			name: "Insufficient laps",
			desc: "загрявка, 6 x 1000 / 600 (3:45 - 3:50 мин), разпускане",
			laps: []strava.Lap{
				{MovingTime: 600, Distance: 2000},
				{MovingTime: 225, Distance: 1000},
				{MovingTime: 90, Distance: 600},
			},
			expected: "загрявка, 6 x 1000 / 600 (3:45 - 3:50 мин), разпускане", // unchanged
		},
		{
			name: "No interval pattern",
			desc: "леко бягане (70 мин, 5:10 - 5:25 мин/км)",
			laps: []strava.Lap{
				{MovingTime: 4200, Distance: 14000},
			},
			expected: "леко бягане (70 мин, 5:10 - 5:25 мин/км)", // unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UpdateIntervalWorkoutDescription(tt.desc, tt.laps)
			if result != tt.expected {
				t.Errorf("UpdateIntervalWorkoutDescription() = %v, want %v", result, tt.expected)
			}
		})
	}
}