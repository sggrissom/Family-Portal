// Schema-level tests for competitive activities: that every record survives a
// round trip through vpack, that each write helper puts the row in every index
// the reads depend on and each delete helper takes it back out, and that a
// linked family sees a routine when and only when the link carries activities.
package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func float64Ptr(v float64) *float64 { return &v }

// roundTrip packs and unpacks one record, failing the test if vpack rejects it.
func roundTrip[T any](t *testing.T, name string, value *T, fn vpack.PackFn[T]) *T {
	t.Helper()
	data := vpack.ToBytes(value, fn)
	decoded := vpack.FromBytes(data, fn)
	if decoded == nil {
		t.Fatalf("%s: FromBytes returned nil", name)
	}
	return decoded
}

func TestActivityRecordsRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	activity := Activity{Id: 1, FamilyId: 7, Name: "Dance", Kind: ActivityKindDance, CreatedAt: now}
	if got := roundTrip(t, "Activity", &activity, PackActivity); *got != activity {
		t.Errorf("Activity round trip = %+v, want %+v", *got, activity)
	}

	season := Season{
		Id: 2, ActivityId: 1, FamilyId: 7, Name: "2025-26 Competition Season",
		StartDate: now, EndDate: now.AddDate(0, 9, 0), Notes: "first year at elite", CreatedAt: now,
	}
	if got := roundTrip(t, "Season", &season, PackSeason); *got != season {
		t.Errorf("Season round trip = %+v, want %+v", *got, season)
	}

	event := Event{
		Id: 3, SeasonId: 2, FamilyId: 7, Name: "Nuvo Nashville", Host: "Nuvo",
		Location: "Nashville, TN", StartDate: now, EndDate: now.AddDate(0, 0, 2),
		Notes: "two-day", CreatedAt: now,
	}
	if got := roundTrip(t, "Event", &event, PackEvent); *got != event {
		t.Errorf("Event round trip = %+v, want %+v", *got, event)
	}

	entry := Entry{
		Id: 4, SeasonId: 2, FamilyId: 7, Name: "Rise Up", Format: "group",
		Style: "Lyrical", Division: "Teen", Level: "Elite", Notes: "", CreatedAt: now,
	}
	if got := roundTrip(t, "Entry", &entry, PackEntry); *got != entry {
		t.Errorf("Entry round trip = %+v, want %+v", *got, entry)
	}

	member := EntryMember{Id: 5, EntryId: 4, PersonId: 11, FamilyId: 7, CreatedAt: now}
	if got := roundTrip(t, "EntryMember", &member, PackEntryMember); *got != member {
		t.Errorf("EntryMember round trip = %+v, want %+v", *got, member)
	}

	appearance := Appearance{Id: 6, EventId: 3, EntryId: 4, FamilyId: 7, OccurredAt: now, Notes: "session 4", CreatedAt: now}
	if got := roundTrip(t, "Appearance", &appearance, PackAppearance); *got != appearance {
		t.Errorf("Appearance round trip = %+v, want %+v", *got, appearance)
	}

	appearancePhoto := AppearancePhoto{Id: 7, AppearanceId: 6, PhotoId: 90, FamilyId: 7, CreatedAt: now}
	if got := roundTrip(t, "AppearancePhoto", &appearancePhoto, PackAppearancePhoto); *got != appearancePhoto {
		t.Errorf("AppearancePhoto round trip = %+v, want %+v", *got, appearancePhoto)
	}

	eventPhoto := EventPhoto{Id: 8, EventId: 3, PhotoId: 91, FamilyId: 7, CreatedAt: now}
	if got := roundTrip(t, "EventPhoto", &eventPhoto, PackEventPhoto); *got != eventPhoto {
		t.Errorf("EventPhoto round trip = %+v, want %+v", *got, eventPhoto)
	}
}

