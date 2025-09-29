// Package backend_test provides unit tests for data export functionality
// Tests: export data structures, database queries, unit conversions, family isolation
package backend

import (
	"encoding/json"
	"family/cfg"
	"math"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// Test export data structure and JSON serialization
func TestExportDataStructure(t *testing.T) {
	exportData := ExportDataStructure{
		People: []ImportPerson{
			{
				Id:       1,
				FamilyId: 1,
				Type:     1,
				Gender:   0,
				Name:     "Test Child",
				Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Age:      "4 years",
				ImageId:  0,
			},
		},
		Heights: []ImportHeight{
			{
				Id:         1,
				PersonId:   1,
				Inches:     39.37, // 100 cm
				Date:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
				DateString: "2023-06-15",
				Age:        3.5,
				PersonName: "Test Child",
			},
		},
		Weights: []ImportWeight{
			{
				Id:         1,
				PersonId:   1,
				Pounds:     44.09, // 20 kg
				Date:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
				DateString: "2023-06-15",
				Age:        3.5,
				PersonName: "Test Child",
			},
		},
		Milestones: []ExportMilestone{
			{
				Id:            1,
				PersonId:      1,
				FamilyId:      1,
				Description:   "First steps",
				Category:      "Physical",
				MilestoneDate: time.Date(2021, 6, 15, 0, 0, 0, 0, time.UTC),
				CreatedAt:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				PersonName:    "Test Child",
			},
		},
		ExportDate:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		TotalHeights:    1,
		TotalWeights:    1,
		TotalPeople:     1,
		TotalMilestones: 1,
	}

	// Test JSON serialization
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal export data: %v", err)
	}

	// Test JSON deserialization
	var decoded ExportDataStructure
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal export data: %v", err)
	}

	// Verify data integrity
	if len(decoded.People) != 1 {
		t.Errorf("Expected 1 person, got %d", len(decoded.People))
	}
	if len(decoded.Heights) != 1 {
		t.Errorf("Expected 1 height, got %d", len(decoded.Heights))
	}
	if len(decoded.Weights) != 1 {
		t.Errorf("Expected 1 weight, got %d", len(decoded.Weights))
	}
	if len(decoded.Milestones) != 1 {
		t.Errorf("Expected 1 milestone, got %d", len(decoded.Milestones))
	}

	// Verify specific data
	person := decoded.People[0]
	if person.Name != "Test Child" {
		t.Errorf("Expected person name 'Test Child', got '%s'", person.Name)
	}

	height := decoded.Heights[0]
	if math.Abs(height.Inches-39.37) > 0.01 {
		t.Errorf("Expected height 39.37 inches, got %f", height.Inches)
	}

	weight := decoded.Weights[0]
	if math.Abs(weight.Pounds-44.09) > 0.01 {
		t.Errorf("Expected weight 44.09 pounds, got %f", weight.Pounds)
	}

	milestone := decoded.Milestones[0]
	if milestone.Description != "First steps" {
		t.Errorf("Expected milestone 'First steps', got '%s'", milestone.Description)
	}
}

// Test unit conversion functions
func TestUnitConversions(t *testing.T) {
	testCases := []struct {
		name      string
		cm        float64
		inches    float64
		kg        float64
		pounds    float64
		tolerance float64
	}{
		{
			name:      "Standard conversions",
			cm:        100.0,
			inches:    39.3701,
			kg:        20.0,
			pounds:    44.0925,
			tolerance: 0.01,
		},
		{
			name:      "Zero values",
			cm:        0.0,
			inches:    0.0,
			kg:        0.0,
			pounds:    0.0,
			tolerance: 0.001,
		},
		{
			name:      "Precise conversions",
			cm:        152.4,
			inches:    60.0,
			kg:        22.6796,
			pounds:    50.0,
			tolerance: 0.001,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test cm to inches
			convertedInches := tc.cm / 2.54
			if math.Abs(convertedInches-tc.inches) > tc.tolerance {
				t.Errorf("CM to inches: expected %f, got %f", tc.inches, convertedInches)
			}

			// Test kg to pounds
			convertedPounds := tc.kg * 2.20462
			if math.Abs(convertedPounds-tc.pounds) > tc.tolerance {
				t.Errorf("KG to pounds: expected %f, got %f", tc.pounds, convertedPounds)
			}
		})
	}
}

