package backend

import (
	"errors"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func (fx resultsFixture) syncSave(t *testing.T, mutationId string, expected *int, req SaveAppearanceRequest) AppearanceMutationResponse {
	t.Helper()
	resp, err := callAs(t, fx, SyncSaveAppearance, SyncSaveAppearanceRequest{
		MutationId: mutationId, ExpectedRevision: expected, Appearance: req,
	})
	if err != nil {
		t.Fatalf("SyncSaveAppearance(%s) error = %v", mutationId, err)
	}
	return resp
}

func (fx resultsFixture) syncDelete(t *testing.T, mutationId string, id int, expected int) AppearanceMutationResponse {
	t.Helper()
	resp, err := callAs(t, fx, SyncDeleteAppearance, SyncDeleteAppearanceRequest{
		MutationId: mutationId, Id: id, ExpectedRevision: &expected,
	})
	if err != nil {
		t.Fatalf("SyncDeleteAppearance(%s) error = %v", mutationId, err)
	}
	return resp
}

func (fx resultsFixture) storedAppearance(t *testing.T, id int) (appearance Appearance) {
	t.Helper()
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { appearance = GetAppearanceById(tx, id) })
	return
}

func (fx resultsFixture) eventAppearanceCount(t *testing.T) (count int) {
	t.Helper()
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { count = len(GetEventAppearances(tx, fx.event.Id)) })
	return
}

// member adds another account to the fixture's family at the given role, and
// returns a fixture that acts as that account.
func (fx resultsFixture) member(t *testing.T, email string, role AccessLevel) resultsFixture {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	other := fx
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		other.owner = AddUserTx(tx, CreateAccountRequest{Name: email, Email: email}, hash)
		if role != AccessNone {
			addMembershipTx(tx, other.owner.Id, fx.familyId, role, time.Now())
		}
		vbolt.TxCommit(tx)
	})
	return other
}

func (fx resultsFixture) adjudication(label string) SaveAppearanceRequest {
	return SaveAppearanceRequest{
		EventId: fx.event.Id, EntryId: fx.entry.Id,
		Results: []ResultInput{{Kind: ResultKindAdjudication, Label: label}},
	}
}

func TestSyncSupportReportsTheMutationVersion(t *testing.T) {
	fx := setupResultsFixture(t)
	resp, err := callAs(t, fx, GetActivitySyncSupport, GetActivitySyncSupportRequest{})
	if err != nil || resp.MutationVersion != ActivityMutationVersion {
		t.Fatalf("GetActivitySyncSupport() = %+v, %v", resp, err)
	}
}

func TestSyncCreateRetriedAfterALostResponseMakesOnePerformance(t *testing.T) {
	fx := setupResultsFixture(t)

	first := fx.syncSave(t, "create-1", nil, fx.adjudication("Platinum"))
	if first.Status != MutationApplied || first.AppearanceId == 0 || first.Revision != 1 {
		t.Fatalf("first create = %+v, want applied at revision 1", first)
	}
	if first.Appearance == nil || first.Appearance.Appearance.Revision != 1 || len(first.Appearance.Results) != 1 {
		t.Fatalf("first create aggregate = %+v, want the saved sheet", first.Appearance)
	}

	retry := fx.syncSave(t, "create-1", nil, fx.adjudication("Platinum"))
	if retry.Status != MutationReplayed || retry.AppearanceId != first.AppearanceId || retry.Revision != 1 {
		t.Fatalf("retry = %+v, want a replay of %d at revision 1", retry, first.AppearanceId)
	}
	if got := fx.eventAppearanceCount(t); got != 1 {
		t.Fatalf("event has %d performances after a retried create, want 1", got)
	}
}

func TestSyncMutationIdIsBoundToItsPayload(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.syncSave(t, "create-1", nil, fx.adjudication("Platinum"))

	_, err := callAs(t, fx, SyncSaveAppearance, SyncSaveAppearanceRequest{
		MutationId: "create-1", Appearance: fx.adjudication("Gold"),
	})
	if !errors.Is(err, ErrMutationIdReused) {
		t.Fatalf("reused id with a new payload: err = %v, want ErrMutationIdReused", err)
	}

	created := fx.syncSave(t, "create-2", nil, fx.adjudication("Gold"))
	_, err = callAs(t, fx, SyncDeleteAppearance, SyncDeleteAppearanceRequest{
		MutationId: "create-2", Id: created.AppearanceId, ExpectedRevision: intPtr(1),
	})
	if !errors.Is(err, ErrMutationIdReused) {
		t.Fatalf("a save's id reused for a delete: err = %v, want ErrMutationIdReused", err)
	}
}

