// Package backend_test provides unit tests for milestone functionality
// Tests: adding milestones, date parsing, validation, multiple milestones
package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestAddMilestone(t *testing.T) {
	testDBPath := "test_milestone.db"
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
		vbolt.TxCommit(tx)
	})

	// Test valid milestone requests
	validRequests := []AddMilestoneRequest{
		{
			PersonId:      testPerson.Id,
			Description:   "First words",
			Category:      "development",
			InputType:     "date",
			MilestoneDate: stringPtr("2021-06-15"),
		},
		{
			PersonId:    testPerson.Id,
			Description: "Started walking",
			Category:    "development",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(2),
		},
		{
			PersonId:    testPerson.Id,
			Description: "Today's achievement",
			Category:    "achievement",
			InputType:   "today",
		},
		{
			PersonId:    testPerson.Id,
			Description: "First birthday party",
			Category:    "first",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(0),
		},
		{
			PersonId:      testPerson.Id,
			Description:   "Doctor checkup went well",
			Category:      "health",
			InputType:     "date",
			MilestoneDate: stringPtr("2023-01-15"),
		},
	}

	// Test adding valid milestones
	var addedMilestones []Milestone
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, req := range validRequests {
			milestone, err := AddMilestoneTx(tx, req, testUser.FamilyId)
			if err != nil {
				t.Fatalf("Failed to add valid milestone: %v", err)
			}
			addedMilestones = append(addedMilestones, milestone)

			// Verify basic fields
			if milestone.PersonId != req.PersonId {
				t.Errorf("Expected PersonId %d, got %d", req.PersonId, milestone.PersonId)
			}
			if milestone.FamilyId != testUser.FamilyId {
				t.Errorf("Expected FamilyId %d, got %d", testUser.FamilyId, milestone.FamilyId)
			}
			if milestone.Description != req.Description {
				t.Errorf("Expected Description %s, got %s", req.Description, milestone.Description)
			}
			if milestone.Category != req.Category {
				t.Errorf("Expected Category %s, got %s", req.Category, milestone.Category)
			}
			if milestone.Id <= 0 {
				t.Errorf("Expected positive ID, got %d", milestone.Id)
			}
			if milestone.CreatedAt.IsZero() {
				t.Error("Expected CreatedAt to be set")
			}
		}
		vbolt.TxCommit(tx)
	})

	// Test invalid milestone requests - database level checks
	databaseInvalidRequests := []struct {
		request     AddMilestoneRequest
		description string
	}{
		{
			request: AddMilestoneRequest{
				PersonId:    99999, // Non-existent person
				Description: "Test milestone",
				Category:    "development",
				InputType:   "today",
			},
			description: "non-existent person",
		},
	}

	// Test requests that should fail at database level
	for _, test := range databaseInvalidRequests {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			_, err := AddMilestoneTx(tx, test.request, testUser.FamilyId)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", test.description)
			}
			// Don't commit invalid transactions
		})
	}

	// Verify data retrieval
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrievedMilestones := GetPersonMilestonesTx(tx, testPerson.Id)
		if len(retrievedMilestones) != len(addedMilestones) {
			t.Errorf("Expected %d milestone records, got %d", len(addedMilestones), len(retrievedMilestones))
		}

		// Verify each record can be retrieved by ID
		for _, original := range addedMilestones {
			retrieved := GetMilestoneById(tx, original.Id)
			if retrieved.Id == 0 {
				t.Errorf("Milestone with ID %d not found", original.Id)
			}
			if retrieved.PersonId != original.PersonId {
				t.Errorf("PersonId mismatch: expected %d, got %d", original.PersonId, retrieved.PersonId)
			}
			if retrieved.Description != original.Description {
				t.Errorf("Description mismatch: expected %s, got %s", original.Description, retrieved.Description)
			}
		}
	})
}

func TestParseMilestoneDate(t *testing.T) {
	// Test person birthday
	birthday := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		request     AddMilestoneRequest
		expected    time.Time
		shouldError bool
	}{
		{
			name: "valid date input",
			request: AddMilestoneRequest{
				InputType:     "date",
				MilestoneDate: stringPtr("2023-06-15"),
			},
			expected:    time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			shouldError: false,
		},
		{
			name: "valid age input - exactly 3 years",
			request: AddMilestoneRequest{
				InputType: "age",
				AgeYears:  intPtr(3),
				AgeMonths: intPtr(0),
			},
			expected:    time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			shouldError: false,
		},
		{
			name: "valid age input - 2 years 6 months",
			request: AddMilestoneRequest{
				InputType: "age",
				AgeYears:  intPtr(2),
				AgeMonths: intPtr(6),
			},
			expected:    time.Date(2022, 12, 15, 0, 0, 0, 0, time.UTC),
			shouldError: false,
		},
		{
			name: "today input type",
			request: AddMilestoneRequest{
				InputType: "today",
			},
			// For today, we just check it doesn't error, exact time will vary
			shouldError: false,
		},
		{
			name: "invalid date format",
			request: AddMilestoneRequest{
				InputType:     "date",
				MilestoneDate: stringPtr("invalid-date"),
			},
			shouldError: true,
		},
		{
			name: "missing date",
			request: AddMilestoneRequest{
				InputType: "date",
			},
			shouldError: true,
		},
		{
			name: "negative age years",
			request: AddMilestoneRequest{
				InputType: "age",
				AgeYears:  intPtr(-1),
				AgeMonths: intPtr(0),
			},
			shouldError: true,
		},
		{
			name: "invalid age months",
			request: AddMilestoneRequest{
				InputType: "age",
				AgeYears:  intPtr(2),
				AgeMonths: intPtr(15), // > 11
			},
			shouldError: true,
		},
		{
			name: "invalid input type",
			request: AddMilestoneRequest{
				InputType: "invalid",
			},
			shouldError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parseMilestoneDate(test.request, birthday)

			if test.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", test.name, err)
				}
				if test.request.InputType != "today" && !result.Equal(test.expected) {
					t.Errorf("Expected date %v, got %v", test.expected, result)
				}
				if test.request.InputType == "today" && result.IsZero() {
					t.Error("Expected today to return non-zero time")
				}
			}
		})
	}
}

