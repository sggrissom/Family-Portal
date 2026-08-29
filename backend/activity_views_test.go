package backend

import (
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type seededSeason struct {
	resultsFixture

	nuvo, showstopper Event
	solo              Entry
	riseUpAtNuvo      Appearance
	riseUpAtShowstop  Appearance
	soloAtNuvo        Appearance
}

func seedSeason(t *testing.T) seededSeason {
	t.Helper()

	fx := seededSeason{resultsFixture: setupResultsFixture(t)}

	fx.nuvo = fx.event
	fx.showstopper = fx.createEvent(t, fx.season.Id, "Showstopper Orlando", "Showstopper", "2026-04-18")
	fx.solo = fx.createEntry(t, fx.season.Id, CreateEntryRequest{
		Name: "On My Own", Format: "solo", Style: "Jazz", Division: "Mini", Level: "Elite",
		PersonIds: []int{fx.bob.Id},
	})

	fx.riseUpAtShowstop = fx.createAppearance(t, fx.showstopper.Id, fx.entry.Id, "2026-04-18")
	fx.riseUpAtNuvo = fx.createAppearance(t, fx.nuvo.Id, fx.entry.Id, "2026-02-07")
	fx.soloAtNuvo = fx.createAppearance(t, fx.nuvo.Id, fx.solo.Id, "")

	fx.setResults(t, fx.riseUpAtNuvo.Id, []ResultInput{
		{Kind: ResultKindAdjudication, Label: "High Gold"},
		{Kind: ResultKindPlacement, Label: "Overall", Rank: intPtr(2), OutOf: intPtr(14),
			Category: "Teen Small Group Lyrical"},
	})
	fx.setResults(t, fx.riseUpAtShowstop.Id, []ResultInput{
		{Kind: ResultKindAdjudication, Label: "Platinum"},
		{Kind: ResultKindAward, Label: "Judges' Choice", PersonId: intPtr(fx.alice.Id)},
	})
	fx.setResults(t, fx.soloAtNuvo.Id, []ResultInput{
		{Kind: ResultKindAdjudication, Label: "high gold"},
	})

	return fx
}

func callAs[Req any, Resp any](t *testing.T, fx resultsFixture, proc func(*vbeam.Context, Req) (Resp, error), req Req) (Resp, error) {
	t.Helper()

	var resp Resp
	var err error
	fx.as(t, func(ctx *vbeam.Context) {
		resp, err = proc(ctx, req)
	})
	return resp, err
}

func TestGetSeasonOverviewReturnsTheWholeSeasonAndOnlyIt(t *testing.T) {
	fx := seedSeason(t)

	resp, err := callAs(t, fx.resultsFixture, GetSeasonOverview, GetSeasonOverviewRequest{SeasonId: fx.season.Id})
	if err != nil {
		t.Fatalf("GetSeasonOverview() error = %v", err)
	}

	if resp.Season.Id != fx.season.Id || resp.Activity.Id != fx.activity.Id {
		t.Errorf("overview = season %d of activity %d, want %d of %d",
			resp.Season.Id, resp.Activity.Id, fx.season.Id, fx.activity.Id)
	}

	if len(resp.Events) != 2 {
		t.Fatalf("got %d events, want 2", len(resp.Events))
	}
	if resp.Events[0].Id != fx.nuvo.Id || resp.Events[1].Id != fx.showstopper.Id {
		t.Errorf("events out of order: %q then %q", resp.Events[0].Name, resp.Events[1].Name)
	}

	if len(resp.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(resp.Entries))
	}
	if resp.Entries[0].Entry.Id != fx.solo.Id || resp.Entries[1].Entry.Id != fx.entry.Id {
		t.Errorf("entries out of name order: %q then %q",
			resp.Entries[0].Entry.Name, resp.Entries[1].Entry.Name)
	}
	if len(resp.Entries[1].PersonIds) != 2 {
		t.Errorf("the group routine came back with %d on its roster, want 2", len(resp.Entries[1].PersonIds))
	}

	if len(resp.Appearances) != 3 {
		t.Fatalf("got %d appearances, want 3", len(resp.Appearances))
	}
	byId := map[int]AppearanceView{}
	for _, view := range resp.Appearances {
		byId[view.Appearance.Id] = view
	}
	if got := len(byId[fx.riseUpAtNuvo.Id].Results); got != 2 {
		t.Errorf("Rise Up at Nuvo came back with %d results, want 2", got)
	}
	if got := byId[fx.riseUpAtNuvo.Id].Results[0].Label; got != "High Gold" {
		t.Errorf("first result = %q, want the adjudication the sheet led with", got)
	}

	for _, view := range resp.Entries {
		if view.Entry.Id == fx.otherEntry.Id {
			t.Error("a routine from another season showed up in the overview")
		}
	}
}

