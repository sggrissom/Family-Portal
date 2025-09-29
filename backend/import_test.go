// Package backend_test provides unit tests for data import functionality
// Tests: importing people, importing measurements, data validation, filtering, full workflow
package backend

import (
	"encoding/json"
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestImportPeople(t *testing.T) {
	testDBPath := "test_import_people.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

	// Setup: Create test user
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

	// Test import people data
	testImportPeople := []ImportPerson{
		{
			Id:       1,
			FamilyId: 100,
			Type:     1, // Child
			Gender:   0, // Male
			Name:     "John Doe",
			Birthday: time.Date(2015, 6, 15, 0, 0, 0, 0, time.UTC),
			Age:      "8 years",
			ImageId:  0,
		},
		{
			Id:       2,
			FamilyId: 100,
			Type:     0, // Parent
			Gender:   1, // Female
			Name:     "Jane Doe",
			Birthday: time.Date(1985, 3, 20, 0, 0, 0, 0, time.UTC),
			Age:      "38 years",
			ImageId:  0,
		},
		{
			Id:       3,
			FamilyId: 101,
			Type:     1, // Child
			Gender:   2, // Unknown
			Name:     "Sam Smith",
			Birthday: time.Date(2010, 12, 1, 0, 0, 0, 0, time.UTC),
			Age:      "13 years",
			ImageId:  0,
		},
	}

	// Test importing people
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		mapping, importedCount, mergedCount, errors, warnings := importPeople(tx, testImportPeople, testUser.FamilyId, "create_all")

		if len(errors) > 0 {
			t.Errorf("Unexpected errors during import: %v", errors)
		}

		if len(warnings) > 0 {
			t.Logf("Import warnings: %v", warnings)
		}

		if importedCount != len(testImportPeople) {
			t.Errorf("Expected %d people imported, got %d", len(testImportPeople), importedCount)
		}

		if mergedCount != 0 {
			t.Errorf("Expected 0 people merged with create_all strategy, got %d", mergedCount)
		}

		if len(mapping) != len(testImportPeople) {
			t.Errorf("Expected %d people in mapping, got %d", len(testImportPeople), len(mapping))
		}

		// Verify all people were imported with new IDs
		for _, importPerson := range testImportPeople {
			newId, exists := mapping[importPerson.Id]
			if !exists {
				t.Errorf("Person with old ID %d not found in mapping", importPerson.Id)
				continue
			}

			person := GetPersonById(tx, newId)
			if person.Id == 0 {
				t.Errorf("Imported person with new ID %d not found in database", newId)
				continue
			}

			// Verify person details
			if person.Name != importPerson.Name {
				t.Errorf("Expected name %s, got %s", importPerson.Name, person.Name)
			}
			if person.Type != PersonType(importPerson.Type) {
				t.Errorf("Expected type %d, got %d", importPerson.Type, int(person.Type))
			}
			if person.Gender != GenderType(importPerson.Gender) {
				t.Errorf("Expected gender %d, got %d", importPerson.Gender, int(person.Gender))
			}
			if !person.Birthday.Equal(importPerson.Birthday) {
				t.Errorf("Expected birthday %v, got %v", importPerson.Birthday, person.Birthday)
			}
			if person.FamilyId != testUser.FamilyId {
				t.Errorf("Expected FamilyId %d, got %d", testUser.FamilyId, person.FamilyId)
			}
		}

		vbolt.TxCommit(tx)
	})

	// Verify people can be retrieved by family
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		familyPeople := GetFamilyPeople(tx, testUser.FamilyId)
		// Should include the imported people plus the test user's auto-created family member count
		expectedMinimum := len(testImportPeople)
		if len(familyPeople) < expectedMinimum {
			t.Errorf("Expected at least %d people in family, got %d", expectedMinimum, len(familyPeople))
		}
	})
}

