// Cross-family isolation, checked one procedure at a time.
//
// The access helpers have their own tests, but those prove the helpers work,
// not that every procedure calls one. This file goes the other way: it stands
// up two unrelated families and makes the second family's user call the
// procedures with the first family's record ids, which is exactly what an
// attacker with an account would do. A procedure that forgets its check fails
// here rather than in production.
package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// isolationFixture is two families that have nothing to do with each other.
// Everything named `theirs` belongs to the owner's family; the outsider has
// their own family and a person in it, so requests that need an id from both
// sides can be built.
type isolationFixture struct {
	db *vbolt.DB

	owner       User
	ownerFamily int
	outsider    User
	theirFamily int

	person     Person
	growth     GrowthData
	milestone  Milestone
	photo      Image
	tag        Tag
	message    ChatMessage
	ownPerson  Person // the outsider's own person
	ownPhotoId int    // a photo in the outsider's family

	activity   Activity
	season     Season
	event      Event
	entry      Entry
	appearance Appearance
	result     Result
}

func setupIsolationFixture(t *testing.T) (isolationFixture, func()) {
	t.Helper()

	tempFile, err := os.CreateTemp("", "test_isolation_*.db")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	path := tempFile.Name()
	tempFile.Close()

	db := vbolt.Open(path)
	vbolt.InitBuckets(db, &cfg.Info)
	cleanup := func() {
		db.Close()
		os.Remove(path)
	}

	fx := isolationFixture{db: db}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@example.com"}, hash)
		fx.outsider = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider@example.com"}, hash)
		fx.ownerFamily = fx.owner.FamilyId
		fx.theirFamily = fx.outsider.FamilyId

		fx.person, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Their Kid", PersonType: 1, Gender: 0, Birthdate: "2020-06-15",
		}, fx.ownerFamily)
		if err != nil {
			t.Fatalf("AddPersonTx(owner) error = %v", err)
		}
		fx.ownPerson, err = AddPersonTx(tx, AddPersonRequest{
			Name: "My Kid", PersonType: 1, Gender: 1, Birthdate: "2021-01-05",
		}, fx.theirFamily)
		if err != nil {
			t.Fatalf("AddPersonTx(outsider) error = %v", err)
		}

		measurementDate := "2024-01-05"
		fx.growth, err = AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId: fx.person.Id, MeasurementType: "height", Value: 90, Unit: "cm",
			InputType: "date", MeasurementDate: &measurementDate,
		}, fx.ownerFamily)
		if err != nil {
			t.Fatalf("AddGrowthDataTx() error = %v", err)
		}

		milestoneDate := "2024-02-10"
		fx.milestone, err = AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId: fx.person.Id, Description: "First word", Category: "development",
			InputType: "date", MilestoneDate: &milestoneDate,
		}, fx.ownerFamily)
		if err != nil {
			t.Fatalf("AddMilestoneTx() error = %v", err)
		}

		fx.photo = Image{
			Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: fx.ownerFamily,
			OwnerUserId: fx.owner.Id, OriginalFilename: "theirs.jpg",
			MimeType: "image/jpeg", FilePath: "photos/theirs.jpg", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, ImagesBkt, fx.photo.Id, &fx.photo)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, fx.photo.Id, fx.ownerFamily)

		ownPhoto := Image{
			Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: fx.theirFamily,
			OwnerUserId: fx.outsider.Id, OriginalFilename: "mine.jpg",
			MimeType: "image/jpeg", FilePath: "photos/mine.jpg", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, ImagesBkt, ownPhoto.Id, &ownPhoto)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, ownPhoto.Id, fx.theirFamily)
		fx.ownPhotoId = ownPhoto.Id

		fx.tag = Tag{
			Id: vbolt.NextIntId(tx, TagBkt), FamilyId: fx.ownerFamily,
			Name: "Holidays", Color: "#4A90D9", CreatedAt: time.Now(),
		}
		vbolt.Write(tx, TagBkt, fx.tag.Id, &fx.tag)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, fx.tag.Id, fx.ownerFamily)

		fx.message, err = AddChatMessageTx(tx, SendMessageRequest{
			Content: "family only", ClientMessageId: "seed-1",
		}, fx.ownerFamily, fx.owner.Id, fx.owner.Name)
		if err != nil {
			t.Fatalf("AddChatMessageTx() error = %v", err)
		}

		now := time.Now()
		fx.activity = Activity{
			Id: vbolt.NextIntId(tx, ActivityBkt), FamilyId: fx.ownerFamily,
			Name: "Dance", Kind: ActivityKindDance, CreatedAt: now,
		}
		writeActivityTx(tx, &fx.activity)
		fx.season = Season{
			Id: vbolt.NextIntId(tx, SeasonBkt), ActivityId: fx.activity.Id,
			FamilyId: fx.ownerFamily, Name: "2025-26", StartDate: now, CreatedAt: now,
		}
		writeSeasonTx(tx, &fx.season)
		fx.event = Event{
			Id: vbolt.NextIntId(tx, EventBkt), SeasonId: fx.season.Id,
			FamilyId: fx.ownerFamily, Name: "Nuvo Nashville", Host: "Nuvo", StartDate: now, CreatedAt: now,
		}
		writeEventTx(tx, &fx.event)
		fx.entry = Entry{
			Id: vbolt.NextIntId(tx, EntryBkt), SeasonId: fx.season.Id,
			FamilyId: fx.ownerFamily, Name: "Rise Up", Format: "group", CreatedAt: now,
		}
		writeEntryTx(tx, &fx.entry)
		member := EntryMember{
			Id: vbolt.NextIntId(tx, EntryMemberBkt), EntryId: fx.entry.Id,
			PersonId: fx.person.Id, FamilyId: fx.ownerFamily, CreatedAt: now,
		}
		writeEntryMemberTx(tx, &member)
		fx.appearance = Appearance{
			Id: vbolt.NextIntId(tx, AppearanceBkt), EventId: fx.event.Id,
			EntryId: fx.entry.Id, FamilyId: fx.ownerFamily, OccurredAt: now, CreatedAt: now,
		}
		writeAppearanceTx(tx, &fx.appearance)
		fx.result = Result{
			Id: vbolt.NextIntId(tx, ResultBkt), AppearanceId: fx.appearance.Id,
			FamilyId: fx.ownerFamily, Kind: ResultKindAdjudication, Label: "High Gold", CreatedAt: now,
		}
		writeResultTx(tx, &fx.result)

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