// The optional fields are the whole reason Result needs its own test: a zero
// Rank means "did not place" only if it is distinguishable from a nil one.
func TestResultRoundTripPreservesOptionalFields(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	filled := Result{
		Id: 1, AppearanceId: 6, FamilyId: 7, Kind: ResultKindPlacement,
		Label: "Overall", Rank: intPtr(1), OutOf: intPtr(14),
		Category: "Teen Small Group Jazz", Score: float64Ptr(287.5),
		PersonId: intPtr(11), Notes: "judges' pick", SortOrder: 2, CreatedAt: now,
	}
	got := roundTrip(t, "Result", &filled, PackResult)
	for _, check := range []struct {
		name      string
		got, want *int
	}{
		{"Rank", got.Rank, filled.Rank},
		{"OutOf", got.OutOf, filled.OutOf},
		{"PersonId", got.PersonId, filled.PersonId},
	} {
		if check.got == nil || *check.got != *check.want {
			t.Errorf("Result.%s = %v, want %d", check.name, check.got, *check.want)
		}
	}
	if got.Score == nil || *got.Score != *filled.Score {
		t.Errorf("Result.Score = %v, want %v", got.Score, *filled.Score)
	}
	if got.Kind != filled.Kind || got.Label != filled.Label || got.SortOrder != filled.SortOrder {
		t.Errorf("Result scalars round tripped wrong: %+v", *got)
	}

	// A zero-valued optional is not the same as an absent one.
	zeroed := filled
	zeroed.Rank = intPtr(0)
	zeroed.Score = float64Ptr(0)
	got = roundTrip(t, "Result(zeroed)", &zeroed, PackResult)
	if got.Rank == nil || *got.Rank != 0 {
		t.Errorf("a zero Rank came back as %v, want a pointer to 0", got.Rank)
	}
	if got.Score == nil || *got.Score != 0 {
		t.Errorf("a zero Score came back as %v, want a pointer to 0", got.Score)
	}

	empty := Result{
		Id: 2, AppearanceId: 6, FamilyId: 7, Kind: ResultKindAdjudication,
		Label: "High Gold", CreatedAt: now,
	}
	got = roundTrip(t, "Result(empty)", &empty, PackResult)
	if got.Rank != nil || got.OutOf != nil || got.Score != nil || got.PersonId != nil {
		t.Errorf("absent optionals came back set: %+v", *got)
	}
}

// activityFixture hangs a season's worth of records off the three-household
// link fixture: a group routine alice is in, and a solo bob has to himself.
// Alice is the child shared into family B, so the group routine is the one that
// a link carrying activities should reach and the solo is the one it must not.
type activityFixture struct {
	familyLinkFixture

	activity   Activity
	season     Season
	event      Event
	groupEntry Entry // alice + bob
	soloEntry  Entry // bob only
	groupAppr  Appearance
	soloAppr   Appearance
	adjudicate Result // on the group appearance, names nobody
	aliceAward Result // on the group appearance, names alice
}

