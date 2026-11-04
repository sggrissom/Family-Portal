package backend

import (
	"family/cfg"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func recordTimeTestDB(t *testing.T) *vbolt.DB {
	t.Helper()
	db := vbolt.Open(filepath.Join(t.TempDir(), "record_time.db"))
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { db.Close() })
	return db
}

func agePeriod(anchor, years int) RecordTime {
	return RecordTime{Version: RecordTimeVersion, Meaning: MeaningPeriod, Basis: BasisAge,
		Precision: PrecisionYear, AnchorPersonId: anchor, AgeYears: years, Source: SourceFamily}
}

func TestPackMilestoneReadsV3AsLegacyExact(t *testing.T) {
	packV3 := func(self *Milestone, buf *vpack.Buffer) {
		vpack.Version(3, buf)
		vpack.Int(&self.Id, buf)
		vpack.Int(&self.PersonId, buf)
		vpack.Int(&self.FamilyId, buf)
		vpack.String(&self.Description, buf)
		vpack.String(&self.Category, buf)
		vpack.Time(&self.MilestoneDate, buf)
		vpack.Time(&self.CreatedAt, buf)
		vpack.String(&self.Context, buf)
		vpack.Int(&self.InterviewId, buf)
		vpack.Int(&self.QuestionId, buf)
	}
	old := Milestone{Id: 4, PersonId: 2, FamilyId: 1, Description: "x", Category: "first",
		MilestoneDate: day("2024-06-15"), CreatedAt: day("2024-06-16")}
	var got Milestone
	vpack.FromBytesInto(vpack.ToBytes(&old, packV3), &got, PackMilestone)
	if got.Id != 4 || !got.MilestoneDate.Equal(old.MilestoneDate) || got.RecordTime.Source != SourceLegacy || got.RecordTime.From != "2024-06-15" {
		t.Fatalf("v3 decode = %+v", got)
	}
}

func TestBackfillRecordTimesIsLegacyExactAndIdempotent(t *testing.T) {
	db := recordTimeTestDB(t)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		dated := Milestone{Id: 1, PersonId: 1, FamilyId: 1, Description: "a", MilestoneDate: day("2024-06-15")}
		undated := Milestone{Id: 2, PersonId: 1, FamilyId: 1, Description: "b"}
		photo := Image{Id: 1, FamilyId: 1, PhotoDate: time.Date(2023, 5, 1, 18, 30, 0, 0, time.UTC)}
		vbolt.Write(tx, MilestoneBkt, 1, &dated)
		vbolt.Write(tx, MilestoneBkt, 2, &undated)
		vbolt.Write(tx, ImagesBkt, 1, &photo)
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		milestones, images := BackfillRecordTimes(tx)
		if milestones != 2 || images != 1 {
			t.Errorf("backfilled %d moments, %d photos", milestones, images)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		dated := GetMilestoneById(tx, 1)
		if !dated.RecordTime.ExactDay() || dated.RecordTime.Source != SourceLegacy || dated.RecordTime.From != "2024-06-15" {
			t.Errorf("dated = %+v", dated.RecordTime)
		}
		if undated := GetMilestoneById(tx, 2); undated.RecordTime.Basis != BasisUnknown || undated.RecordTime.Resolved {
			t.Errorf("a zero date must become unknown, not year 1: %+v", undated.RecordTime)
		}
		photo := GetImageById(tx, 1)
		if photo.RecordTime.From != "2023-05-01" || photo.PhotoDate.Hour() != 18 {
			t.Errorf("photo = %+v at %v", photo.RecordTime, photo.PhotoDate)
		}

		before := GetMilestoneById(tx, 1)
		BackfillRecordTimes(tx)
		if after := GetMilestoneById(tx, 1); after.RecordTime != before.RecordTime || !after.MilestoneDate.Equal(before.MilestoneDate) {
			t.Errorf("re-run changed %+v to %+v", before.RecordTime, after.RecordTime)
		}

		edited := GetMilestoneById(tx, 1)
		setExactDate(&edited.RecordTime, &edited.MilestoneDate, day("2024-06-16"), SourceFamily, 1)
		writeMilestone(tx, &edited)
		if milestones, _ := BackfillRecordTimes(tx); milestones != 1 {
			t.Errorf("re-run touched %d moments, want only the legacy one", milestones)
		}
	})
}

func TestSyncRecordTime(t *testing.T) {
	db := recordTimeTestDB(t)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		child := Person{Id: 1, FamilyId: 1, Birthday: day("2020-06-15")}
		vbolt.Write(tx, PeopleBkt, 1, &child)

		exact := Milestone{Id: 1, PersonId: 1, FamilyId: 1, RecordTime: exactRecordTime(day("2024-01-01"), SourceFamily)}
		exact.MilestoneDate = day("2024-02-02")
		writeMilestone(tx, &exact)
		if exact.RecordTime.From != "2024-02-02" || exact.RecordTime.Source != SourceFamily {
			t.Errorf("an exact day follows its legacy timestamp: %+v", exact.RecordTime)
		}

		period := Milestone{Id: 2, PersonId: 1, FamilyId: 1, RecordTime: agePeriod(1, 4), MilestoneDate: day("2030-01-01")}
		writeMilestone(tx, &period)
		if !period.MilestoneDate.Equal(day("2024-06-15")) || period.RecordTime.End != "2025-06-15" {
			t.Errorf("a period owns its sort key: %v %+v", period.MilestoneDate, period.RecordTime)
		}
		if ids := anchoredMilestoneIds(tx, 1); len(ids) != 1 || ids[0] != 2 {
			t.Errorf("anchored = %v", ids)
		}
	})
}

