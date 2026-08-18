// Structure CRUD for competitive activities: the program, its seasons, the
// competitions in a season, the routines in a season, and who is in a routine.
//
// Results and the aggregate read procs are a separate phase; this file is only
// the skeleton a season hangs on. See docs/activities-plan.md.
//
// Access is split the way the schema is. Activity, Season and Event are
// family-scoped with no person dimension, so they take plain family access.
// Entry reaches people through its roster, so reads go through canAccessEntry —
// which lets a linked household see the group routine its child is in. Writes
// never take that path: a link is capped at AccessView, so asking for
// AccessContribute is what keeps every mutation membership-only.
package backend

import (
	"errors"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterActivityMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListActivities)
	vbeam.RegisterProc(app, CreateActivity)
	vbeam.RegisterProc(app, UpdateActivity)
	vbeam.RegisterProc(app, DeleteActivity)

	vbeam.RegisterProc(app, ListSeasons)
	vbeam.RegisterProc(app, CreateSeason)
	vbeam.RegisterProc(app, UpdateSeason)
	vbeam.RegisterProc(app, DeleteSeason)

	vbeam.RegisterProc(app, CreateEvent)
	vbeam.RegisterProc(app, UpdateEvent)
	vbeam.RegisterProc(app, DeleteEvent)

	vbeam.RegisterProc(app, CreateEntry)
	vbeam.RegisterProc(app, UpdateEntry)
	vbeam.RegisterProc(app, DeleteEntry)
	vbeam.RegisterProc(app, SetEntryRoster)
}

var (
	ErrActivityNotFound  = errors.New("Activity not found")
	ErrSeasonNotFound    = errors.New("Season not found")
	ErrEventNotFound     = errors.New("Competition not found")
	ErrEntryNotFound     = errors.New("Entry not found")
	ErrNameRequired      = errors.New("A name is required")
	ErrPersonNotOnRoster = errors.New("That person is not on this family's roster")
)

// maxNameLength and maxNotesLength cap what a form can put in a record. Free
// text is the whole point of the label fields — different competitions use
// different vocabularies — but unbounded free text is a storage problem rather
// than a feature.
const (
	maxNameLength  = 200
	maxLabelLength = 100
	maxNotesLength = 4000
)

func trimField(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		value = value[:max]
	}
	return value
}

// parseActivityDate reads an optional YYYY-MM-DD. An absent or empty date is
// the zero time rather than an error: an event whose dates are not known yet is
// a normal state, and Appearance ordering already falls back when it sees one.
func parseActivityDate(value *string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, errors.New("Dates must be in YYYY-MM-DD format")
	}
	return parsed, nil
}

// ── activities ────────────────────────────────────────────────────────────────

type ListActivitiesRequest struct {
	// FamilyId names which family to list; zero means the caller's primary one.
	FamilyId int `json:"familyId,omitempty"`
}

type ListActivitiesResponse struct {
	FamilyId   int        `json:"familyId"`
	Activities []Activity `json:"activities"`
}