func TestSyncRequestsAreValidatedBeforeAnythingElse(t *testing.T) {
	fx := setupResultsFixture(t)

	for _, id := range []string{"", "has space", "slash/no", string(make([]byte, maxMutationIdLength+1))} {
		if _, err := callAs(t, fx, SyncSaveAppearance, SyncSaveAppearanceRequest{
			MutationId: id, Appearance: fx.adjudication("Gold"),
		}); !errors.Is(err, ErrMutationIdInvalid) {
			t.Errorf("mutation id %q: err = %v, want ErrMutationIdInvalid", id, err)
		}
	}

	appearance := fx.newAppearance(t)
	edit := fx.adjudication("Gold")
	edit.Id = appearance.Id
	if _, err := callAs(t, fx, SyncSaveAppearance, SyncSaveAppearanceRequest{
		MutationId: "edit-1", Appearance: edit,
	}); !errors.Is(err, ErrExpectedRevisionRequired) {
		t.Errorf("edit without a revision: err = %v, want ErrExpectedRevisionRequired", err)
	}
	if _, err := callAs(t, fx, SyncDeleteAppearance, SyncDeleteAppearanceRequest{
		MutationId: "delete-1", Id: appearance.Id,
	}); !errors.Is(err, ErrExpectedRevisionRequired) {
		t.Errorf("delete without a revision: err = %v, want ErrExpectedRevisionRequired", err)
	}
}

func TestSyncEditAgainstTheCurrentRevisionApplies(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))

	edit := fx.adjudication("Platinum")
	edit.Id = created.AppearanceId
	resp := fx.syncSave(t, "edit-1", intPtr(created.Revision), edit)

	if resp.Status != MutationApplied || resp.Revision != created.Revision+1 {
		t.Fatalf("edit = %+v, want applied at revision %d", resp, created.Revision+1)
	}
	if got := fx.resultsOf(t, created.AppearanceId); len(got) != 1 || got[0].Label != "Platinum" {
		t.Errorf("results = %+v, want the edit's sheet", got)
	}
}

func TestEveryWebsiteWriteAdvancesTheRevision(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)
	photo := fx.addPhoto(t, fx.familyId, "stage.jpg")
	revision := fx.storedAppearance(t, appearance.Id).Revision
	if revision != 1 {
		t.Fatalf("a new performance is at revision %d, want 1", revision)
	}

	writes := []struct {
		name  string
		write func()
	}{
		{"UpdateAppearance", func() {
			if _, err := callAs(t, fx, UpdateAppearance, UpdateAppearanceRequest{Id: appearance.Id, Notes: "n"}); err != nil {
				t.Fatalf("UpdateAppearance() error = %v", err)
			}
		}},
		{"SetAppearanceResults", func() {
			fx.setResults(t, appearance.Id, []ResultInput{{Kind: ResultKindAward, Label: "Solo", PersonId: intPtr(fx.alice.Id)}})
		}},
		{"SetAppearancePhotos", func() {
			if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
				AppearanceId: appearance.Id, PhotoIds: []int{photo.Id},
			}); err != nil {
				t.Fatalf("SetAppearancePhotos() error = %v", err)
			}
		}},
		{"SaveAppearance", func() {
			req := fx.adjudication("High Gold")
			req.Id = appearance.Id
			req.PhotoIds = []int{photo.Id}
			req.Results = append(req.Results, ResultInput{Kind: ResultKindAward, Label: "Solo", PersonId: intPtr(fx.alice.Id)})
			if _, err := callAs(t, fx, SaveAppearance, req); err != nil {
				t.Fatalf("SaveAppearance() error = %v", err)
			}
		}},
		{"merging a person into another", func() {
			vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
				movePersonActivitiesTx(tx, fx.alice.Id, fx.bob.Id)
				vbolt.TxCommit(tx)
			})
		}},
		{"removing a person", func() {
			vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
				removePersonFromActivitiesTx(tx, fx.bob.Id)
				vbolt.TxCommit(tx)
			})
		}},
		{"deleting an attached photo", func() {
			vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
				removePhotoFromActivities(tx, photo.Id)
				vbolt.TxCommit(tx)
			})
		}},
	}
	for _, w := range writes {
		w.write()
		next := fx.storedAppearance(t, appearance.Id).Revision
		if next <= revision {
			t.Errorf("%s left the revision at %d", w.name, next)
		}
		revision = next
	}
}

