package backend

import (
	"encoding/json"
	"family/cfg"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
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

func TestExportFamilyDataIsolation(t *testing.T) {
	testDBPath := "test_export_isolation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser1, testUser2 User
	var family1Id, family2Id int

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

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		family1People := GetFamilyPeople(tx, family1Id)
		family2People := GetFamilyPeople(tx, family2Id)

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

		if family1Id == family2Id {
			t.Error("Different users should have different family IDs")
		}
	})
}

func TestExportDataCompleteness(t *testing.T) {
	testDBPath := "test_export_completeness.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var testPerson Person

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
			Gender:    0,
			Birthdate: "2020-06-15",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add person: %v", err)
		}

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
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

		var milestoneIds []int
		vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, testUser.FamilyId, &milestoneIds, vbolt.Window{})

		if len(milestoneIds) != 0 {
			t.Errorf("Expected 0 milestones for new family, got %d", len(milestoneIds))
		}

		var growthDataIds []int
		vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, testUser.FamilyId, &growthDataIds, vbolt.Window{})

		if len(growthDataIds) != 0 {
			t.Errorf("Expected 0 growth data entries for new family, got %d", len(growthDataIds))
		}
	})
}

func TestEmptyFamilyExport(t *testing.T) {
	testDBPath := "test_empty_export.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

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

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		people := GetFamilyPeople(tx, testUser.FamilyId)

		var milestoneIds []int
		vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, testUser.FamilyId, &milestoneIds, vbolt.Window{})

		var growthDataIds []int
		vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, testUser.FamilyId, &growthDataIds, vbolt.Window{})

		exportData := ExportDataStructure{
			People:          []ImportPerson{},
			Heights:         []ImportHeight{},
			Weights:         []ImportWeight{},
			Milestones:      []ExportMilestone{},
			ExportDate:      time.Now(),
			TotalHeights:    0,
			TotalWeights:    0,
			TotalPeople:     len(people),
			TotalMilestones: len(milestoneIds),
		}

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

		if exportData.ExportDate.IsZero() {
			t.Error("Export date should be set even for empty export")
		}
	})
}

func TestExportPhotoStructure(t *testing.T) {
	ep := ExportPhoto{
		Id:        42,
		Title:     "Birthday",
		ZipPath:   "photos/abc_original.jpg",
		PersonIds: []int{1, 2},
		TagIds:    []int{10},
	}
	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ExportPhoto
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ZipPath != "photos/abc_original.jpg" {
		t.Errorf("ZipPath mismatch: %s", decoded.ZipPath)
	}
	if len(decoded.PersonIds) != 2 {
		t.Errorf("PersonIds length: %d", len(decoded.PersonIds))
	}
}

func TestExportDataStructurePhotoOmitempty(t *testing.T) {
	with := ExportDataStructure{
		Photos:      []ExportPhoto{{Id: 1, ZipPath: "photos/a_original.jpg"}},
		TotalPhotos: 1,
		ExportDate:  time.Now(),
	}
	data, _ := json.Marshal(with)
	if !strings.Contains(string(data), `"photos"`) {
		t.Error("expected photos field in JSON")
	}

	without := ExportDataStructure{ExportDate: time.Now()}
	data2, _ := json.Marshal(without)
	if strings.Contains(string(data2), `"photos"`) {
		t.Error("expected photos field to be absent")
	}
}
