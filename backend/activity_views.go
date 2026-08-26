package backend

import (
	"errors"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterActivityViewMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetSeasonOverview)
	vbeam.RegisterProc(app, GetEventDetail)
	vbeam.RegisterProc(app, GetEntryHistory)
	vbeam.RegisterProc(app, GetPersonSeason)
	vbeam.RegisterProc(app, ListActivityVocabulary)
}

var ErrPersonNotFound = errors.New("Person not found or not in your family")

type SeasonSummary struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

type EventSummary struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Location  string    `json:"location"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

func seasonSummary(tx *vbolt.Tx, season Season) SeasonSummary {
	return SeasonSummary{
		Id: season.Id, Name: season.Name,
		Kind:      GetActivityById(tx, season.ActivityId).Kind,
		StartDate: season.StartDate, EndDate: season.EndDate,
	}
}

func eventSummary(event Event) EventSummary {
	return EventSummary{
		Id: event.Id, Name: event.Name, Host: event.Host, Location: event.Location,
		StartDate: event.StartDate, EndDate: event.EndDate,
	}
}

type AppearanceDetail struct {
	Appearance Appearance   `json:"appearance"`
	Results    []Result     `json:"results"`
	PhotoIds   []int        `json:"photoIds"`
	Entry      Entry        `json:"entry"`
	Event      EventSummary `json:"event"`
}

func appearanceOrder(a, b AppearanceDetail) bool {
	at, bt := a.Appearance.OccurredAt, b.Appearance.OccurredAt
	if at.IsZero() {
		at = a.Event.StartDate
	}
	if bt.IsZero() {
		bt = b.Event.StartDate
	}
	if !at.Equal(bt) {
		return at.Before(bt)
	}
	return a.Appearance.Id < b.Appearance.Id
}

type eventCache map[int]EventSummary

func (c eventCache) get(tx *vbolt.Tx, eventId int) EventSummary {
	if summary, ok := c[eventId]; ok {
		return summary
	}
	summary := eventSummary(GetEventById(tx, eventId))
	c[eventId] = summary
	return summary
}

type entryCache map[int]Entry

func (c entryCache) get(tx *vbolt.Tx, entryId int) Entry {
	if entry, ok := c[entryId]; ok {
		return entry
	}
	entry := GetEntryById(tx, entryId)
	c[entryId] = entry
	return entry
}

type GetSeasonOverviewRequest struct {
	SeasonId int `json:"seasonId"`
}

type GetSeasonOverviewResponse struct {
	Activity    Activity         `json:"activity"`
	Season      Season           `json:"season"`
	Events      []Event          `json:"events"`
	Entries     []EntryView      `json:"entries"`
	Appearances []AppearanceView `json:"appearances"`
}

func GetSeasonOverview(ctx *vbeam.Context, req GetSeasonOverviewRequest) (resp GetSeasonOverviewResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	season, err := getSeasonForUser(ctx.Tx, req.SeasonId, user, AccessView)
	if err != nil {
		return
	}

	resp.Season = season
	resp.Activity = GetActivityById(ctx.Tx, season.ActivityId)

	resp.Events = GetSeasonEvents(ctx.Tx, season.Id)
	sort.Slice(resp.Events, func(i, j int) bool {
		if !resp.Events[i].StartDate.Equal(resp.Events[j].StartDate) {
			return resp.Events[i].StartDate.Before(resp.Events[j].StartDate)
		}
		return resp.Events[i].Id < resp.Events[j].Id
	})

	entries := GetSeasonEntries(ctx.Tx, season.Id)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Id < entries[j].Id
	})
	resp.Entries = make([]EntryView, 0, len(entries))
	for _, entry := range entries {
		resp.Entries = append(resp.Entries, entryView(ctx.Tx, entry))
	}

	resp.Appearances = []AppearanceView{}
	for _, event := range resp.Events {
		for _, appearance := range GetEventAppearances(ctx.Tx, event.Id) {
			resp.Appearances = append(resp.Appearances, appearanceView(ctx.Tx, user, appearance))
		}
	}
	return
}

type GetEventDetailRequest struct {
	EventId int `json:"eventId"`
}

type GetEventDetailResponse struct {
	Event       Event              `json:"event"`
	Season      SeasonSummary      `json:"season"`
	PhotoIds    []int              `json:"photoIds"`
	Appearances []AppearanceDetail `json:"appearances"`
}

func GetEventDetail(ctx *vbeam.Context, req GetEventDetailRequest) (resp GetEventDetailResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	event, err := getEventForUser(ctx.Tx, req.EventId, user, AccessView)
	if err != nil {
		return
	}

	resp.Event = event
	resp.Season = seasonSummary(ctx.Tx, GetSeasonById(ctx.Tx, event.SeasonId))
	resp.PhotoIds = visiblePhotoIds(ctx.Tx, user, GetEventPhotoIds(ctx.Tx, event.Id))

	summary := eventSummary(event)
	entries := entryCache{}
	resp.Appearances = []AppearanceDetail{}
	for _, appearance := range GetEventAppearances(ctx.Tx, event.Id) {
		resp.Appearances = append(resp.Appearances, AppearanceDetail{
			Appearance: appearance,
			Results:    sortResults(GetAppearanceResults(ctx.Tx, appearance.Id)),
			PhotoIds:   visiblePhotoIds(ctx.Tx, user, GetAppearancePhotoIds(ctx.Tx, appearance.Id)),
			Entry:      entries.get(ctx.Tx, appearance.EntryId),
			Event:      summary,
		})
	}
	sort.Slice(resp.Appearances, func(i, j int) bool {
		return appearanceOrder(resp.Appearances[i], resp.Appearances[j])
	})
	return
}

type GetEntryHistoryRequest struct {
	EntryId int `json:"entryId"`
}

type GetEntryHistoryResponse struct {
	Entry       EntryView          `json:"entry"`
	Season      SeasonSummary      `json:"season"`
	Appearances []AppearanceDetail `json:"appearances"`
}

func GetEntryHistory(ctx *vbeam.Context, req GetEntryHistoryRequest) (resp GetEntryHistoryResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	entry, err := getEntryForUser(ctx.Tx, req.EntryId, user, AccessView)
	if err != nil {
		return
	}

	resp.Entry = entryView(ctx.Tx, entry)
	resp.Season = seasonSummary(ctx.Tx, GetSeasonById(ctx.Tx, entry.SeasonId))

	events := eventCache{}
	resp.Appearances = []AppearanceDetail{}
	for _, appearance := range GetEntryAppearances(ctx.Tx, entry.Id) {
		resp.Appearances = append(resp.Appearances, AppearanceDetail{
			Appearance: appearance,
			Results:    sortResults(GetAppearanceResults(ctx.Tx, appearance.Id)),
			PhotoIds:   visiblePhotoIds(ctx.Tx, user, GetAppearancePhotoIds(ctx.Tx, appearance.Id)),
			Entry:      entry,
			Event:      events.get(ctx.Tx, appearance.EventId),
		})
	}
	sort.Slice(resp.Appearances, func(i, j int) bool {
		return appearanceOrder(resp.Appearances[i], resp.Appearances[j])
	})
	return
}

type GetPersonSeasonRequest struct {
	PersonId int `json:"personId"`
	SeasonId int `json:"seasonId,omitempty"`
}

type GetPersonSeasonResponse struct {
	PersonId    int                `json:"personId"`
	SeasonId    int                `json:"seasonId"`
	Seasons     []SeasonSummary    `json:"seasons"`
	Entries     []EntryView        `json:"entries"`
	Appearances []AppearanceDetail `json:"appearances"`
}

func GetPersonSeason(ctx *vbeam.Context, req GetPersonSeasonRequest) (resp GetPersonSeasonResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopeActivities, AccessView) {
		err = ErrPersonNotFound
		return
	}

	resp.PersonId = person.Id
	resp.SeasonId = req.SeasonId
	resp.Seasons = []SeasonSummary{}
	resp.Entries = []EntryView{}
	resp.Appearances = []AppearanceDetail{}

	events := eventCache{}
	seenSeasons := map[int]struct{}{}
	for _, member := range GetPersonEntryMembers(ctx.Tx, person.Id) {
		entry := GetEntryById(ctx.Tx, member.EntryId)
		if entry.Id == 0 {
			continue
		}
		if req.SeasonId != 0 && entry.SeasonId != req.SeasonId {
			continue
		}
		if !canAccessEntry(ctx.Tx, user, entry, AccessView) {
			continue
		}

		resp.Entries = append(resp.Entries, entryView(ctx.Tx, entry))
		if _, seen := seenSeasons[entry.SeasonId]; !seen {
			seenSeasons[entry.SeasonId] = struct{}{}
			resp.Seasons = append(resp.Seasons, seasonSummary(ctx.Tx, GetSeasonById(ctx.Tx, entry.SeasonId)))
		}
		for _, appearance := range GetEntryAppearances(ctx.Tx, entry.Id) {
			resp.Appearances = append(resp.Appearances, AppearanceDetail{
				Appearance: appearance,
				Results:    sortResults(GetAppearanceResults(ctx.Tx, appearance.Id)),
				PhotoIds:   visiblePhotoIds(ctx.Tx, user, GetAppearancePhotoIds(ctx.Tx, appearance.Id)),
				Entry:      entry,
				Event:      events.get(ctx.Tx, appearance.EventId),
			})
		}
	}

	sort.Slice(resp.Entries, func(i, j int) bool {
		if resp.Entries[i].Entry.Name != resp.Entries[j].Entry.Name {
			return resp.Entries[i].Entry.Name < resp.Entries[j].Entry.Name
		}
		return resp.Entries[i].Entry.Id < resp.Entries[j].Entry.Id
	})
	sort.Slice(resp.Seasons, func(i, j int) bool {
		if !resp.Seasons[i].StartDate.Equal(resp.Seasons[j].StartDate) {
			return resp.Seasons[i].StartDate.After(resp.Seasons[j].StartDate)
		}
		return resp.Seasons[i].Id > resp.Seasons[j].Id
	})
	sort.Slice(resp.Appearances, func(i, j int) bool {
		return appearanceOrder(resp.Appearances[i], resp.Appearances[j])
	})
	return
}

type ListActivityVocabularyRequest struct {
	ActivityId int `json:"activityId"`
}

type ListActivityVocabularyResponse struct {
	ActivityId    int      `json:"activityId"`
	Adjudications []string `json:"adjudications"`
	Awards        []string `json:"awards"`
	Categories    []string `json:"categories"`
	Styles        []string `json:"styles"`
	Divisions     []string `json:"divisions"`
	Levels        []string `json:"levels"`
	Formats       []string `json:"formats"`
	Hosts         []string `json:"hosts"`
}

const maxVocabularyEntries = 200

type vocabulary struct {
	seen   map[string]struct{}
	values []string
}

func (v *vocabulary) add(value string) {
	value = strings.TrimSpace(value)
	if value == "" || len(v.values) >= maxVocabularyEntries {
		return
	}
	if v.seen == nil {
		v.seen = map[string]struct{}{}
	}
	key := strings.ToLower(value)
	if _, already := v.seen[key]; already {
		return
	}
	v.seen[key] = struct{}{}
	v.values = append(v.values, value)
}

func (v *vocabulary) sorted() []string {
	values := v.values
	if values == nil {
		values = []string{}
	}
	sort.Slice(values, func(i, j int) bool {
		li, lj := strings.ToLower(values[i]), strings.ToLower(values[j])
		if li != lj {
			return li < lj
		}
		return values[i] < values[j]
	})
	return values
}

func ListActivityVocabulary(ctx *vbeam.Context, req ListActivityVocabularyRequest) (resp ListActivityVocabularyResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	activity, err := getActivityForUser(ctx.Tx, req.ActivityId, user, AccessView)
	if err != nil {
		return
	}
	resp.ActivityId = activity.Id

	seasonIds := map[int]struct{}{}
	for _, season := range GetActivitySeasons(ctx.Tx, activity.Id) {
		seasonIds[season.Id] = struct{}{}
	}

	var styles, divisions, levels, formats, hosts vocabulary
	entryIds := map[int]struct{}{}
	for _, entry := range GetFamilyEntries(ctx.Tx, activity.FamilyId) {
		if _, ok := seasonIds[entry.SeasonId]; !ok {
			continue
		}
		entryIds[entry.Id] = struct{}{}
		styles.add(entry.Style)
		divisions.add(entry.Division)
		levels.add(entry.Level)
		formats.add(entry.Format)
	}

	for _, event := range GetFamilyEvents(ctx.Tx, activity.FamilyId) {
		if _, ok := seasonIds[event.SeasonId]; !ok {
			continue
		}
		hosts.add(event.Host)
	}

	appearanceIds := map[int]struct{}{}
	for _, appearance := range GetFamilyAppearances(ctx.Tx, activity.FamilyId) {
		if _, ok := entryIds[appearance.EntryId]; ok {
			appearanceIds[appearance.Id] = struct{}{}
		}
	}

	var adjudications, awards, categories vocabulary
	for _, result := range GetFamilyResults(ctx.Tx, activity.FamilyId) {
		if _, ok := appearanceIds[result.AppearanceId]; !ok {
			continue
		}
		categories.add(result.Category)
		switch result.Kind {
		case ResultKindAdjudication:
			adjudications.add(result.Label)
		case ResultKindAward:
			awards.add(result.Label)
		}
	}

	resp.Adjudications = adjudications.sorted()
	resp.Awards = awards.sorted()
	resp.Categories = categories.sorted()
	resp.Styles = styles.sorted()
	resp.Divisions = divisions.sorted()
	resp.Levels = levels.sorted()
	resp.Formats = formats.sorted()
	resp.Hosts = hosts.sorted()
	return
}