func TestSyncEditAfterAWebsiteChangeIsAConflictThatWritesNothing(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))
	fx.setResults(t, created.AppearanceId, []ResultInput{{Kind: ResultKindAward, Label: "From the website"}})

	edit := fx.adjudication("From the phone")
	edit.Id = created.AppearanceId
	resp := fx.syncSave(t, "edit-1", intPtr(created.Revision), edit)

	if resp.Status != MutationConflict || resp.Revision != created.Revision+1 {
		t.Fatalf("stale edit = %+v, want a conflict at revision %d", resp, created.Revision+1)
	}
	if resp.Appearance == nil || len(resp.Appearance.Results) != 1 || resp.Appearance.Results[0].Label != "From the website" {
		t.Fatalf("conflict aggregate = %+v, want the website's copy to reconcile against", resp.Appearance)
	}
	if got := fx.resultsOf(t, created.AppearanceId); got[0].Label != "From the website" {
		t.Errorf("a conflicting edit was written: %+v", got)
	}

	// Reapplied against the current revision under a new id, it goes through.
	again := fx.syncSave(t, "edit-2", intPtr(resp.Revision), edit)
	if again.Status != MutationApplied {
		t.Fatalf("reapplied edit = %+v, want applied", again)
	}
}

func TestSyncReplayAfterANewerChangeReportsBothRevisions(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))
	edit := fx.adjudication("Platinum")
	edit.Id = created.AppearanceId
	applied := fx.syncSave(t, "edit-1", intPtr(created.Revision), edit)
	fx.setResults(t, created.AppearanceId, []ResultInput{{Kind: ResultKindAward, Label: "Newer"}})

	replay := fx.syncSave(t, "edit-1", intPtr(created.Revision), edit)

	if replay.Status != MutationReplayed || replay.Revision != applied.Revision {
		t.Fatalf("replay = %+v, want the original acknowledgement at revision %d", replay, applied.Revision)
	}
	if replay.Appearance == nil || replay.Appearance.Appearance.Revision <= applied.Revision {
		t.Fatalf("replay aggregate = %+v, want the newer server copy", replay.Appearance)
	}
	if got := fx.resultsOf(t, created.AppearanceId); got[0].Label != "Newer" {
		t.Errorf("a replay reapplied the old edit over a newer one: %+v", got)
	}
}

func TestSyncDeleteLeavesATombstoneAndIsIdempotent(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))

	deleted := fx.syncDelete(t, "delete-1", created.AppearanceId, created.Revision)
	if deleted.Status != MutationApplied || deleted.Revision != created.Revision+1 {
		t.Fatalf("delete = %+v, want applied at revision %d", deleted, created.Revision+1)
	}
	if fx.eventAppearanceCount(t) != 0 || len(fx.resultsOf(t, created.AppearanceId)) != 0 {
		t.Fatal("the delete left the performance or its results")
	}

	if replay := fx.syncDelete(t, "delete-1", created.AppearanceId, created.Revision); replay.Status != MutationReplayed {
		t.Errorf("retried delete = %+v, want replayed", replay)
	}

	edit := fx.adjudication("Late edit")
	edit.Id = created.AppearanceId
	if late := fx.syncSave(t, "edit-1", intPtr(created.Revision), edit); late.Status != MutationDeleted || late.Revision != deleted.Revision {
		t.Errorf("edit after delete = %+v, want deleted at revision %d", late, deleted.Revision)
	}
	if again := fx.syncDelete(t, "delete-2", created.AppearanceId, created.Revision); again.Status != MutationDeleted {
		t.Errorf("second delete = %+v, want deleted", again)
	}
	if fx.eventAppearanceCount(t) != 0 {
		t.Error("an edit after the delete resurrected the performance")
	}
}

