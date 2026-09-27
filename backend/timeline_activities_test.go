package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestFamilyTimelineActivities(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	call := func(user User, req GetFamilyTimelineRequest) GetFamilyTimelineResponse {
		t.Helper()
		token, err := generateJwtTokenString(user)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		var resp GetFamilyTimelineResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = GetFamilyTimeline(&vbeam.Context{Tx: tx, Token: token}, req)
		})
		if err != nil {
			t.Fatalf("GetFamilyTimeline() error = %v", err)
		}
		return resp
	}

	with := call(fx.owner, GetFamilyTimelineRequest{IncludeActivities: true})
	if len(with.Appearances) != 1 {
		t.Fatalf("Appearances = %+v, want the fixture appearance", with.Appearances)
	}
	got := with.Appearances[0]
	if got.Detail.Appearance.Id != fx.appearance.Id || got.Detail.Event.Id != fx.event.Id {
		t.Errorf("appearance = %+v", got.Detail)
	}
	if len(got.PersonIds) != 1 || got.PersonIds[0] != fx.person.Id {
		t.Errorf("PersonIds = %v, want [%d]", got.PersonIds, fx.person.Id)
	}
	if len(got.Detail.Results) != 1 || got.Detail.Results[0].Label != "High Gold" {
		t.Errorf("Results = %+v", got.Detail.Results)
	}

	if without := call(fx.owner, GetFamilyTimelineRequest{}); len(without.Appearances) != 0 {
		t.Errorf("appearances without IncludeActivities = %d, want 0", len(without.Appearances))
	}

	lastYear := time.Now().AddDate(-1, 0, 0).Format(dateOnlyLayout)
	windowed := call(fx.owner, GetFamilyTimelineRequest{IncludeActivities: true, To: lastYear})
	if len(windowed.Appearances) != 0 {
		t.Errorf("an appearance outside the window was returned")
	}
	thisYear := time.Now().UTC().Year()
	found := false
	for _, year := range windowed.Years {
		found = found || year == thisYear
	}
	if !found {
		t.Errorf("Years = %v, want the appearance's year counted", windowed.Years)
	}

	if outsider := call(fx.outsider, GetFamilyTimelineRequest{IncludeActivities: true}); len(outsider.Appearances) != 0 {
		t.Errorf("an outsider saw %d appearances from another family", len(outsider.Appearances))
	}
}