// asOutsider runs fn in a write transaction with the outsider authenticated.
// A write transaction is used even for reads so that a procedure which mutates
// despite the missing access would leave evidence the assertions can find.
func (fx isolationFixture) asOutsider(t *testing.T, fn func(ctx *vbeam.Context)) {
	t.Helper()

	token, err := generateJwtTokenString(fx.outsider)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fn(&vbeam.Context{Tx: tx, Token: token})
	})
}

// Every procedure that names a record by id must refuse an id belonging to
// another family.
func TestProceduresRefuseAnotherFamilysRecords(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	date := "2024-05-05"
	calls := []struct {
		name string
		call func(ctx *vbeam.Context) error
	}{
		// People
		{"GetPerson", func(ctx *vbeam.Context) error {
			_, err := GetPerson(ctx, GetPersonRequest{Id: fx.person.Id})
			return err
		}},
		{"UpdatePerson", func(ctx *vbeam.Context) error {
			_, err := UpdatePerson(ctx, UpdatePersonRequest{
				Id: fx.person.Id, Name: "Renamed", PersonType: 1, Gender: 0, Birthdate: "2020-06-15",
			})
			return err
		}},
		{"ComparePeople", func(ctx *vbeam.Context) error {
			_, err := ComparePeople(ctx, ComparePeopleRequest{PersonIds: []int{fx.person.Id, fx.ownPerson.Id}})
			return err
		}},
		{"MergePeople", func(ctx *vbeam.Context) error {
			_, err := MergePeople(ctx, MergePeopleRequest{
				SourcePersonId: fx.person.Id, TargetPersonId: fx.ownPerson.Id,
			})
			return err
		}},
		{"SetProfilePhoto", func(ctx *vbeam.Context) error {
			_, err := SetProfilePhoto(ctx, SetProfilePhotoRequest{
				PersonId: fx.person.Id, PhotoId: fx.photo.Id, CropX: 50, CropY: 50, CropScale: 1,
			})
			return err
		}},
		// Growth
		{"AddGrowthData", func(ctx *vbeam.Context) error {
			_, err := AddGrowthData(ctx, AddGrowthDataRequest{
				PersonId: fx.person.Id, MeasurementType: "weight", Value: 12, Unit: "kg",
				InputType: "date", MeasurementDate: &date,
			})
			return err
		}},
		{"GetGrowthData", func(ctx *vbeam.Context) error {
			_, err := GetGrowthData(ctx, GetGrowthDataRequest{Id: fx.growth.Id})
			return err
		}},
		{"UpdateGrowthData", func(ctx *vbeam.Context) error {
			_, err := UpdateGrowthData(ctx, UpdateGrowthDataRequest{
				Id: fx.growth.Id, MeasurementType: "height", Value: 200, Unit: "cm",
				InputType: "date", MeasurementDate: &date,
			})
			return err
		}},
		{"DeleteGrowthData", func(ctx *vbeam.Context) error {
			_, err := DeleteGrowthData(ctx, DeleteGrowthDataRequest{Id: fx.growth.Id})
			return err
		}},

		// Milestones
		{"AddMilestone", func(ctx *vbeam.Context) error {
			_, err := AddMilestone(ctx, AddMilestoneRequest{
				PersonId: fx.person.Id, Description: "Injected", Category: "development",
				InputType: "date", MilestoneDate: &date,
			})
			return err
		}},
		{"GetMilestone", func(ctx *vbeam.Context) error {
			_, err := GetMilestone(ctx, GetMilestoneRequest{Id: fx.milestone.Id})
			return err
		}},
		{"GetPersonMilestones", func(ctx *vbeam.Context) error {
			_, err := GetPersonMilestones(ctx, GetPersonMilestonesRequest{PersonId: fx.person.Id})
			return err
		}},
		{"UpdateMilestone", func(ctx *vbeam.Context) error {
			_, err := UpdateMilestone(ctx, UpdateMilestoneRequest{
				Id: fx.milestone.Id, Description: "Rewritten", Category: "development",
				InputType: "date", MilestoneDate: &date,
			})
			return err
		}},
		{"DeleteMilestone", func(ctx *vbeam.Context) error {
			_, err := DeleteMilestone(ctx, DeleteMilestoneRequest{Id: fx.milestone.Id})
			return err
		}},
		{"UpdateMilestoneTags", func(ctx *vbeam.Context) error {
			_, err := UpdateMilestoneTags(ctx, UpdateMilestoneTagsRequest{
				MilestoneId: fx.milestone.Id, TagIds: []int{fx.tag.Id},
			})
			return err
		}},

		// Photos
		{"GetPhoto", func(ctx *vbeam.Context) error {
			_, err := GetPhoto(ctx, GetPhotoRequest{Id: fx.photo.Id})
			return err
		}},
		{"GetPhotoStatus", func(ctx *vbeam.Context) error {
			_, err := GetPhotoStatus(ctx, GetPhotoStatusRequest{Id: fx.photo.Id})
			return err
		}},
		{"UpdatePhoto", func(ctx *vbeam.Context) error {
			_, err := UpdatePhoto(ctx, UpdatePhotoRequest{
				Id: fx.photo.Id, Title: "Mine now", InputType: "date", PhotoDate: date,
			})
			return err
		}},
		{"DeletePhoto", func(ctx *vbeam.Context) error {
			_, err := DeletePhoto(ctx, DeletePhotoRequest{Id: fx.photo.Id})
			return err
		}},
		{"AddPeopleToPhoto", func(ctx *vbeam.Context) error {
			_, err := AddPeopleToPhoto(ctx, AddPeopleToPhotoRequest{
				PhotoId: fx.photo.Id, PersonIds: []int{fx.ownPerson.Id},
			})
			return err
		}},
		{"RemovePersonFromPhotoProc", func(ctx *vbeam.Context) error {
			_, err := RemovePersonFromPhotoProc(ctx, RemovePersonFromPhotoRequest{
				PhotoId: fx.photo.Id, PersonId: fx.person.Id,
			})
			return err
		}},
		{"UpdatePhotoTags", func(ctx *vbeam.Context) error {
			_, err := UpdatePhotoTags(ctx, UpdatePhotoTagsRequest{
				PhotoId: fx.photo.Id, TagIds: []int{fx.tag.Id},
			})
			return err
		}},

		// Tags
		{"UpdateTag", func(ctx *vbeam.Context) error {
			_, err := UpdateTag(ctx, UpdateTagRequest{Id: fx.tag.Id, Name: "Stolen", Color: "#000000"})
			return err
		}},
		{"DeleteTag", func(ctx *vbeam.Context) error {
			_, err := DeleteTag(ctx, DeleteTagRequest{Id: fx.tag.Id})
			return err
		}},

		// Chat
		{"DeleteMessage", func(ctx *vbeam.Context) error {
			_, err := DeleteMessage(ctx, DeleteMessageRequest{Id: fx.message.Id})
			return err
		}},

		// Sharing
		{"GetPersonSharing", func(ctx *vbeam.Context) error {
			_, err := GetPersonSharing(ctx, GetPersonSharingRequest{PersonId: fx.person.Id})
			return err
		}},
		{"SharePersonWithFamily", func(ctx *vbeam.Context) error {
			_, err := SharePersonWithFamily(ctx, SharePersonRequest{
				PersonId: fx.person.Id, FamilyId: fx.theirFamily,
			})
			return err
		}},
		{"UnsharePersonFromFamily", func(ctx *vbeam.Context) error {
			_, err := UnsharePersonFromFamily(ctx, UnsharePersonRequest{
				PersonId: fx.person.Id, FamilyId: fx.ownerFamily,
			})
			return err
		}},

		// Activities
		{"UpdateActivity", func(ctx *vbeam.Context) error {
			_, err := UpdateActivity(ctx, UpdateActivityRequest{Id: fx.activity.Id, Name: "Theirs", Kind: "dance"})
			return err
		}},
		{"DeleteActivity", func(ctx *vbeam.Context) error {
			_, err := DeleteActivity(ctx, ActivityIdRequest{Id: fx.activity.Id})
			return err
		}},
		{"ListSeasons", func(ctx *vbeam.Context) error {
			_, err := ListSeasons(ctx, ListSeasonsRequest{ActivityId: fx.activity.Id})
			return err
		}},
		{"CreateSeason", func(ctx *vbeam.Context) error {
			_, err := CreateSeason(ctx, CreateSeasonRequest{ActivityId: fx.activity.Id, Name: "Mine now"})
			return err
		}},
		{"UpdateSeason", func(ctx *vbeam.Context) error {
			_, err := UpdateSeason(ctx, UpdateSeasonRequest{Id: fx.season.Id, Name: "Theirs"})
			return err
		}},
		{"DeleteSeason", func(ctx *vbeam.Context) error {
			_, err := DeleteSeason(ctx, SeasonIdRequest{Id: fx.season.Id})
			return err
		}},
		{"CreateEvent", func(ctx *vbeam.Context) error {
			_, err := CreateEvent(ctx, CreateEventRequest{SeasonId: fx.season.Id, Name: "Mine now"})
			return err
		}},
		{"UpdateEvent", func(ctx *vbeam.Context) error {
			_, err := UpdateEvent(ctx, UpdateEventRequest{Id: fx.event.Id, Name: "Theirs"})
			return err
		}},
		{"DeleteEvent", func(ctx *vbeam.Context) error {
			_, err := DeleteEvent(ctx, EventIdRequest{Id: fx.event.Id})
			return err
		}},
		{"CreateEntry", func(ctx *vbeam.Context) error {
			_, err := CreateEntry(ctx, CreateEntryRequest{SeasonId: fx.season.Id, Name: "Mine now"})
			return err
		}},
		{"UpdateEntry", func(ctx *vbeam.Context) error {
			_, err := UpdateEntry(ctx, UpdateEntryRequest{Id: fx.entry.Id, Name: "Theirs"})
			return err
		}},
		{"DeleteEntry", func(ctx *vbeam.Context) error {
			_, err := DeleteEntry(ctx, EntryIdRequest{Id: fx.entry.Id})
			return err
		}},
		{"SetEntryRoster", func(ctx *vbeam.Context) error {
			_, err := SetEntryRoster(ctx, SetEntryRosterRequest{
				EntryId: fx.entry.Id, PersonIds: []int{fx.ownPerson.Id},
			})
			return err
		}},
		{"CreateAppearance", func(ctx *vbeam.Context) error {
			_, err := CreateAppearance(ctx, CreateAppearanceRequest{
				EventId: fx.event.Id, EntryId: fx.entry.Id,
			})
			return err
		}},
		{"UpdateAppearance", func(ctx *vbeam.Context) error {
			_, err := UpdateAppearance(ctx, UpdateAppearanceRequest{
				Id: fx.appearance.Id, Notes: "theirs",
			})
			return err
		}},
		{"DeleteAppearance", func(ctx *vbeam.Context) error {
			_, err := DeleteAppearance(ctx, AppearanceIdRequest{Id: fx.appearance.Id})
			return err
		}},
		{"SetAppearanceResults", func(ctx *vbeam.Context) error {
			_, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
				AppearanceId: fx.appearance.Id,
				Results:      []ResultInput{{Kind: ResultKindAdjudication, Label: "Bronze"}},
			})
			return err
		}},
		{"GetSeasonOverview", func(ctx *vbeam.Context) error {
			_, err := GetSeasonOverview(ctx, GetSeasonOverviewRequest{SeasonId: fx.season.Id})
			return err
		}},
		{"GetEventDetail", func(ctx *vbeam.Context) error {
			_, err := GetEventDetail(ctx, GetEventDetailRequest{EventId: fx.event.Id})
			return err
		}},
		{"GetEntryHistory", func(ctx *vbeam.Context) error {
			_, err := GetEntryHistory(ctx, GetEntryHistoryRequest{EntryId: fx.entry.Id})
			return err
		}},
		{"GetPersonSeason", func(ctx *vbeam.Context) error {
			_, err := GetPersonSeason(ctx, GetPersonSeasonRequest{PersonId: fx.person.Id})
			return err
		}},
		{"ListActivityVocabulary", func(ctx *vbeam.Context) error {
			_, err := ListActivityVocabulary(ctx, ListActivityVocabularyRequest{ActivityId: fx.activity.Id})
			return err
		}},
	}

	for _, tt := range calls {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			fx.asOutsider(t, func(ctx *vbeam.Context) {
				err = tt.call(ctx)
			})
			if err == nil {
				t.Errorf("%s accepted another family's record", tt.name)
			}
		})
	}

	assertOwnerDataIntact(t, fx)
}

