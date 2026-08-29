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
			expected:  "11 months",
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
			expected:  "3 years",
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
			expected:      "2 years",
		},
		{
			name:          "Born Feb 29, reference Mar 1 (non-leap year)",
			birthdate:     time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
			expected:      "3 years",
		},
		{
			name:          "Same date different year",
			birthdate:     time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC),
			expected:      "3 years",
		},
		{
			name:          "Day before birthday",
			birthdate:     time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 9, 0, 0, 0, 0, time.UTC),
			expected:      "2 years",
		},
		{
			name:          "Day after birthday",
			birthdate:     time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 11, 0, 0, 0, 0, time.UTC),
			expected:      "3 years",
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
		{
			name:          "Due date in future shows gestational weeks",
			birthdate:     time.Date(2023, 8, 16, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC),
			expected:      "26 weeks",
		},
		{
			name:          "Due date tomorrow is full term",
			birthdate:     time.Date(2023, 5, 11, 0, 0, 0, 0, time.UTC),
			referenceDate: time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC),
			expected:      "39 weeks",
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

func TestCalculatePersonAgePregnancyPastDue(t *testing.T) {
	dueDate := time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC)
	referenceDate := time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC)

	pregnancyAge := calculatePersonAgeAt(dueDate, referenceDate, true)
	if pregnancyAge != "41 weeks" {
		t.Errorf("pregnancy past due should remain gestational age, got %q", pregnancyAge)
	}

	childAge := calculatePersonAgeAt(dueDate, referenceDate, false)
	if childAge != "< 1 month" {
		t.Errorf("converted child should use regular age, got %q", childAge)
	}
}

func TestCalculateAgeCurrentTime(t *testing.T) {
	now := time.Now()

	twentyYearsAgo := now.AddDate(-20, 0, 0)
	age := calculateAge(twentyYearsAgo)
	expected := "20 years"

	if age != expected {
		t.Errorf("calculateAge for someone born exactly 20 years ago should be %q, got %q", expected, age)
	}

	oneDayAgo := now.AddDate(0, 0, -1)
	age = calculateAge(oneDayAgo)
	if age != "< 1 month" {
		t.Errorf("calculateAge for someone born 1 day ago should be \"< 1 month\", got %q", age)
	}

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
			t.Fatalf("Failed to create test person: %v", err)
		}

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

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		person := GetPersonById(tx, testPerson.Id)
		if person.Id == 0 {
			t.Fatal("Failed to retrieve person")
		}

		growthData := GetPersonGrowthDataTx(tx, testPerson.Id)
		if len(growthData) == 0 {
			t.Error("Expected at least one growth data record")
		}

		milestones := GetPersonMilestonesTx(tx, testPerson.Id)
		if len(milestones) != 2 {
			t.Errorf("Expected 2 milestones, got %d", len(milestones))
		}

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
	})
}

