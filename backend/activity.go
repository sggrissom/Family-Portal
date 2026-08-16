// Competitive activities: seasons, competitions, routines, and results.
//
// The schema is deliberately activity-agnostic. Nothing in this file knows the
// word "routine" or "performance" — dance vocabulary lives in the frontend
// label map keyed by Activity.Kind, so a sport label pack is a second entry in
// that map rather than a second set of tables. See docs/activities-plan.md.
//
// The hinge is Appearance: one Entry at one Event. Both of the views this
// feature exists to serve fall out of it as an index walk rather than a scan —
// "how did this routine do across competitions?" is AppearanceByEntryIndex, and
// "how did this competition go?" is AppearanceByEventIndex. Neither is
// privileged over the other.
package backend

import (
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Activity is a program a family participates in. Kind drives vocabulary and
// nothing else — the schema below is identical for dance, soccer, and swim.
type Activity struct {
	Id        int       `json:"id"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"` // "Dance"
	Kind      string    `json:"kind"` // ActivityKind* below
	CreatedAt time.Time `json:"createdAt"`
}

const (
	ActivityKindDance   = "dance"
	ActivityKindSport   = "sport"
	ActivityKindGeneric = "generic"
)

type Season struct {
	Id         int       `json:"id"`
	ActivityId int       `json:"activityId"`
	FamilyId   int       `json:"familyId"`
	Name       string    `json:"name"` // "2025-26 Competition Season"
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Event is one competition (sport: one game, meet, or tournament).
type Event struct {
	Id        int       `json:"id"`
	SeasonId  int       `json:"seasonId"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`     // "Nuvo Nashville"
	Host      string    `json:"host"`     // free text: "Nuvo", "Showstopper"
	Location  string    `json:"location"` //
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"` // zero for single-day events
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
}

// Entry is the recurring competitive unit within a season — a routine in dance,
// a team in soccer, an event ("50 Free") in swim.
//
// It is season-scoped and has no lineage: a routine's roster, age division, and
// competitive level are properties of a season, so binding Entry to Season keeps
// them accurate without a per-season overlay. A group that carries over year to
// year is re-created rather than reused.
type Entry struct {
	Id        int       `json:"id"`
	SeasonId  int       `json:"seasonId"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`     // "Rise Up"
	Format    string    `json:"format"`   // "solo" | "duet" | "trio" | "group" (free text; sport: "team")
	Style     string    `json:"style"`    // "Jazz", "Lyrical" (sport: position/discipline)
	Division  string    `json:"division"` // "Teen", "Senior"
	Level     string    `json:"level"`    // "Elite", "Rec"
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
}

// EntryMember is the roster join. Two siblings in the same group dance are two
// rows; one child in eight dances is eight rows.
type EntryMember struct {
	Id        int       `json:"id"`
	EntryId   int       `json:"entryId"`
	PersonId  int       `json:"personId"`
	FamilyId  int       `json:"familyId"`
	CreatedAt time.Time `json:"createdAt"`
}

// Appearance is one Entry at one Event.
//
// The name is deliberately colorless. The obvious word in dance is
// "performance", but you do not go see a soccer performance, and this is the one
// table that has to hold still across every activity. The domain word comes back
// at the label layer, not here.
type Appearance struct {
	Id         int       `json:"id"`
	EventId    int       `json:"eventId"`
	EntryId    int       `json:"entryId"`
	FamilyId   int       `json:"familyId"`
	OccurredAt time.Time `json:"occurredAt"` // zero if unknown; ordering falls back to Event.StartDate
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Result is deliberately one flat record with a Kind discriminator rather than
// four tables. The fields a placement uses (Rank, OutOf, Category) and the ones
// a score uses (Score) are disjoint, but they are all small, all optional, and
// all read together — splitting them buys nothing and costs three more buckets.
//
// Adjudication labels are free text. A season view can list and count them by
// exact label, but it cannot rank or trend them; ordering is a later, additive
// change (a ScaleId/TierRank pair behind a PackResult version bump).
type Result struct {
	Id           int       `json:"id"`
	AppearanceId int       `json:"appearanceId"`
	FamilyId     int       `json:"familyId"`
	Kind         string    `json:"kind"`               // ResultKind* below
	Label        string    `json:"label"`              // "High Gold", "Judges' Choice", "Overall"
	Rank         *int      `json:"rank,omitempty"`     // placement: 1, 2, 3...
	OutOf        *int      `json:"outOf,omitempty"`    // placement: "...of 14"
	Category     string    `json:"category"`           // "Teen Small Group Jazz"
	Score        *float64  `json:"score,omitempty"`    // numeric score or time
	PersonId     *int      `json:"personId,omitempty"` // narrows an award to one dancer in a group
	Notes        string    `json:"notes"`
	SortOrder    int       `json:"sortOrder"` // display order within an appearance
	CreatedAt    time.Time `json:"createdAt"`
}

const (
	ResultKindAdjudication = "adjudication" // "Diamond", "High Gold", "Blown Speaker"
	ResultKindPlacement    = "placement"    // Rank / OutOf / Category
	ResultKindAward        = "award"        // judges' award, special award, title
	ResultKindScore        = "score"        // numeric — sports and scored dance formats
)

// AppearancePhoto joins photos to one routine at one competition.
type AppearancePhoto struct {
	Id           int       `json:"id"`
	AppearanceId int       `json:"appearanceId"`
	PhotoId      int       `json:"photoId"`
	FamilyId     int       `json:"familyId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// EventPhoto joins photos to the competition itself — the ones from the weekend
// that are not of any one routine.
type EventPhoto struct {
	Id        int       `json:"id"`
	EventId   int       `json:"eventId"`
	PhotoId   int       `json:"photoId"`
	FamilyId  int       `json:"familyId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ── packing ───────────────────────────────────────────────────────────────────

// packOptionalInt stores a nil-able int as a present flag followed by the value,
// so "no placement" stays distinguishable from "1st". vpack has no native
// optional, and a zero sentinel would be wrong for Rank and actively misleading
// for Score.
func packOptionalInt(n **int, buf *vpack.Buffer) {
	if buf.Writing {
		present := *n != nil
		vpack.Bool(&present, buf)
		if present {
			v := **n
			vpack.Int(&v, buf)
		}
		return
	}
	var present bool
	vpack.Bool(&present, buf)
	if !present {
		*n = nil
		return
	}
	var v int
	vpack.Int(&v, buf)
	*n = &v
}

func packOptionalFloat64(n **float64, buf *vpack.Buffer) {
	if buf.Writing {
		present := *n != nil
		vpack.Bool(&present, buf)
		if present {
			v := **n
			vpack.Float64(&v, buf)
		}
		return
	}
	var present bool
	vpack.Bool(&present, buf)
	if !present {
		*n = nil
		return
	}
	var v float64
	vpack.Float64(&v, buf)
	*n = &v
}

func PackActivity(self *Activity, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Kind, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackSeason(self *Season, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.ActivityId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.Time(&self.StartDate, buf)
	vpack.Time(&self.EndDate, buf)
	vpack.String(&self.Notes, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackEvent(self *Event, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.SeasonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Host, buf)
	vpack.String(&self.Location, buf)
	vpack.Time(&self.StartDate, buf)
	vpack.Time(&self.EndDate, buf)
	vpack.String(&self.Notes, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackEntry(self *Entry, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.SeasonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Format, buf)
	vpack.String(&self.Style, buf)
	vpack.String(&self.Division, buf)
	vpack.String(&self.Level, buf)
	vpack.String(&self.Notes, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackEntryMember(self *EntryMember, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.EntryId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackAppearance(self *Appearance, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.EventId, buf)
	vpack.Int(&self.EntryId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.OccurredAt, buf)
	vpack.String(&self.Notes, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackResult(self *Result, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.AppearanceId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Kind, buf)
	vpack.String(&self.Label, buf)
	packOptionalInt(&self.Rank, buf)
	packOptionalInt(&self.OutOf, buf)
	vpack.String(&self.Category, buf)
	packOptionalFloat64(&self.Score, buf)
	packOptionalInt(&self.PersonId, buf)
	vpack.String(&self.Notes, buf)
	vpack.Int(&self.SortOrder, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackAppearancePhoto(self *AppearancePhoto, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.AppearanceId, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

func PackEventPhoto(self *EventPhoto, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.EventId, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

// ── buckets ───────────────────────────────────────────────────────────────────

var ActivityBkt = vbolt.Bucket(&cfg.Info, "activities", vpack.FInt, PackActivity)
var SeasonBkt = vbolt.Bucket(&cfg.Info, "seasons", vpack.FInt, PackSeason)
var EventBkt = vbolt.Bucket(&cfg.Info, "activity_events", vpack.FInt, PackEvent)
var EntryBkt = vbolt.Bucket(&cfg.Info, "activity_entries", vpack.FInt, PackEntry)
var EntryMemberBkt = vbolt.Bucket(&cfg.Info, "entry_members", vpack.FInt, PackEntryMember)
var AppearanceBkt = vbolt.Bucket(&cfg.Info, "appearances", vpack.FInt, PackAppearance)
var ResultBkt = vbolt.Bucket(&cfg.Info, "activity_results", vpack.FInt, PackResult)
var AppearancePhotoBkt = vbolt.Bucket(&cfg.Info, "appearance_photos", vpack.FInt, PackAppearancePhoto)
var EventPhotoBkt = vbolt.Bucket(&cfg.Info, "activity_event_photos", vpack.FInt, PackEventPhoto)

// ── indexes ───────────────────────────────────────────────────────────────────

// ActivityByFamilyIndex: term = family_id, target = activity_id
var ActivityByFamilyIndex = vbolt.Index(&cfg.Info, "activity_by_family", vpack.FInt, vpack.FInt)

// SeasonByActivityIndex: term = activity_id, target = season_id
var SeasonByActivityIndex = vbolt.Index(&cfg.Info, "season_by_activity", vpack.FInt, vpack.FInt)

// SeasonByFamilyIndex: term = family_id, target = season_id
var SeasonByFamilyIndex = vbolt.Index(&cfg.Info, "season_by_family", vpack.FInt, vpack.FInt)

// EventBySeasonIndex: term = season_id, target = event_id
var EventBySeasonIndex = vbolt.Index(&cfg.Info, "activity_event_by_season", vpack.FInt, vpack.FInt)

// EventByFamilyIndex: term = family_id, target = event_id
var EventByFamilyIndex = vbolt.Index(&cfg.Info, "activity_event_by_family", vpack.FInt, vpack.FInt)

// EntryBySeasonIndex: term = season_id, target = entry_id
var EntryBySeasonIndex = vbolt.Index(&cfg.Info, "activity_entry_by_season", vpack.FInt, vpack.FInt)

// EntryByFamilyIndex: term = family_id, target = entry_id
var EntryByFamilyIndex = vbolt.Index(&cfg.Info, "activity_entry_by_family", vpack.FInt, vpack.FInt)

// EntryMemberByEntryIndex: term = entry_id, target = entry_member_id
var EntryMemberByEntryIndex = vbolt.Index(&cfg.Info, "entry_member_by_entry", vpack.FInt, vpack.FInt)

// EntryMemberByPersonIndex: term = person_id, target = entry_member_id.
// This is what answers "which routines is this kid in?" without a scan.
var EntryMemberByPersonIndex = vbolt.Index(&cfg.Info, "entry_member_by_person", vpack.FInt, vpack.FInt)

// EntryMemberByFamilyIndex: term = family_id, target = entry_member_id
var EntryMemberByFamilyIndex = vbolt.Index(&cfg.Info, "entry_member_by_family", vpack.FInt, vpack.FInt)

// AppearanceByEventIndex: term = event_id, target = appearance_id — competition view.
var AppearanceByEventIndex = vbolt.Index(&cfg.Info, "appearance_by_event", vpack.FInt, vpack.FInt)

// AppearanceByEntryIndex: term = entry_id, target = appearance_id — routine-across-competitions view.
var AppearanceByEntryIndex = vbolt.Index(&cfg.Info, "appearance_by_entry", vpack.FInt, vpack.FInt)

// AppearanceByFamilyIndex: term = family_id, target = appearance_id
var AppearanceByFamilyIndex = vbolt.Index(&cfg.Info, "appearance_by_family", vpack.FInt, vpack.FInt)

// ResultByAppearanceIndex: term = appearance_id, target = result_id
var ResultByAppearanceIndex = vbolt.Index(&cfg.Info, "activity_result_by_appearance", vpack.FInt, vpack.FInt)

// ResultByPersonIndex: term = person_id, target = result_id. Written only for
// results that name a person — an award given to one dancer inside a group.
var ResultByPersonIndex = vbolt.Index(&cfg.Info, "activity_result_by_person", vpack.FInt, vpack.FInt)

// ResultByFamilyIndex: term = family_id, target = result_id
var ResultByFamilyIndex = vbolt.Index(&cfg.Info, "activity_result_by_family", vpack.FInt, vpack.FInt)

// AppearancePhotoBy*: the by-photo index is not optional — deleting a photo has
// to clear its joins, exactly as MilestonePhotoByPhotoIndex does today.
var AppearancePhotoByAppearanceIndex = vbolt.Index(&cfg.Info, "appearance_photo_by_appearance", vpack.FInt, vpack.FInt)
var AppearancePhotoByPhotoIndex = vbolt.Index(&cfg.Info, "appearance_photo_by_photo", vpack.FInt, vpack.FInt)
var AppearancePhotoByFamilyIndex = vbolt.Index(&cfg.Info, "appearance_photo_by_family", vpack.FInt, vpack.FInt)

var EventPhotoByEventIndex = vbolt.Index(&cfg.Info, "activity_event_photo_by_event", vpack.FInt, vpack.FInt)
var EventPhotoByPhotoIndex = vbolt.Index(&cfg.Info, "activity_event_photo_by_photo", vpack.FInt, vpack.FInt)
var EventPhotoByFamilyIndex = vbolt.Index(&cfg.Info, "activity_event_photo_by_family", vpack.FInt, vpack.FInt)

// ── reads ─────────────────────────────────────────────────────────────────────

func GetActivityById(tx *vbolt.Tx, id int) (activity Activity) {
	vbolt.Read(tx, ActivityBkt, id, &activity)
	return
}

func GetSeasonById(tx *vbolt.Tx, id int) (season Season) {
	vbolt.Read(tx, SeasonBkt, id, &season)
	return
}

func GetEventById(tx *vbolt.Tx, id int) (event Event) {
	vbolt.Read(tx, EventBkt, id, &event)
	return
}

func GetEntryById(tx *vbolt.Tx, id int) (entry Entry) {
	vbolt.Read(tx, EntryBkt, id, &entry)
	return
}

func GetAppearanceById(tx *vbolt.Tx, id int) (appearance Appearance) {
	vbolt.Read(tx, AppearanceBkt, id, &appearance)
	return
}

func GetResultById(tx *vbolt.Tx, id int) (result Result) {
	vbolt.Read(tx, ResultBkt, id, &result)
	return
}

// readByTerm is the ReadTermTargets/ReadSlice pair every list below is made of.
// vbolt.ReadSlice on an empty id list is avoided the same way milestone.go
// avoids it.
func readByTerm[T any](tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], bkt *vbolt.BucketInfo[int, T], term int) []T {
	items := []T{}
	var ids []int
	vbolt.ReadTermTargets(tx, index, term, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, bkt, ids, &items)
	}
	return items
}

func GetFamilyActivities(tx *vbolt.Tx, familyId int) []Activity {
	return readByTerm(tx, ActivityByFamilyIndex, ActivityBkt, familyId)
}

func GetActivitySeasons(tx *vbolt.Tx, activityId int) []Season {
	return readByTerm(tx, SeasonByActivityIndex, SeasonBkt, activityId)
}

func GetFamilySeasons(tx *vbolt.Tx, familyId int) []Season {
	return readByTerm(tx, SeasonByFamilyIndex, SeasonBkt, familyId)
}

func GetSeasonEvents(tx *vbolt.Tx, seasonId int) []Event {
	return readByTerm(tx, EventBySeasonIndex, EventBkt, seasonId)
}

func GetFamilyEvents(tx *vbolt.Tx, familyId int) []Event {
	return readByTerm(tx, EventByFamilyIndex, EventBkt, familyId)
}

func GetSeasonEntries(tx *vbolt.Tx, seasonId int) []Entry {
	return readByTerm(tx, EntryBySeasonIndex, EntryBkt, seasonId)
}

func GetFamilyEntries(tx *vbolt.Tx, familyId int) []Entry {
	return readByTerm(tx, EntryByFamilyIndex, EntryBkt, familyId)
}

func GetEntryMembers(tx *vbolt.Tx, entryId int) []EntryMember {
	return readByTerm(tx, EntryMemberByEntryIndex, EntryMemberBkt, entryId)
}

func GetPersonEntryMembers(tx *vbolt.Tx, personId int) []EntryMember {
	return readByTerm(tx, EntryMemberByPersonIndex, EntryMemberBkt, personId)
}

func GetFamilyEntryMembers(tx *vbolt.Tx, familyId int) []EntryMember {
	return readByTerm(tx, EntryMemberByFamilyIndex, EntryMemberBkt, familyId)
}

// GetEntryPersonIds is the roster of an entry as plain person ids, which is what
// every access check wants.
func GetEntryPersonIds(tx *vbolt.Tx, entryId int) []int {
	members := GetEntryMembers(tx, entryId)
	personIds := make([]int, 0, len(members))
	for _, member := range members {
		personIds = append(personIds, member.PersonId)
	}
	return personIds
}

func GetEventAppearances(tx *vbolt.Tx, eventId int) []Appearance {
	return readByTerm(tx, AppearanceByEventIndex, AppearanceBkt, eventId)
}

func GetEntryAppearances(tx *vbolt.Tx, entryId int) []Appearance {
	return readByTerm(tx, AppearanceByEntryIndex, AppearanceBkt, entryId)
}

func GetFamilyAppearances(tx *vbolt.Tx, familyId int) []Appearance {
	return readByTerm(tx, AppearanceByFamilyIndex, AppearanceBkt, familyId)
}

func GetAppearanceResults(tx *vbolt.Tx, appearanceId int) []Result {
	return readByTerm(tx, ResultByAppearanceIndex, ResultBkt, appearanceId)
}

func GetPersonResults(tx *vbolt.Tx, personId int) []Result {
	return readByTerm(tx, ResultByPersonIndex, ResultBkt, personId)
}

func GetFamilyResults(tx *vbolt.Tx, familyId int) []Result {
	return readByTerm(tx, ResultByFamilyIndex, ResultBkt, familyId)
}

func GetAppearancePhotoJoins(tx *vbolt.Tx, appearanceId int) []AppearancePhoto {
	return readByTerm(tx, AppearancePhotoByAppearanceIndex, AppearancePhotoBkt, appearanceId)
}

func GetFamilyAppearancePhotos(tx *vbolt.Tx, familyId int) []AppearancePhoto {
	return readByTerm(tx, AppearancePhotoByFamilyIndex, AppearancePhotoBkt, familyId)
}

func GetEventPhotoJoins(tx *vbolt.Tx, eventId int) []EventPhoto {
	return readByTerm(tx, EventPhotoByEventIndex, EventPhotoBkt, eventId)
}

func GetFamilyEventPhotos(tx *vbolt.Tx, familyId int) []EventPhoto {
	return readByTerm(tx, EventPhotoByFamilyIndex, EventPhotoBkt, familyId)
}

// ── writes ────────────────────────────────────────────────────────────────────
//
// Each write helper owns its record's index entries, and each delete helper
// clears exactly the same set. Keeping the pair adjacent is what stops an index
// from outliving the row it points at.

func writeActivityTx(tx *vbolt.Tx, activity *Activity) {
	vbolt.Write(tx, ActivityBkt, activity.Id, activity)
	vbolt.SetTargetSingleTerm(tx, ActivityByFamilyIndex, activity.Id, activity.FamilyId)
}

func writeSeasonTx(tx *vbolt.Tx, season *Season) {
	vbolt.Write(tx, SeasonBkt, season.Id, season)
	vbolt.SetTargetSingleTerm(tx, SeasonByActivityIndex, season.Id, season.ActivityId)
	vbolt.SetTargetSingleTerm(tx, SeasonByFamilyIndex, season.Id, season.FamilyId)
}

func writeEventTx(tx *vbolt.Tx, event *Event) {
	vbolt.Write(tx, EventBkt, event.Id, event)
	vbolt.SetTargetSingleTerm(tx, EventBySeasonIndex, event.Id, event.SeasonId)
	vbolt.SetTargetSingleTerm(tx, EventByFamilyIndex, event.Id, event.FamilyId)
}

func writeEntryTx(tx *vbolt.Tx, entry *Entry) {
	vbolt.Write(tx, EntryBkt, entry.Id, entry)
	vbolt.SetTargetSingleTerm(tx, EntryBySeasonIndex, entry.Id, entry.SeasonId)
	vbolt.SetTargetSingleTerm(tx, EntryByFamilyIndex, entry.Id, entry.FamilyId)
}

func writeEntryMemberTx(tx *vbolt.Tx, member *EntryMember) {
	vbolt.Write(tx, EntryMemberBkt, member.Id, member)
	vbolt.SetTargetSingleTerm(tx, EntryMemberByEntryIndex, member.Id, member.EntryId)
	vbolt.SetTargetSingleTerm(tx, EntryMemberByPersonIndex, member.Id, member.PersonId)
	vbolt.SetTargetSingleTerm(tx, EntryMemberByFamilyIndex, member.Id, member.FamilyId)
}

func writeAppearanceTx(tx *vbolt.Tx, appearance *Appearance) {
	vbolt.Write(tx, AppearanceBkt, appearance.Id, appearance)
	vbolt.SetTargetSingleTerm(tx, AppearanceByEventIndex, appearance.Id, appearance.EventId)
	vbolt.SetTargetSingleTerm(tx, AppearanceByEntryIndex, appearance.Id, appearance.EntryId)
	vbolt.SetTargetSingleTerm(tx, AppearanceByFamilyIndex, appearance.Id, appearance.FamilyId)
}

// writeResultTx indexes by person only when the result names one. -1 is vbolt's
// "no term", so a result that stops naming a person drops out of the index on
// the next write rather than lingering under the old id.
func writeResultTx(tx *vbolt.Tx, result *Result) {
	vbolt.Write(tx, ResultBkt, result.Id, result)
	vbolt.SetTargetSingleTerm(tx, ResultByAppearanceIndex, result.Id, result.AppearanceId)
	vbolt.SetTargetSingleTerm(tx, ResultByFamilyIndex, result.Id, result.FamilyId)
	personTerm := -1
	if result.PersonId != nil && *result.PersonId > 0 {
		personTerm = *result.PersonId
	}
	vbolt.SetTargetSingleTerm(tx, ResultByPersonIndex, result.Id, personTerm)
}

func writeAppearancePhotoTx(tx *vbolt.Tx, join *AppearancePhoto) {
	vbolt.Write(tx, AppearancePhotoBkt, join.Id, join)
	vbolt.SetTargetSingleTerm(tx, AppearancePhotoByAppearanceIndex, join.Id, join.AppearanceId)
	vbolt.SetTargetSingleTerm(tx, AppearancePhotoByPhotoIndex, join.Id, join.PhotoId)
	vbolt.SetTargetSingleTerm(tx, AppearancePhotoByFamilyIndex, join.Id, join.FamilyId)
}

func writeEventPhotoTx(tx *vbolt.Tx, join *EventPhoto) {
	vbolt.Write(tx, EventPhotoBkt, join.Id, join)
	vbolt.SetTargetSingleTerm(tx, EventPhotoByEventIndex, join.Id, join.EventId)
	vbolt.SetTargetSingleTerm(tx, EventPhotoByPhotoIndex, join.Id, join.PhotoId)
	vbolt.SetTargetSingleTerm(tx, EventPhotoByFamilyIndex, join.Id, join.FamilyId)
}

// ── row deletion ──────────────────────────────────────────────────────────────
//
// These delete one row and its index entries and nothing else. Cascades — an
// Event taking its Appearances with it — are built on top of these.

func deleteActivityRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, ActivityBkt, id)
	vbolt.SetTargetSingleTerm(tx, ActivityByFamilyIndex, id, -1)
}

func deleteSeasonRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, SeasonBkt, id)
	vbolt.SetTargetSingleTerm(tx, SeasonByActivityIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, SeasonByFamilyIndex, id, -1)
}

func deleteEventRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, EventBkt, id)
	vbolt.SetTargetSingleTerm(tx, EventBySeasonIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, EventByFamilyIndex, id, -1)
}

func deleteEntryRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, EntryBkt, id)
	vbolt.SetTargetSingleTerm(tx, EntryBySeasonIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, EntryByFamilyIndex, id, -1)
}

func deleteEntryMemberRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, EntryMemberBkt, id)
	vbolt.SetTargetSingleTerm(tx, EntryMemberByEntryIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, EntryMemberByPersonIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, EntryMemberByFamilyIndex, id, -1)
}

func deleteAppearanceRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, AppearanceBkt, id)
	vbolt.SetTargetSingleTerm(tx, AppearanceByEventIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, AppearanceByEntryIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, AppearanceByFamilyIndex, id, -1)
}

func deleteResultRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, ResultBkt, id)
	vbolt.SetTargetSingleTerm(tx, ResultByAppearanceIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, ResultByPersonIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, ResultByFamilyIndex, id, -1)
}

func deleteAppearancePhotoRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, AppearancePhotoBkt, id)
	vbolt.SetTargetSingleTerm(tx, AppearancePhotoByAppearanceIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, AppearancePhotoByPhotoIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, AppearancePhotoByFamilyIndex, id, -1)
}

func deleteEventPhotoRowTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, EventPhotoBkt, id)
	vbolt.SetTargetSingleTerm(tx, EventPhotoByEventIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, EventPhotoByPhotoIndex, id, -1)
	vbolt.SetTargetSingleTerm(tx, EventPhotoByFamilyIndex, id, -1)
}

// deleteFamilyActivitiesTx empties all nine buckets for one family.
//
// It lands with the schema rather than with the rest of the cascade work
// because a bucket account deletion does not know about is a data-retention
// bug, and there is no window in which these buckets may exist unswept.
func deleteFamilyActivitiesTx(tx *vbolt.Tx, familyId int) {
	for _, join := range GetFamilyAppearancePhotos(tx, familyId) {
		deleteAppearancePhotoRowTx(tx, join.Id)
	}
	for _, join := range GetFamilyEventPhotos(tx, familyId) {
		deleteEventPhotoRowTx(tx, join.Id)
	}
	for _, result := range GetFamilyResults(tx, familyId) {
		deleteResultRowTx(tx, result.Id)
	}
	for _, appearance := range GetFamilyAppearances(tx, familyId) {
		deleteAppearanceRowTx(tx, appearance.Id)
	}
	for _, member := range GetFamilyEntryMembers(tx, familyId) {
		deleteEntryMemberRowTx(tx, member.Id)
	}
	for _, entry := range GetFamilyEntries(tx, familyId) {
		deleteEntryRowTx(tx, entry.Id)
	}
	for _, event := range GetFamilyEvents(tx, familyId) {
		deleteEventRowTx(tx, event.Id)
	}
	for _, season := range GetFamilySeasons(tx, familyId) {
		deleteSeasonRowTx(tx, season.Id)
	}
	for _, activity := range GetFamilyActivities(tx, familyId) {
		deleteActivityRowTx(tx, activity.Id)
	}
}