func setupActivityFixture(t *testing.T) (activityFixture, func()) {
	t.Helper()

	base, cleanup := setupFamilyLinkFixture(t)
	fx := activityFixture{familyLinkFixture: base}
	now := time.Now()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fx.activity = Activity{
			Id: vbolt.NextIntId(tx, ActivityBkt), FamilyId: fx.famA,
			Name: "Dance", Kind: ActivityKindDance, CreatedAt: now,
		}
		writeActivityTx(tx, &fx.activity)

		fx.season = Season{
			Id: vbolt.NextIntId(tx, SeasonBkt), ActivityId: fx.activity.Id, FamilyId: fx.famA,
			Name: "2025-26", StartDate: now, EndDate: now.AddDate(0, 9, 0), CreatedAt: now,
		}
		writeSeasonTx(tx, &fx.season)

		fx.event = Event{
			Id: vbolt.NextIntId(tx, EventBkt), SeasonId: fx.season.Id, FamilyId: fx.famA,
			Name: "Nuvo Nashville", Host: "Nuvo", StartDate: now, CreatedAt: now,
		}
		writeEventTx(tx, &fx.event)

		fx.groupEntry = Entry{
			Id: vbolt.NextIntId(tx, EntryBkt), SeasonId: fx.season.Id, FamilyId: fx.famA,
			Name: "Rise Up", Format: "group", Style: "Lyrical", Division: "Teen", CreatedAt: now,
		}
		writeEntryTx(tx, &fx.groupEntry)

		fx.soloEntry = Entry{
			Id: vbolt.NextIntId(tx, EntryBkt), SeasonId: fx.season.Id, FamilyId: fx.famA,
			Name: "On My Own", Format: "solo", Style: "Jazz", Division: "Mini", CreatedAt: now,
		}
		writeEntryTx(tx, &fx.soloEntry)

		for _, seed := range []struct {
			entryId  int
			personId int
		}{
			{fx.groupEntry.Id, fx.alice.Id},
			{fx.groupEntry.Id, fx.bob.Id},
			{fx.soloEntry.Id, fx.bob.Id},
		} {
			member := EntryMember{
				Id: vbolt.NextIntId(tx, EntryMemberBkt), EntryId: seed.entryId,
				PersonId: seed.personId, FamilyId: fx.famA, CreatedAt: now,
			}
			writeEntryMemberTx(tx, &member)
		}

		fx.groupAppr = Appearance{
			Id: vbolt.NextIntId(tx, AppearanceBkt), EventId: fx.event.Id,
			EntryId: fx.groupEntry.Id, FamilyId: fx.famA, OccurredAt: now, CreatedAt: now,
		}
		writeAppearanceTx(tx, &fx.groupAppr)

		fx.soloAppr = Appearance{
			Id: vbolt.NextIntId(tx, AppearanceBkt), EventId: fx.event.Id,
			EntryId: fx.soloEntry.Id, FamilyId: fx.famA, OccurredAt: now, CreatedAt: now,
		}
		writeAppearanceTx(tx, &fx.soloAppr)

		fx.adjudicate = Result{
			Id: vbolt.NextIntId(tx, ResultBkt), AppearanceId: fx.groupAppr.Id, FamilyId: fx.famA,
			Kind: ResultKindAdjudication, Label: "High Gold", CreatedAt: now,
		}
		writeResultTx(tx, &fx.adjudicate)

		fx.aliceAward = Result{
			Id: vbolt.NextIntId(tx, ResultBkt), AppearanceId: fx.groupAppr.Id, FamilyId: fx.famA,
			Kind: ResultKindAward, Label: "Judges' Choice", PersonId: intPtr(fx.alice.Id),
			SortOrder: 1, CreatedAt: now,
		}
		writeResultTx(tx, &fx.aliceAward)

		join := AppearancePhoto{
			Id: vbolt.NextIntId(tx, AppearancePhotoBkt), AppearanceId: fx.groupAppr.Id,
			PhotoId: fx.alicePhoto.Id, FamilyId: fx.famA, CreatedAt: now,
		}
		writeAppearancePhotoTx(tx, &join)

		eventJoin := EventPhoto{
			Id: vbolt.NextIntId(tx, EventPhotoBkt), EventId: fx.event.Id,
			PhotoId: fx.untaggedPhoto.Id, FamilyId: fx.famA, CreatedAt: now,
		}
		writeEventPhotoTx(tx, &eventJoin)

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

// Every read the feature depends on is an index walk, so every index has to be
// written by the helper that writes the row.
func TestActivityWritesPopulateEveryIndex(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilyActivities(tx, fx.famA)); got != 1 {
			t.Errorf("GetFamilyActivities = %d, want 1", got)
		}
		if got := len(GetActivitySeasons(tx, fx.activity.Id)); got != 1 {
			t.Errorf("GetActivitySeasons = %d, want 1", got)
		}
		if got := len(GetFamilySeasons(tx, fx.famA)); got != 1 {
			t.Errorf("GetFamilySeasons = %d, want 1", got)
		}
		if got := len(GetSeasonEvents(tx, fx.season.Id)); got != 1 {
			t.Errorf("GetSeasonEvents = %d, want 1", got)
		}
		if got := len(GetSeasonEntries(tx, fx.season.Id)); got != 2 {
			t.Errorf("GetSeasonEntries = %d, want 2", got)
		}
		if got := len(GetEntryMembers(tx, fx.groupEntry.Id)); got != 2 {
			t.Errorf("GetEntryMembers(group) = %d, want 2", got)
		}

		// "Which routines is this kid in?" without a scan.
		if got := len(GetPersonEntryMembers(tx, fx.bob.Id)); got != 2 {
			t.Errorf("GetPersonEntryMembers(bob) = %d, want 2", got)
		}
		if got := len(GetPersonEntryMembers(tx, fx.alice.Id)); got != 1 {
			t.Errorf("GetPersonEntryMembers(alice) = %d, want 1", got)
		}

		// The two views the whole schema exists to serve.
		if got := len(GetEventAppearances(tx, fx.event.Id)); got != 2 {
			t.Errorf("GetEventAppearances = %d, want 2", got)
		}
		if got := len(GetEntryAppearances(tx, fx.groupEntry.Id)); got != 1 {
			t.Errorf("GetEntryAppearances = %d, want 1", got)
		}

		if got := len(GetAppearanceResults(tx, fx.groupAppr.Id)); got != 2 {
			t.Errorf("GetAppearanceResults = %d, want 2", got)
		}
		if got := len(GetAppearanceResults(tx, fx.soloAppr.Id)); got != 0 {
			t.Errorf("GetAppearanceResults(solo) = %d, want 0", got)
		}
		if got := len(GetAppearancePhotoJoins(tx, fx.groupAppr.Id)); got != 1 {
			t.Errorf("GetAppearancePhotoJoins = %d, want 1", got)
		}
		if got := len(GetEventPhotoJoins(tx, fx.event.Id)); got != 1 {
			t.Errorf("GetEventPhotoJoins = %d, want 1", got)
		}
	})
}

