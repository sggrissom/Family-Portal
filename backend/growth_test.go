// Package backend_test provides unit tests for growth data functionality
// Tests: adding measurements, date parsing, validation, multiple measurements
package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestAddGrowthData(t *testing.T) {
	testDBPath := "test_growth.db"
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
			Name:      "Test Child",
			PersonType: 1, // Child
			Gender:    0, // Male
			Birthdate: "2020-06-15",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create test person: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	// Test valid growth data requests
	validRequests := []AddGrowthDataRequest{
		{
			PersonId:        testPerson.Id,
			MeasurementType: "height",
			Value:           90.5,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2023-06-15"),
		},
		{
			PersonId:        testPerson.Id,
			MeasurementType: "weight",
			Value:           25.3,
			Unit:            "kg",
			InputType:       "date",
			MeasurementDate: stringPtr("2023-06-15"),
		},
		{
			PersonId:    testPerson.Id,
			MeasurementType: "height",
			Value:       36.5,
			Unit:        "in",
			InputType:   "age",
			AgeYears:    intPtr(3),
			AgeMonths:   intPtr(0),
		},
		{
			PersonId:    testPerson.Id,
			MeasurementType: "weight",
			Value:       55.7,
			Unit:        "lbs",
			InputType:   "age",
			AgeYears:    intPtr(2),
			AgeMonths:   intPtr(6),
		},
	}

	// Test adding valid growth data
	var addedGrowthData []GrowthData
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, req := range validRequests {
			growthData, err := AddGrowthDataTx(tx, req, testUser.FamilyId)
			if err != nil {
				t.Fatalf("Failed to add valid growth data: %v", err)
			}
			addedGrowthData = append(addedGrowthData, growthData)

			// Verify basic fields
			if growthData.PersonId != req.PersonId {
				t.Errorf("Expected PersonId %d, got %d", req.PersonId, growthData.PersonId)
			}
			if growthData.FamilyId != testUser.FamilyId {
				t.Errorf("Expected FamilyId %d, got %d", testUser.FamilyId, growthData.FamilyId)
			}
			if growthData.Value != req.Value {
				t.Errorf("Expected Value %f, got %f", req.Value, growthData.Value)
			}
			if growthData.Unit != req.Unit {
				t.Errorf("Expected Unit %s, got %s", req.Unit, growthData.Unit)
			}
		}
		vbolt.TxCommit(tx)
	})

	// Test invalid growth data requests - database level checks
	databaseInvalidRequests := []struct {
		request AddGrowthDataRequest
		description string
	}{
		{
			request: AddGrowthDataRequest{
				PersonId:        99999, // Non-existent person
				MeasurementType: "height",
				Value:           90.5,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			description: "non-existent person",
		},
		{
			request: AddGrowthDataRequest{
				PersonId:        testPerson.Id,
				MeasurementType: "invalid", // Invalid measurement type
				Value:           90.5,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			description: "invalid measurement type",
		},
	}

	// Test requests that should fail at database level
	for _, test := range databaseInvalidRequests {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			_, err := AddGrowthDataTx(tx, test.request, testUser.FamilyId)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", test.description)
			}
			// Don't commit invalid transactions
		})
	}

	// Test validation-level failures (these should be caught by validateAddGrowthDataRequest)
	validationInvalidRequests := []AddGrowthDataRequest{
		{
			PersonId:        testPerson.Id,
			MeasurementType: "height",
			Value:           -10.5, // Negative value
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2023-06-15"),
		},
		{
			PersonId:        testPerson.Id,
			MeasurementType: "height",
			Value:           90.5,
			Unit:            "invalid", // Invalid unit
			InputType:       "date",
			MeasurementDate: stringPtr("2023-06-15"),
		},
	}

	// Test validation failures
	for _, req := range validationInvalidRequests {
		err := validateAddGrowthDataRequest(req)
		if err == nil {
			t.Errorf("Expected validation error for request with value %f and unit %s", req.Value, req.Unit)
		}
	}

	// Verify data retrieval
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrievedGrowthData := GetPersonGrowthDataTx(tx, testPerson.Id)
		if len(retrievedGrowthData) != len(addedGrowthData) {
			t.Errorf("Expected %d growth data records, got %d", len(addedGrowthData), len(retrievedGrowthData))
		}

		// Verify each record can be retrieved by ID
		for _, original := range addedGrowthData {
			retrieved := GetGrowthDataById(tx, original.Id)
			if retrieved.Id == 0 {
				t.Errorf("Growth data with ID %d not found", original.Id)
			}
			if retrieved.PersonId != original.PersonId {
				t.Errorf("PersonId mismatch: expected %d, got %d", original.PersonId, retrieved.PersonId)
			}
		}
	})
}