func TestSyncDeleteOfAChangedPerformanceIsAConflict(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))
	fx.setResults(t, created.AppearanceId, []ResultInput{{Kind: ResultKindAward, Label: "Newer"}})

	resp := fx.syncDelete(t, "delete-1", created.AppearanceId, created.Revision)

	if resp.Status != MutationConflict || resp.Appearance == nil {
		t.Fatalf("stale delete = %+v, want a conflict with the current copy", resp)
	}
	if fx.eventAppearanceCount(t) != 1 {
		t.Error("a conflicting delete was applied")
	}
}

func TestDeletingTheParentTombstonesItsPerformances(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))
	if _, err := callAs(t, fx, DeleteEvent, EventIdRequest{Id: fx.event.Id}); err != nil {
		t.Fatalf("DeleteEvent() error = %v", err)
	}

	edit := fx.adjudication("Late")
	edit.Id = created.AppearanceId
	if resp := fx.syncSave(t, "edit-1", intPtr(created.Revision), edit); resp.Status != MutationDeleted {
		t.Errorf("edit under a deleted competition = %+v, want deleted", resp)
	}
	if resp := fx.syncSave(t, "create-2", nil, fx.adjudication("Late")); resp.Status != MutationInvalid || resp.Code != InvalidParent {
		t.Errorf("create under a deleted competition = %+v, want invalid %s", resp, InvalidParent)
	}
}

func TestSyncRefusalsCarryACodeAndWriteNothing(t *testing.T) {
	fx := setupResultsFixture(t)
	theirs := fx.addPhoto(t, fx.familyId+999, "theirs.jpg")
	badDate := "14/03/2026"

	cases := []struct {
		name string
		req  SaveAppearanceRequest
		code string
	}{
		{"off the roster", SaveAppearanceRequest{EventId: fx.event.Id, EntryId: fx.entry.Id,
			Results: []ResultInput{{Kind: ResultKindAward, Label: "Solo", PersonId: intPtr(fx.carol.Id)}}}, InvalidPersonNotOnEntry},
		{"placement without a rank", SaveAppearanceRequest{EventId: fx.event.Id, EntryId: fx.entry.Id,
			Results: []ResultInput{{Kind: ResultKindPlacement}}}, InvalidResult},
		{"another family's photo", SaveAppearanceRequest{EventId: fx.event.Id, EntryId: fx.entry.Id,
			PhotoIds: []int{theirs.Id}}, InvalidPhoto},
		{"bad date", SaveAppearanceRequest{EventId: fx.event.Id, EntryId: fx.entry.Id, OccurredAt: &badDate}, InvalidDate},
		{"bad matchup", SaveAppearanceRequest{EventId: fx.event.Id, EntryId: fx.entry.Id,
			Matchup: &MatchupInput{Outcome: "tie-ish"}}, InvalidMatchup},
		{"entry from another season", SaveAppearanceRequest{EventId: fx.event.Id, EntryId: fx.otherEntry.Id}, InvalidEntryNotInSeason},
	}
	for i, tc := range cases {
		id := "refused-" + string(rune('a'+i))
		resp := fx.syncSave(t, id, nil, tc.req)
		if resp.Status != MutationInvalid || resp.Code != tc.code || resp.Message == "" {
			t.Errorf("%s = %+v, want invalid %s with a message", tc.name, resp, tc.code)
		}
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			if _, found := getMutationReceipt(tx, mutationKey(fx.owner.Id, id)); found {
				t.Errorf("%s left a receipt, so a corrected retry under the same id would be refused", tc.name)
			}
		})
	}
	if got := fx.eventAppearanceCount(t); got != 0 {
		t.Errorf("refused saves left %d performances", got)
	}
}

func TestSyncTellsAViewerFromAStranger(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))
	edit := fx.adjudication("Edit")
	edit.Id = created.AppearanceId

	viewer := fx.member(t, "viewer@example.com", AccessView)
	if resp := viewer.syncSave(t, "edit-1", intPtr(created.Revision), edit); resp.Status != MutationForbidden || resp.Appearance == nil {
		t.Errorf("viewer edit = %+v, want forbidden with the copy they can see", resp)
	}
	if resp := viewer.syncSave(t, "create-1", nil, fx.adjudication("Gold")); resp.Status != MutationForbidden {
		t.Errorf("viewer create = %+v, want forbidden", resp)
	}
	if resp := viewer.syncDelete(t, "delete-1", created.AppearanceId, created.Revision); resp.Status != MutationForbidden {
		t.Errorf("viewer delete = %+v, want forbidden", resp)
	}

	stranger := fx.member(t, "stranger@example.com", AccessNone)
	if resp := stranger.syncSave(t, "edit-1", intPtr(created.Revision), edit); resp.Status != MutationNotFound || resp.Appearance != nil {
		t.Errorf("stranger edit = %+v, want not found and nothing shown", resp)
	}
	if fx.storedAppearance(t, created.AppearanceId).Revision != created.Revision {
		t.Error("a refused account's request changed the performance")
	}
}