// ResultByPersonIndex is written only for results that name a person, and stops
// being written the moment one stops naming one.
func TestResultPersonIndexFollowsThePointer(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		results := GetPersonResults(tx, fx.alice.Id)
		if len(results) != 1 || results[0].Id != fx.aliceAward.Id {
			t.Fatalf("GetPersonResults(alice) = %+v, want just the award", results)
		}
		if got := len(GetPersonResults(tx, fx.bob.Id)); got != 0 {
			t.Errorf("GetPersonResults(bob) = %d, want 0", got)
		}
	})

	// Clearing PersonId — what person deletion does, rather than deleting the
	// result, because the routine still placed.
	cleared := fx.aliceAward
	cleared.PersonId = nil
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		writeResultTx(tx, &cleared)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetPersonResults(tx, fx.alice.Id)); got != 0 {
			t.Errorf("GetPersonResults(alice) = %d after clearing PersonId, want 0", got)
		}
		if got := len(GetAppearanceResults(tx, fx.groupAppr.Id)); got != 2 {
			t.Errorf("the result itself should survive; GetAppearanceResults = %d, want 2", got)
		}
	})
}

// A bucket account deletion does not know about is a data-retention bug, so the
// sweep has to leave all nine empty — indexes included, which is what the
// re-reads below actually check.
func TestDeleteFamilyActivitiesLeavesNoOrphans(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteFamilyActivitiesTx(tx, fx.famA)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		checks := []struct {
			name string
			got  int
		}{
			{"activities", len(GetFamilyActivities(tx, fx.famA))},
			{"seasons by family", len(GetFamilySeasons(tx, fx.famA))},
			{"seasons by activity", len(GetActivitySeasons(tx, fx.activity.Id))},
			{"events by family", len(GetFamilyEvents(tx, fx.famA))},
			{"events by season", len(GetSeasonEvents(tx, fx.season.Id))},
			{"entries by family", len(GetFamilyEntries(tx, fx.famA))},
			{"entries by season", len(GetSeasonEntries(tx, fx.season.Id))},
			{"members by family", len(GetFamilyEntryMembers(tx, fx.famA))},
			{"members by entry", len(GetEntryMembers(tx, fx.groupEntry.Id))},
			{"members by person", len(GetPersonEntryMembers(tx, fx.bob.Id))},
			{"appearances by family", len(GetFamilyAppearances(tx, fx.famA))},
			{"appearances by event", len(GetEventAppearances(tx, fx.event.Id))},
			{"appearances by entry", len(GetEntryAppearances(tx, fx.groupEntry.Id))},
			{"results by family", len(GetFamilyResults(tx, fx.famA))},
			{"results by appearance", len(GetAppearanceResults(tx, fx.groupAppr.Id))},
			{"results by person", len(GetPersonResults(tx, fx.alice.Id))},
			{"appearance photos by family", len(GetFamilyAppearancePhotos(tx, fx.famA))},
			{"appearance photos by appearance", len(GetAppearancePhotoJoins(tx, fx.groupAppr.Id))},
			{"event photos by family", len(GetFamilyEventPhotos(tx, fx.famA))},
			{"event photos by event", len(GetEventPhotoJoins(tx, fx.event.Id))},
		}
		for _, check := range checks {
			if check.got != 0 {
				t.Errorf("%s still has %d rows after the family sweep", check.name, check.got)
			}
		}
		if GetEntryById(tx, fx.groupEntry.Id).Id != 0 {
			t.Error("the entry record itself survived the sweep")
		}
		if GetResultById(tx, fx.adjudicate.Id).Id != 0 {
			t.Error("the result record itself survived the sweep")
		}
	})
}

