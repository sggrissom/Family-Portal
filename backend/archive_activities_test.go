package backend

import (
	"testing"

	"go.hasen.dev/vbolt"
)

func TestImportedActivitiesJoinTheOnesAFamilyHas(t *testing.T) {
	fx := setupArchiveFixture(t)
	records := fx.exportRecords(t, fx.owner)
	restorer := fx.newUser(t, "restorer")
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		existing := Activity{Id: vbolt.NextIntId(tx, ActivityBkt), FamilyId: restorer.FamilyId, Name: "swim club", Kind: ActivityKindSport}
		writeActivityTx(tx, &existing)
		vbolt.TxCommit(tx)
	})

	resp, err := fx.importRecords(t, restorer, ImportDataRequest{JsonData: records, ImportActivities: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ImportedActivities.Activities != 0 || resp.ImportedActivities.Reused != 1 || resp.ImportedActivities.Seasons != 1 {
		t.Errorf("imported %+v; want the season added to the family's own swim club", resp.ImportedActivities)
	}
	source := readArchiveContents(t, fx.db, fx.familyId)
	restored := readArchiveContents(t, fx.db, restorer.FamilyId)
	if len(restored["activities"]) != 1 || restored["activities"][0]["Name"] != "swim club" {
		t.Errorf("restored activities %v; want only the family's own", restored["activities"])
	}
	for _, bucket := range []string{"seasons", "activity_events", "activity_entries", "appearances", "activity_results"} {
		if len(restored[bucket]) != len(source[bucket]) {
			t.Errorf("restored %d %s; want %d", len(restored[bucket]), bucket, len(source[bucket]))
		}
	}
}

func TestEveryRoundOfABracketSurvivesReimport(t *testing.T) {
	fx := setupArchiveFixture(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		heat := GetFamilyAppearances(tx, fx.familyId)[0]
		for _, round := range []struct{ name, opponent, outcome string }{
			{"Semifinal", "Lane 3", OutcomeWin}, {"Final", "Lane 5", OutcomeLoss},
		} {
			a := heat
			a.Id, a.Revision = vbolt.NextIntId(tx, AppearanceBkt), 0
			a.Round, a.Opponent, a.Outcome = round.name, round.opponent, round.outcome
			writeAppearanceTx(tx, &a)
		}
		vbolt.TxCommit(tx)
	})
	records := fx.exportRecords(t, fx.owner)
	restorer := fx.newUser(t, "restorer")
	req := ImportDataRequest{JsonData: records, ImportActivities: true}

	for range 2 {
		if _, err := fx.importRecords(t, restorer, req); err != nil {
			t.Fatal(err)
		}
	}
	want := readArchiveContents(t, fx.db, fx.familyId).without(perImportFields)["appearances"]
	got := readArchiveContents(t, fx.db, restorer.FamilyId).without(perImportFields)["appearances"]
	for _, d := range diffArchiveContents(archiveContents{"appearances": want}, archiveContents{"appearances": got}) {
		t.Errorf("after importing twice: %s", d)
	}
}