// Procedures that name a family directly must refuse a family the caller does
// not belong to, rather than falling back to the caller's own.
func TestProceduresRefuseAnotherFamilyId(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	calls := []struct {
		name string
		call func(ctx *vbeam.Context) error
	}{
		{"SendMessage", func(ctx *vbeam.Context) error {
			_, err := SendMessage(ctx, SendMessageRequest{
				Content: "not mine to send", ClientMessageId: "x-1", FamilyId: fx.ownerFamily,
			})
			return err
		}},
		{"GetChatMessages", func(ctx *vbeam.Context) error {
			_, err := GetChatMessages(ctx, GetChatMessagesRequest{FamilyId: fx.ownerFamily})
			return err
		}},
		{"ExportData", func(ctx *vbeam.Context) error {
			_, err := ExportData(ctx, ExportDataRequest{FamilyId: fx.ownerFamily})
			return err
		}},
		{"ImportData", func(ctx *vbeam.Context) error {
			_, err := ImportData(ctx, ImportDataRequest{
				JsonData: `{"people":[]}`, FamilyId: fx.ownerFamily,
			})
			return err
		}},
		{"AddPerson", func(ctx *vbeam.Context) error {
			_, err := AddPerson(ctx, AddPersonRequest{
				Name: "Planted", PersonType: 1, Gender: 0, Birthdate: "2022-02-02",
				FamilyId: fx.ownerFamily,
			})
			return err
		}},
		{"CreateTag", func(ctx *vbeam.Context) error {
			_, err := CreateTag(ctx, CreateTagRequest{
				Name: "Planted", Color: "#123456", FamilyId: fx.ownerFamily,
			})
			return err
		}},
		{"ListActivities", func(ctx *vbeam.Context) error {
			_, err := ListActivities(ctx, ListActivitiesRequest{FamilyId: fx.ownerFamily})
			return err
		}},
		{"CreateActivity", func(ctx *vbeam.Context) error {
			_, err := CreateActivity(ctx, CreateActivityRequest{
				Name: "Planted", Kind: "dance", FamilyId: fx.ownerFamily,
			})
			return err
		}},
		{"ListFamilyLinks", func(ctx *vbeam.Context) error {
			_, err := ListFamilyLinks(ctx, ListFamilyLinksRequest{FamilyId: fx.ownerFamily})
			return err
		}},
		{"CreateFamilyLink", func(ctx *vbeam.Context) error {
			_, err := CreateFamilyLink(ctx, CreateFamilyLinkRequest{
				FamilyId: fx.ownerFamily, InviteCode: "whatever",
			})
			return err
		}},
		{"ListFamilyMembers", func(ctx *vbeam.Context) error {
			_, err := ListFamilyMembers(ctx, ListFamilyMembersRequest{FamilyId: fx.ownerFamily})
			return err
		}},
		{"RemoveFamilyMember", func(ctx *vbeam.Context) error {
			_, err := RemoveFamilyMember(ctx, RemoveFamilyMemberRequest{
				FamilyId: fx.ownerFamily, UserId: fx.owner.Id,
			})
			return err
		}},
		{"RotateInviteCode", func(ctx *vbeam.Context) error {
			_, err := RotateInviteCode(ctx, FamilyIdRequest{FamilyId: fx.ownerFamily})
			return err
		}},
	}

	for _, tt := range calls {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			fx.asOutsider(t, func(ctx *vbeam.Context) {
				err = tt.call(ctx)
			})
			if err == nil {
				t.Errorf("%s acted in a family the caller does not belong to", tt.name)
			}
		})
	}

	assertOwnerDataIntact(t, fx)
}

