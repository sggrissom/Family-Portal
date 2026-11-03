package backend

import (
	"errors"
	"strings"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestMomentEntryWithRecordTime(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	add := func(rt RecordTime) (Milestone, error) {
		var m Milestone
		var err error
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			m, err = AddMilestoneTx(tx, AddMilestoneRequest{PersonId: fx.person.Id, Description: "loved painting",
				Category: "other", RecordTime: &rt, UserId: fx.owner.Id}, fx.ownerFamily)
			if err == nil {
				vbolt.TxCommit(tx)
			}
		})
		return m, err
	}

	if _, err := add(agePeriod(fx.person.Id, 4)); !errors.Is(err, ErrApproximateDatesOff) {
		t.Fatalf("with the flag off: %v", err)
	}
	exact, err := add(exactRecordTime(day("2024-06-15"), SourceFamily))
	if err != nil || !exact.MilestoneDate.Equal(day("2024-06-15")) || exact.RecordTime.ChangedBy != fx.owner.Id {
		t.Fatalf("an exact day is allowed with the flag off: %+v %v", exact.RecordTime, err)
	}

	t.Setenv("APPROXIMATE_DATES", "on")
	m, err := add(agePeriod(fx.person.Id, 4))
	if err != nil {
		t.Fatal(err)
	}
	if m.RecordTime.Start != "2024-06-15" || m.RecordTime.Label() != "At age 4" || !m.MilestoneDate.Equal(day("2024-06-15")) {
		t.Errorf("stored %+v at %v", m.RecordTime, m.MilestoneDate)
	}

	forged := agePeriod(fx.person.Id, 4)
	forged.Start, forged.End, forged.Resolved = "1999-01-01", "1999-01-02", true
	if m, _ := add(forged); m.RecordTime.Start != "2024-06-15" {
		t.Errorf("client-sent bounds must be ignored: %+v", m.RecordTime)
	}
	if _, err := add(agePeriod(fx.person.Id, 40)); err == nil {
		t.Error("an age that hasn't started must be rejected")
	}
	if _, err := add(agePeriod(fx.ownPerson.Id, 4)); err == nil {
		t.Error("an anchor from another family must be rejected")
	}
	legacy := agePeriod(fx.person.Id, 4)
	legacy.Source = SourceLegacy
	if _, err := add(legacy); err == nil {
		t.Error("clients can't claim legacy provenance")
	}

	update := func(req UpdateMilestoneRequest) (Milestone, error) {
		var out Milestone
		var err error
		req.Id, req.Category = m.Id, "other"
		if req.Description == "" {
			req.Description = "loved painting"
		}
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			out, err = UpdateMilestoneTx(tx, req, fx.ownerFamily)
			if err == nil {
				vbolt.TxCommit(tx)
			}
		})
		return out, err
	}
	same := "2024-06-15"
	if out, err := update(UpdateMilestoneRequest{InputType: "date", MilestoneDate: &same, Description: "watercolours"}); err != nil || out.RecordTime.Label() != "At age 4" || out.Description != "watercolours" {
		t.Errorf("an older client resending the shown date keeps the period: %+v %v", out.RecordTime, err)
	}
	other := "2024-07-01"
	if _, err := update(UpdateMilestoneRequest{InputType: "date", MilestoneDate: &other}); !errors.Is(err, ErrApproximateDateNeedsUpdate) {
		t.Errorf("a legacy date change must not flatten the period: %v", err)
	}
	if stored := GetMilestoneFromDB(t, fx, m.Id); stored.RecordTime.Label() != "At age 4" {
		t.Errorf("after the rejected edit: %+v", stored.RecordTime)
	}
	party := exactRecordTime(day("2024-06-20"), SourceFamily)
	if out, err := update(UpdateMilestoneRequest{RecordTime: &party}); err != nil || !out.RecordTime.ExactDay() || !out.MilestoneDate.Equal(day("2024-06-20")) {
		t.Errorf("changing back to an exact day: %+v %v %v", out.RecordTime, out.MilestoneDate, err)
	}
}

func GetMilestoneFromDB(t *testing.T, fx rtConsumerFixture, id int) (m Milestone) {
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { m = GetMilestoneById(tx, id) })
	return
}

func TestPhotoUpdateWithRecordTime(t *testing.T) {
	fx := setupRecordTimeConsumers(t)
	t.Setenv("APPROXIMATE_DATES", "on")
	update := func(req UpdatePhotoRequest) (UpdatePhotoResponse, error) {
		var resp UpdatePhotoResponse
		var err error
		req.Id = fx.photo.Id
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = UpdatePhoto(fx.ctx(tx), req)
		})
		return resp, err
	}
	around := RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisCalendar, Precision: PrecisionRange,
		Estimated: true, From: "1968", To: "1972", Around: "1970", Source: SourceInscription, Note: "pencil on the back"}
	resp, err := update(UpdatePhotoRequest{RecordTime: &around})
	if err != nil || resp.Image.RecordTime.Label() != "Around 1970 (1968–1972)" || resp.Image.RecordTime.Source != SourceInscription {
		t.Fatalf("%+v %v", resp.Image.RecordTime, err)
	}
	if strings.HasPrefix(resp.Image.Title, "Photo from") {
		t.Errorf("default title invented a day: %q", resp.Image.Title)
	}
	period := agePeriod(fx.person.Id, 4)
	if _, err := update(UpdatePhotoRequest{RecordTime: &period}); err == nil {
		t.Error("a photo can't describe a period")
	}
	if resp, err := update(UpdatePhotoRequest{InputType: "keep", Title: "Grandma"}); err != nil || resp.Image.RecordTime.Label() != "Around 1970 (1968–1972)" {
		t.Errorf("keep: %+v %v", resp.Image.RecordTime, err)
	}
	if _, err := update(UpdatePhotoRequest{InputType: "today"}); !errors.Is(err, ErrApproximateDateNeedsUpdate) {
		t.Errorf("legacy today on an estimate: %v", err)
	}
}
