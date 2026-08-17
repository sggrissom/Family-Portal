// Proc-level tests for appearances and results.
//
// The schema tests in activity_test.go prove the buckets and indexes behave;
// these prove the procedures on top of them do — that an appearance cannot be
// hung between two seasons, that a bad row in a results sheet leaves the old set
// alone, and that replace-all actually replaces rather than accumulates.
//
// Cross-family rejection for the same procs lives in cross_family_isolation_test.go.
package backend

import (
	"family/cfg"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// resultsFixture is one family with a season, one competition, and a group
// routine with two kids on it — plus a second season, which is what makes the
// "entry from another season" case reachable through the procs.
type resultsFixture struct {
	db *vbolt.DB

	owner    User
	familyId int
	alice    Person
	bob      Person
	carol    Person // in the family but not on the routine

	activity    Activity
	season      Season
	otherSeason Season
	event       Event
	entry       Entry
	otherEntry  Entry // belongs to otherSeason
}

func setupResultsFixture(t *testing.T) resultsFixture {
	t.Helper()

	db := vbolt.Open(t.TempDir() + "/activity_results.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })
	appDb = db
	jwtKey = []byte("activity-results-test-secret-key-at-least-32")

	fx := resultsFixture{db: db}
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@example.com"}, hash)
		fx.familyId = fx.owner.FamilyId

		var err error
		for _, seed := range []struct {
			name string
			into *Person
		}{{"Alice", &fx.alice}, {"Bob", &fx.bob}, {"Carol", &fx.carol}} {
			*seed.into, err = AddPersonTx(tx, AddPersonRequest{
				Name: seed.name, PersonType: 1, Gender: 0, Birthdate: "2014-03-02",
			}, fx.familyId)
			if err != nil {
				t.Fatalf("AddPersonTx(%s) error = %v", seed.name, err)
			}
		}
		vbolt.TxCommit(tx)
	})

	// Each proc commits its own transaction, so each one gets its own `as`.
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateActivity(ctx, CreateActivityRequest{Name: "Dance", Kind: ActivityKindDance})
		if err != nil {
			t.Fatalf("CreateActivity() error = %v", err)
		}
		fx.activity = resp.Activity
	})
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateSeason(ctx, CreateSeasonRequest{ActivityId: fx.activity.Id, Name: "2025-26"})
		if err != nil {
			t.Fatalf("CreateSeason() error = %v", err)
		}
		fx.season = resp.Season
	})
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateSeason(ctx, CreateSeasonRequest{ActivityId: fx.activity.Id, Name: "2024-25"})
		if err != nil {
			t.Fatalf("CreateSeason(other) error = %v", err)
		}
		fx.otherSeason = resp.Season
	})
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateEvent(ctx, CreateEventRequest{
			SeasonId: fx.season.Id, Name: "Nuvo Nashville", Host: "Nuvo",
		})
		if err != nil {
			t.Fatalf("CreateEvent() error = %v", err)
		}
		fx.event = resp.Event
	})
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateEntry(ctx, CreateEntryRequest{
			SeasonId: fx.season.Id, Name: "Rise Up", Format: "group",
			PersonIds: []int{fx.alice.Id, fx.bob.Id},
		})
		if err != nil {
			t.Fatalf("CreateEntry() error = %v", err)
		}
		fx.entry = resp.Entry.Entry
	})
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateEntry(ctx, CreateEntryRequest{
			SeasonId: fx.otherSeason.Id, Name: "Last Year", Format: "group",
			PersonIds: []int{fx.alice.Id},
		})
		if err != nil {
			t.Fatalf("CreateEntry(other season) error = %v", err)
		}
		fx.otherEntry = resp.Entry.Entry
	})

	return fx
}

// as runs fn in a write transaction with the owner authenticated. Procedures
// commit their own transaction, so fn calls exactly one of them.
func (fx resultsFixture) as(t *testing.T, fn func(ctx *vbeam.Context)) {
	t.Helper()

	token, err := generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fn(&vbeam.Context{Tx: tx, Token: token})
	})
}