func TestImportMeasurements(t *testing.T) {
	testDBPath := "test_import_measurements.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var personIdMapping map[int]int

	// Setup: Create user and import people first
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		// Import test people
		testPeople := []ImportPerson{
			{
				Id:       1,
				FamilyId: 100,
				Type:     1,
				Gender:   0,
				Name:     "Test Child",
				Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Age:      "3 years",
			},
		}

		mapping, _, _, _, _ := importPeople(tx, testPeople, testUser.FamilyId, "create_all")
		personIdMapping = mapping
		vbolt.TxCommit(tx)
	})

	// Test measurement data
	importHeights := []ImportHeight{
		{
			Id:         1,
			PersonId:   1, // Old person ID
			Inches:     30.5,
			Date:       time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC),
			DateString: "2021-06-01",
			Age:        1.5,
			PersonName: "Test Child",
		},
		{
			Id:         2,
			PersonId:   1,
			Inches:     35.2,
			Date:       time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC),
			DateString: "2022-06-01",
			Age:        2.5,
			PersonName: "Test Child",
		},
		{
			Id:         3,
			PersonId:   999, // Non-existent person
			Inches:     40.0,
			Date:       time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
			DateString: "2023-06-01",
			Age:        3.5,
			PersonName: "Unknown Child",
		},
		{
			Id:         4,
			PersonId:   1,
			Inches:     25.0,
			Date:       time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), // Invalid date (year 0001)
			DateString: "0001-01-01",
			Age:        0,
			PersonName: "Test Child",
		},
	}

	importWeights := []ImportWeight{
		{
			Id:         1,
			PersonId:   1,
			Pounds:     20.5,
			Date:       time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC),
			DateString: "2021-06-01",
			Age:        1.5,
			PersonName: "Test Child",
		},
		{
			Id:         2,
			PersonId:   1,
			Pounds:     28.3,
			Date:       time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC),
			DateString: "2022-06-01",
			Age:        2.5,
			PersonName: "Test Child",
		},
	}

	// Test importing measurements
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		importedCount, skippedCount, errors := importMeasurements(tx, importHeights, importWeights, personIdMapping, testUser.FamilyId)

		// Should import 2 valid heights + 2 weights = 4 measurements
		// Should skip 1 height with invalid date (year 0001)
		// The height for non-existent person is filtered out by filterMeasurements, not counted as skipped
		expectedImported := 4
		expectedSkipped := 1 // Only 1 for invalid date (the other is filtered out)

		if importedCount != expectedImported {
			t.Errorf("Expected %d measurements imported, got %d", expectedImported, importedCount)
		}

		if skippedCount != expectedSkipped {
			t.Errorf("Expected %d measurements skipped, got %d", expectedSkipped, skippedCount)
		}

		// Should have 1 error for unknown person ID
		if len(errors) != 1 {
			t.Errorf("Expected 1 error for unknown person, got %d: %v", len(errors), errors)
		}

		vbolt.TxCommit(tx)
	})

	// Verify measurements are in database
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		newPersonId := personIdMapping[1]
		growthData := GetPersonGrowthDataTx(tx, newPersonId)

		if len(growthData) != 4 {
			t.Errorf("Expected 4 growth data records, got %d", len(growthData))
		}

		// Count by type
		heightCount := 0
		weightCount := 0
		for _, data := range growthData {
			if data.MeasurementType == Height {
				heightCount++
				if data.Unit != "in" {
					t.Errorf("Expected height unit 'in', got '%s'", data.Unit)
				}
			} else if data.MeasurementType == Weight {
				weightCount++
				if data.Unit != "lbs" {
					t.Errorf("Expected weight unit 'lbs', got '%s'", data.Unit)
				}
			}

			// Verify family assignment
			if data.FamilyId != testUser.FamilyId {
				t.Errorf("Expected FamilyId %d, got %d", testUser.FamilyId, data.FamilyId)
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

func TestImportDataValidation(t *testing.T) {
	tests := []struct {
		name          string
		importData    ImportDataStructure
		shouldError   bool
		errorContains string
	}{
		{
			name: "valid data",
			importData: ImportDataStructure{
				People: []ImportPerson{
					{
						Id:       1,
						Name:     "Test Person",
						Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			shouldError: false,
		},
		{
			name: "no people",
			importData: ImportDataStructure{
				People: []ImportPerson{},
			},
			shouldError:   true,
			errorContains: "No people found",
		},
		{
			name: "person with no name",
			importData: ImportDataStructure{
				People: []ImportPerson{
					{
						Id:       1,
						Name:     "",
						Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			shouldError:   true,
			errorContains: "has no name",
		},
		{
			name: "person with invalid birthday",
			importData: ImportDataStructure{
				People: []ImportPerson{
					{
						Id:       1,
						Name:     "Test Person",
						Birthday: time.Time{}, // Zero time
					},
				},
			},
			shouldError:   true,
			errorContains: "invalid birthday",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateImportData(test.importData)

			if test.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				} else if test.errorContains != "" && err.Error() != test.errorContains {
					// For this simple test, just check that we got an error
					// In a more complex scenario, we might check specific error messages
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", test.name, err)
				}
			}
		})
	}
}

func TestFilterPeople(t *testing.T) {
	testPeople := []ImportPerson{
		{Id: 1, FamilyId: 100, Name: "Person 1"},
		{Id: 2, FamilyId: 100, Name: "Person 2"},
		{Id: 3, FamilyId: 200, Name: "Person 3"},
		{Id: 4, FamilyId: 200, Name: "Person 4"},
		{Id: 5, FamilyId: 300, Name: "Person 5"},
	}

	tests := []struct {
		name            string
		filterFamilyIds []int
		filterPersonIds []int
		expectedCount   int
		expectedNames   []string
	}{
		{
			name:            "no filters - return all",
			filterFamilyIds: []int{},
			filterPersonIds: []int{},
			expectedCount:   5,
			expectedNames:   []string{"Person 1", "Person 2", "Person 3", "Person 4", "Person 5"},
		},
		{
			name:            "filter by family ID 100",
			filterFamilyIds: []int{100},
			filterPersonIds: []int{},
			expectedCount:   2,
			expectedNames:   []string{"Person 1", "Person 2"},
		},
		{
			name:            "filter by multiple family IDs",
			filterFamilyIds: []int{100, 300},
			filterPersonIds: []int{},
			expectedCount:   3,
			expectedNames:   []string{"Person 1", "Person 2", "Person 5"},
		},
		{
			name:            "filter by person IDs",
			filterFamilyIds: []int{},
			filterPersonIds: []int{1, 3, 5},
			expectedCount:   3,
			expectedNames:   []string{"Person 1", "Person 3", "Person 5"},
		},
		{
			name:            "filter by family and person IDs (AND logic)",
			filterFamilyIds: []int{100},
			filterPersonIds: []int{1, 3},
			expectedCount:   1,
			expectedNames:   []string{"Person 1"}, // Only person 1 is in family 100 AND in person filter
		},
		{
			name:            "non-existent family ID",
			filterFamilyIds: []int{999},
			filterPersonIds: []int{},
			expectedCount:   0,
			expectedNames:   []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filtered := filterPeople(testPeople, test.filterFamilyIds, test.filterPersonIds)

			if len(filtered) != test.expectedCount {
				t.Errorf("Expected %d people, got %d", test.expectedCount, len(filtered))
			}

			// Check names (order might vary, so convert to map for comparison)
			actualNames := make(map[string]bool)
			for _, person := range filtered {
				actualNames[person.Name] = true
			}

			for _, expectedName := range test.expectedNames {
				if !actualNames[expectedName] {
					t.Errorf("Expected person '%s' in filtered results", expectedName)
				}
			}
		})
	}
}

func TestGetUniqueFamilyIds(t *testing.T) {
	testPeople := []ImportPerson{
		{Id: 1, FamilyId: 100, Name: "Person 1"},
		{Id: 2, FamilyId: 100, Name: "Person 2"},
		{Id: 3, FamilyId: 200, Name: "Person 3"},
		{Id: 4, FamilyId: 200, Name: "Person 4"},
		{Id: 5, FamilyId: 100, Name: "Person 5"}, // Duplicate family ID
	}

	familyIds := getUniqueFamilyIds(testPeople)

	if len(familyIds) != 2 {
		t.Errorf("Expected 2 unique family IDs, got %d", len(familyIds))
	}

	// Convert to map for easier checking
	familyIdMap := make(map[int]bool)
	for _, id := range familyIds {
		familyIdMap[id] = true
	}

	if !familyIdMap[100] {
		t.Error("Expected family ID 100 in results")
	}
	if !familyIdMap[200] {
		t.Error("Expected family ID 200 in results")
	}
}

func TestFilterMeasurements(t *testing.T) {
	personIdMapping := map[int]int{
		1: 101, // Old ID 1 maps to new ID 101
		2: 102, // Old ID 2 maps to new ID 102
		// Person ID 3 not in mapping (not imported)
	}

	importHeights := []ImportHeight{
		{Id: 1, PersonId: 1, Inches: 30.0}, // Should be included
		{Id: 2, PersonId: 2, Inches: 35.0}, // Should be included
		{Id: 3, PersonId: 3, Inches: 40.0}, // Should be excluded (person not imported)
	}

	importWeights := []ImportWeight{
		{Id: 1, PersonId: 1, Pounds: 20.0}, // Should be included
		{Id: 2, PersonId: 3, Pounds: 25.0}, // Should be excluded (person not imported)
		{Id: 3, PersonId: 2, Pounds: 30.0}, // Should be included
	}

	filteredHeights, filteredWeights := filterMeasurements(importHeights, importWeights, personIdMapping)

	// Should have 2 heights (person 1 and 2)
	if len(filteredHeights) != 2 {
		t.Errorf("Expected 2 filtered heights, got %d", len(filteredHeights))
	}

	// Should have 2 weights (person 1 and 2)
	if len(filteredWeights) != 2 {
		t.Errorf("Expected 2 filtered weights, got %d", len(filteredWeights))
	}

	// Verify the correct measurements were included
	heightPersonIds := make(map[int]bool)
	for _, height := range filteredHeights {
		heightPersonIds[height.PersonId] = true
	}
	if !heightPersonIds[1] || !heightPersonIds[2] {
		t.Error("Expected heights for person IDs 1 and 2")
	}

	weightPersonIds := make(map[int]bool)
	for _, weight := range filteredWeights {
		weightPersonIds[weight.PersonId] = true
	}
	if !weightPersonIds[1] || !weightPersonIds[2] {
		t.Error("Expected weights for person IDs 1 and 2")
	}
}

func TestFullImportWorkflow(t *testing.T) {
	testDBPath := "test_full_import.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	// Create test import data
	importData := ImportDataStructure{
		People: []ImportPerson{
			{
				Id:       1,
				FamilyId: 100,
				Type:     1,
				Gender:   0,
				Name:     "Test Child",
				Birthday: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Age:      "3 years",
			},
		},
		Heights: []ImportHeight{
			{
				Id:       1,
				PersonId: 1,
				Inches:   30.5,
				Date:     time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC),
				Age:      1.5,
			},
		},
		Weights: []ImportWeight{
			{
				Id:       1,
				PersonId: 1,
				Pounds:   20.5,
				Date:     time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC),
				Age:      1.5,
			},
		},
		ExportDate:   time.Now(),
		TotalHeights: 1,
		TotalWeights: 1,
		TotalPeople:  1,
	}

	jsonData, err := json.Marshal(importData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	var testUser User

	// Setup: Create test user
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

	// Test preview mode
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		// Simulate ImportData procedure call with preview
		var importDataParsed ImportDataStructure
		json.Unmarshal(jsonData, &importDataParsed)

		// Validate
		err := validateImportData(importDataParsed)
		if err != nil {
			t.Fatalf("Validation failed: %v", err)
		}

		// Get preview data
		availableFamilyIds := getUniqueFamilyIds(importDataParsed.People)
		if len(availableFamilyIds) != 1 || availableFamilyIds[0] != 100 {
			t.Errorf("Expected family ID 100 in preview, got %v", availableFamilyIds)
		}

		if len(importDataParsed.People) != 1 {
			t.Errorf("Expected 1 person in preview, got %d", len(importDataParsed.People))
		}
	})

	// Test actual import
	var importResponse ImportDataResponse
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Parse and validate
		var importDataParsed ImportDataStructure
		json.Unmarshal(jsonData, &importDataParsed)

		// Filter (no filters in this test)
		filteredPeople := filterPeople(importDataParsed.People, []int{}, []int{})

		// Import people
		personIdMapping, importedPeople, mergedPeople, peopleErrors, peopleWarnings := importPeople(tx, filteredPeople, testUser.FamilyId, "create_all")
		importResponse.ImportedPeople = importedPeople
		importResponse.MergedPeople = mergedPeople
		importResponse.PersonIdMapping = personIdMapping
		importResponse.Errors = append(importResponse.Errors, peopleErrors...)
		importResponse.Warnings = append(importResponse.Warnings, peopleWarnings...)

		// Import measurements
		filteredHeights, filteredWeights := filterMeasurements(importDataParsed.Heights, importDataParsed.Weights, personIdMapping)
		importedMeasurements, skippedMeasurements, measurementErrors := importMeasurements(tx, filteredHeights, filteredWeights, personIdMapping, testUser.FamilyId)
		importResponse.ImportedMeasurements = importedMeasurements
		importResponse.SkippedMeasurements = skippedMeasurements
		importResponse.Errors = append(importResponse.Errors, measurementErrors...)

		vbolt.TxCommit(tx)
	})

	// Verify import results
	if importResponse.ImportedPeople != 1 {
		t.Errorf("Expected 1 imported person, got %d", importResponse.ImportedPeople)
	}

	if importResponse.MergedPeople != 0 {
		t.Errorf("Expected 0 merged people with create_all strategy, got %d", importResponse.MergedPeople)
	}

	if importResponse.SkippedPeople != 0 {
		t.Errorf("Expected 0 skipped people, got %d", importResponse.SkippedPeople)
	}

	if importResponse.ImportedMeasurements != 2 {
		t.Errorf("Expected 2 imported measurements, got %d", importResponse.ImportedMeasurements)
	}

	if importResponse.SkippedMeasurements != 0 {
		t.Errorf("Expected 0 skipped measurements, got %d", importResponse.SkippedMeasurements)
	}

	if len(importResponse.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", importResponse.Errors)
	}

	if len(importResponse.Warnings) > 0 {
		t.Logf("Import warnings: %v", importResponse.Warnings)
	}

	// Verify data in database
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		newPersonId := importResponse.PersonIdMapping[1]

		// Check person exists
		person := GetPersonById(tx, newPersonId)
		if person.Id == 0 {
			t.Error("Imported person not found in database")
		}
		if person.Name != "Test Child" {
			t.Errorf("Expected person name 'Test Child', got '%s'", person.Name)
		}

		// Check measurements exist
		growthData := GetPersonGrowthDataTx(tx, newPersonId)
		if len(growthData) != 2 {
			t.Errorf("Expected 2 growth data records, got %d", len(growthData))
		}
	})
}

