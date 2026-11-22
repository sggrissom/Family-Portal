package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func (fx resultsFixture) addMember(t *testing.T, email string, role AccessLevel) User {
	t.Helper()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	var user User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: email, Email: email}, hash)
		addMembershipTx(tx, user.Id, fx.familyId, role, time.Now())
		vbolt.TxCommit(tx)
	})
	return user
}

func (fx resultsFixture) currentAppearance(t *testing.T, id int) Appearance {
	t.Helper()

	var appearance Appearance
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		appearance = GetAppearanceById(tx, id)
	})
	return appearance
}

func (fx resultsFixture) recapInput(t *testing.T, appearance Appearance) RecapInput {
	t.Helper()

	current := fx.currentAppearance(t, appearance.Id)
	return RecapInput{
		EventName: fx.event.Name,
		Subtitle:  "2025-26 · Feb 7, 2026",
		Place:     "Nuvo",
		Routines: []RecapRoutineInput{{
			EntryId: fx.entry.Id,
			Name:    fx.entry.Name,
			Performances: []RecapPerformanceInput{{
				AppearanceId: current.Id,
				Revision:     current.Revision,
				Results: []PublishedResult{
					{Kind: ResultKindPlacement, Text: "1st of 12"},
					{Kind: ResultKindAward, Text: "Judges' Choice", Detail: "Individual"},
				},
			}},
		}},
	}
}

func (fx resultsFixture) publishRecap(t *testing.T, req CreateRecapLinkRequest) CreateRecapLinkResponse {
	t.Helper()

	var resp CreateRecapLinkResponse
	fx.as(t, func(ctx *vbeam.Context) {
		var err error
		resp, err = CreateRecapLink(ctx, req)
		if err != nil {
			t.Fatalf("CreateRecapLink() error = %v", err)
		}
	})
	return resp
}

func (fx resultsFixture) readRecap(token string) (resp GetRecapResponse, err error) {
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err = GetRecap(&vbeam.Context{Tx: tx}, GetRecapRequest{Token: token})
	})
	return
}

func recapTokenFromURL(t *testing.T, url string) string {
	t.Helper()
	_, token, found := strings.Cut(url, RecapPathPrefix)
	if !found || token == "" {
		t.Fatal("published URL is missing a recap token")
	}
	return token
}

func setupRecapFixture(t *testing.T) (resultsFixture, Appearance) {
	t.Helper()

	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)
	rank, outOf := 1, 12
	fx.setResults(t, appearance.Id, []ResultInput{
		{Kind: ResultKindPlacement, Rank: &rank, OutOf: &outOf},
		{Kind: ResultKindAward, Label: "Judges' Choice", PersonId: &fx.alice.Id},
	})
	return fx, appearance
}