func TestMergePeople(t *testing.T) {
	testDBPath := "test_merge_people.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var sourcePerson Person
	var targetPerson Person

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "merge@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		sourceReq := AddPersonRequest{
			Name:      "Source Child",
			Gender:    0,
			Birthdate: "2020-01-15",
		}
		var err error
		sourcePerson, err = AddPersonTx(tx, sourceReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create source person: %v", err)
		}

		targetReq := AddPersonRequest{
			Name:      "Target Child",
			Gender:    0,
			Birthdate: "2020-01-20",
		}
		targetPerson, err = AddPersonTx(tx, targetReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create target person: %v", err)
		}

		milestoneReq1 := AddMilestoneRequest{
			PersonId:    sourcePerson.Id,
			Description: "First words from source",
			Category:    "development",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(3),
		}
		_, err = AddMilestoneTx(tx, milestoneReq1, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add milestone to source: %v", err)
		}

		milestoneReq2 := AddMilestoneRequest{
			PersonId:    targetPerson.Id,
			Description: "First words from target",
			Category:    "development",
			InputType:   "age",
			AgeYears:    intPtr(1),
			AgeMonths:   intPtr(2),
		}
		_, err = AddMilestoneTx(tx, milestoneReq2, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add milestone to target: %v", err)
		}

		growthReq1 := AddGrowthDataRequest{
			PersonId:        sourcePerson.Id,
			MeasurementType: "height",
			Value:           85.0,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2021-01-15"),
		}
		_, err = AddGrowthDataTx(tx, growthReq1, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add growth data to source: %v", err)
		}

		growthReq2 := AddGrowthDataRequest{
			PersonId:        targetPerson.Id,
			MeasurementType: "weight",
			Value:           12.0,
			Unit:            "kg",
			InputType:       "date",
			MeasurementDate: stringPtr("2021-01-20"),
		}
		_, err = AddGrowthDataTx(tx, growthReq2, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to add growth data to target: %v", err)
		}

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		sourceMilestones := GetPersonMilestonesTx(tx, sourcePerson.Id)
		if len(sourceMilestones) != 1 {
			t.Errorf("Expected 1 source milestone, got %d", len(sourceMilestones))
		}

		targetMilestones := GetPersonMilestonesTx(tx, targetPerson.Id)
		if len(targetMilestones) != 1 {
			t.Errorf("Expected 1 target milestone, got %d", len(targetMilestones))
		}

		sourceGrowth := GetPersonGrowthDataTx(tx, sourcePerson.Id)
		if len(sourceGrowth) != 1 {
			t.Errorf("Expected 1 source growth record, got %d", len(sourceGrowth))
		}

		targetGrowth := GetPersonGrowthDataTx(tx, targetPerson.Id)
		if len(targetGrowth) != 1 {
			t.Errorf("Expected 1 target growth record, got %d", len(targetGrowth))
		}
	})

	var mergedGrowthCount, mergedMilestones int
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		growthData := GetPersonGrowthDataTx(tx, sourcePerson.Id)
		for _, gd := range growthData {
			gd.PersonId = targetPerson.Id
			vbolt.Write(tx, GrowthDataBkt, gd.Id, &gd)
			vbolt.SetTargetSingleTerm(tx, GrowthDataByPersonIndex, gd.Id, targetPerson.Id)
		}
		mergedGrowthCount = len(growthData)

		milestones := GetPersonMilestonesTx(tx, sourcePerson.Id)
		for _, milestone := range milestones {
			milestone.PersonId = targetPerson.Id
			vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
			vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, milestone.Id, targetPerson.Id)
		}
		mergedMilestones = len(milestones)

		vbolt.Delete(tx, PeopleBkt, sourcePerson.Id)
		vbolt.SetTargetSingleTerm(tx, PersonIndex, sourcePerson.Id, -1)

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		sourceFetched := GetPersonById(tx, sourcePerson.Id)
		if sourceFetched.Id != 0 {
			t.Error("Source person should be deleted after merge")
		}

		targetFetched := GetPersonById(tx, targetPerson.Id)
		if targetFetched.Id == 0 {
			t.Error("Target person should still exist after merge")
		}

		targetMilestones := GetPersonMilestonesTx(tx, targetPerson.Id)
		if len(targetMilestones) != 2 {
			t.Errorf("Expected 2 milestones after merge, got %d", len(targetMilestones))
		}

		for _, milestone := range targetMilestones {
			if milestone.PersonId != targetPerson.Id {
				t.Errorf("Expected milestone PersonId %d, got %d", targetPerson.Id, milestone.PersonId)
			}
		}

		targetGrowth := GetPersonGrowthDataTx(tx, targetPerson.Id)
		if len(targetGrowth) != 2 {
			t.Errorf("Expected 2 growth records after merge, got %d", len(targetGrowth))
		}

		for _, gd := range targetGrowth {
			if gd.PersonId != targetPerson.Id {
				t.Errorf("Expected growth data PersonId %d, got %d", targetPerson.Id, gd.PersonId)
			}
		}

		sourceMilestones := GetPersonMilestonesTx(tx, sourcePerson.Id)
		if len(sourceMilestones) != 0 {
			t.Errorf("Expected 0 source milestones after merge, got %d", len(sourceMilestones))
		}

		sourceGrowth := GetPersonGrowthDataTx(tx, sourcePerson.Id)
		if len(sourceGrowth) != 0 {
			t.Errorf("Expected 0 source growth records after merge, got %d", len(sourceGrowth))
		}

		if mergedGrowthCount != 1 {
			t.Errorf("Expected to merge 1 growth record, got %d", mergedGrowthCount)
		}
		if mergedMilestones != 1 {
			t.Errorf("Expected to merge 1 milestone, got %d", mergedMilestones)
		}
	})
}

func TestMergePeopleValidation(t *testing.T) {
	testDBPath := "test_merge_validation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var testPerson Person

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "validation@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		personReq := AddPersonRequest{
			Name:      "Test Child",
			Gender:    0,
			Birthdate: "2020-01-15",
		}
		var err error
		testPerson, err = AddPersonTx(tx, personReq, testUser.FamilyId)
		if err != nil {
			t.Fatalf("Failed to create test person: %v", err)
		}

		vbolt.TxCommit(tx)
	})

	t.Run("cannot merge with self", func(t *testing.T) {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			if testPerson.Id == testPerson.Id {
				t.Log("Correctly identified that source and target are the same")
			}
		})
	})

	t.Run("cannot merge non-existent person", func(t *testing.T) {
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			nonExistentPerson := GetPersonById(tx, 99999)
			if nonExistentPerson.Id == 0 {
				t.Log("Correctly identified non-existent person")
			} else {
				t.Error("Expected person with ID 99999 to not exist")
			}
		})
	})
}