// A person deleted in their own household can still be rostered in another
// household's group routine, shared in by a link. Sweeping the deleted person's
// family leaves those rows pointing at somebody who no longer exists.
//
// The roster row goes; the result does not. A routine that placed second still
// placed second after one of its dancers is deleted.
func TestDeletingAPersonClearsTheirActivityJoinsEverywhere(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	// Alice is on the group routine and is the one the judges' award names.
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		removePersonFromActivitiesTx(tx, fx.alice.Id)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetPersonEntryMembers(tx, fx.alice.Id)); got != 0 {
			t.Errorf("alice is still on %d rosters", got)
		}
		if roster := GetEntryPersonIds(tx, fx.groupEntry.Id); len(roster) != 1 || roster[0] != fx.bob.Id {
			t.Errorf("group roster = %v, want just bob", roster)
		}
		if got := len(GetPersonResults(tx, fx.alice.Id)); got != 0 {
			t.Errorf("%d results still name alice", got)
		}

		// Both results survive — including the one that used to name her.
		results := GetAppearanceResults(tx, fx.groupAppr.Id)
		if len(results) != 2 {
			t.Fatalf("got %d results, want both to survive", len(results))
		}
		for _, result := range results {
			if result.PersonId != nil {
				t.Errorf("result %d still points at person %d", result.Id, *result.PersonId)
			}
			if result.Label == "" {
				t.Errorf("result %d lost its label", result.Id)
			}
		}

		// Bob was not deleted and keeps everything.
		if got := len(GetPersonEntryMembers(tx, fx.bob.Id)); got != 2 {
			t.Errorf("bob is on %d rosters, want 2", got)
		}
	})
}

// Activities is not in DefaultLinkScopes and is the highest bit, so no link that
// predates the feature can read back as sharing it.
func TestActivitiesScopeIsOptInAndAdditive(t *testing.T) {
	if DefaultLinkScopes().Activities {
		t.Error("new links share activities by default")
	}

	// Bits 0-4 are what existing masks hold. Every one of them must decode with
	// Activities off.
	legacyMask := ScopePeople.bit() | ScopeMilestones.bit() | ScopePhotos.bit() | ScopeGrowth.bit()
	if linkScopesFromMask(legacyMask).Activities {
		t.Error("a mask written before the feature existed decodes as sharing activities")
	}

	scopes := LinkScopes{People: true, Milestones: true, Photos: true, Growth: true, Activities: true}
	if got := linkScopesFromMask(scopes.ToMask()); got != scopes {
		t.Errorf("mask round trip = %+v, want %+v", got, scopes)
	}

	// Activity reads resolve through a rostered person, so activities without
	// people is as incoherent as milestones without people.
	if !normalizeLinkScopes(LinkScopes{Activities: true}).People {
		t.Error("activities did not imply people")
	}
}

