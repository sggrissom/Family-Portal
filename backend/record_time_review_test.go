package backend

import (
	"errors"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestBirthdayClusterReviewCorrectsAndUndoes(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	four, two := 4, 2
	var ids []int
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, text := range []string{"loved painting", "afraid of the dark", "favourite song was Baby Shark"} {
			m, err := AddMilestoneTx(tx, AddMilestoneRequest{PersonId: fx.person.Id, Description: text, Category: "other",
				InputType: "age", AgeYears: &four}, fx.ownerFamily)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, m.Id)
		}
		if _, err := AddMilestoneTx(tx, AddMilestoneRequest{PersonId: fx.person.Id, Description: "only one at two",
			Category: "other", InputType: "age", AgeYears: &two}, fx.ownerFamily); err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})
	fx.addPhoto("cake", legacyRecordTime(day("2024-06-15")), day("2024-06-15"))

	var clusters ListBirthdayClustersResponse
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		clusters, err = ListBirthdayClusters(fx.ctx(tx), Empty{})
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(clusters.Clusters) != 1 {
		t.Fatalf("clusters = %+v, want only the age-4 one", clusters.Clusters)
	}
	cluster := clusters.Clusters[0]
	if cluster.Age != 4 || cluster.Day != "2024-06-15" || len(cluster.Records) != 4 {
		t.Fatalf("cluster = %+v", cluster)
	}

	outsiderToken, _ := generateJwtTokenString(fx.outsider)
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		theirs, _ := ListBirthdayClusters(&vbeam.Context{Tx: tx, Token: outsiderToken}, Empty{})
		if len(theirs.Clusters) != 0 {
			t.Errorf("an outsider saw another family's records: %+v", theirs.Clusters)
		}
	})

	period := agePeriod(fx.person.Id, 4)
	changes := []RecordTimeChange{{Kind: RecordKindMoment, Id: ids[0], RecordTime: period}, {Kind: RecordKindMoment, Id: ids[1], RecordTime: period}}
	correct := func(token string, changes []RecordTimeChange) (resp CorrectRecordTimesResponse, err error) {
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = CorrectRecordTimes(&vbeam.Context{Tx: tx, Token: token}, CorrectRecordTimesRequest{Changes: changes})
		})
		return
	}
	ownerToken, _ := generateJwtTokenString(fx.owner)

	if _, err := correct(ownerToken, changes); !errors.Is(err, ErrApproximateDatesOff) {
		t.Fatalf("with the flag off: %v", err)
	}
	t.Setenv("APPROXIMATE_DATES", "on")
	if _, err := correct(outsiderToken, changes); err == nil {
		t.Fatal("an outsider corrected another family's moments")
	}
	if m := GetMilestoneFromDB(t, fx, ids[0]); !m.RecordTime.ExactDay() {
		t.Fatal("a refused correction must not change anything")
	}

	resp, err := correct(ownerToken, changes)
	if err != nil || resp.Changed != 2 {
		t.Fatalf("correct = %+v %v", resp, err)
	}
	if m := GetMilestoneFromDB(t, fx, ids[0]); m.RecordTime.Label() != "At age 4" || m.RecordTime.ChangedBy != fx.owner.Id {
		t.Errorf("after correction: %+v", m.RecordTime)
	}
	if m := GetMilestoneFromDB(t, fx, ids[2]); !m.RecordTime.ExactDay() {
		t.Error("a record left out of the correction keeps its exact day")
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		dashboard, err := GetDashboard(fx.ctx(tx), GetDashboardRequest{Today: "2025-06-15"})
		if err != nil {
			t.Fatal(err)
		}
		for _, year := range dashboard.OnThisDay {
			for _, m := range year.Milestones {
				if m.Id == ids[0] || m.Id == ids[1] {
					t.Errorf("a corrected moment is still an anniversary: %q", m.Description)
				}
			}
		}
	})

	edited := exactRecordTime(day("2024-07-01"), SourceFamily)
	if _, err := correct(ownerToken, []RecordTimeChange{{Kind: RecordKindMoment, Id: ids[1], RecordTime: edited}}); err != nil {
		t.Fatal(err)
	}

	var undo UndoRecordTimeCorrectionResponse
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		undo, err = UndoRecordTimeCorrection(&vbeam.Context{Tx: tx, Token: ownerToken}, UndoRecordTimeCorrectionRequest{CorrectionId: resp.CorrectionId})
	})
	if err != nil || undo.Restored != 1 || undo.Skipped != 1 {
		t.Fatalf("undo = %+v %v; the record edited since should be skipped", undo, err)
	}
	if m := GetMilestoneFromDB(t, fx, ids[0]); !m.RecordTime.ExactDay() || m.RecordTime.From != "2024-06-15" || m.RecordTime.Source != SourceFamily {
		t.Errorf("after undo: %+v", m.RecordTime)
	}
	if m := GetMilestoneFromDB(t, fx, ids[1]); m.RecordTime.From != "2024-07-01" {
		t.Errorf("undo overwrote a later edit: %+v", m.RecordTime)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		_, err = UndoRecordTimeCorrection(&vbeam.Context{Tx: tx, Token: ownerToken}, UndoRecordTimeCorrectionRequest{CorrectionId: resp.CorrectionId})
	})
	if !errors.Is(err, ErrCorrectionNotFound) {
		t.Errorf("a second undo: %v", err)
	}
}
