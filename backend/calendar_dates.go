package backend

import (
	"time"

	"go.hasen.dev/vbolt"
)

// Record dates (birthdays, measurements, milestones, activity dates) are
// calendar days stored as UTC midnight. Photo dates are the local wall-clock
// time of capture, stored with a UTC label so their UTC day is the capture day.

func wallClockUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

func normalizedDay(t *time.Time) bool {
	return normalizeTo(t, dayStart(*t))
}

func normalizedWallClock(t *time.Time) bool {
	return normalizeTo(t, wallClockUTC(*t))
}

func normalizeTo(t *time.Time, normalized time.Time) bool {
	if t.Equal(normalized) && t.Location() == time.UTC {
		return false
	}
	*t = normalized
	return true
}

// NormalizeStoredRecordDates rewrites stored record dates as UTC-midnight
// calendar days and photo dates as UTC-labelled wall-clock times, reading each
// in the offset it was saved with.
func NormalizeStoredRecordDates(tx *vbolt.Tx) (changed int) {
	changed += normalizeBucket(tx, PeopleBkt, func(p *Person) bool {
		return normalizedDay(&p.Birthday)
	})
	changed += normalizeBucket(tx, GrowthDataBkt, func(g *GrowthData) bool {
		return normalizedDay(&g.MeasurementDate)
	})
	milestones := normalizeBucket(tx, MilestoneBkt, func(m *Milestone) bool {
		return normalizedDay(&m.MilestoneDate)
	})
	changed += milestones
	changed += normalizeBucket(tx, SeasonBkt, func(s *Season) bool {
		start := normalizedDay(&s.StartDate)
		end := normalizedDay(&s.EndDate)
		return start || end
	})
	changed += normalizeBucket(tx, EventBkt, func(e *Event) bool {
		start := normalizedDay(&e.StartDate)
		end := normalizedDay(&e.EndDate)
		return start || end
	})
	changed += normalizeBucket(tx, AppearanceBkt, func(a *Appearance) bool {
		return normalizedDay(&a.OccurredAt)
	})
	photos := normalizeBucket(tx, ImagesBkt, func(i *Image) bool {
		return normalizedWallClock(&i.PhotoDate)
	})
	changed += photos

	if milestones > 0 {
		RebuildMilestoneSearchIndex(tx)
	}
	if photos > 0 {
		BackfillPhotoDateIndexes(tx)
	}
	return changed
}

func normalizeBucket[T any](tx *vbolt.Tx, bucket *vbolt.BucketInfo[int, T], normalize func(*T) bool) int {
	updates := map[int]T{}
	vbolt.IterateAll(tx, bucket, func(id int, item T) bool {
		if normalize(&item) {
			updates[id] = item
		}
		return true
	})
	for id, item := range updates {
		vbolt.Write(tx, bucket, id, &item)
	}
	return len(updates)
}