// The routine is the shared object. A link that shares one child reaches the
// group routines that child is in — and nothing she is not in.
func TestCanAccessEntryFollowsTheRoster(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	// Default scopes carry no activities, so nothing is reachable yet.
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if canAccessEntry(tx, fx.userB, fx.groupEntry, AccessView) {
			t.Error("a routine was readable through a link that does not share activities")
		}
		if canAccessAppearance(tx, fx.userB, fx.groupAppr, AccessView) {
			t.Error("an appearance was readable through a link that does not share activities")
		}
		if canAccessResult(tx, fx.userB, fx.adjudicate, AccessView) {
			t.Error("a result was readable through a link that does not share activities")
		}
		// The owning family is unaffected by any of this.
		if !canAccessEntry(tx, fx.userA, fx.soloEntry, AccessContribute) {
			t.Error("the owning family lost access to its own routine")
		}
	})

	setLinkScopes(t, fx.familyLinkFixture, fx.linkAB, LinkScopes{People: true, Activities: true})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !canAccessEntry(tx, fx.userB, fx.groupEntry, AccessView) {
			t.Error("the routine the shared child is in was denied after granting activities")
		}
		if !canAccessAppearance(tx, fx.userB, fx.groupAppr, AccessView) {
			t.Error("that routine's appearance was denied")
		}
		if !canAccessResult(tx, fx.userB, fx.adjudicate, AccessView) {
			t.Error("that appearance's result was denied")
		}

		// Bob was never shared, and the solo has only him on its roster.
		if canAccessEntry(tx, fx.userB, fx.soloEntry, AccessView) {
			t.Error("a routine with nobody shared in it leaked through the link")
		}
		if canAccessAppearance(tx, fx.userB, fx.soloAppr, AccessView) {
			t.Error("the unshared routine's appearance leaked through the link")
		}

		// A link is read-only, and it is not membership.
		if canAccessEntry(tx, fx.userB, fx.groupEntry, AccessContribute) {
			t.Error("a link granted writes on a shared routine")
		}
		if canAccessSeason(tx, fx.userB, fx.season, AccessView) {
			t.Error("a link reached the season, which has no person dimension")
		}
		if canAccessEvent(tx, fx.userB, fx.event, AccessView) {
			t.Error("a link reached the competition, which has no person dimension")
		}
		if canAccessActivity(tx, fx.userB, fx.activity, AccessView) {
			t.Error("a link reached the activity, which has no person dimension")
		}

		// A -> B and B -> C still do not add up to A -> C.
		if canAccessEntry(tx, fx.userC, fx.groupEntry, AccessView) {
			t.Error("C reached A's routine through B")
		}
	})
}

// An entry nobody is rostered on has no person to resolve through, so it stays
// with its own family rather than defaulting open.
func TestEntryWithEmptyRosterStaysPrivate(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	setLinkScopes(t, fx.familyLinkFixture, fx.linkAB, LinkScopes{People: true, Activities: true})

	var orphan Entry
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		orphan = Entry{
			Id: vbolt.NextIntId(tx, EntryBkt), SeasonId: fx.season.Id, FamilyId: fx.famA,
			Name: "Untitled", Format: "group", CreatedAt: time.Now(),
		}
		writeEntryTx(tx, &orphan)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !canAccessEntry(tx, fx.userA, orphan, AccessView) {
			t.Error("the owning family cannot see its own rosterless routine")
		}
		if canAccessEntry(tx, fx.userB, orphan, AccessView) {
			t.Error("a rosterless routine was visible to a linked family")
		}
	})
}