func TestGetEventDetailNamesTheRoutineOnEveryRow(t *testing.T) {
	fx := seedSeason(t)

	resp, err := callAs(t, fx.resultsFixture, GetEventDetail, GetEventDetailRequest{EventId: fx.nuvo.Id})
	if err != nil {
		t.Fatalf("GetEventDetail() error = %v", err)
	}

	if resp.Event.Id != fx.nuvo.Id || resp.Season.Id != fx.season.Id {
		t.Errorf("detail = event %d in season %d, want %d in %d",
			resp.Event.Id, resp.Season.Id, fx.nuvo.Id, fx.season.Id)
	}
	if len(resp.Appearances) != 2 {
		t.Fatalf("got %d performances, want 2", len(resp.Appearances))
	}

	if resp.Appearances[0].Appearance.Id != fx.riseUpAtNuvo.Id {
		t.Errorf("first performance = %d, want Rise Up (%d)",
			resp.Appearances[0].Appearance.Id, fx.riseUpAtNuvo.Id)
	}
	for i, row := range resp.Appearances {
		if row.Entry.Id == 0 || row.Entry.Name == "" {
			t.Errorf("performance %d came back without the routine that danced it", i)
		}
		if row.Event.Id != fx.nuvo.Id || row.Event.Host != "Nuvo" {
			t.Errorf("performance %d carries the wrong competition: %+v", i, row.Event)
		}
	}
	if got := len(resp.Appearances[0].Results); got != 2 {
		t.Errorf("Rise Up's row has %d results, want 2", got)
	}

	other, err := callAs(t, fx.resultsFixture, GetEventDetail, GetEventDetailRequest{EventId: fx.showstopper.Id})
	if err != nil {
		t.Fatalf("GetEventDetail(showstopper) error = %v", err)
	}
	if len(other.Appearances) != 1 || other.Appearances[0].Appearance.Id != fx.riseUpAtShowstop.Id {
		t.Errorf("showstopper = %+v, want just Rise Up's performance", other.Appearances)
	}
}

func TestGetEntryHistoryWalksTheSeasonInOrder(t *testing.T) {
	fx := seedSeason(t)

	resp, err := callAs(t, fx.resultsFixture, GetEntryHistory, GetEntryHistoryRequest{EntryId: fx.entry.Id})
	if err != nil {
		t.Fatalf("GetEntryHistory() error = %v", err)
	}

	if resp.Entry.Entry.Id != fx.entry.Id || len(resp.Entry.PersonIds) != 2 {
		t.Errorf("history = entry %d with %d dancers, want %d with 2",
			resp.Entry.Entry.Id, len(resp.Entry.PersonIds), fx.entry.Id)
	}
	if resp.Season.Id != fx.season.Id || resp.Season.Name != fx.season.Name {
		t.Errorf("history season = %+v, want a summary of %q", resp.Season, fx.season.Name)
	}

	if len(resp.Appearances) != 2 {
		t.Fatalf("got %d performances, want 2", len(resp.Appearances))
	}
	if resp.Appearances[0].Event.Id != fx.nuvo.Id || resp.Appearances[1].Event.Id != fx.showstopper.Id {
		t.Errorf("performances out of date order: %q then %q",
			resp.Appearances[0].Event.Name, resp.Appearances[1].Event.Name)
	}
	if resp.Appearances[0].Results[0].Label != "High Gold" ||
		resp.Appearances[1].Results[0].Label != "Platinum" {
		t.Errorf("results did not follow their performances: %+v", resp.Appearances)
	}
}

