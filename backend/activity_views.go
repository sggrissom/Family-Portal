// The four questions a season gets asked, one proc each, plus the vocabulary
// that keeps free-text labels from fragmenting.
//
// Each proc returns everything its page needs so the frontend makes one call
// rather than fanning out over ids it just received. That is the whole reason
// Appearance exists as its own table: "how did this competition go?" and "how
// has this routine done all season?" are both index walks off it, and neither
// is privileged over the other. See docs/activities-plan.md.
//
// Access splits these in half, and the split is deliberate:
//
//   - GetSeasonOverview and GetEventDetail are whole-family views. A season and
//     a competition have no person dimension — there is no one child they are
//     "about" — so they take plain family access and a link never reaches them.
//   - GetEntryHistory and GetPersonSeason resolve through a roster, so a linked
//     household reading a shared child gets exactly the routines that child is
//     in. They carry EventSummary and SeasonSummary rather than the full
//     records: an appearance with no competition name attached is unreadable,
//     but the notes on that competition are nobody else's business.
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

// ── shared shapes ─────────────────────────────────────────────────────────────

// SeasonSummary and EventSummary are the parent context a record needs to be
// readable, and nothing more. They exist because the roster-scoped views cross
// a link boundary: a household that was shared one child should learn which
// competition a performance happened at, not what the host family wrote in the
// notes field about it.
type SeasonSummary struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
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

func seasonSummary(season Season) SeasonSummary {
	return SeasonSummary{
		Id: season.Id, Name: season.Name,
		StartDate: season.StartDate, EndDate: season.EndDate,
	}
}

func eventSummary(event Event) EventSummary {
	return EventSummary{
		Id: event.Id, Name: event.Name, Host: event.Host, Location: event.Location,
		StartDate: event.StartDate, EndDate: event.EndDate,
	}
}

// AppearanceDetail is one row of either roster-scoped view. It carries both the
// entry that performed and the competition it happened at, even though each
// view already knows one of them — the routine view came in through the entry,
// the competition view through the event. Carrying both means one frontend
// component renders a performance wherever it appears.
type AppearanceDetail struct {
	Appearance Appearance   `json:"appearance"`
	Results    []Result     `json:"results"`
	Entry      Entry        `json:"entry"`
	Event      EventSummary `json:"event"`
}

// appearanceOrder sorts performances chronologically. OccurredAt is often zero —
// "sometime that weekend" is a normal state for a competition schedule — so the
// event's start date is the fallback, and the id breaks the remaining ties so
// two performances entered off the same sheet keep the order they were entered
// in.
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

// eventCache keeps one read per event rather than one per appearance. A routine
// history spans a dozen competitions; a competition holds a dozen routines. Both
// walks would otherwise re-read the same parent for every row.
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

// ── season overview ───────────────────────────────────────────────────────────

type GetSeasonOverviewRequest struct {
	SeasonId int `json:"seasonId"`
}

// GetSeasonOverviewResponse ships events and entries once each and appearances
// as the bare hinge, rather than repeating the parents on every row. A full
// season is a dozen competitions by a dozen routines, so AppearanceDetail here
// would send each entry a dozen times over; the client joins on EntryId and
// EventId, which it already has.
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

	// Walking the events rather than the family keeps this to the season asked
	// for. A family with three seasons of history would otherwise pay for all
	// of them on every load.
	resp.Appearances = []AppearanceView{}
	for _, event := range resp.Events {
		for _, appearance := range GetEventAppearances(ctx.Tx, event.Id) {
			resp.Appearances = append(resp.Appearances, appearanceView(ctx.Tx, appearance))
		}
	}
	return
}

// ── competition detail ────────────────────────────────────────────────────────

type GetEventDetailRequest struct {
	EventId int `json:"eventId"`
}

type GetEventDetailResponse struct {
	Event       Event              `json:"event"`
	Season      SeasonSummary      `json:"season"`
	Appearances []AppearanceDetail `json:"appearances"`
}

// GetEventDetail answers "how did this competition go?" — one walk of
// AppearanceByEventIndex, no scan.
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
	resp.Season = seasonSummary(GetSeasonById(ctx.Tx, event.SeasonId))

	summary := eventSummary(event)
	entries := entryCache{}
	resp.Appearances = []AppearanceDetail{}
	for _, appearance := range GetEventAppearances(ctx.Tx, event.Id) {
		resp.Appearances = append(resp.Appearances, AppearanceDetail{
			Appearance: appearance,
			Results:    sortResults(GetAppearanceResults(ctx.Tx, appearance.Id)),
			Entry:      entries.get(ctx.Tx, appearance.EntryId),
			Event:      summary,
		})
	}
	sort.Slice(resp.Appearances, func(i, j int) bool {
		return appearanceOrder(resp.Appearances[i], resp.Appearances[j])
	})
	return
}

// ── routine history ───────────────────────────────────────────────────────────

type GetEntryHistoryRequest struct {
	EntryId int `json:"entryId"`
}

type GetEntryHistoryResponse struct {
	Entry       EntryView          `json:"entry"`
	Season      SeasonSummary      `json:"season"`
	Appearances []AppearanceDetail `json:"appearances"`
}

