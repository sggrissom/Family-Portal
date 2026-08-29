package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type accessFixture struct {
	db      *vbolt.DB
	userA   User
	userB   User
	records map[string]int
}

func setupAccessFixture(t *testing.T) (accessFixture, func()) {
	t.Helper()

	testDBPath := "test_access.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	cleanup := func() {
		db.Close()
		os.Remove(testDBPath)
	}

	fx := accessFixture{db: db, records: make(map[string]int)}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		fx.userA = AddUserTx(tx, CreateAccountRequest{
			Name:  "User A",
			Email: "a@example.com",
		}, hash)
		fx.userB = AddUserTx(tx, CreateAccountRequest{
			Name:  "User B",
			Email: "b@example.com",
		}, hash)

		famA := fx.userA.FamilyId

		person, err := AddPersonTx(tx, AddPersonRequest{
			Name:      "Child A",
			Gender:    0,
			Birthdate: "2020-06-15",
		}, famA)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}
		fx.records["Person"] = person.FamilyId

		growth, err := AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId:        person.Id,
			MeasurementType: "height",
			Value:           90.5,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2023-06-15"),
		}, famA)
		if err != nil {
			t.Fatalf("AddGrowthDataTx: %v", err)
		}
		fx.records["GrowthData"] = growth.FamilyId

		milestone, err := AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId:      person.Id,
			Description:   "First steps",
			Category:      "development",
			InputType:     "date",
			MilestoneDate: stringPtr("2021-08-01"),
		}, famA)
		if err != nil {
			t.Fatalf("AddMilestoneTx: %v", err)
		}
		fx.records["Milestone"] = milestone.FamilyId

		message, err := AddChatMessageTx(tx, SendMessageRequest{
			Content: "hello",
		}, famA, fx.userA.Id, fx.userA.Name)
		if err != nil {
			t.Fatalf("AddChatMessageTx: %v", err)
		}
		fx.records["ChatMessage"] = message.FamilyId

		image := Image{
			Id:               vbolt.NextIntId(tx, ImagesBkt),
			FamilyId:         famA,
			OwnerUserId:      fx.userA.Id,
			OriginalFilename: "a.jpg",
			MimeType:         "image/jpeg",
			CreatedAt:        time.Now(),
		}
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, image.FamilyId)
		fx.records["Image"] = image.FamilyId

		photoPersonId := AddPersonToPhoto(tx, image.Id, person.Id, famA)
		fx.records["PhotoPerson"] = GetPhotoPersonById(tx, photoPersonId).FamilyId

		tag := Tag{
			Id:        vbolt.NextIntId(tx, TagBkt),
			FamilyId:  famA,
			Name:      "Trips",
			Color:     "#4A90D9",
			CreatedAt: time.Now(),
		}
		vbolt.Write(tx, TagBkt, tag.Id, &tag)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, tag.Id, tag.FamilyId)
		fx.records["Tag"] = tag.FamilyId

		addTagToMilestone(tx, milestone.Id, tag.Id, famA)
		var mtIds []int
		vbolt.ReadTermTargets(tx, MilestoneTagByMilestoneIndex, milestone.Id, &mtIds, vbolt.Window{})
		var mts []MilestoneTag
		vbolt.ReadSlice(tx, MilestoneTagBkt, mtIds, &mts)
		if len(mts) == 0 {
			t.Fatal("addTagToMilestone wrote no MilestoneTag")
		}
		fx.records["MilestoneTag"] = mts[0].FamilyId

		addTagToPhoto(tx, image.Id, tag.Id, famA)
		var ptIds []int
		vbolt.ReadTermTargets(tx, PhotoTagByPhotoIndex, image.Id, &ptIds, vbolt.Window{})
		var pts []PhotoTag
		vbolt.ReadSlice(tx, PhotoTagBkt, ptIds, &pts)
		if len(pts) == 0 {
			t.Fatal("addTagToPhoto wrote no PhotoTag")
		}
		fx.records["PhotoTag"] = pts[0].FamilyId

		vbolt.TxCommit(tx)
	})

	if fx.userA.FamilyId == fx.userB.FamilyId || fx.userA.FamilyId == 0 {
		cleanup()
		t.Fatalf("fixture users must be in distinct non-zero families, got %d and %d",
			fx.userA.FamilyId, fx.userB.FamilyId)
	}

	return fx, cleanup
}

func TestCanAccessFamilyDeniesForeignFamily(t *testing.T) {
	fx, cleanup := setupAccessFixture(t)
	defer cleanup()

	entityTypes := []string{
		"Person", "GrowthData", "Milestone", "Image",
		"PhotoPerson", "MilestoneTag", "PhotoTag", "Tag", "ChatMessage",
	}
	levels := []AccessLevel{AccessView, AccessContribute, AccessAdmin}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, entity := range entityTypes {
			familyId, ok := fx.records[entity]
			if !ok {
				t.Errorf("%s: fixture created no record", entity)
				continue
			}
			if familyId != fx.userA.FamilyId {
				t.Errorf("%s: expected FamilyId %d, got %d", entity, fx.userA.FamilyId, familyId)
				continue
			}

			for _, need := range levels {
				if CanAccessFamily(tx, fx.userB, familyId, need) {
					t.Errorf("%s: foreign family granted access at level %d", entity, need)
				}
				if !CanAccessFamily(tx, fx.userA, familyId, need) {
					t.Errorf("%s: owning family denied access at level %d", entity, need)
				}
			}
		}
	})
}

