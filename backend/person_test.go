// Package backend_test provides unit tests for person-related functionality
// Tests: age calculation functions and edge cases
package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestCalculateAge(t *testing.T) {
	referenceDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		birthdate time.Time
		expected  string
	}{
		{
			name:      "Born exactly 1 year ago",
			birthdate: time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			expected:  "1 year",
		},
		{
			name:      "Born 10 years ago",
			birthdate: time.Date(2014, 1, 15, 0, 0, 0, 0, time.UTC),
			expected:  "10 years",
		},
		{
			name:      "Born 25 years ago",
			birthdate: time.Date(1999, 1, 15, 0, 0, 0, 0, time.UTC),
			expected:  "25 years",
		},
		{
			name:      "Birthday is tomorrow (hasn't happened yet)",
			birthdate: time.Date(2023, 1, 16, 0, 0, 0, 0, time.UTC),
			expected:  "11 months", // 11 months old
		},
		{
			name:      "Birthday was yesterday",
			birthdate: time.Date(2023, 1, 14, 0, 0, 0, 0, time.UTC),
			expected:  "1 year",
		},
		{
			name:      "Birthday next month (hasn't happened yet)",
			birthdate: time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
			expected:  "11 months",
		},
		{
			name:      "Birthday last month",
			birthdate: time.Date(2022, 12, 15, 0, 0, 0, 0, time.UTC),
			expected:  "1 year",
		},
		{
			name:      "Born in leap year (Feb 29, 2020)",
			birthdate: time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC),
			expected:  "3 years", // Born Feb 29, 2020, should be 3 years old on Jan 15, 2024
		},
		{
			name:      "Born same day different year",
			birthdate: time.Date(2000, 1, 15, 0, 0, 0, 0, time.UTC),
			expected:  "24 years",
		},
		{
			name:      "Very young (born 6 months ago)",
			birthdate: time.Date(2023, 7, 15, 0, 0, 0, 0, time.UTC),
			expected:  "6 months",
		},
		{
			name:      "Born 1 month ago",
			birthdate: time.Date(2023, 12, 15, 0, 0, 0, 0, time.UTC),
			expected:  "1 month",
		},
		{
			name:      "Born very recently (2 weeks ago)",
			birthdate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:  "< 1 month",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAgeAt(tt.birthdate, referenceDate)
			if result != tt.expected {
				t.Errorf("calculateAgeAt(%v, %v) = %q, expected %q", tt.birthdate, referenceDate, result, tt.expected)
			}
		})
	}
}

func TestCalculateAgeEdgeCases(t *testing.T) {
	// Test edge cases around leap years and specific dates
	tests := []struct {
		name          string
		birthdate     time.Time
		referenceDate time.Time
		expected      string
	}{
		{
			name:          "Born Feb 29, reference Feb 28 (non-leap year)",
			birthdate:     time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC),
			expected:      "2 years", // Birthday hasn't occurred yet in 2023
		},
		{
			name:          "Born Feb 29, reference Mar 1 (non-leap year)",
			birthdate:     time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
			expected:      "3 years", // Birthday has passed in 2023
		},
		{
			name:          "Same date different year",
			birthdate:     time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC),
			expected:      "3 years", // Exactly 3 years old
		},
		{
			name:          "Day before birthday",
			birthdate:     time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 9, 0, 0, 0, 0, time.UTC),
			expected:      "2 years", // Still 2, birthday is tomorrow
		},
		{
			name:          "Day after birthday",
			birthdate:     time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 11, 0, 0, 0, 0, time.UTC),
			expected:      "3 years", // Now 3, birthday was yesterday
		},
		{
			name:          "Baby born 3 months ago",
			birthdate:     time.Date(2023, 2, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC),
			expected:      "3 months",
		},
		{
			name:          "Baby born last week",
			birthdate:     time.Date(2023, 5, 3, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC),
			expected:      "< 1 month",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAgeAt(tt.birthdate, tt.referenceDate)
			if result != tt.expected {
				t.Errorf("calculateAgeAt(%v, %v) = %q, expected %q",
					tt.birthdate.Format("2006-01-02"),
					tt.referenceDate.Format("2006-01-02"),
					result, tt.expected)
			}
		})
	}
}

