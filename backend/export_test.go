package backend

import (
	"math"
	"testing"
	"time"
)

func TestCalculateAgeAtDate(t *testing.T) {
	testCases := []struct {
		name        string
		birthday    time.Time
		targetDate  time.Time
		expectedAge float64
		tolerance   float64
	}{
		{
			name:        "Exactly 1 year",
			birthday:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			targetDate:  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedAge: 1.0,
			tolerance:   0.01,
		},
		{
			name:        "1 year 6 months",
			birthday:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			targetDate:  time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC),
			expectedAge: 1.5,
			tolerance:   0.01,
		},
		{
			name:        "6 months",
			birthday:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			targetDate:  time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC),
			expectedAge: 0.5,
			tolerance:   0.01,
		},
		{
			name:        "Same date",
			birthday:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			targetDate:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedAge: 0.0,
			tolerance:   0.01,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			age := calculateAgeAtDate(tc.birthday, tc.targetDate)
			if math.Abs(age-tc.expectedAge) > tc.tolerance {
				t.Errorf("Expected age %f, got %f", tc.expectedAge, age)
			}
		})
	}
}