func TestParseMeasurementDate(t *testing.T) {
	testDBPath := "test_date_parsing.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Test person birthday
	birthday := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		request     AddGrowthDataRequest
		expected    time.Time
		shouldError bool
	}{
		{
			name: "valid date input",
			request: AddGrowthDataRequest{
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			expected: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			shouldError: false,
		},
		{
			name: "valid age input - exactly 3 years",
			request: AddGrowthDataRequest{
				InputType: "age",
				AgeYears:  intPtr(3),
				AgeMonths: intPtr(0),
			},
			expected: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			shouldError: false,
		},
		{
			name: "valid age input - 2 years 6 months",
			request: AddGrowthDataRequest{
				InputType: "age",
				AgeYears:  intPtr(2),
				AgeMonths: intPtr(6),
			},
			expected: time.Date(2022, 12, 15, 0, 0, 0, 0, time.UTC),
			shouldError: false,
		},
		{
			name: "invalid date format",
			request: AddGrowthDataRequest{
				InputType:       "date",
				MeasurementDate: stringPtr("invalid-date"),
			},
			shouldError: true,
		},
		{
			name: "missing date",
			request: AddGrowthDataRequest{
				InputType: "date",
			},
			shouldError: true,
		},
		{
			name: "negative age years",
			request: AddGrowthDataRequest{
				InputType: "age",
				AgeYears:  intPtr(-1),
				AgeMonths: intPtr(0),
			},
			shouldError: true,
		},
		{
			name: "invalid age months",
			request: AddGrowthDataRequest{
				InputType: "age",
				AgeYears:  intPtr(2),
				AgeMonths: intPtr(15), // > 11
			},
			shouldError: true,
		},
		{
			name: "invalid input type",
			request: AddGrowthDataRequest{
				InputType: "invalid",
			},
			shouldError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parseMeasurementDate(test.request, birthday)

			if test.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", test.name, err)
				}
				if !result.Equal(test.expected) {
					t.Errorf("Expected date %v, got %v", test.expected, result)
				}
			}
		})
	}
}

func TestGrowthDataValidation(t *testing.T) {
	tests := []struct {
		name        string
		request     AddGrowthDataRequest
		shouldError bool
	}{
		{
			name: "valid height in cm",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           90.5,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: false,
		},
		{
			name: "valid height in inches",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           36.5,
				Unit:            "in",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: false,
		},
		{
			name: "valid weight in kg",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "weight",
				Value:           25.3,
				Unit:            "kg",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: false,
		},
		{
			name: "valid weight in lbs",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "weight",
				Value:           55.7,
				Unit:            "lbs",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: false,
		},
		{
			name: "invalid person ID",
			request: AddGrowthDataRequest{
				PersonId:        0,
				MeasurementType: "height",
				Value:           90.5,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "invalid measurement type",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "invalid",
				Value:           90.5,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "zero value",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           0,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "negative value",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           -10.5,
				Unit:            "cm",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "missing unit",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           90.5,
				Unit:            "",
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "invalid height unit",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           90.5,
				Unit:            "kg", // Wrong unit for height
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "invalid weight unit",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "weight",
				Value:           25.3,
				Unit:            "cm", // Wrong unit for weight
				InputType:       "date",
				MeasurementDate: stringPtr("2023-06-15"),
			},
			shouldError: true,
		},
		{
			name: "invalid input type",
			request: AddGrowthDataRequest{
				PersonId:        1,
				MeasurementType: "height",
				Value:           90.5,
				Unit:            "cm",
				InputType:       "invalid",
			},
			shouldError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAddGrowthDataRequest(test.request)
			if test.shouldError && err == nil {
				t.Errorf("Expected validation error for %s, but got none", test.name)
			}
			if !test.shouldError && err != nil {
				t.Errorf("Unexpected validation error for %s: %v", test.name, err)
			}
		})
	}
}

func TestMultipleGrowthMeasurements(t *testing.T) {
	testDBPath := "test_multiple_growth.db"
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
			Name:      "Test Child",
			PersonType: 1,
			Gender:    0,
			Birthdate: "2020-01-01",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create test person: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	// Add multiple measurements over time
	measurements := []AddGrowthDataRequest{
		{
			PersonId:        testPerson.Id,
			MeasurementType: "height",
			Value:           50.0,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2020-06-01"),
		},
		{
			PersonId:        testPerson.Id,
			MeasurementType: "height",
			Value:           55.0,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2020-12-01"),
		},
		{
			PersonId:        testPerson.Id,
			MeasurementType: "weight",
			Value:           8.5,
			Unit:            "kg",
			InputType:       "date",
			MeasurementDate: stringPtr("2020-06-01"),
		},
		{
			PersonId:        testPerson.Id,
			MeasurementType: "weight",
			Value:           10.2,
			Unit:            "kg",
			InputType:       "date",
			MeasurementDate: stringPtr("2020-12-01"),
		},
	}

	var addedMeasurements []GrowthData
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, req := range measurements {
			growthData, err := AddGrowthDataTx(tx, req, testUser.FamilyId)
			if err != nil {
				t.Fatalf("Failed to add measurement: %v", err)
			}
			addedMeasurements = append(addedMeasurements, growthData)
		}
		vbolt.TxCommit(tx)
	})

	// Verify all measurements are retrievable
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrievedData := GetPersonGrowthDataTx(tx, testPerson.Id)
		if len(retrievedData) != len(measurements) {
			t.Errorf("Expected %d measurements, got %d", len(measurements), len(retrievedData))
		}

		// Count by type
		heightCount := 0
		weightCount := 0
		for _, data := range retrievedData {
			if data.MeasurementType == Height {
				heightCount++
			} else if data.MeasurementType == Weight {
				weightCount++
			}
		}

		if heightCount != 2 {
			t.Errorf("Expected 2 height measurements, got %d", heightCount)
		}
		if weightCount != 2 {
			t.Errorf("Expected 2 weight measurements, got %d", weightCount)
		}
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}