// ── access ────────────────────────────────────────────────────────────────────

// canAccessEntry allows a member of the owning family, or any user who can
// reach at least one rostered person through an accepted family link carrying
// ScopeActivities.
//
// The deliberate consequence: a link that shares one child exposes the group
// routines that child is in, including the co-performers' names. That is what a
// shared routine means — the routine is the shared object, not any one child —
// but it is worth stating rather than discovering.
//
// An entry with an empty roster is reachable by its own family only. There is no
// person to resolve through, and defaulting the other way would make a
// half-filled form visible to every linked household.
func canAccessEntry(tx *vbolt.Tx, user User, entry Entry, need AccessLevel) bool {
	if entry.Id == 0 || entry.FamilyId == 0 {
		return false
	}
	if CanAccessFamily(tx, user, entry.FamilyId, need) {
		return true
	}
	for _, personId := range GetEntryPersonIds(tx, entry.Id) {
		if CanAccessRecordOfPerson(tx, user, entry.FamilyId, personId, ScopeActivities, need) {
			return true
		}
	}
	return false
}

// canAccessAppearance resolves to the entry and defers to it. An appearance
// carries no roster of its own — it is the entry that names the people.
func canAccessAppearance(tx *vbolt.Tx, user User, appearance Appearance, need AccessLevel) bool {
	if appearance.Id == 0 {
		return false
	}
	return canAccessEntry(tx, user, GetEntryById(tx, appearance.EntryId), need)
}

// canAccessResult resolves through its appearance. Result.PersonId narrows who a
// result is *about*; it does not widen who may read it.
func canAccessResult(tx *vbolt.Tx, user User, result Result, need AccessLevel) bool {
	if result.Id == 0 {
		return false
	}
	return canAccessAppearance(tx, user, GetAppearanceById(tx, result.AppearanceId), need)
}

// canAccessSeason and friends have no person dimension, so plain family access
// answers them. They are named here so call sites read the same way as the
// roster-backed checks above.
func canAccessSeason(tx *vbolt.Tx, user User, season Season, need AccessLevel) bool {
	return season.Id != 0 && CanAccessFamily(tx, user, season.FamilyId, need)
}

func canAccessEvent(tx *vbolt.Tx, user User, event Event, need AccessLevel) bool {
	return event.Id != 0 && CanAccessFamily(tx, user, event.FamilyId, need)
}

func canAccessActivity(tx *vbolt.Tx, user User, activity Activity, need AccessLevel) bool {
	return activity.Id != 0 && CanAccessFamily(tx, user, activity.FamilyId, need)
}