func TestCalculateAgeCurrentTime(t *testing.T) {
	now := time.Now()

	// Test someone born exactly 20 years ago
	twentyYearsAgo := now.AddDate(-20, 0, 0)
	age := calculateAge(twentyYearsAgo)
	expected := "20 years"

	if age != expected {
		t.Errorf("calculateAge for someone born exactly 20 years ago should be %q, got %q", expected, age)
	}

	// Test someone born 1 day ago (should show months or "< 1 month")
	oneDayAgo := now.AddDate(0, 0, -1)
	age = calculateAge(oneDayAgo)
	// This should be "< 1 month" since it's only been 1 day
	if age != "< 1 month" {
		t.Errorf("calculateAge for someone born 1 day ago should be \"< 1 month\", got %q", age)
	}

	// Test someone born 1 year and 1 day ago (should be 1 year)
	oneYearOneDayAgo := now.AddDate(-1, 0, -1)
	age = calculateAge(oneYearOneDayAgo)
	expected = "1 year"

	if age != expected {
		t.Errorf("calculateAge for someone born 1 year and 1 day ago should be %q, got %q", expected, age)
	}
}

func TestGetPersonWithMilestones(t *testing.T) {
	testDBPath := "test_person_milestones.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var testPerson Person

	// Setup: Create test user and person
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Create user
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		// Create person
		personReq := AddPersonRequest{
			Name:       "Test Child",
			PersonType: 1, // Child
			Gender:     0, // Male
			Birthdate:  "2020-06-15",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create test person: %v", err)
		}

		// Add some test milestones
		milestoneReq1 := AddMilestoneRequest{
			PersonId:    testPerson.Id,
			Description: "First words",
			Category:    "development",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(3),
		}
		_, err = AddMilestoneTx(tx, milestoneReq1, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add test milestone 1: %v", err)
		}

		milestoneReq2 := AddMilestoneRequest{
			PersonId:    testPerson.Id,
			Description: "Started walking",
			Category:    "development",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(6),
		}
		_, err = AddMilestoneTx(tx, milestoneReq2, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add test milestone 2: %v", err)
		}

		// Add some test growth data
		growthReq := AddGrowthDataRequest{
			PersonId:        testPerson.Id,
			MeasurementType: "height",
			Value:           85.0,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2021-06-15"),
		}
		_, err = AddGrowthDataTx(tx, growthReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add test growth data: %v", err)
		}

		vbolt.TxCommit(tx)
	})

	// Test GetPersonById with transaction - simulating what GetPerson procedure does
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		// Get the person
		person := GetPersonById(tx, testPerson.Id)
		if person.Id == 0 {
			t.Fatal("Failed to retrieve person")
		}

		// Get growth data
		growthData := GetPersonGrowthDataTx(tx, testPerson.Id)
		if len(growthData) == 0 {
			t.Error("Expected at least one growth data record")
		}

		// Get milestones
		milestones := GetPersonMilestonesTx(tx, testPerson.Id)
		if len(milestones) != 2 {
			t.Errorf("Expected 2 milestones, got %d", len(milestones))
		}

		// Verify milestone content
		for _, milestone := range milestones {
			if milestone.PersonId != testPerson.Id {
				t.Errorf("Expected milestone PersonId %d, got %d", testPerson.Id, milestone.PersonId)
			}
			if milestone.FamilyId != testUser.FamilyId {
				t.Errorf("Expected milestone FamilyId %d, got %d", testUser.FamilyId, milestone.FamilyId)
			}
			if milestone.Category != "development" {
				t.Errorf("Expected milestone category 'development', got %s", milestone.Category)
			}
			if milestone.Description == "" {
				t.Error("Expected milestone description to not be empty")
			}
			if milestone.CreatedAt.IsZero() {
				t.Error("Expected milestone CreatedAt to be set")
			}
		}

		// Verify that only milestones for this person are returned
		// (this is implicitly tested by the PersonId check above, but worth noting)
	})
}