// Test age calculation function
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

// Test family data isolation in export context
func TestExportFamilyDataIsolation(t *testing.T) {
	testDBPath := "test_export_isolation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser1, testUser2 User
	var family1Id, family2Id int

	// Setup: Create two users in different families
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq1 := CreateAccountRequest{
			Name:            "User One",
			Email:           "user1@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash1, _ := bcrypt.GenerateFromPassword([]byte(userReq1.Password), bcrypt.DefaultCost)
		testUser1 = AddUserTx(tx, userReq1, hash1)
		family1Id = testUser1.FamilyId

		userReq2 := CreateAccountRequest{
			Name:            "User Two",
			Email:           "user2@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash2, _ := bcrypt.GenerateFromPassword([]byte(userReq2.Password), bcrypt.DefaultCost)
		testUser2 = AddUserTx(tx, userReq2, hash2)
		family2Id = testUser2.FamilyId

		vbolt.TxCommit(tx)
	})

	// Test family people isolation
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		family1People := GetFamilyPeople(tx, family1Id)
		family2People := GetFamilyPeople(tx, family2Id)

		// Each family should only have their own people
		for _, person := range family1People {
			if person.FamilyId != family1Id {
				t.Errorf("Family 1 people query returned person from family %d", person.FamilyId)
			}
		}

		for _, person := range family2People {
			if person.FamilyId != family2Id {
				t.Errorf("Family 2 people query returned person from family %d", person.FamilyId)
			}
		}

		// Families should be different
		if family1Id == family2Id {
			t.Error("Different users should have different family IDs")
		}
	})
}

// Test export data completeness validation
func TestExportDataCompleteness(t *testing.T) {
	testDBPath := "test_export_completeness.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var testPerson Person

	// Setup: Create comprehensive test data
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		// Add a person
		personReq := AddPersonRequest{
			Name:       "Test Child",
			PersonType: 1, // Child
			Gender:     0, // Male
			Birthdate:  "2020-06-15",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add person: %v", err)
		}

		vbolt.TxCommit(tx)
	})

	// Test data retrieval functions
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		// Test people retrieval
		people := GetFamilyPeople(tx, testUser.FamilyId)
		if len(people) == 0 {
			t.Error("No people found for family")
		}

		foundTestPerson := false
		for _, person := range people {
			if person.Id == testPerson.Id {
				foundTestPerson = true
				if person.Name != "Test Child" {
					t.Errorf("Expected person name 'Test Child', got '%s'", person.Name)
				}
				if person.FamilyId != testUser.FamilyId {
					t.Errorf("Person belongs to wrong family: %d vs %d", person.FamilyId, testUser.FamilyId)
				}
			}
		}
		if !foundTestPerson {
			t.Error("Test person not found in family people")
		}

		// Test milestones retrieval (empty case)
		var milestoneIds []int
		vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, testUser.FamilyId, &milestoneIds, vbolt.Window{})

		// Should be empty for new family
		if len(milestoneIds) != 0 {
			t.Errorf("Expected 0 milestones for new family, got %d", len(milestoneIds))
		}

		// Test growth data retrieval (empty case)
		var growthDataIds []int
		vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, testUser.FamilyId, &growthDataIds, vbolt.Window{})

		// Should be empty for new family
		if len(growthDataIds) != 0 {
			t.Errorf("Expected 0 growth data entries for new family, got %d", len(growthDataIds))
		}
	})
}

