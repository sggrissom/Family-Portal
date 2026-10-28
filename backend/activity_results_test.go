package backend

import (
	"family/cfg"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type resultsFixture struct {
	db *vbolt.DB

	owner    User
	familyId int
	alice    Person
	bob      Person
	carol    Person

	activity    Activity
	season      Season
	otherSeason Season
	event       Event
	entry       Entry
	otherEntry  Entry
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
				Name: seed.name, Gender: 0, Birthdate: "2014-03-02",
			}, fx.familyId)
			if err != nil {
				t.Fatalf("AddPersonTx(%s) error = %v", seed.name, err)
			}
		}
		vbolt.TxCommit(tx)
	})

	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateActivity(ctx, CreateActivityRequest{Name: "Dance", Kind: ActivityKindDance})
		if err != nil {
			t.Fatalf("CreateActivity() error = %v", err)
		}
		fx.activity = resp.Activity
	})
	fx.as(t, func(ctx *vbeam.Context) {
		start := "2025-08-01"
		resp, err := CreateSeason(ctx, CreateSeasonRequest{
			ActivityId: fx.activity.Id, Name: "2025-26", StartDate: &start,
		})
		if err != nil {
			t.Fatalf("CreateSeason() error = %v", err)
		}
		fx.season = resp.Season
	})
	fx.as(t, func(ctx *vbeam.Context) {
		start := "2024-08-01"
		resp, err := CreateSeason(ctx, CreateSeasonRequest{
			ActivityId: fx.activity.Id, Name: "2024-25", StartDate: &start,
		})
		if err != nil {
			t.Fatalf("CreateSeason(other) error = %v", err)
		}
		fx.otherSeason = resp.Season
	})
	fx.as(t, func(ctx *vbeam.Context) {
		start := "2026-02-07"
		resp, err := CreateEvent(ctx, CreateEventRequest{
			SeasonId: fx.season.Id, Name: "Nuvo Nashville", Host: "Nuvo", StartDate: &start,
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

func (fx resultsFixture) createEvent(t *testing.T, seasonId int, name string, host string, startDate string) Event {
	t.Helper()

	var event Event
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateEvent(ctx, CreateEventRequest{
			SeasonId: seasonId, Name: name, Host: host, StartDate: &startDate,
		})
		if err != nil {
			t.Fatalf("CreateEvent(%s) error = %v", name, err)
		}
		event = resp.Event
	})
	return event
}

func (fx resultsFixture) createEntry(t *testing.T, seasonId int, req CreateEntryRequest) Entry {
	t.Helper()

	req.SeasonId = seasonId
	var entry Entry
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateEntry(ctx, req)
		if err != nil {
			t.Fatalf("CreateEntry(%s) error = %v", req.Name, err)
		}
		entry = resp.Entry.Entry
	})
	return entry
}

func (fx resultsFixture) createAppearance(t *testing.T, eventId int, entryId int, occurredAt string) Appearance {
	t.Helper()

	req := CreateAppearanceRequest{EventId: eventId, EntryId: entryId}
	if occurredAt != "" {
		req.OccurredAt = &occurredAt
	}
	var appearance Appearance
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateAppearance(ctx, req)
		if err != nil {
			t.Fatalf("CreateAppearance() error = %v", err)
		}
		appearance = resp.Appearance.Appearance
	})
	return appearance
}

func (fx resultsFixture) setResults(t *testing.T, appearanceId int, results []ResultInput) {
	t.Helper()

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SetAppearanceResults(ctx, SetAppearanceResultsRequest{
			AppearanceId: appearanceId, Results: results,
		}); err != nil {
			t.Fatalf("SetAppearanceResults() error = %v", err)
		}
	})
}

func (fx resultsFixture) resultsOf(t *testing.T, appearanceId int) []Result {
	t.Helper()

	var results []Result
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		results = sortResults(GetAppearanceResults(tx, appearanceId))
	})
	return results
}

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

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetPersonResults(tx, fx.alice.Id)); got != 0 {
			t.Errorf("GetPersonResults(alice) = %d after the award was dropped, want 0", got)
		}
		if got := len(GetFamilyResults(tx, fx.familyId)); got != 1 {
			t.Errorf("GetFamilyResults = %d, want 1", got)
		}
	})

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
				{Kind: ResultKindPlacement, Label: "Overall"},
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