// newAppearance is the setup line most of the tests below start with.
func (fx resultsFixture) newAppearance(t *testing.T) Appearance {
	t.Helper()

	var appearance Appearance
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateAppearance(ctx, CreateAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.entry.Id,
		})
		if err != nil {
			t.Fatalf("CreateAppearance() error = %v", err)
		}
		appearance = resp.Appearance.Appearance
	})
	return appearance
}

func (fx resultsFixture) resultsOf(t *testing.T, appearanceId int) []Result {
	t.Helper()

	var results []Result
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		results = sortResults(GetAppearanceResults(tx, appearanceId))
	})
	return results
}

// An appearance is one entry at one event, and both parents have to agree on
// which season that is — otherwise it is a row neither view can explain.
func TestCreateAppearanceRequiresBothParentsInOneSeason(t *testing.T) {
	fx := setupResultsFixture(t)

	appearance := fx.newAppearance(t)
	if appearance.Id == 0 || appearance.EventId != fx.event.Id || appearance.EntryId != fx.entry.Id {
		t.Fatalf("CreateAppearance() = %+v, want it hung off the event and the entry", appearance)
	}
	if appearance.FamilyId != fx.familyId {
		t.Errorf("appearance family = %d, want %d", appearance.FamilyId, fx.familyId)
	}

	fx.as(t, func(ctx *vbeam.Context) {
		_, err := CreateAppearance(ctx, CreateAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.otherEntry.Id,
		})
		if err != ErrEntryNotInSeason {
			t.Errorf("CreateAppearance(entry from another season) error = %v, want %v", err, ErrEntryNotInSeason)
		}
	})

	// A routine that dances its category and again in the overall round is two
	// performances, not one, so the second is allowed.
	second := fx.newAppearance(t)
	if second.Id == appearance.Id {
		t.Error("a second appearance of the same entry at the same event reused the first")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetEventAppearances(tx, fx.event.Id)); got != 2 {
			t.Errorf("GetEventAppearances = %d, want 2", got)
		}
	})
}

func TestUpdateAppearanceEditsOnlyItsOwnFields(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	occurredAt := "2026-03-14"
	var updated Appearance
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := UpdateAppearance(ctx, UpdateAppearanceRequest{
			Id: appearance.Id, OccurredAt: &occurredAt, Notes: "session 4, 8:40am",
		})
		if err != nil {
			t.Fatalf("UpdateAppearance() error = %v", err)
		}
		updated = resp.Appearance.Appearance
	})

	if updated.Notes != "session 4, 8:40am" {
		t.Errorf("notes = %q, want the new value", updated.Notes)
	}
	if got := updated.OccurredAt.Format("2006-01-02"); got != occurredAt {
		t.Errorf("occurredAt = %q, want %q", got, occurredAt)
	}
	if updated.EventId != appearance.EventId || updated.EntryId != appearance.EntryId {
		t.Errorf("update moved the appearance: %+v", updated)
	}
}

