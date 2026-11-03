package backend

import (
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

var MilestoneByAnchorIndex = vbolt.Index(&cfg.Info, "milestones_by_record_anchor", vpack.FInt, vpack.FInt)
var ImageByAnchorIndex = vbolt.Index(&cfg.Info, "images_by_record_anchor", vpack.FInt, vpack.FInt)

func unknownRecordTime(source string) RecordTime {
	return RecordTime{
		Version:         RecordTimeVersion,
		Meaning:         MeaningOccurrence,
		Basis:           BasisUnknown,
		Precision:       PrecisionUnknown,
		Source:          source,
		ResolverVersion: recordResolverVersion,
	}
}

func legacyRecordTime(at time.Time) RecordTime {
	if at.IsZero() {
		return unknownRecordTime(SourceLegacy)
	}
	return exactRecordTime(at, SourceLegacy)
}

// syncRecordTime keeps a record's assertion and its legacy timestamp in step.
// An exact day follows the timestamp, which is all older writers change; any
// other assertion owns the timestamp, which becomes its sort key.
func syncRecordTime(tx *vbolt.Tx, rt *RecordTime, at *time.Time) {
	if rt.Version == 0 {
		*rt = legacyRecordTime(*at)
		return
	}
	if rt.ExactDay() {
		if day := civilOf(*at).String(); day != rt.From || !rt.Resolved {
			rt.From = day
			resolveRecordTime(rt, RecordAnchor{})
		}
		return
	}
	anchor := RecordAnchor{}
	if rt.Basis == BasisAge && rt.AnchorPersonId > 0 {
		anchor = anchorOf(GetPersonById(tx, rt.AnchorPersonId))
	}
	resolveRecordTime(rt, anchor)
	key, _ := rt.SortKey()
	*at = key
}

func setExactDate(rt *RecordTime, at *time.Time, value time.Time, source string, userId int) {
	*rt = exactRecordTime(value, source)
	rt.ChangedBy = userId
	rt.ChangedAt = time.Now().UTC()
	*at = value
}

func anchorTerm(rt RecordTime) []int {
	if rt.Basis == BasisAge {
		return positiveTerm(rt.AnchorPersonId)
	}
	return nil
}

func writeImage(tx *vbolt.Tx, image *Image) {
	syncRecordTime(tx, &image.RecordTime, &image.PhotoDate)
	vbolt.Write(tx, ImagesBkt, image.Id, image)
	vbolt.SetTargetTermsPlain(tx, ImageByAnchorIndex, image.Id, anchorTerm(image.RecordTime))
}

func anchoredMilestoneIds(tx *vbolt.Tx, personId int) (ids []int) {
	vbolt.ReadTermTargets(tx, MilestoneByAnchorIndex, personId, &ids, vbolt.Window{})
	return
}

func anchoredImageIds(tx *vbolt.Tx, personId int) (ids []int) {
	vbolt.ReadTermTargets(tx, ImageByAnchorIndex, personId, &ids, vbolt.Window{})
	return
}

// rewriteAnchoredRecords re-resolves every record whose age assertion is
// anchored to personId, after edit changes what it is anchored to.
func rewriteAnchoredRecords(tx *vbolt.Tx, personId int, edit func(*RecordTime)) {
	for _, id := range anchoredMilestoneIds(tx, personId) {
		milestone := GetMilestoneById(tx, id)
		if milestone.Id == 0 {
			continue
		}
		edit(&milestone.RecordTime)
		writeMilestone(tx, &milestone)
	}
	for _, id := range anchoredImageIds(tx, personId) {
		image := GetImageById(tx, id)
		if image.Id == 0 {
			continue
		}
		edit(&image.RecordTime)
		writeImage(tx, &image)
		ReindexPhotoDates(tx, image.Id)
	}
}

// ReresolveAnchoredRecords recomputes the calendar placement of age
// assertions after the anchor's birthday changes; the ages themselves stay.
func ReresolveAnchoredRecords(tx *vbolt.Tx, personId int) {
	rewriteAnchoredRecords(tx, personId, func(*RecordTime) {})
}

func ReanchorRecords(tx *vbolt.Tx, fromPersonId, toPersonId int) {
	rewriteAnchoredRecords(tx, fromPersonId, func(rt *RecordTime) {
		rt.AnchorPersonId = toPersonId
	})
}

// UnanchorRecords keeps a deleted person's age assertions readable but
// unresolved, rather than pointing at a person who no longer exists.
func UnanchorRecords(tx *vbolt.Tx, personId int) {
	rewriteAnchoredRecords(tx, personId, func(rt *RecordTime) {
		rt.AnchorPersonId = 0
	})
}

// BackfillRecordTimes gives every moment and photo written before RecordTime
// existed an exact-day assertion with legacy provenance. That preserves
// today's behaviour; it does not prove those dates were exact. Records that
// already carry an assertion are left alone, so it is safe to re-run.
func BackfillRecordTimes(tx *vbolt.Tx) (milestones, images int) {
	milestoneUpdates := map[int]Milestone{}
	vbolt.IterateAll(tx, MilestoneBkt, func(id int, m Milestone) bool {
		if m.RecordTime.Version == 0 {
			milestoneUpdates[id] = m
		}
		return true
	})
	for _, m := range milestoneUpdates {
		writeMilestone(tx, &m)
	}

	imageUpdates := map[int]Image{}
	vbolt.IterateAll(tx, ImagesBkt, func(id int, image Image) bool {
		if image.RecordTime.Version == 0 {
			imageUpdates[id] = image
		}
		return true
	})
	for _, image := range imageUpdates {
		writeImage(tx, &image)
	}
	return len(milestoneUpdates), len(imageUpdates)
}

// importRecordTime carries an exported assertion into this database. Anchors
// are remapped like any other person reference; one that wasn't imported
// leaves the age readable but unresolved. Files from before RecordTime keep
// their old date behaviour with legacy provenance.
func importRecordTime(exported *RecordTime, at time.Time, personIdMapping map[int]int, allowPeriod bool) (RecordTime, time.Time, error) {
	if exported == nil {
		return legacyRecordTime(at), at, nil
	}
	rt := *exported
	if err := validateRecordTime(rt, allowPeriod); err != nil {
		return rt, at, err
	}
	if rt.Basis == BasisAge {
		rt.AnchorPersonId = personIdMapping[rt.AnchorPersonId]
	}
	rt.Start, rt.End, rt.Resolved = "", "", false
	if rt.ExactDay() {
		if civilOf(at).String() != rt.From {
			start, _, _, _ := parseCivil(rt.From)
			at = start.Time()
		}
		resolveRecordTime(&rt, RecordAnchor{})
	}
	return rt, at, nil
}