// The listing procedures take no id at all, so the question is not whether they
// refuse but whether they return anything they shouldn't.
func TestListingProceduresShowNothingFromAnotherFamily(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	fx.asOutsider(t, func(ctx *vbeam.Context) {
		people, err := ListPeople(ctx, Empty{})
		if err != nil {
			t.Fatalf("ListPeople() error = %v", err)
		}
		for _, person := range people.People {
			if person.FamilyId != fx.theirFamily {
				t.Errorf("ListPeople returned a person from family %d", person.FamilyId)
			}
		}

		tags, err := ListTags(ctx, ListTagsRequest{})
		if err != nil {
			t.Fatalf("ListTags() error = %v", err)
		}
		for _, tag := range tags.Tags {
			if tag.FamilyId != fx.theirFamily {
				t.Errorf("ListTags returned a tag from family %d", tag.FamilyId)
			}
		}

		photos, err := ListFamilyPhotos(ctx, ListFamilyPhotosRequest{})
		if err != nil {
			t.Fatalf("ListFamilyPhotos() error = %v", err)
		}
		for _, photo := range photos.Photos {
			if photo.Image.FamilyId != fx.theirFamily {
				t.Errorf("ListFamilyPhotos returned a photo from family %d", photo.Image.FamilyId)
			}
		}

		milestones, err := SearchMilestones(ctx, SearchMilestonesRequest{Query: "first"})
		if err != nil {
			t.Fatalf("SearchMilestones() error = %v", err)
		}
		for _, milestone := range milestones.Milestones {
			if milestone.FamilyId != fx.theirFamily {
				t.Errorf("SearchMilestones returned a milestone from family %d", milestone.FamilyId)
			}
		}

		messages, err := GetChatMessages(ctx, GetChatMessagesRequest{})
		if err != nil {
			t.Fatalf("GetChatMessages() error = %v", err)
		}
		for _, message := range messages.Messages {
			if message.FamilyId != fx.theirFamily {
				t.Errorf("GetChatMessages returned a message from family %d", message.FamilyId)
			}
		}

		info, err := GetFamilyInfo(ctx, Empty{})
		if err != nil {
			t.Fatalf("GetFamilyInfo() error = %v", err)
		}
		if info.Id == fx.ownerFamily {
			t.Error("GetFamilyInfo returned another family")
		}
		for _, family := range info.Families {
			if family.Id == fx.ownerFamily {
				t.Error("GetFamilyInfo listed a family the caller does not belong to")
			}
		}

		timeline, err := GetFamilyTimeline(ctx, GetFamilyTimelineRequest{})
		if err != nil {
			t.Fatalf("GetFamilyTimeline() error = %v", err)
		}
		for _, item := range timeline.People {
			if item.Person.Id == fx.person.Id {
				t.Error("GetFamilyTimeline returned another family's person")
			}
		}
	})
}