func TestCanAccessFamilyRejectsZeroFamily(t *testing.T) {
	fx, cleanup := setupAccessFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if CanAccessFamily(tx, fx.userA, 0, AccessView) {
			t.Error("family 0 must never be accessible")
		}

		familyless := User{Id: 999, Name: "Orphan"}
		if CanAccessFamily(tx, familyless, fx.userA.FamilyId, AccessView) {
			t.Error("a user with no family must never be granted access")
		}
		if CanAccessFamily(tx, familyless, 0, AccessView) {
			t.Error("family 0 must not match a familyless user")
		}
	})
}

func TestRequireFamilyAccess(t *testing.T) {
	fx, cleanup := setupAccessFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if err := RequireFamilyAccess(tx, fx.userA, fx.userA.FamilyId, AccessAdmin); err != nil {
			t.Errorf("owning family should be permitted, got %v", err)
		}
		if err := RequireFamilyAccess(tx, fx.userB, fx.userA.FamilyId, AccessView); err == nil {
			t.Error("foreign family should be refused")
		}

		if err := RequireFamilyAccessFrom(tx, fx.userA.FamilyId, fx.userA.FamilyId, AccessAdmin); err != nil {
			t.Errorf("same family should be permitted, got %v", err)
		}
		if err := RequireFamilyAccessFrom(tx, fx.userB.FamilyId, fx.userA.FamilyId, AccessView); err == nil {
			t.Error("foreign family should be refused")
		}
	})
}

func TestFamiliesVisibleTo(t *testing.T) {
	fx, cleanup := setupAccessFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		visible := familiesVisibleTo(tx, fx.userA)
		if len(visible) != 1 || visible[0] != fx.userA.FamilyId {
			t.Errorf("expected [%d], got %v", fx.userA.FamilyId, visible)
		}

		for _, familyId := range familiesVisibleTo(tx, fx.userB) {
			if familyId == fx.userA.FamilyId {
				t.Error("userB must not see userA's family")
			}
		}

		if got := familiesVisibleTo(tx, User{Id: 999}); len(got) != 0 {
			t.Errorf("a familyless user sees nothing, got %v", got)
		}
	})
}

func TestFamilyScopedHelpersDenyForeignFamily(t *testing.T) {
	fx, cleanup := setupAccessFixture(t)
	defer cleanup()

	foreign := fx.userB.FamilyId

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		var growthIds []int
		vbolt.ReadTermTargets(tx, GrowthDataByFamilyIndex, fx.userA.FamilyId, &growthIds, vbolt.Window{})
		if len(growthIds) == 0 {
			t.Fatal("fixture wrote no growth data")
		}
		if _, err := GetGrowthDataByIdAndFamily(tx, growthIds[0], foreign); err == nil {
			t.Error("GetGrowthDataByIdAndFamily allowed a foreign family")
		}

		var milestoneIds []int
		vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, fx.userA.FamilyId, &milestoneIds, vbolt.Window{})
		if len(milestoneIds) == 0 {
			t.Fatal("fixture wrote no milestones")
		}
		if _, err := GetMilestoneByIdAndFamily(tx, milestoneIds[0], foreign); err == nil {
			t.Error("GetMilestoneByIdAndFamily allowed a foreign family")
		}

		var messageIds []int
		vbolt.ReadTermTargets(tx, ChatMessagesByFamilyIndex, fx.userA.FamilyId, &messageIds, vbolt.Window{})
		if len(messageIds) == 0 {
			t.Fatal("fixture wrote no chat messages")
		}
		if _, err := GetChatMessageByIdAndFamily(tx, messageIds[0], foreign); err == nil {
			t.Error("GetChatMessageByIdAndFamily allowed a foreign family")
		}

		if people := GetVisiblePeople(tx, fx.userB); len(people) != 0 {
			t.Errorf("GetVisiblePeople leaked %d people to a foreign family", len(people))
		}
		if images := GetVisibleImages(tx, fx.userB); len(images) != 0 {
			t.Errorf("GetVisibleImages leaked %d images to a foreign family", len(images))
		}
		if tags := getVisibleTags(tx, fx.userB); len(tags) != 0 {
			t.Errorf("getVisibleTags leaked %d tags to a foreign family", len(tags))
		}
		if _, err := GetChatMessageForUser(tx, messageIds[0], fx.userB, AccessView); err == nil {
			t.Error("GetChatMessageForUser allowed a foreign family")
		}
	})

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		people := GetFamilyPeople(tx, fx.userA.FamilyId)
		if len(people) == 0 {
			t.Fatal("fixture wrote no people")
		}
		personId := people[0].Id

		if _, err := AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId:        personId,
			MeasurementType: "height",
			Value:           95,
			Unit:            "cm",
			InputType:       "date",
			MeasurementDate: stringPtr("2023-07-15"),
		}, foreign); err == nil {
			t.Error("AddGrowthDataTx allowed writing against a foreign family's person")
		}

		if _, err := AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId:      personId,
			Description:   "Should not land",
			Category:      "development",
			InputType:     "date",
			MilestoneDate: stringPtr("2021-09-01"),
		}, foreign); err == nil {
			t.Error("AddMilestoneTx allowed writing against a foreign family's person")
		}
	})
}
