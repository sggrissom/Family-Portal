package backend

import (
	"family/cfg"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestNormalizeStoredRecordDates(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/calendar_dates.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })

	chicago := time.FixedZone("CDT", -5*3600)
	tokyo := time.FixedZone("JST", 9*3600)
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		people := []Person{
			{Id: 1, Birthday: day(2020, 6, 15)},
			{Id: 2, Birthday: time.Date(2020, 6, 15, 0, 0, 0, 0, chicago)},
			{Id: 3, Birthday: time.Date(2020, 6, 15, 18, 45, 0, 0, time.UTC)},
			{Id: 4},
		}
		for i := range people {
			vbolt.Write(tx, PeopleBkt, people[i].Id, &people[i])
		}
		growth := []GrowthData{
			{Id: 1, PersonId: 1, MeasurementDate: time.Date(2024, 3, 1, 22, 10, 0, 0, chicago)},
			{Id: 2, PersonId: 1, MeasurementDate: time.Date(2024, 3, 2, 4, 0, 0, 0, time.UTC)},
		}
		for i := range growth {
			vbolt.Write(tx, GrowthDataBkt, growth[i].Id, &growth[i])
		}
		milestone := Milestone{Id: 1, PersonId: 1, Description: "Walked", MilestoneDate: time.Date(2024, 5, 1, 7, 0, 0, 0, tokyo)}
		vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
		season := Season{Id: 1, StartDate: time.Date(2024, 9, 1, 12, 0, 0, 0, chicago), EndDate: day(2025, 5, 31)}
		vbolt.Write(tx, SeasonBkt, season.Id, &season)
		event := Event{Id: 1, StartDate: day(2024, 10, 12), EndDate: time.Date(2024, 10, 13, 1, 0, 0, 0, tokyo)}
		vbolt.Write(tx, EventBkt, event.Id, &event)
		appearance := Appearance{Id: 1, OccurredAt: time.Date(2024, 10, 12, 20, 0, 0, 0, chicago)}
		vbolt.Write(tx, AppearanceBkt, appearance.Id, &appearance)
		images := []Image{
			{Id: 1, FamilyId: 1, PhotoDate: time.Date(2024, 7, 4, 21, 30, 15, 0, chicago)},
			{Id: 2, FamilyId: 1, PhotoDate: time.Date(2024, 7, 4, 21, 30, 15, 0, time.UTC)},
			{Id: 3, FamilyId: 1, PhotoDate: day(2024, 7, 4)},
		}
		for i := range images {
			vbolt.Write(tx, ImagesBkt, images[i].Id, &images[i])
		}

		if changed := NormalizeStoredRecordDates(tx); changed != 9 {
			t.Fatalf("changed = %d, want 9", changed)
		}
		if changed := NormalizeStoredRecordDates(tx); changed != 0 {
			t.Fatalf("second pass changed = %d, want 0", changed)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		check := func(name string, got, want time.Time) {
			t.Helper()
			if got != want {
				t.Errorf("%s = %v, want %v", name, got, want)
			}
		}
		var person Person
		for id, want := range map[int]time.Time{1: day(2020, 6, 15), 2: day(2020, 6, 15), 3: day(2020, 6, 15), 4: {}} {
			vbolt.Read(tx, PeopleBkt, id, &person)
			check("birthday", person.Birthday, want)
		}
		var growth GrowthData
		vbolt.Read(tx, GrowthDataBkt, 1, &growth)
		check("measurement in its own offset", growth.MeasurementDate, day(2024, 3, 1))
		vbolt.Read(tx, GrowthDataBkt, 2, &growth)
		check("measurement stored in UTC", growth.MeasurementDate, day(2024, 3, 2))
		var milestone Milestone
		vbolt.Read(tx, MilestoneBkt, 1, &milestone)
		check("milestone", milestone.MilestoneDate, day(2024, 5, 1))
		var season Season
		vbolt.Read(tx, SeasonBkt, 1, &season)
		check("season start", season.StartDate, day(2024, 9, 1))
		check("season end", season.EndDate, day(2025, 5, 31))
		var event Event
		vbolt.Read(tx, EventBkt, 1, &event)
		check("event end", event.EndDate, day(2024, 10, 13))
		var appearance Appearance
		vbolt.Read(tx, AppearanceBkt, 1, &appearance)
		check("appearance", appearance.OccurredAt, day(2024, 10, 12))
		var image Image
		vbolt.Read(tx, ImagesBkt, 1, &image)
		check("offset photo keeps its wall clock", image.PhotoDate, time.Date(2024, 7, 4, 21, 30, 15, 0, time.UTC))
		vbolt.Read(tx, ImagesBkt, 3, &image)
		check("midnight photo", image.PhotoDate, day(2024, 7, 4))

		var indexed []int
		vbolt.ReadTermTargets(tx, ImageByFamilyDateIndex, 1, &indexed, vbolt.Window{})
		if len(indexed) != 3 {
			t.Errorf("family date index holds %v, want all three photos", indexed)
		}
	})
}

func TestWallClockUTC(t *testing.T) {
	captured := time.Date(2024, 12, 31, 23, 15, 0, 0, time.FixedZone("PST", -8*3600))
	if got := wallClockUTC(captured); got != time.Date(2024, 12, 31, 23, 15, 0, 0, time.UTC) {
		t.Fatalf("wallClockUTC = %v, want the capture day and time labelled UTC", got)
	}
}