type CreateActivityRequest struct {
	FamilyId int    `json:"familyId,omitempty"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type ActivityResponse struct {
	Activity Activity `json:"activity"`
}

type UpdateActivityRequest struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type ActivityIdRequest struct {
	Id int `json:"id"`
}

type DeleteResponse struct {
	Success bool `json:"success"`
}

// normalizeActivityKind keeps Kind to the set the label map knows about.
// Anything unrecognized becomes generic, which renders neutral vocabulary
// rather than failing the write — Kind drives wording and nothing else.
func normalizeActivityKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ActivityKindDance:
		return ActivityKindDance
	case ActivityKindSport:
		return ActivityKindSport
	default:
		return ActivityKindGeneric
	}
}

func ListActivities(ctx *vbeam.Context, req ListActivitiesRequest) (resp ListActivitiesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}

	activities := GetFamilyActivities(ctx.Tx, familyId)
	sort.Slice(activities, func(i, j int) bool { return activities[i].Name < activities[j].Name })

	resp.FamilyId = familyId
	resp.Activities = activities
	return
}

func CreateActivity(ctx *vbeam.Context, req CreateActivityRequest) (resp ActivityResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	activity := Activity{
		Id:        vbolt.NextIntId(ctx.Tx, ActivityBkt),
		FamilyId:  familyId,
		Name:      name,
		Kind:      normalizeActivityKind(req.Kind),
		CreatedAt: time.Now(),
	}
	writeActivityTx(ctx.Tx, &activity)
	vbolt.TxCommit(ctx.Tx)

	resp.Activity = activity
	return
}

// getActivityForUser is the read-then-check every activity proc starts with.
// The record names its family; the request never carries a second copy that
// could disagree with it.
func getActivityForUser(tx *vbolt.Tx, id int, user User, need AccessLevel) (Activity, error) {
	activity := GetActivityById(tx, id)
	if activity.Id == 0 {
		return activity, ErrActivityNotFound
	}
	if !canAccessActivity(tx, user, activity, need) {
		return Activity{}, ErrActivityNotFound
	}
	return activity, nil
}

func UpdateActivity(ctx *vbeam.Context, req UpdateActivityRequest) (resp ActivityResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}

	activity, err := getActivityForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	activity.Name = name
	activity.Kind = normalizeActivityKind(req.Kind)
	writeActivityTx(ctx.Tx, &activity)
	vbolt.TxCommit(ctx.Tx)

	resp.Activity = activity
	return
}

func DeleteActivity(ctx *vbeam.Context, req ActivityIdRequest) (resp DeleteResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	activity, err := getActivityForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteActivityTx(ctx.Tx, activity.Id)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

// ── seasons ───────────────────────────────────────────────────────────────────

type ListSeasonsRequest struct {
	ActivityId int `json:"activityId"`
}

type ListSeasonsResponse struct {
	ActivityId int      `json:"activityId"`
	Seasons    []Season `json:"seasons"`
}

type CreateSeasonRequest struct {
	ActivityId int     `json:"activityId"`
	Name       string  `json:"name"`
	StartDate  *string `json:"startDate,omitempty"` // YYYY-MM-DD
	EndDate    *string `json:"endDate,omitempty"`   // YYYY-MM-DD
	Notes      string  `json:"notes"`
}

type SeasonResponse struct {
	Season Season `json:"season"`
}

type UpdateSeasonRequest struct {
	Id        int     `json:"id"`
	Name      string  `json:"name"`
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	Notes     string  `json:"notes"`
}

type SeasonIdRequest struct {
	Id int `json:"id"`
}

func getSeasonForUser(tx *vbolt.Tx, id int, user User, need AccessLevel) (Season, error) {
	season := GetSeasonById(tx, id)
	if season.Id == 0 {
		return season, ErrSeasonNotFound
	}
	if !canAccessSeason(tx, user, season, need) {
		return Season{}, ErrSeasonNotFound
	}
	return season, nil
}

// ListSeasons orders newest first: a season list is almost always consulted to
// get at the current season.
func ListSeasons(ctx *vbeam.Context, req ListSeasonsRequest) (resp ListSeasonsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	activity, err := getActivityForUser(ctx.Tx, req.ActivityId, user, AccessView)
	if err != nil {
		return
	}

	seasons := GetActivitySeasons(ctx.Tx, activity.Id)
	sort.Slice(seasons, func(i, j int) bool {
		if !seasons[i].StartDate.Equal(seasons[j].StartDate) {
			return seasons[i].StartDate.After(seasons[j].StartDate)
		}
		return seasons[i].Id > seasons[j].Id
	})

	resp.ActivityId = activity.Id
	resp.Seasons = seasons
	return
}

func CreateSeason(ctx *vbeam.Context, req CreateSeasonRequest) (resp SeasonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}
	startDate, err := parseActivityDate(req.StartDate)
	if err != nil {
		return
	}
	endDate, err := parseActivityDate(req.EndDate)
	if err != nil {
		return
	}

	// The parent names the family. A season cannot be created into a family
	// that does not own the activity it belongs to.
	activity, err := getActivityForUser(ctx.Tx, req.ActivityId, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	season := Season{
		Id:         vbolt.NextIntId(ctx.Tx, SeasonBkt),
		ActivityId: activity.Id,
		FamilyId:   activity.FamilyId,
		Name:       name,
		StartDate:  startDate,
		EndDate:    endDate,
		Notes:      trimField(req.Notes, maxNotesLength),
		CreatedAt:  time.Now(),
	}
	writeSeasonTx(ctx.Tx, &season)
	vbolt.TxCommit(ctx.Tx)

	resp.Season = season
	return
}

func UpdateSeason(ctx *vbeam.Context, req UpdateSeasonRequest) (resp SeasonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}
	startDate, err := parseActivityDate(req.StartDate)
	if err != nil {
		return
	}
	endDate, err := parseActivityDate(req.EndDate)
	if err != nil {
		return
	}

	season, err := getSeasonForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	season.Name = name
	season.StartDate = startDate
	season.EndDate = endDate
	season.Notes = trimField(req.Notes, maxNotesLength)
	writeSeasonTx(ctx.Tx, &season)
	vbolt.TxCommit(ctx.Tx)

	resp.Season = season
	return
}

func DeleteSeason(ctx *vbeam.Context, req SeasonIdRequest) (resp DeleteResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	season, err := getSeasonForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteSeasonTx(ctx.Tx, season.Id)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

// ── events ────────────────────────────────────────────────────────────────────

type CreateEventRequest struct {
	SeasonId  int     `json:"seasonId"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Location  string  `json:"location"`
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	Notes     string  `json:"notes"`
}