// Replace-all has to actually replace. A second call with a shorter set must
// leave the shorter set, not the union of the two.
func TestSetAppearanceResultsReplacesTheWholeSet(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	fx.as(t, func(ctx *vbeam.Context) {
		_, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results: []ResultInput{
				{Kind: ResultKindAdjudication, Label: "High Gold"},
				{Kind: ResultKindPlacement, Label: "Overall", Rank: intPtr(2), OutOf: intPtr(14),
					Category: "Teen Small Group Jazz"},
				{Kind: ResultKindAward, Label: "Judges' Choice", PersonId: intPtr(fx.alice.Id)},
			},
		})
		if err != nil {
			t.Fatalf("SetAppearanceResults() error = %v", err)
		}
	})

	results := fx.resultsOf(t, appearance.Id)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	// SortOrder is the array position, so the set comes back in the order it
	// was sent rather than in id order by accident.
	for i, want := range []string{"High Gold", "Overall", "Judges' Choice"} {
		if results[i].Label != want {
			t.Errorf("result %d label = %q, want %q", i, results[i].Label, want)
		}
		if results[i].SortOrder != i {
			t.Errorf("result %d sortOrder = %d, want %d", i, results[i].SortOrder, i)
		}
	}
	if results[1].Rank == nil || *results[1].Rank != 2 || results[1].OutOf == nil || *results[1].OutOf != 14 {
		t.Errorf("placement lost its rank: %+v", results[1])
	}
	if results[2].PersonId == nil || *results[2].PersonId != fx.alice.Id {
		t.Errorf("award lost the dancer it names: %+v", results[2])
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetPersonResults(tx, fx.alice.Id)); got != 1 {
			t.Errorf("GetPersonResults(alice) = %d, want 1", got)
		}
		if got := len(GetPersonResults(tx, fx.bob.Id)); got != 0 {
			t.Errorf("GetPersonResults(bob) = %d, want 0", got)
		}
	})

	fx.as(t, func(ctx *vbeam.Context) {
		_, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results:      []ResultInput{{Kind: ResultKindAdjudication, Label: "Platinum"}},
		})
		if err != nil {
			t.Fatalf("SetAppearanceResults(second) error = %v", err)
		}
	})

	results = fx.resultsOf(t, appearance.Id)
	if len(results) != 1 || results[0].Label != "Platinum" {
		t.Fatalf("after replacing, results = %+v, want just Platinum", results)
	}

	// The dropped rows have to leave every index they were in, not just the
	// by-appearance one the read above walks.
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetPersonResults(tx, fx.alice.Id)); got != 0 {
			t.Errorf("GetPersonResults(alice) = %d after the award was dropped, want 0", got)
		}
		if got := len(GetFamilyResults(tx, fx.familyId)); got != 1 {
			t.Errorf("GetFamilyResults = %d, want 1", got)
		}
	})

	// An empty set is how a mis-entered sheet is cleared, so it is a normal
	// call rather than an error.
	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id, Results: []ResultInput{},
		}); err != nil {
			t.Fatalf("SetAppearanceResults(empty) error = %v", err)
		}
	})
	if got := len(fx.resultsOf(t, appearance.Id)); got != 0 {
		t.Errorf("after clearing, got %d results, want 0", got)
	}
}

// Each kind carries the field it exists for. Everything else stays free text.
func TestSetAppearanceResultsValidatesEachKind(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	cases := []struct {
		name  string
		input ResultInput
		want  error
	}{
		{"unknown kind", ResultInput{Kind: "vibes", Label: "Great"}, ErrInvalidResultKind},
		{"adjudication without a label", ResultInput{Kind: ResultKindAdjudication}, ErrResultLabelRequired},
		{"award without a label", ResultInput{Kind: ResultKindAward}, ErrResultLabelRequired},
		{"placement without a rank", ResultInput{Kind: ResultKindPlacement, Label: "Overall"}, ErrResultRankRequired},
		{"score without a score", ResultInput{Kind: ResultKindScore, Label: "Total"}, ErrResultScoreRequired},
		{"rank below one", ResultInput{Kind: ResultKindPlacement, Rank: intPtr(0)}, ErrResultRankOutOfRange},
		{"rank past the field size", ResultInput{Kind: ResultKindPlacement, Rank: intPtr(9), OutOf: intPtr(4)}, ErrResultRankOutOfRange},
		{"a stranger's award", ResultInput{
			Kind: ResultKindAward, Label: "Judges' Choice", PersonId: intPtr(fx.carol.Id),
		}, ErrResultPersonNotOnEntry},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fx.as(t, func(ctx *vbeam.Context) {
				_, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
					AppearanceId: appearance.Id, Results: []ResultInput{tt.input},
				})
				if err != tt.want {
					t.Errorf("error = %v, want %v", err, tt.want)
				}
			})
		})
	}

	// A score of zero and a rank of one are both real values, and a person id of
	// zero means "names nobody" rather than a rejected write.
	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results: []ResultInput{
				{Kind: ResultKindScore, Label: "Total", Score: float64Ptr(0)},
				{Kind: ResultKindPlacement, Rank: intPtr(1), OutOf: intPtr(1)},
				{Kind: ResultKindAdjudication, Label: "Gold", PersonId: intPtr(0)},
			},
		}); err != nil {
			t.Fatalf("SetAppearanceResults(valid edge cases) error = %v", err)
		}
	})
	results := fx.resultsOf(t, appearance.Id)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if results[0].Score == nil || *results[0].Score != 0 {
		t.Errorf("a zero score came back as %v", results[0].Score)
	}
	if results[2].PersonId != nil {
		t.Errorf("a person id of 0 was stored as %v, want nil", results[2].PersonId)
	}
}