func TestAppearanceMatchupRoundTrips(t *testing.T) {
	fx := setupResultsFixture(t)

	var created Appearance
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := CreateAppearance(ctx, CreateAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.entry.Id,
			Matchup: &MatchupInput{
				Round: " Semifinal ", Opponent: "Crew Z", Outcome: "WIN",
				ScoreFor: intPtr(3), ScoreAgainst: intPtr(0),
			},
		})
		if err != nil {
			t.Fatalf("CreateAppearance() error = %v", err)
		}
		created = resp.Appearance.Appearance
	})

	var stored Appearance
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		stored = GetAppearanceById(tx, created.Id)
	})
	if stored.Round != "Semifinal" || stored.Opponent != "Crew Z" || stored.Outcome != OutcomeWin {
		t.Errorf("stored matchup = %q / %q / %q", stored.Round, stored.Opponent, stored.Outcome)
	}
	if stored.ScoreFor == nil || *stored.ScoreFor != 3 || stored.ScoreAgainst == nil || *stored.ScoreAgainst != 0 {
		t.Errorf("stored score = %v-%v, want 3-0", stored.ScoreFor, stored.ScoreAgainst)
	}
}

func TestUpdateAppearanceWithoutAMatchupKeepsIt(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := UpdateAppearance(ctx, UpdateAppearanceRequest{
			Id: appearance.Id, Matchup: &MatchupInput{Round: "Final", Outcome: OutcomeLoss},
		}); err != nil {
			t.Fatalf("UpdateAppearance(matchup) error = %v", err)
		}
	})
	var kept Appearance
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := UpdateAppearance(ctx, UpdateAppearanceRequest{Id: appearance.Id, Notes: "close one"})
		if err != nil {
			t.Fatalf("UpdateAppearance(notes) error = %v", err)
		}
		kept = resp.Appearance.Appearance
	})
	if kept.Round != "Final" || kept.Outcome != OutcomeLoss || kept.Notes != "close one" {
		t.Errorf("an update without a matchup changed it: %+v", kept)
	}

	var cleared Appearance
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := UpdateAppearance(ctx, UpdateAppearanceRequest{Id: appearance.Id, Matchup: &MatchupInput{}})
		if err != nil {
			t.Fatalf("UpdateAppearance(empty matchup) error = %v", err)
		}
		cleared = resp.Appearance.Appearance
	})
	if cleared.Round != "" || cleared.Outcome != "" {
		t.Errorf("an empty matchup did not clear it: %+v", cleared)
	}
}