func TestAnchoredRecordsFollowTheirPerson(t *testing.T) {
	db := recordTimeTestDB(t)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, p := range []Person{
			{Id: 1, FamilyId: 1, Birthday: day("2020-06-15")},
			{Id: 2, FamilyId: 1, Birthday: day("2018-01-10")},
		} {
			vbolt.Write(tx, PeopleBkt, p.Id, &p)
		}
		m := Milestone{Id: 1, PersonId: 1, FamilyId: 1, RecordTime: agePeriod(1, 4)}
		writeMilestone(tx, &m)
		photo := Image{Id: 1, FamilyId: 1, RecordTime: agePeriod(1, 4)}
		photo.RecordTime.Meaning = MeaningOccurrence
		writeImage(tx, &photo)

		child := GetPersonById(tx, 1)
		child.Birthday = day("2020-07-01")
		vbolt.Write(tx, PeopleBkt, 1, &child)
		ReresolveAnchoredRecords(tx, 1)
		m = GetMilestoneById(tx, 1)
		if m.RecordTime.Start != "2024-07-01" || m.RecordTime.Label() != "At age 4" || !m.MilestoneDate.Equal(day("2024-07-01")) {
			t.Errorf("after birthday fix: %+v %v", m.RecordTime, m.MilestoneDate)
		}
		if p := GetImageById(tx, 1); p.RecordTime.Start != "2024-07-01" {
			t.Errorf("photo after birthday fix: %+v", p.RecordTime)
		}

		ReanchorRecords(tx, 1, 2)
		m = GetMilestoneById(tx, 1)
		if m.RecordTime.AnchorPersonId != 2 || m.RecordTime.Start != "2022-01-10" {
			t.Errorf("after merge: %+v", m.RecordTime)
		}
		if ids := anchoredMilestoneIds(tx, 1); len(ids) != 0 {
			t.Errorf("old anchor still indexed: %v", ids)
		}

		UnanchorRecords(tx, 2)
		m = GetMilestoneById(tx, 1)
		if m.RecordTime.AnchorPersonId != 0 || m.RecordTime.Resolved || m.RecordTime.Label() != "At age 4" || !m.MilestoneDate.IsZero() {
			t.Errorf("after deletion: %+v %v", m.RecordTime, m.MilestoneDate)
		}
	})
}

func TestImportRecordTime(t *testing.T) {
	period := agePeriod(10, 4)
	rt, _, err := importRecordTime(&period, time.Time{}, map[int]int{10: 77}, true)
	if err != nil || rt.AnchorPersonId != 77 || rt.Resolved {
		t.Errorf("anchor remap: %+v %v", rt, err)
	}
	if _, _, err := importRecordTime(&period, time.Time{}, nil, false); err == nil {
		t.Error("photos can't import a period assertion")
	}

	exact := exactRecordTime(day("2022-10-31"), SourceFamily)
	rt, at, err := importRecordTime(&exact, time.Date(2022, 10, 31, 15, 4, 0, 0, time.UTC), nil, false)
	if err != nil || at.Hour() != 15 || rt.Source != SourceFamily {
		t.Errorf("exact photo keeps its wall clock: %+v %v %v", rt, at, err)
	}

	rt, at, _ = importRecordTime(nil, day("2019-03-03"), nil, true)
	if rt.Source != SourceLegacy || !rt.ExactDay() || !at.Equal(day("2019-03-03")) {
		t.Errorf("legacy import: %+v %v", rt, at)
	}

	future := RecordTime{Version: 2}
	if _, _, err := importRecordTime(&future, time.Time{}, nil, true); err == nil {
		t.Error("an unsupported version must be rejected")
	}
}

func TestAddMilestoneRecordsFamilyProvenance(t *testing.T) {
	db := recordTimeTestDB(t)
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		child := Person{Id: 1, FamilyId: 1, Birthday: day("2020-06-15")}
		vbolt.Write(tx, PeopleBkt, 1, &child)
		m, err := AddMilestoneTx(tx, AddMilestoneRequest{PersonId: 1, Description: "x", Category: "first",
			RecordTime: onDay("2022-10-31")}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !m.RecordTime.ExactDay() || m.RecordTime.Source != SourceFamily || m.RecordTime.From != "2022-10-31" {
			t.Errorf("record time = %+v", m.RecordTime)
		}
		if stored := GetMilestoneById(tx, m.Id); stored.RecordTime != m.RecordTime {
			t.Errorf("stored %+v, returned %+v", stored.RecordTime, m.RecordTime)
		}
	})
}