// Validation runs over the whole sheet before anything is written, so a bad row
// partway down leaves the appearance holding what it held before.
func TestSetAppearanceResultsRejectsTheWholeSheetAtomically(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results:      []ResultInput{{Kind: ResultKindAdjudication, Label: "High Gold"}},
		}); err != nil {
			t.Fatalf("SetAppearanceResults() error = %v", err)
		}
	})

	fx.as(t, func(ctx *vbeam.Context) {
		_, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results: []ResultInput{
				{Kind: ResultKindAdjudication, Label: "Platinum"},
				{Kind: ResultKindPlacement, Label: "Overall"}, // no rank
			},
		})
		if err != ErrResultRankRequired {
			t.Fatalf("error = %v, want %v", err, ErrResultRankRequired)
		}
	})

	results := fx.resultsOf(t, appearance.Id)
	if len(results) != 1 || results[0].Label != "High Gold" {
		t.Errorf("a rejected sheet changed the stored set: %+v", results)
	}
}

func TestSetAppearanceResultsCapsTheSheetSize(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	oversized := make([]ResultInput, maxResultsPerAppearance+1)
	for i := range oversized {
		oversized[i] = ResultInput{Kind: ResultKindAward, Label: "Special"}
	}
	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id, Results: oversized,
		}); err != ErrTooManyResults {
			t.Errorf("error = %v, want %v", err, ErrTooManyResults)
		}
	})
	if got := len(fx.resultsOf(t, appearance.Id)); got != 0 {
		t.Errorf("an oversized sheet wrote %d results", got)
	}
}

// Deleting an appearance takes its results with it. A result nothing can reach
// is worse than a deleted one.
func TestDeleteAppearanceTakesItsResults(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results: []ResultInput{
				{Kind: ResultKindAdjudication, Label: "High Gold"},
				{Kind: ResultKindAward, Label: "Judges' Choice", PersonId: intPtr(fx.bob.Id)},
			},
		}); err != nil {
			t.Fatalf("SetAppearanceResults() error = %v", err)
		}
	})

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := DeleteAppearance(ctx, AppearanceIdRequest{Id: appearance.Id}); err != nil {
			t.Fatalf("DeleteAppearance() error = %v", err)
		}
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetAppearanceById(tx, appearance.Id).Id != 0 {
			t.Error("the appearance survived its own deletion")
		}
		for _, check := range []struct {
			name string
			got  int
		}{
			{"results by appearance", len(GetAppearanceResults(tx, appearance.Id))},
			{"results by family", len(GetFamilyResults(tx, fx.familyId))},
			{"results by person", len(GetPersonResults(tx, fx.bob.Id))},
			{"appearances by event", len(GetEventAppearances(tx, fx.event.Id))},
			{"appearances by entry", len(GetEntryAppearances(tx, fx.entry.Id))},
		} {
			if check.got != 0 {
				t.Errorf("%s still has %d rows", check.name, check.got)
			}
		}
	})
}

// Deleting the routine has to reach the results two levels down, which is the
// case an entity-at-a-time cascade is easiest to get wrong.
func TestDeleteEntryReachesResultsThroughAppearances(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearance.Id,
			Results:      []ResultInput{{Kind: ResultKindAdjudication, Label: "High Gold"}},
		}); err != nil {
			t.Fatalf("SetAppearanceResults() error = %v", err)
		}
	})

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := DeleteEntry(ctx, EntryIdRequest{Id: fx.entry.Id}); err != nil {
			t.Fatalf("DeleteEntry() error = %v", err)
		}
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilyResults(tx, fx.familyId)); got != 0 {
			t.Errorf("%d results outlived the routine they belong to", got)
		}
		if got := len(GetFamilyAppearances(tx, fx.familyId)); got != 0 {
			t.Errorf("%d appearances outlived the routine", got)
		}
		if got := len(GetEntryMembers(tx, fx.entry.Id)); got != 0 {
			t.Errorf("%d roster rows outlived the routine", got)
		}
	})
}