func TestMilestoneValidation(t *testing.T) {
	tests := []struct {
		name        string
		request     AddMilestoneRequest
		shouldError bool
	}{
		{
			name: "valid development milestone with date",
			request: AddMilestoneRequest{
				PersonId:      1,
				Description:   "First words",
				Category:      "development",
				InputType:     "date",
				MilestoneDate: stringPtr("2023-06-15"),
			},
			shouldError: false,
		},
		{
			name: "valid behavior milestone with age",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "Started sharing toys",
				Category:    "behavior",
				InputType:   "age",
				AgeYears:    intPtr(2),
				AgeMonths:   intPtr(6),
			},
			shouldError: false,
		},
		{
			name: "valid health milestone with today",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "Doctor checkup",
				Category:    "health",
				InputType:   "today",
			},
			shouldError: false,
		},
		{
			name: "valid achievement milestone",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "Won a prize",
				Category:    "achievement",
				InputType:   "today",
			},
			shouldError: false,
		},
		{
			name: "valid first milestone",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "First day of school",
				Category:    "first",
				InputType:   "today",
			},
			shouldError: false,
		},
		{
			name: "valid other milestone",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "Something special",
				Category:    "other",
				InputType:   "today",
			},
			shouldError: false,
		},
		{
			name: "invalid person ID",
			request: AddMilestoneRequest{
				PersonId:    0,
				Description: "Test milestone",
				Category:    "development",
				InputType:   "today",
			},
			shouldError: true,
		},
		{
			name: "empty description",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "",
				Category:    "development",
				InputType:   "today",
			},
			shouldError: true,
		},
		{
			name: "whitespace only description",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "   ",
				Category:    "development",
				InputType:   "today",
			},
			shouldError: true,
		},
		{
			name: "invalid category",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "Test milestone",
				Category:    "invalid",
				InputType:   "today",
			},
			shouldError: true,
		},
		{
			name: "invalid input type",
			request: AddMilestoneRequest{
				PersonId:    1,
				Description: "Test milestone",
				Category:    "development",
				InputType:   "invalid",
			},
			shouldError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAddMilestoneRequest(test.request)
			if test.shouldError && err == nil {
				t.Errorf("Expected validation error for %s, but got none", test.name)
			}
			if !test.shouldError && err != nil {
				t.Errorf("Unexpected validation error for %s: %v", test.name, err)
			}
		})
	}
}

func TestMultipleMilestones(t *testing.T) {
	testDBPath := "test_multiple_milestones.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var testPerson Person

	// Setup
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		personReq := AddPersonRequest{
			Name:       "Test Child",
			PersonType: 1,
			Gender:     0,
			Birthdate:  "2020-01-01",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create test person: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	// Add multiple milestones over time
	milestones := []AddMilestoneRequest{
		{
			PersonId:      testPerson.Id,
			Description:   "First smile",
			Category:      "development",
			InputType:     "date",
			MilestoneDate: stringPtr("2020-03-01"),
		},
		{
			PersonId:    testPerson.Id,
			Description: "Started crawling",
			Category:    "development",
			InputType:   "age",
			AgeYears:    intPtr(0),
			AgeMonths:   intPtr(8),
		},
		{
			PersonId:    testPerson.Id,
			Description: "First birthday",
			Category:    "first",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(0),
		},
		{
			PersonId:    testPerson.Id,
			Description: "Started being more social",
			Category:    "behavior",
			InputType:   "age",
			AgeYears:    intPtr(2),
			AgeMonths:   intPtr(3),
		},
	}

	var addedMilestones []Milestone
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, req := range milestones {
			milestone, err := AddMilestoneTx(tx, req, testUser.FamilyId)
			if err != nil {
				t.Fatalf("Failed to add milestone: %v", err)
			}
			addedMilestones = append(addedMilestones, milestone)
		}
		vbolt.TxCommit(tx)
	})

	// Verify all milestones are retrievable
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrievedMilestones := GetPersonMilestonesTx(tx, testPerson.Id)
		if len(retrievedMilestones) != len(milestones) {
			t.Errorf("Expected %d milestones, got %d", len(milestones), len(retrievedMilestones))
		}

		// Count by category
		categoryCount := make(map[string]int)
		for _, milestone := range retrievedMilestones {
			categoryCount[milestone.Category]++
		}

		expectedCounts := map[string]int{
			"development": 2,
			"first":       1,
			"behavior":    1,
		}

		for category, expectedCount := range expectedCounts {
			if actualCount := categoryCount[category]; actualCount != expectedCount {
				t.Errorf("Expected %d %s milestones, got %d", expectedCount, category, actualCount)
			}
		}
	})
}
