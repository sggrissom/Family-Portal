package backend

import (
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

type Activity struct {
	Id        int       `json:"id"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
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
	Name       string    `json:"name"`
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Event struct {
	Id        int       `json:"id"`
	SeasonId  int       `json:"seasonId"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Location  string    `json:"location"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
}

type Entry struct {
	Id        int       `json:"id"`
	SeasonId  int       `json:"seasonId"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	Style     string    `json:"style"`
	Division  string    `json:"division"`
	Level     string    `json:"level"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
}

type EntryMember struct {
	Id        int       `json:"id"`
	EntryId   int       `json:"entryId"`
	PersonId  int       `json:"personId"`
	FamilyId  int       `json:"familyId"`
	CreatedAt time.Time `json:"createdAt"`
}

type Appearance struct {
	Id         int       `json:"id"`
	EventId    int       `json:"eventId"`
	EntryId    int       `json:"entryId"`
	FamilyId   int       `json:"familyId"`
	OccurredAt time.Time `json:"occurredAt"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Result struct {
	Id           int       `json:"id"`
	AppearanceId int       `json:"appearanceId"`
	FamilyId     int       `json:"familyId"`
	Kind         string    `json:"kind"`
	Label        string    `json:"label"`
	Rank         *int      `json:"rank,omitempty"`
	OutOf        *int      `json:"outOf,omitempty"`
	Category     string    `json:"category"`
	Score        *float64  `json:"score,omitempty"`
	PersonId     *int      `json:"personId,omitempty"`
	Notes        string    `json:"notes"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
}

const (
	ResultKindAdjudication = "adjudication"
	ResultKindPlacement    = "placement"
	ResultKindAward        = "award"
	ResultKindScore        = "score"
)

type AppearancePhoto struct {
	Id           int       `json:"id"`
	AppearanceId int       `json:"appearanceId"`
	PhotoId      int       `json:"photoId"`
	FamilyId     int       `json:"familyId"`
	CreatedAt    time.Time `json:"createdAt"`
}

type EventPhoto struct {
	Id        int       `json:"id"`
	EventId   int       `json:"eventId"`
	PhotoId   int       `json:"photoId"`
	FamilyId  int       `json:"familyId"`
	CreatedAt time.Time `json:"createdAt"`
}

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

var ActivityBkt = vbolt.Bucket(&cfg.Info, "activities", vpack.FInt, PackActivity)
var SeasonBkt = vbolt.Bucket(&cfg.Info, "seasons", vpack.FInt, PackSeason)
var EventBkt = vbolt.Bucket(&cfg.Info, "activity_events", vpack.FInt, PackEvent)
var EntryBkt = vbolt.Bucket(&cfg.Info, "activity_entries", vpack.FInt, PackEntry)
var EntryMemberBkt = vbolt.Bucket(&cfg.Info, "entry_members", vpack.FInt, PackEntryMember)
var AppearanceBkt = vbolt.Bucket(&cfg.Info, "appearances", vpack.FInt, PackAppearance)
var ResultBkt = vbolt.Bucket(&cfg.Info, "activity_results", vpack.FInt, PackResult)
var AppearancePhotoBkt = vbolt.Bucket(&cfg.Info, "appearance_photos", vpack.FInt, PackAppearancePhoto)
var EventPhotoBkt = vbolt.Bucket(&cfg.Info, "activity_event_photos", vpack.FInt, PackEventPhoto)

var ActivityByFamilyIndex = vbolt.Index(&cfg.Info, "activity_by_family", vpack.FInt, vpack.FInt)

var SeasonByActivityIndex = vbolt.Index(&cfg.Info, "season_by_activity", vpack.FInt, vpack.FInt)

var SeasonByFamilyIndex = vbolt.Index(&cfg.Info, "season_by_family", vpack.FInt, vpack.FInt)

var EventBySeasonIndex = vbolt.Index(&cfg.Info, "activity_event_by_season", vpack.FInt, vpack.FInt)

var EventByFamilyIndex = vbolt.Index(&cfg.Info, "activity_event_by_family", vpack.FInt, vpack.FInt)

var EntryBySeasonIndex = vbolt.Index(&cfg.Info, "activity_entry_by_season", vpack.FInt, vpack.FInt)

var EntryByFamilyIndex = vbolt.Index(&cfg.Info, "activity_entry_by_family", vpack.FInt, vpack.FInt)

var EntryMemberByEntryIndex = vbolt.Index(&cfg.Info, "entry_member_by_entry", vpack.FInt, vpack.FInt)

var EntryMemberByPersonIndex = vbolt.Index(&cfg.Info, "entry_member_by_person", vpack.FInt, vpack.FInt)

var EntryMemberByFamilyIndex = vbolt.Index(&cfg.Info, "entry_member_by_family", vpack.FInt, vpack.FInt)

var AppearanceByEventIndex = vbolt.Index(&cfg.Info, "appearance_by_event", vpack.FInt, vpack.FInt)

var AppearanceByEntryIndex = vbolt.Index(&cfg.Info, "appearance_by_entry", vpack.FInt, vpack.FInt)

var AppearanceByFamilyIndex = vbolt.Index(&cfg.Info, "appearance_by_family", vpack.FInt, vpack.FInt)

var ResultByAppearanceIndex = vbolt.Index(&cfg.Info, "activity_result_by_appearance", vpack.FInt, vpack.FInt)

var ResultByPersonIndex = vbolt.Index(&cfg.Info, "activity_result_by_person", vpack.FInt, vpack.FInt)

var ResultByFamilyIndex = vbolt.Index(&cfg.Info, "activity_result_by_family", vpack.FInt, vpack.FInt)

var AppearancePhotoByAppearanceIndex = vbolt.Index(&cfg.Info, "appearance_photo_by_appearance", vpack.FInt, vpack.FInt)
var AppearancePhotoByPhotoIndex = vbolt.Index(&cfg.Info, "appearance_photo_by_photo", vpack.FInt, vpack.FInt)
var AppearancePhotoByFamilyIndex = vbolt.Index(&cfg.Info, "appearance_photo_by_family", vpack.FInt, vpack.FInt)

var EventPhotoByEventIndex = vbolt.Index(&cfg.Info, "activity_event_photo_by_event", vpack.FInt, vpack.FInt)
var EventPhotoByPhotoIndex = vbolt.Index(&cfg.Info, "activity_event_photo_by_photo", vpack.FInt, vpack.FInt)
var EventPhotoByFamilyIndex = vbolt.Index(&cfg.Info, "activity_event_photo_by_family", vpack.FInt, vpack.FInt)

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

func deleteAppearanceTx(tx *vbolt.Tx, appearanceId int) {
	for _, result := range GetAppearanceResults(tx, appearanceId) {
		deleteResultRowTx(tx, result.Id)
	}
	for _, join := range GetAppearancePhotoJoins(tx, appearanceId) {
		deleteAppearancePhotoRowTx(tx, join.Id)
	}
	deleteAppearanceRowTx(tx, appearanceId)
}

func deleteEntryTx(tx *vbolt.Tx, entryId int) {
	for _, member := range GetEntryMembers(tx, entryId) {
		deleteEntryMemberRowTx(tx, member.Id)
	}
	for _, appearance := range GetEntryAppearances(tx, entryId) {
		deleteAppearanceTx(tx, appearance.Id)
	}
	deleteEntryRowTx(tx, entryId)
}

func deleteEventTx(tx *vbolt.Tx, eventId int) {
	for _, appearance := range GetEventAppearances(tx, eventId) {
		deleteAppearanceTx(tx, appearance.Id)
	}
	for _, join := range GetEventPhotoJoins(tx, eventId) {
		deleteEventPhotoRowTx(tx, join.Id)
	}
	deleteEventRowTx(tx, eventId)
}

func deleteSeasonTx(tx *vbolt.Tx, seasonId int) {
	for _, event := range GetSeasonEvents(tx, seasonId) {
		deleteEventTx(tx, event.Id)
	}
	for _, entry := range GetSeasonEntries(tx, seasonId) {
		deleteEntryTx(tx, entry.Id)
	}
	deleteSeasonRowTx(tx, seasonId)
}

func deleteActivityTx(tx *vbolt.Tx, activityId int) {
	for _, season := range GetActivitySeasons(tx, activityId) {
		deleteSeasonTx(tx, season.Id)
	}
	deleteActivityRowTx(tx, activityId)
}

func removePersonFromActivitiesTx(tx *vbolt.Tx, personId int) {
	for _, member := range GetPersonEntryMembers(tx, personId) {
		deleteEntryMemberRowTx(tx, member.Id)
	}
	for _, result := range GetPersonResults(tx, personId) {
		result.PersonId = nil
		writeResultTx(tx, &result)
	}
}

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

func canAccessAppearance(tx *vbolt.Tx, user User, appearance Appearance, need AccessLevel) bool {
	if appearance.Id == 0 {
		return false
	}
	return canAccessEntry(tx, user, GetEntryById(tx, appearance.EntryId), need)
}

func canAccessResult(tx *vbolt.Tx, user User, result Result, need AccessLevel) bool {
	if result.Id == 0 {
		return false
	}
	return canAccessAppearance(tx, user, GetAppearanceById(tx, result.AppearanceId), need)
}

func canAccessSeason(tx *vbolt.Tx, user User, season Season, need AccessLevel) bool {
	return season.Id != 0 && CanAccessFamily(tx, user, season.FamilyId, need)
}

func canAccessEvent(tx *vbolt.Tx, user User, event Event, need AccessLevel) bool {
	return event.Id != 0 && CanAccessFamily(tx, user, event.FamilyId, need)
}

func canAccessActivity(tx *vbolt.Tx, user User, activity Activity, need AccessLevel) bool {
	return activity.Id != 0 && CanAccessFamily(tx, user, activity.FamilyId, need)
}