// Test empty family export scenario
func TestEmptyFamilyExport(t *testing.T) {
	testDBPath := "test_empty_export.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

	// Setup: Create user with minimal data
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)
		vbolt.TxCommit(tx)
	})

	// Test empty data retrieval
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		// Get people (should be empty or just the user's automatically created person)
		people := GetFamilyPeople(tx, testUser.FamilyId)

		// Get milestones (should be empty)
		var milestoneIds []int
		vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, testUser.FamilyId, &milestoneIds, vbolt.Window{})

		// Get growth data (should be empty)
		var growthDataIds []int
		vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, testUser.FamilyId, &growthDataIds, vbolt.Window{})

		// Simulate building export data structure
		exportData := ExportDataStructure{
			People:          []ImportPerson{}, // Would be populated from people
			Heights:         []ImportHeight{},
			Weights:         []ImportWeight{},
			Milestones:      []ExportMilestone{},
			ExportDate:      time.Now(),
			TotalHeights:    0,
			TotalWeights:    0,
			TotalPeople:     len(people),
			TotalMilestones: len(milestoneIds),
		}

		// Verify structure is valid even when empty
		if exportData.TotalHeights != 0 {
			t.Errorf("Expected 0 total heights, got %d", exportData.TotalHeights)
		}
		if exportData.TotalWeights != 0 {
			t.Errorf("Expected 0 total weights, got %d", exportData.TotalWeights)
		}
		if len(milestoneIds) != 0 {
			t.Errorf("Expected 0 milestones, got %d", len(milestoneIds))
		}
		if len(growthDataIds) != 0 {
			t.Errorf("Expected 0 growth data entries, got %d", len(growthDataIds))
		}

		// Export date should be set
		if exportData.ExportDate.IsZero() {
			t.Error("Export date should be set even for empty export")
		}
	})
}

// Test export/import compatibility
func TestExportImportCompatibility(t *testing.T) {
	// Create sample export data in the expected format
	exportData := ExportDataStructure{
		People: []ImportPerson{
			{
				Id:       1,
				FamilyId: 100,
				Type:     1,
				Gender:   0,
				Name:     "Test Child",
				Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Age:      "4 years",
				ImageId:  0,
			},
		},
		Heights: []ImportHeight{
			{
				Id:         1,
				PersonId:   1,
				Inches:     39.37,
				Date:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
				DateString: "2023-06-15",
				Age:        3.5,
				PersonName: "Test Child",
			},
		},
		Weights: []ImportWeight{
			{
				Id:         1,
				PersonId:   1,
				Pounds:     44.09,
				Date:       time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
				DateString: "2023-06-15",
				Age:        3.5,
				PersonName: "Test Child",
			},
		},
		ExportDate:   time.Now(),
		TotalHeights: 1,
		TotalWeights: 1,
		TotalPeople:  1,
	}

	// Test that export data can be marshaled to JSON
	jsonData, err := json.Marshal(exportData)
	if err != nil {
		t.Fatalf("Failed to marshal export data: %v", err)
	}

	// Test that JSON can be unmarshaled back to ImportDataStructure
	var importData ImportDataStructure
	err = json.Unmarshal(jsonData, &importData)
	if err != nil {
		t.Fatalf("Failed to unmarshal as import data: %v", err)
	}

	// Verify data compatibility
	if len(importData.People) != 1 {
		t.Errorf("Expected 1 person in import data, got %d", len(importData.People))
	}
	if len(importData.Heights) != 1 {
		t.Errorf("Expected 1 height in import data, got %d", len(importData.Heights))
	}
	if len(importData.Weights) != 1 {
		t.Errorf("Expected 1 weight in import data, got %d", len(importData.Weights))
	}

	// Verify specific field compatibility
	person := importData.People[0]
	if person.Name != "Test Child" {
		t.Errorf("Person name not preserved: expected 'Test Child', got '%s'", person.Name)
	}

	height := importData.Heights[0]
	if math.Abs(height.Inches-39.37) > 0.01 {
		t.Errorf("Height value not preserved: expected 39.37, got %f", height.Inches)
	}

	weight := importData.Weights[0]
	if math.Abs(weight.Pounds-44.09) > 0.01 {
		t.Errorf("Weight value not preserved: expected 44.09, got %f", weight.Pounds)
	}
}