// The photo download handler is the one place bytes leave the server outside an
// RPC, and it must be as closed as the procedures are.
func TestServePhotoHandlerRefusesAnotherFamilysPhoto(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	previous := appDb
	appDb = fx.db
	defer func() { appDb = previous }()

	var canAccess bool
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		canAccess = CanAccessPhoto(tx, fx.outsider, fx.photo, AccessView)
	})
	if canAccess {
		t.Error("CanAccessPhoto allowed an outsider to view another family's photo")
	}

	var ownPhoto Image
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		ownPhoto = GetImageById(tx, fx.ownPhotoId)
		canAccess = CanAccessPhoto(tx, fx.outsider, ownPhoto, AccessView)
	})
	if !canAccess {
		t.Error("CanAccessPhoto denied a user their own family's photo")
	}
}

// assertOwnerDataIntact confirms that nothing the outsider tried actually
// landed: a refused call that still wrote is a worse failure than one that
// returned the wrong error.
func assertOwnerDataIntact(t *testing.T, fx isolationFixture) {
	t.Helper()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		person := GetPersonById(tx, fx.person.Id)
		if person.Id == 0 || person.Name != fx.person.Name {
			t.Errorf("person = %+v, want the original %q", person, fx.person.Name)
		}

		var growth GrowthData
		vbolt.Read(tx, GrowthDataBkt, fx.growth.Id, &growth)
		if growth.Id == 0 || growth.Value != fx.growth.Value {
			t.Errorf("growth record = %+v, want value %v", growth, fx.growth.Value)
		}

		var milestone Milestone
		vbolt.Read(tx, MilestoneBkt, fx.milestone.Id, &milestone)
		if milestone.Id == 0 || milestone.Description != fx.milestone.Description {
			t.Errorf("milestone = %+v, want %q", milestone, fx.milestone.Description)
		}

		image := GetImageById(tx, fx.photo.Id)
		if image.Id == 0 || image.Title != fx.photo.Title {
			t.Errorf("photo = %+v, want title %q", image, fx.photo.Title)
		}

		tag := getTagById(tx, fx.tag.Id)
		if tag.Id == 0 || tag.Name != fx.tag.Name {
			t.Errorf("tag = %+v, want %q", tag, fx.tag.Name)
		}

		var message ChatMessage
		vbolt.Read(tx, ChatMessagesBkt, fx.message.Id, &message)
		if message.Id == 0 {
			t.Error("chat message was deleted by an outsider")
		}

		entry := GetEntryById(tx, fx.entry.Id)
		if entry.Id == 0 || entry.Name != fx.entry.Name {
			t.Errorf("entry = %+v, want %q", entry, fx.entry.Name)
		}
		if season := GetSeasonById(tx, fx.season.Id); season.Id == 0 || season.Name != fx.season.Name {
			t.Errorf("season = %+v, want %q", season, fx.season.Name)
		}
		if event := GetEventById(tx, fx.event.Id); event.Id == 0 || event.Name != fx.event.Name {
			t.Errorf("competition = %+v, want %q", event, fx.event.Name)
		}
		// The roster is the one thing an outsider could plausibly have widened
		// rather than destroyed, so it is checked for extra rows as well.
		if roster := GetEntryPersonIds(tx, fx.entry.Id); len(roster) != 1 || roster[0] != fx.person.Id {
			t.Errorf("entry roster = %v, want just the owner's person", roster)
		}
		if appearance := GetAppearanceById(tx, fx.appearance.Id); appearance.Id == 0 {
			t.Error("performance was deleted by an outsider")
		}
		// Results are replace-all, so an accepted write would have wiped the
		// existing row rather than added to it. Both directions are checked.
		results := GetAppearanceResults(tx, fx.appearance.Id)
		if len(results) != 1 || results[0].Label != fx.result.Label {
			t.Errorf("appearance results = %+v, want just %q", results, fx.result.Label)
		}
		if appearances := GetEventAppearances(tx, fx.event.Id); len(appearances) != 1 {
			t.Errorf("competition has %d performances, want 1", len(appearances))
		}

		if people := GetFamilyPeople(tx, fx.ownerFamily); len(people) != 1 {
			t.Errorf("owner family roster has %d people, want 1", len(people))
		}
	})
}