func TestRecapLinkServesTheApprovedSnapshotWithoutSigningIn(t *testing.T) {
	fx, appearance := setupRecapFixture(t)

	resp := fx.publishRecap(t, CreateRecapLinkRequest{EventId: fx.event.Id, Recap: fx.recapInput(t, appearance)})
	snapshot, err := fx.readRecap(recapTokenFromURL(t, resp.URL))
	if err != nil {
		t.Fatalf("GetRecap() without a session: %v", err)
	}
	body, err := json.Marshal(snapshot.Recap)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"eventName":"Nuvo Nashville","subtitle":"2025-26 · Feb 7, 2026","place":"Nuvo","snapshotLabel":"","routines":[{"name":"Rise Up","descriptors":"","roster":"","summary":"","performances":[{"when":"","matchup":"","results":[{"kind":"placement","text":"1st of 12","detail":""},{"kind":"award","text":"Judges' Choice","detail":"Individual"}]}]}]}`
	if string(body) != want {
		t.Errorf("public recap = %s, want %s", body, want)
	}
}

func TestRecapPageHeaders(t *testing.T) {
	app := vbeam.NewApplication("Recap header test", nil)
	path := RecapPathPrefix + "test-token"
	app.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	NewSecurityWrapper(app).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	headers := rec.Header()
	if got := headers.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", got)
	}
	if got := headers.Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Errorf("X-Robots-Tag = %q, want noindex", got)
	}
}

func TestRecapLinkStaysAsApprovedAfterTheEventChanges(t *testing.T) {
	fx, appearance := setupRecapFixture(t)

	resp := fx.publishRecap(t, CreateRecapLinkRequest{EventId: fx.event.Id, Recap: fx.recapInput(t, appearance)})
	token := recapTokenFromURL(t, resp.URL)
	before, err := fx.readRecap(token)
	if err != nil {
		t.Fatalf("GetRecap() before editing: %v", err)
	}

	rank := 3
	fx.setResults(t, appearance.Id, []ResultInput{{Kind: ResultKindPlacement, Rank: &rank}})
	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := UpdateEntry(ctx, UpdateEntryRequest{Id: fx.entry.Id, Name: "Renamed"}); err != nil {
			t.Fatalf("UpdateEntry() error = %v", err)
		}
	})

	after, err := fx.readRecap(token)
	if err != nil {
		t.Fatalf("GetRecap() after editing: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Error("published recap changed after the event was edited")
	}
}

func TestRecapLinkFailsOnceRevokedOrExpired(t *testing.T) {
	fx, appearance := setupRecapFixture(t)

	revoked := fx.publishRecap(t, CreateRecapLinkRequest{EventId: fx.event.Id, Recap: fx.recapInput(t, appearance)})
	expiring := fx.publishRecap(t, CreateRecapLinkRequest{EventId: fx.event.Id, ExpiresInDays: 7, Recap: fx.recapInput(t, appearance)})

	if _, err := fx.readRecap(recapTokenFromURL(t, expiring.URL)); err != nil {
		t.Fatalf("GetRecap() before expiry: %v", err)
	}

	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := RevokeRecapLink(ctx, RevokeRecapLinkRequest{Id: revoked.Link.Id}); err != nil {
			t.Fatalf("RevokeRecapLink() error = %v", err)
		}
	})
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		link := GetRecapLinkById(tx, expiring.Link.Id)
		link.ExpiresAt = time.Now().Add(-time.Minute)
		writeRecapLinkTx(tx, &link)
		vbolt.TxCommit(tx)
	})

	for name, token := range map[string]string{
		"revoked": recapTokenFromURL(t, revoked.URL),
		"expired": recapTokenFromURL(t, expiring.URL),
		"unknown": "not-a-token",
		"empty":   "",
	} {
		if _, err := fx.readRecap(token); err != ErrRecapUnavailable {
			t.Errorf("%s: GetRecap() error = %v, want %v", name, err, ErrRecapUnavailable)
		}
	}
}

func TestCreateRecapLinkChecksEveryIncludedRecord(t *testing.T) {
	fx, appearance := setupRecapFixture(t)

	otherEvent := fx.createEvent(t, fx.season.Id, "Other", "", "2026-03-01")
	otherAppearance := fx.createAppearance(t, otherEvent.Id, fx.entry.Id, "")

	stale := fx.recapInput(t, appearance)
	stale.Routines[0].Performances[0].Revision--

	foreignAppearance := fx.recapInput(t, appearance)
	foreignAppearance.Routines[0].Performances[0].AppearanceId = otherAppearance.Id

	foreignEntry := fx.recapInput(t, appearance)
	foreignEntry.Routines[0].EntryId = fx.otherEntry.Id

	duplicated := fx.recapInput(t, appearance)
	duplicated.Routines = append(duplicated.Routines, duplicated.Routines[0])

	badKind := fx.recapInput(t, appearance)
	badKind.Routines[0].Performances[0].Results[0].Kind = "note"

	empty := fx.recapInput(t, appearance)
	empty.Routines = nil

	cases := []struct {
		name string
		req  CreateRecapLinkRequest
		want error
	}{
		{"stale revision", CreateRecapLinkRequest{EventId: fx.event.Id, Recap: stale}, ErrRecapOutOfDate},
		{"appearance from another event", CreateRecapLinkRequest{EventId: fx.event.Id, Recap: foreignAppearance}, ErrRecapNotThisEvent},
		{"entry from another season", CreateRecapLinkRequest{EventId: fx.event.Id, Recap: foreignEntry}, ErrRecapNotThisEvent},
		{"routine twice", CreateRecapLinkRequest{EventId: fx.event.Id, Recap: duplicated}, ErrRecapNotThisEvent},
		{"unknown result kind", CreateRecapLinkRequest{EventId: fx.event.Id, Recap: badKind}, ErrInvalidResultKind},
		{"no routines", CreateRecapLinkRequest{EventId: fx.event.Id, Recap: empty}, ErrRecapEmpty},
		{"expiry over a year", CreateRecapLinkRequest{EventId: fx.event.Id, ExpiresInDays: 400, Recap: fx.recapInput(t, appearance)}, ErrRecapExpiry},
	}
	for _, tc := range cases {
		fx.as(t, func(ctx *vbeam.Context) {
			if _, err := CreateRecapLink(ctx, tc.req); err != tc.want {
				t.Errorf("%s: CreateRecapLink() error = %v, want %v", tc.name, err, tc.want)
			}
		})
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if links := getEventRecapLinks(tx, fx.event.Id); len(links) != 0 {
			t.Errorf("rejected requests stored %d links", len(links))
		}
	})
}

func TestRecapLinkPermissions(t *testing.T) {
	fx, appearance := setupRecapFixture(t)

	viewer := fx.addMember(t, "viewer@example.com", AccessView)
	contributor := fx.addMember(t, "contributor@example.com", AccessContribute)
	otherContributor := fx.addMember(t, "other@example.com", AccessContribute)
	req := CreateRecapLinkRequest{EventId: fx.event.Id, Recap: fx.recapInput(t, appearance)}

	fx.asUser(t, viewer, func(ctx *vbeam.Context) {
		if _, err := CreateRecapLink(ctx, req); err != ErrEventNotFound {
			t.Errorf("viewer CreateRecapLink() error = %v, want %v", err, ErrEventNotFound)
		}
	})

	var contributorLink CreateRecapLinkResponse
	fx.asUser(t, contributor, func(ctx *vbeam.Context) {
		var err error
		contributorLink, err = CreateRecapLink(ctx, req)
		if err != nil {
			t.Fatalf("contributor CreateRecapLink() error = %v", err)
		}
	})

	fx.asUser(t, otherContributor, func(ctx *vbeam.Context) {
		resp, err := ListRecapLinks(ctx, ListRecapLinksRequest{EventId: fx.event.Id})
		if err != nil || len(resp.Links) != 1 {
			t.Fatalf("ListRecapLinks() = %+v, %v; want the one link", resp, err)
		}
		if resp.Links[0].CanRevoke {
			t.Error("another contributor is offered revoke on a link they did not create")
		}
		if _, err := RevokeRecapLink(ctx, RevokeRecapLinkRequest{Id: contributorLink.Link.Id}); err != ErrRecapLinkNotAllowed {
			t.Errorf("other contributor RevokeRecapLink() error = %v, want %v", err, ErrRecapLinkNotAllowed)
		}
	})

	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := ListRecapLinks(ctx, ListRecapLinksRequest{EventId: fx.event.Id})
		if err != nil || len(resp.Links) != 1 || !resp.Links[0].CanRevoke {
			t.Fatalf("owner ListRecapLinks() = %+v, %v; want a revocable link", resp, err)
		}
		if resp.Links[0].CreatedByName != contributor.Name {
			t.Errorf("CreatedByName = %q, want %q", resp.Links[0].CreatedByName, contributor.Name)
		}
		if _, err := RevokeRecapLink(ctx, RevokeRecapLinkRequest{Id: contributorLink.Link.Id}); err != nil {
			t.Errorf("owner RevokeRecapLink() error = %v", err)
		}
	})
}

func TestDeletingAnEventRemovesItsRecapLinks(t *testing.T) {
	fx, appearance := setupRecapFixture(t)

	resp := fx.publishRecap(t, CreateRecapLinkRequest{EventId: fx.event.Id, Recap: fx.recapInput(t, appearance)})
	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := DeleteEvent(ctx, EventIdRequest{Id: fx.event.Id}); err != nil {
			t.Fatalf("DeleteEvent() error = %v", err)
		}
	})

	if _, err := fx.readRecap(recapTokenFromURL(t, resp.URL)); err != ErrRecapUnavailable {
		t.Errorf("GetRecap() after deleting event = %v, want %v", err, ErrRecapUnavailable)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if link := GetRecapLinkById(tx, resp.Link.Id); link.Id != 0 {
			t.Error("recap link row survived its event")
		}
	})
}

func TestRecapPathsAreRedactedAndRateLimited(t *testing.T) {
	if got := redactRecapPath("/recap/abc123"); got != "/recap/[redacted]" {
		t.Errorf("redactRecapPath(page) = %q", got)
	}
	if got := redactRecapPath("/recap/abc123/recap.css"); got != "/recap/[redacted]/recap.css" {
		t.Errorf("redactRecapPath(asset) = %q", got)
	}
	if got := redactRecapPath("/api/photo/1"); got != "/api/photo/1" {
		t.Errorf("redactRecapPath(other) = %q", got)
	}
	if rule, ok := ruleForPath("/recap/abc123"); !ok || rule.Name != rateRuleRecapRead.Name {
		t.Errorf("ruleForPath(/recap/...) = %+v, %v; want the recap-read rule", rule, ok)
	}
}