// GetEntryHistory answers "how has this routine done all season?" — the other
// direction off the same hinge, walking AppearanceByEntryIndex.
//
// This is one of the two procs a linked household can reach, so what it returns
// about the season and each competition is a summary rather than the record.
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
	resp.Season = seasonSummary(GetSeasonById(ctx.Tx, entry.SeasonId))

	events := eventCache{}
	resp.Appearances = []AppearanceDetail{}
	for _, appearance := range GetEntryAppearances(ctx.Tx, entry.Id) {
		resp.Appearances = append(resp.Appearances, AppearanceDetail{
			Appearance: appearance,
			Results:    sortResults(GetAppearanceResults(ctx.Tx, appearance.Id)),
			Entry:      entry,
			Event:      events.get(ctx.Tx, appearance.EventId),
		})
	}
	sort.Slice(resp.Appearances, func(i, j int) bool {
		return appearanceOrder(resp.Appearances[i], resp.Appearances[j])
	})
	return
}

// ── a kid's season ────────────────────────────────────────────────────────────

type GetPersonSeasonRequest struct {
	PersonId int `json:"personId"`
	// SeasonId narrows to one season. Zero means every season the person has
	// ever been in, which is what a linked household needs: it cannot list
	// seasons — those have no person dimension — so requiring one would leave
	// it with no way to ask the question at all.
	SeasonId int `json:"seasonId,omitempty"`
}

type GetPersonSeasonResponse struct {
	PersonId int `json:"personId"`
	SeasonId int `json:"seasonId"`
	// Seasons holds only the ones the returned entries belong to, in the same
	// summary form as everywhere else on the link-reachable path.
	Seasons     []SeasonSummary    `json:"seasons"`
	Entries     []EntryView        `json:"entries"`
	Appearances []AppearanceDetail `json:"appearances"`
}

// GetPersonSeason answers "how is this kid's season going?" by walking
// EntryMemberByPersonIndex — the index that exists for exactly this question.
//
// Access is checked per entry rather than once up front. Being able to see the
// child is what makes their routines visible, and canAccessEntry is what decides
// each one; a group routine the child is in is reachable, and so are the
// co-performers' names on it, which is what a shared routine means.
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
		// The person check above says this user may see the child. Whether they
		// may see a particular routine the child is in is still the entry's
		// call — an entry can be reached through any of its rostered people,
		// and only some of them may be shared.
		if !canAccessEntry(ctx.Tx, user, entry, AccessView) {
			continue
		}

		resp.Entries = append(resp.Entries, entryView(ctx.Tx, entry))
		if _, seen := seenSeasons[entry.SeasonId]; !seen {
			seenSeasons[entry.SeasonId] = struct{}{}
			resp.Seasons = append(resp.Seasons, seasonSummary(GetSeasonById(ctx.Tx, entry.SeasonId)))
		}
		for _, appearance := range GetEntryAppearances(ctx.Tx, entry.Id) {
			resp.Appearances = append(resp.Appearances, AppearanceDetail{
				Appearance: appearance,
				Results:    sortResults(GetAppearanceResults(ctx.Tx, appearance.Id)),
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

// ── vocabulary ────────────────────────────────────────────────────────────────

type ListActivityVocabularyRequest struct {
	ActivityId int `json:"activityId"`
}

// ListActivityVocabularyResponse is one list per free-text field, so a form can
// autocomplete each one from what this family has already typed.
type ListActivityVocabularyResponse struct {
	ActivityId    int      `json:"activityId"`
	Adjudications []string `json:"adjudications"` // "High Gold", "Diamond"
	Awards        []string `json:"awards"`        // "Judges' Choice"
	Categories    []string `json:"categories"`    // "Teen Small Group Jazz"
	Styles        []string `json:"styles"`        // "Jazz", "Lyrical"
	Divisions     []string `json:"divisions"`     // "Teen", "Senior"
	Levels        []string `json:"levels"`        // "Elite", "Rec"
	Formats       []string `json:"formats"`       // "solo", "group"
	Hosts         []string `json:"hosts"`         // "Nuvo", "Showstopper"
}

// maxVocabularyEntries bounds each list. A family that has genuinely used two
// hundred distinct adjudication labels has a data-entry problem that a longer
// autocomplete list would not fix.
const maxVocabularyEntries = 200

// vocabulary collects distinct values case-insensitively while keeping the
// spelling first seen. That is the entire point: without it "High Gold" becomes
// "high gold" and "Hi-Gold" and the season view cannot even count, let alone
// rank — adjudication labels are free text by design and nothing normalizes
// them at write time.
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

// sorted returns the list case-insensitively ordered, and never nil — an
// autocomplete source that is sometimes null is a client-side branch for no
// reason.
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

// ListActivityVocabulary is computed per call rather than maintained in an
// index. It walks the family's entries, competitions, and results once and
// filters to this activity by season, which at family scale is a few hundred
// records — cheaper than the writes an index would add to every result saved.
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

	// Scoping to the activity is what keeps a soccer season's score labels out
	// of a dance form's autocomplete. Membership is by season, so the sets are
	// built top down: seasons of this activity, then entries and events in
	// those seasons, then the results under those entries' appearances.
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