func TestSyncReceiptsAreScopedToTheirAccount(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.syncSave(t, "shared-id", nil, fx.adjudication("Gold"))

	partner := fx.member(t, "partner@example.com", AccessContribute)
	resp := partner.syncSave(t, "shared-id", nil, fx.adjudication("Platinum"))

	if resp.Status != MutationApplied {
		t.Fatalf("another account's use of the id = %+v, want its own write", resp)
	}
	if got := fx.eventAppearanceCount(t); got != 2 {
		t.Errorf("event has %d performances, want one per account", got)
	}
}

func TestSyncReplayAfterLosingAccessShowsNothing(t *testing.T) {
	fx := setupResultsFixture(t)
	partner := fx.member(t, "partner@example.com", AccessContribute)
	partner.syncSave(t, "create-1", nil, fx.adjudication("Gold"))

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if membership, found := FindMembership(tx, partner.owner.Id, fx.familyId); found {
			deleteMembershipTx(tx, membership)
		}
		vbolt.TxCommit(tx)
	})

	if resp := partner.syncSave(t, "create-1", nil, fx.adjudication("Gold")); resp.Status != MutationNotFound || resp.Appearance != nil {
		t.Errorf("replay after leaving the family = %+v, want not found", resp)
	}
}

func TestFamilyAndAccountDeletionSweepSyncRecords(t *testing.T) {
	fx := setupResultsFixture(t)
	created := fx.syncSave(t, "create-1", nil, fx.adjudication("Gold"))
	fx.syncDelete(t, "delete-1", created.AppearanceId, created.Revision)
	fx.syncSave(t, "create-2", nil, fx.adjudication("Gold"))

	partner := fx.member(t, "partner@example.com", AccessContribute)
	partner.syncSave(t, "create-3", nil, fx.adjudication("Silver"))

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteUserAppearanceMutationsTx(tx, partner.owner.Id)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := getMutationReceipt(tx, mutationKey(partner.owner.Id, "create-3")); found {
			t.Error("the account's receipts survived it")
		}
		if _, found := getMutationReceipt(tx, mutationKey(fx.owner.Id, "create-2")); !found {
			t.Error("deleting one account took another's receipts")
		}
	})

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteFamilyActivitiesTx(tx, fx.familyId)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		var keys []string
		vbolt.ReadTermTargets(tx, AppearanceMutationByFamilyIndex, fx.familyId, &keys, vbolt.Window{})
		if len(keys) != 0 {
			t.Errorf("%d receipts survived the family", len(keys))
		}
		if GetAppearanceTombstone(tx, created.AppearanceId).Id != 0 {
			t.Error("a tombstone survived the family")
		}
	})
}

func TestAppearanceRevisionRoundTrips(t *testing.T) {
	in := Appearance{Id: 3, EventId: 4, EntryId: 5, FamilyId: 6, Revision: 9}
	if out := roundTrip(t, "appearance", &in, PackAppearance); out.Revision != 9 {
		t.Errorf("revision = %d after a round trip, want 9", out.Revision)
	}
	tombstone := AppearanceTombstone{Id: 3, FamilyId: 6, EventId: 4, EntryId: 5, Revision: 10, DeletedAt: time.Unix(1700000000, 0)}
	out := roundTrip(t, "tombstone", &tombstone, PackAppearanceTombstone)
	if out.Id != 3 || out.FamilyId != 6 || out.EventId != 4 || out.EntryId != 5 || out.Revision != 10 || !out.DeletedAt.Equal(tombstone.DeletedAt) {
		t.Errorf("tombstone = %+v after a round trip, want %+v", *out, tombstone)
	}
}