type EventResponse struct {
	Event Event `json:"event"`
}

type UpdateEventRequest struct {
	Id        int     `json:"id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Location  string  `json:"location"`
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	Notes     string  `json:"notes"`
}

type EventIdRequest struct {
	Id int `json:"id"`
}

func getEventForUser(tx *vbolt.Tx, id int, user User, need AccessLevel) (Event, error) {
	event := GetEventById(tx, id)
	if event.Id == 0 {
		return event, ErrEventNotFound
	}
	if !canAccessEvent(tx, user, event, need) {
		return Event{}, ErrEventNotFound
	}
	return event, nil
}

func CreateEvent(ctx *vbeam.Context, req CreateEventRequest) (resp EventResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}
	startDate, err := parseActivityDate(req.StartDate)
	if err != nil {
		return
	}
	endDate, err := parseActivityDate(req.EndDate)
	if err != nil {
		return
	}

	season, err := getSeasonForUser(ctx.Tx, req.SeasonId, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	event := Event{
		Id:        vbolt.NextIntId(ctx.Tx, EventBkt),
		SeasonId:  season.Id,
		FamilyId:  season.FamilyId,
		Name:      name,
		Host:      trimField(req.Host, maxLabelLength),
		Location:  trimField(req.Location, maxNameLength),
		StartDate: startDate,
		EndDate:   endDate,
		Notes:     trimField(req.Notes, maxNotesLength),
		CreatedAt: time.Now(),
	}
	writeEventTx(ctx.Tx, &event)
	vbolt.TxCommit(ctx.Tx)

	resp.Event = event
	return
}

func UpdateEvent(ctx *vbeam.Context, req UpdateEventRequest) (resp EventResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}
	startDate, err := parseActivityDate(req.StartDate)
	if err != nil {
		return
	}
	endDate, err := parseActivityDate(req.EndDate)
	if err != nil {
		return
	}

	event, err := getEventForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	event.Name = name
	event.Host = trimField(req.Host, maxLabelLength)
	event.Location = trimField(req.Location, maxNameLength)
	event.StartDate = startDate
	event.EndDate = endDate
	event.Notes = trimField(req.Notes, maxNotesLength)
	writeEventTx(ctx.Tx, &event)
	vbolt.TxCommit(ctx.Tx)

	resp.Event = event
	return
}

func DeleteEvent(ctx *vbeam.Context, req EventIdRequest) (resp DeleteResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	event, err := getEventForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteEventTx(ctx.Tx, event.Id)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

// ── entries and rosters ───────────────────────────────────────────────────────

type CreateEntryRequest struct {
	SeasonId  int    `json:"seasonId"`
	Name      string `json:"name"`
	Format    string `json:"format"`
	Style     string `json:"style"`
	Division  string `json:"division"`
	Level     string `json:"level"`
	Notes     string `json:"notes"`
	PersonIds []int  `json:"personIds,omitempty"`
}

// EntryView is an entry with its roster, which is how every caller wants it —
// an entry without its people cannot be rendered or access-checked.
type EntryView struct {
	Entry     Entry `json:"entry"`
	PersonIds []int `json:"personIds"`
}

type EntryResponse struct {
	Entry EntryView `json:"entry"`
}

type UpdateEntryRequest struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Format   string `json:"format"`
	Style    string `json:"style"`
	Division string `json:"division"`
	Level    string `json:"level"`
	Notes    string `json:"notes"`
}

type EntryIdRequest struct {
	Id int `json:"id"`
}

type SetEntryRosterRequest struct {
	EntryId   int   `json:"entryId"`
	PersonIds []int `json:"personIds"`
}

func getEntryForUser(tx *vbolt.Tx, id int, user User, need AccessLevel) (Entry, error) {
	entry := GetEntryById(tx, id)
	if entry.Id == 0 {
		return entry, ErrEntryNotFound
	}
	if !canAccessEntry(tx, user, entry, need) {
		return Entry{}, ErrEntryNotFound
	}
	return entry, nil
}

func entryView(tx *vbolt.Tx, entry Entry) EntryView {
	return EntryView{Entry: entry, PersonIds: GetEntryPersonIds(tx, entry.Id)}
}

// applyEntryFields is the shared half of create and update. Everything except
// Name is free text by design: competitions do not agree on what a division or
// a level is called, and normalizing here would lose the distinction.
func applyEntryFields(entry *Entry, name string, format string, style string, division string, level string, notes string) {
	entry.Name = name
	entry.Format = trimField(format, maxLabelLength)
	entry.Style = trimField(style, maxLabelLength)
	entry.Division = trimField(division, maxLabelLength)
	entry.Level = trimField(level, maxLabelLength)
	entry.Notes = trimField(notes, maxNotesLength)
}

// setEntryRosterTx replaces the roster wholesale. Rosters are small and always
// edited as a set — nobody adds one dancer to a routine in isolation — so
// replace-all is both the honest shape and half the procs of add/remove.
//
// Every person must already be on the owning family's roster. A routine can hold
// children from two households, but only because the other household shared them
// in; this proc is not a second way to reach a person.
func setEntryRosterTx(tx *vbolt.Tx, entry Entry, personIds []int) error {
	desired := make(map[int]struct{}, len(personIds))
	ordered := make([]int, 0, len(personIds))
	for _, personId := range personIds {
		if personId <= 0 {
			continue
		}
		if _, seen := desired[personId]; seen {
			continue
		}
		person := GetPersonById(tx, personId)
		if person.Id == 0 {
			return ErrPersonNotOnRoster
		}
		if _, onRoster := FindPersonFamily(tx, personId, entry.FamilyId); !onRoster && person.FamilyId != entry.FamilyId {
			return ErrPersonNotOnRoster
		}
		desired[personId] = struct{}{}
		ordered = append(ordered, personId)
	}

	existing := make(map[int]struct{})
	for _, member := range GetEntryMembers(tx, entry.Id) {
		if _, keep := desired[member.PersonId]; !keep {
			deleteEntryMemberRowTx(tx, member.Id)
			continue
		}
		// A duplicate row for the same person is a bug elsewhere, but dropping
		// it here costs nothing and keeps the roster a set.
		if _, already := existing[member.PersonId]; already {
			deleteEntryMemberRowTx(tx, member.Id)
			continue
		}
		existing[member.PersonId] = struct{}{}
	}

	now := time.Now()
	for _, personId := range ordered {
		if _, already := existing[personId]; already {
			continue
		}
		member := EntryMember{
			Id:        vbolt.NextIntId(tx, EntryMemberBkt),
			EntryId:   entry.Id,
			PersonId:  personId,
			FamilyId:  entry.FamilyId,
			CreatedAt: now,
		}
		writeEntryMemberTx(tx, &member)
	}
	return nil
}

func CreateEntry(ctx *vbeam.Context, req CreateEntryRequest) (resp EntryResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}

	season, err := getSeasonForUser(ctx.Tx, req.SeasonId, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	entry := Entry{
		Id:        vbolt.NextIntId(ctx.Tx, EntryBkt),
		SeasonId:  season.Id,
		FamilyId:  season.FamilyId,
		CreatedAt: time.Now(),
	}
	applyEntryFields(&entry, name, req.Format, req.Style, req.Division, req.Level, req.Notes)
	writeEntryTx(ctx.Tx, &entry)

	if req.PersonIds != nil {
		if err = setEntryRosterTx(ctx.Tx, entry, req.PersonIds); err != nil {
			return
		}
	}
	vbolt.TxCommit(ctx.Tx)

	resp.Entry = entryView(ctx.Tx, entry)
	return
}

func UpdateEntry(ctx *vbeam.Context, req UpdateEntryRequest) (resp EntryResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := trimField(req.Name, maxNameLength)
	if name == "" {
		err = ErrNameRequired
		return
	}

	entry, err := getEntryForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	applyEntryFields(&entry, name, req.Format, req.Style, req.Division, req.Level, req.Notes)
	writeEntryTx(ctx.Tx, &entry)
	vbolt.TxCommit(ctx.Tx)

	resp.Entry = entryView(ctx.Tx, entry)
	return
}

func SetEntryRoster(ctx *vbeam.Context, req SetEntryRosterRequest) (resp EntryResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	entry, err := getEntryForUser(ctx.Tx, req.EntryId, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	if err = setEntryRosterTx(ctx.Tx, entry, req.PersonIds); err != nil {
		return
	}
	vbolt.TxCommit(ctx.Tx)

	resp.Entry = entryView(ctx.Tx, entry)
	return
}

func DeleteEntry(ctx *vbeam.Context, req EntryIdRequest) (resp DeleteResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	entry, err := getEntryForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteEntryTx(ctx.Tx, entry.Id)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}