func TestAppearanceMatchupValidation(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	for _, tc := range []struct {
		name    string
		matchup MatchupInput
		want    error
	}{
		{"unknown outcome", MatchupInput{Outcome: "forfeit"}, ErrInvalidOutcome},
		{"negative score", MatchupInput{ScoreFor: intPtr(-1)}, ErrMatchupScoreOutOfRange},
		{"huge score", MatchupInput{ScoreAgainst: intPtr(maxMatchupScore + 1)}, ErrMatchupScoreOutOfRange},
	} {
		matchup := tc.matchup
		fx.as(t, func(ctx *vbeam.Context) {
			_, err := UpdateAppearance(ctx, UpdateAppearanceRequest{Id: appearance.Id, Matchup: &matchup})
			if err != tc.want {
				t.Errorf("%s: error = %v, want %v", tc.name, err, tc.want)
			}
		})
		fx.as(t, func(ctx *vbeam.Context) {
			_, err := CreateAppearance(ctx, CreateAppearanceRequest{
				EventId: fx.event.Id, EntryId: fx.entry.Id, Matchup: &matchup,
			})
			if err != tc.want {
				t.Errorf("%s on create: error = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetEventAppearances(tx, fx.event.Id)); got != 1 {
			t.Errorf("rejected creates left %d appearances, want 1", got)
		}
	})
}

func TestSaveAppearanceCreatesTheWholePerformanceAtOnce(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhoto(t, fx.familyId, "stage.jpg")
	occurredAt := "2026-02-08"

	resp, err := callAs(t, fx, SaveAppearance, SaveAppearanceRequest{
		EventId: fx.event.Id, EntryId: fx.entry.Id, OccurredAt: &occurredAt, Notes: "  clean run  ",
		Results: []ResultInput{
			{Kind: ResultKindPlacement, Rank: intPtr(1), OutOf: intPtr(9), Category: "Teen Group Jazz"},
			{Kind: ResultKindAdjudication, Label: "Platinum"},
		},
		PhotoIds: []int{photo.Id},
	})
	if err != nil {
		t.Fatalf("SaveAppearance(new) error = %v", err)
	}

	saved := resp.Appearance
	if saved.Appearance.Id == 0 || saved.Appearance.EventId != fx.event.Id || saved.Appearance.EntryId != fx.entry.Id {
		t.Fatalf("SaveAppearance(new) = %+v, want it hung off the event and the entry", saved.Appearance)
	}
	if saved.Appearance.Notes != "clean run" {
		t.Errorf("notes = %q, want trimmed", saved.Appearance.Notes)
	}
	if len(saved.Results) != 2 || saved.Results[0].Kind != ResultKindPlacement || saved.Results[1].Label != "Platinum" {
		t.Errorf("results = %+v, want the placement then the adjudication", saved.Results)
	}
	if saved.Results[0].AppearanceId != saved.Appearance.Id {
		t.Errorf("result appearance = %d, want %d", saved.Results[0].AppearanceId, saved.Appearance.Id)
	}
	if len(saved.PhotoIds) != 1 || saved.PhotoIds[0] != photo.Id {
		t.Errorf("photoIds = %v, want the stage photo", saved.PhotoIds)
	}
}

func TestSaveAppearanceReplacesResultsAndPhotosOnUpdate(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)
	fx.setResults(t, appearance.Id, []ResultInput{{Kind: ResultKindAdjudication, Label: "High Gold"}})
	first := fx.addPhoto(t, fx.familyId, "one.jpg")
	second := fx.addPhoto(t, fx.familyId, "two.jpg")
	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{first.Id},
	}); err != nil {
		t.Fatalf("SetAppearancePhotos() error = %v", err)
	}

	resp, err := callAs(t, fx, SaveAppearance, SaveAppearanceRequest{
		Id: appearance.Id, EventId: 0, EntryId: fx.otherEntry.Id,
		Results:  []ResultInput{{Kind: ResultKindAward, Label: "Judges' Choice"}},
		PhotoIds: []int{second.Id},
	})
	if err != nil {
		t.Fatalf("SaveAppearance(update) error = %v", err)
	}

	saved := resp.Appearance
	if saved.Appearance.Id != appearance.Id || saved.Appearance.EntryId != fx.entry.Id {
		t.Errorf("update moved or re-created the appearance: %+v", saved.Appearance)
	}
	results := fx.resultsOf(t, appearance.Id)
	if len(results) != 1 || results[0].Label != "Judges' Choice" {
		t.Errorf("results = %+v, want only the award", results)
	}
	if len(saved.PhotoIds) != 1 || saved.PhotoIds[0] != second.Id {
		t.Errorf("photoIds = %v, want only the second photo", saved.PhotoIds)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetEventAppearances(tx, fx.event.Id)); got != 1 {
			t.Errorf("GetEventAppearances = %d, want the one appearance", got)
		}
	})
}

func TestSaveAppearanceWritesNothingWhenAnyPartIsRejected(t *testing.T) {
	fx := setupResultsFixture(t)
	ours := fx.addPhoto(t, fx.familyId, "ours.jpg")
	theirs := fx.addPhoto(t, fx.familyId+999, "theirs.jpg")

	cases := []struct {
		name string
		req  SaveAppearanceRequest
	}{
		{"bad result", SaveAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.entry.Id,
			Results: []ResultInput{{Kind: ResultKindAdjudication, Label: "Gold"}, {Kind: ResultKindPlacement}},
		}},
		{"another family's photo", SaveAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.entry.Id, PhotoIds: []int{ours.Id, theirs.Id},
		}},
		{"entry from another season", SaveAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.otherEntry.Id,
		}},
		{"result for someone off the roster", SaveAppearanceRequest{
			EventId: fx.event.Id, EntryId: fx.entry.Id,
			Results: []ResultInput{{Kind: ResultKindAward, Label: "Solo", PersonId: intPtr(fx.carol.Id)}},
		}},
	}
	for _, tc := range cases {
		if _, err := callAs(t, fx, SaveAppearance, tc.req); err == nil {
			t.Errorf("SaveAppearance(%s) succeeded", tc.name)
		}
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetEventAppearances(tx, fx.event.Id)); got != 0 {
			t.Errorf("rejected saves left %d appearances behind", got)
		}
		if got := len(GetFamilyAppearancePhotos(tx, fx.familyId)); got != 0 {
			t.Errorf("rejected saves left %d photo joins behind", got)
		}
	})
}