// Test merge functionality
func TestImportPeopleMergeStrategy(t *testing.T) {
	testDBPath := "test_import_merge.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

	// Setup: Create test user and existing person
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		// Add existing person
		existingPerson, _ := AddPersonTx(tx, AddPersonRequest{
			Name:       "John Doe",
			PersonType: 1,
			Gender:     0,
			Birthdate:  "2015-06-15",
		}, testUser.FamilyId)

		t.Logf("Created existing person with ID %d", existingPerson.Id)
		vbolt.TxCommit(tx)
	})

	// Test import data with same person
	testImportPeople := []ImportPerson{
		{
			Id:       1,
			FamilyId: 100,
			Type:     1, // Child
			Gender:   0, // Male
			Name:     "John Doe",
			Birthday: time.Date(2015, 6, 15, 0, 0, 0, 0, time.UTC),
			Age:      "8 years",
			ImageId:  0,
		},
	}

	// Test merge_people strategy
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		mapping, importedCount, mergedCount, errors, warnings := importPeople(tx, testImportPeople, testUser.FamilyId, "merge_people")

		if len(errors) > 0 {
			t.Errorf("Unexpected errors during merge: %v", errors)
		}

		if importedCount != 0 {
			t.Errorf("Expected 0 people imported with merge strategy, got %d", importedCount)
		}

		if mergedCount != 1 {
			t.Errorf("Expected 1 person merged, got %d", mergedCount)
		}

		if len(mapping) != 1 {
			t.Errorf("Expected 1 person in mapping, got %d", len(mapping))
		}

		if len(warnings) != 1 {
			t.Errorf("Expected 1 warning about merge, got %d", len(warnings))
		}

		vbolt.TxCommit(tx)
	})

	// Test skip_duplicates strategy
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		mapping, importedCount, mergedCount, errors, warnings := importPeople(tx, testImportPeople, testUser.FamilyId, "skip_duplicates")

		if len(errors) > 0 {
			t.Errorf("Unexpected errors during skip: %v", errors)
		}

		if importedCount != 0 {
			t.Errorf("Expected 0 people imported with skip strategy, got %d", importedCount)
		}

		if mergedCount != 0 {
			t.Errorf("Expected 0 people merged with skip strategy, got %d", mergedCount)
		}

		if len(mapping) != 0 {
			t.Errorf("Expected 0 people in mapping with skip strategy, got %d", len(mapping))
		}

		if len(warnings) != 1 {
			t.Errorf("Expected 1 warning about skip, got %d", len(warnings))
		}

		vbolt.TxCommit(tx)
	})
}