func TestGetPersonSeasonFiltersBySeason(t *testing.T) {
	fx := seedSeason(t)

	all, err := callAs(t, fx.resultsFixture, GetPersonSeason, GetPersonSeasonRequest{PersonId: fx.alice.Id})
	if err != nil {
		t.Fatalf("GetPersonSeason(all seasons) error = %v", err)
	}
	if len(all.Entries) != 2 {
		t.Fatalf("alice is in %d routines across all seasons, want 2", len(all.Entries))
	}
	if len(all.Seasons) != 2 {
		t.Errorf("got %d seasons, want both", len(all.Seasons))
	}
	if all.Seasons[0].Id != fx.season.Id {
		t.Errorf("seasons lead with %q, want the most recent", all.Seasons[0].Name)
	}

	scoped, err := callAs(t, fx.resultsFixture, GetPersonSeason,
		GetPersonSeasonRequest{PersonId: fx.alice.Id, SeasonId: fx.season.Id})
	if err != nil {
		t.Fatalf("GetPersonSeason(one season) error = %v", err)
	}
	if len(scoped.Entries) != 1 || scoped.Entries[0].Entry.Id != fx.entry.Id {
		t.Fatalf("scoped entries = %+v, want just Rise Up", scoped.Entries)
	}
	if len(scoped.Appearances) != 2 {
		t.Errorf("got %d performances, want both of Rise Up's", len(scoped.Appearances))
	}

	bob, err := callAs(t, fx.resultsFixture, GetPersonSeason,
		GetPersonSeasonRequest{PersonId: fx.bob.Id, SeasonId: fx.season.Id})
	if err != nil {
		t.Fatalf("GetPersonSeason(bob) error = %v", err)
	}
	if len(bob.Entries) != 2 {
		t.Errorf("bob is in %d routines, want 2", len(bob.Entries))
	}
	if len(bob.Appearances) != 3 {
		t.Errorf("bob has %d performances, want 3", len(bob.Appearances))
	}

	carol, err := callAs(t, fx.resultsFixture, GetPersonSeason, GetPersonSeasonRequest{PersonId: fx.carol.Id})
	if err != nil {
		t.Fatalf("GetPersonSeason(carol) error = %v", err)
	}
	if len(carol.Entries) != 0 || len(carol.Appearances) != 0 {
		t.Errorf("carol came back with %d routines and %d performances, want none",
			len(carol.Entries), len(carol.Appearances))
	}
}

func TestListActivityVocabularyFoldsCaseAndScopesToTheActivity(t *testing.T) {
	fx := seedSeason(t)

	resp, err := callAs(t, fx.resultsFixture, ListActivityVocabulary,
		ListActivityVocabularyRequest{ActivityId: fx.activity.Id})
	if err != nil {
		t.Fatalf("ListActivityVocabulary() error = %v", err)
	}

	if len(resp.Adjudications) != 2 {
		t.Fatalf("adjudications = %v, want High Gold and Platinum folded to two", resp.Adjudications)
	}
	if resp.Adjudications[0] != "High Gold" || resp.Adjudications[1] != "Platinum" {
		t.Errorf("adjudications = %v, want the first spelling of each, sorted", resp.Adjudications)
	}

	if len(resp.Awards) != 1 || resp.Awards[0] != "Judges' Choice" {
		t.Errorf("awards = %v, want just the judges' award", resp.Awards)
	}
	if len(resp.Categories) != 1 || resp.Categories[0] != "Teen Small Group Lyrical" {
		t.Errorf("categories = %v", resp.Categories)
	}

	assertHas := func(name string, list []string, want string) {
		t.Helper()
		for _, value := range list {
			if value == want {
				return
			}
		}
		t.Errorf("%s = %v, missing %q", name, list, want)
	}
	assertHas("styles", resp.Styles, "Jazz")
	assertHas("divisions", resp.Divisions, "Mini")
	assertHas("levels", resp.Levels, "Elite")
	assertHas("formats", resp.Formats, "group")
	assertHas("hosts", resp.Hosts, "Nuvo")
	assertHas("hosts", resp.Hosts, "Showstopper")

	for _, list := range [][]string{resp.Styles, resp.Divisions, resp.Levels, resp.Formats, resp.Hosts} {
		for _, value := range list {
			if value == "" {
				t.Error("an empty string got into the vocabulary")
			}
		}
	}

	var soccer Activity
	fx.as(t, func(ctx *vbeam.Context) {
		soccerResp, createErr := CreateActivity(ctx, CreateActivityRequest{Name: "Soccer", Kind: ActivityKindSport})
		if createErr != nil {
			t.Fatalf("CreateActivity(soccer) error = %v", createErr)
		}
		soccer = soccerResp.Activity
	})
	empty, err := callAs(t, fx.resultsFixture, ListActivityVocabulary,
		ListActivityVocabularyRequest{ActivityId: soccer.Id})
	if err != nil {
		t.Fatalf("ListActivityVocabulary(soccer) error = %v", err)
	}
	if len(empty.Adjudications) != 0 || len(empty.Hosts) != 0 || len(empty.Styles) != 0 {
		t.Errorf("a brand new activity inherited the dance vocabulary: %+v", empty)
	}
	if empty.Adjudications == nil || empty.Hosts == nil {
		t.Error("empty vocabulary lists came back nil rather than empty")
	}
}

func TestLinkedHouseholdReachesOnlyTheRosterScopedViews(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()
	jwtKey = []byte("activity-views-test-secret-key-at-least-32")

	setLinkScopes(t, fx.familyLinkFixture, fx.linkAB, LinkScopes{People: true, Activities: true})

	asUserB := func(fn func(ctx *vbeam.Context)) {
		t.Helper()
		token, err := generateJwtTokenString(fx.userB)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			fn(&vbeam.Context{Tx: tx, Token: token})
		})
	}

	asUserB(func(ctx *vbeam.Context) {
		resp, err := GetEntryHistory(ctx, GetEntryHistoryRequest{EntryId: fx.groupEntry.Id})
		if err != nil {
			t.Fatalf("GetEntryHistory(shared routine) error = %v", err)
		}
		if resp.Entry.Entry.Id != fx.groupEntry.Id {
			t.Errorf("history = %+v, want the shared routine", resp.Entry.Entry)
		}
		if len(resp.Appearances) != 1 || len(resp.Appearances[0].Results) != 2 {
			t.Errorf("the shared routine's results did not come with it: %+v", resp.Appearances)
		}
		if resp.Appearances[0].Event.Name != fx.event.Name {
			t.Errorf("event summary = %+v, want %q", resp.Appearances[0].Event, fx.event.Name)
		}
	})

	asUserB(func(ctx *vbeam.Context) {
		resp, err := GetPersonSeason(ctx, GetPersonSeasonRequest{PersonId: fx.alice.Id})
		if err != nil {
			t.Fatalf("GetPersonSeason(shared child) error = %v", err)
		}
		if len(resp.Entries) != 1 || resp.Entries[0].Entry.Id != fx.groupEntry.Id {
			t.Errorf("alice's season = %+v, want the group routine", resp.Entries)
		}
		if len(resp.Seasons) != 1 || resp.Seasons[0].Kind != ActivityKindDance {
			t.Errorf("shared season = %+v, want one dance season", resp.Seasons)
		}
	})

	asUserB(func(ctx *vbeam.Context) {
		if _, err := GetEntryHistory(ctx, GetEntryHistoryRequest{EntryId: fx.soloEntry.Id}); err == nil {
			t.Error("a routine with nobody shared in it was readable through the link")
		}
	})
	asUserB(func(ctx *vbeam.Context) {
		if _, err := GetSeasonOverview(ctx, GetSeasonOverviewRequest{SeasonId: fx.season.Id}); err == nil {
			t.Error("the season overview was readable through a link")
		}
	})
	asUserB(func(ctx *vbeam.Context) {
		if _, err := GetEventDetail(ctx, GetEventDetailRequest{EventId: fx.event.Id}); err == nil {
			t.Error("the competition view was readable through a link")
		}
	})
	asUserB(func(ctx *vbeam.Context) {
		if _, err := ListActivityVocabulary(ctx, ListActivityVocabularyRequest{ActivityId: fx.activity.Id}); err == nil {
			t.Error("the vocabulary was readable through a link")
		}
	})

	asUserB(func(ctx *vbeam.Context) {
		if _, err := GetPersonSeason(ctx, GetPersonSeasonRequest{PersonId: fx.bob.Id}); err == nil {
			t.Error("an unshared child's season was readable through the link")
		}
	})

	setLinkScopes(t, fx.familyLinkFixture, fx.linkAB, LinkScopes{People: true})
	asUserB(func(ctx *vbeam.Context) {
		if _, err := GetEntryHistory(ctx, GetEntryHistoryRequest{EntryId: fx.groupEntry.Id}); err == nil {
			t.Error("a routine was readable through a link that does not share activities")
		}
	})
	asUserB(func(ctx *vbeam.Context) {
		if _, err := GetPersonSeason(ctx, GetPersonSeasonRequest{PersonId: fx.alice.Id}); err == nil {
			t.Error("a shared child's season was readable without the activities scope")
		}
	})
}

func TestSeasonSummaryCarriesActivityKind(t *testing.T) {
	fx := seedSeason(t)

	detail, err := callAs(t, fx.resultsFixture, GetEventDetail, GetEventDetailRequest{EventId: fx.nuvo.Id})
	if err != nil {
		t.Fatalf("GetEventDetail error = %v", err)
	}
	if detail.Season.Kind != ActivityKindDance {
		t.Errorf("event detail season kind = %q, want %q", detail.Season.Kind, ActivityKindDance)
	}

	history, err := callAs(t, fx.resultsFixture, GetEntryHistory, GetEntryHistoryRequest{EntryId: fx.entry.Id})
	if err != nil {
		t.Fatalf("GetEntryHistory error = %v", err)
	}
	if history.Season.Kind != ActivityKindDance {
		t.Errorf("routine history season kind = %q, want %q", history.Season.Kind, ActivityKindDance)
	}

	person, err := callAs(t, fx.resultsFixture, GetPersonSeason, GetPersonSeasonRequest{PersonId: fx.alice.Id})
	if err != nil {
		t.Fatalf("GetPersonSeason error = %v", err)
	}
	if len(person.Seasons) == 0 {
		t.Fatal("alice has no seasons")
	}
	for _, season := range person.Seasons {
		if season.Kind != ActivityKindDance {
			t.Errorf("season %q kind = %q, want %q", season.Name, season.Kind, ActivityKindDance)
		}
	}
}
