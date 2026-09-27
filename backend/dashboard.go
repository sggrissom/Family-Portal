package backend

import (
	"family/cfg"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterDashboardMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetDashboard)
	vbeam.RegisterProc(app, ListOpenEvents)
}

const (
	dashboardRecentDays      = 14
	dashboardRecentPhotos    = 120
	onThisDaySpreadDays      = 2
	onThisDayMaxYears        = 30
	onThisDayPhotos          = 8
	birthdayLookaheadDays    = 7
	resultWindowDays         = 3
	measureNudgeInfantMonths = 3
	measureNudgeChildMonths  = 6
	measureNudgeInfantAge    = 24
	measureNudgeMaxAge       = 18 * 12
)

type GetDashboardRequest struct {
	// Today is the device's calendar date, YYYY-MM-DD. The server's own date
	// can be a day off from the family's.
	Today string `json:"today"`
}

type DashboardNudge struct {
	Kind     string `json:"kind"`
	Key      string `json:"key"`
	Text     string `json:"text"`
	PersonId int    `json:"personId"`
	Count    int    `json:"count"`
}

type rankedNudge struct {
	DashboardNudge
	priority int
}

type DashboardSeason struct {
	Season        SeasonSummary `json:"season"`
	ActivityName  string        `json:"activityName"`
	Event         *EventSummary `json:"event"`
	EventTiming   string        `json:"eventTiming"`
	CanAddResults bool          `json:"canAddResults"`
}

type DashboardYear struct {
	YearsAgo   int         `json:"yearsAgo"`
	Photos     []Image     `json:"photos"`
	Milestones []Milestone `json:"milestones"`
}

type DashboardRecent struct {
	From       string            `json:"from"`
	Photos     []PhotoWithPeople `json:"photos"`
	Milestones []Milestone       `json:"milestones"`
	Growth     []GrowthData      `json:"growth"`
}

type GetDashboardResponse struct {
	Today     string            `json:"today"`
	People    []Person          `json:"people"`
	Relations []Relation        `json:"relations"`
	Nudges    []DashboardNudge  `json:"nudges"`
	Seasons   []DashboardSeason `json:"seasons"`
	OnThisDay []DashboardYear   `json:"onThisDay"`
	Recent    DashboardRecent   `json:"recent"`
}

func dashboardToday(requested string) time.Time {
	if day, err := time.Parse(dateOnlyLayout, requested); err == nil {
		return day
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func dayOf(t time.Time) string {
	return t.UTC().Format(dateOnlyLayout)
}

func GetDashboard(ctx *vbeam.Context, req GetDashboardRequest) (resp GetDashboardResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	today := dashboardToday(req.Today)
	resp.Today = dayOf(today)

	people, err := ListPeople(ctx, Empty{})
	if err != nil {
		return
	}
	resp.People = people.People
	resp.Relations = people.Relations

	timeline, err := GetFamilyTimeline(ctx, GetFamilyTimelineRequest{SkipPhotos: true})
	if err != nil {
		return
	}

	resp.Recent, err = dashboardRecent(ctx, today, timeline.People)
	if err != nil {
		return
	}
	resp.OnThisDay, err = onThisDay(ctx, today, timeline.People)
	if err != nil {
		return
	}
	resp.Seasons = seasonsInProgress(ctx.Tx, user, today)
	resp.Nudges = dashboardNudges(ctx.Tx, user, today, timeline.People)
	return
}

func dashboardRecent(ctx *vbeam.Context, today time.Time, items []FamilyTimelineItem) (recent DashboardRecent, err error) {
	from := dayOf(today.AddDate(0, 0, -(dashboardRecentDays - 1)))
	to := dayOf(today)
	recent.From = from
	recent.Milestones = []Milestone{}
	recent.Growth = []GrowthData{}

	for _, item := range items {
		for _, milestone := range item.Milestones {
			if day := dayOf(milestone.MilestoneDate); day >= from && day <= to {
				recent.Milestones = append(recent.Milestones, milestone)
			}
		}
		for _, growth := range item.GrowthData {
			if day := dayOf(growth.MeasurementDate); day >= from && day <= to {
				recent.Growth = append(recent.Growth, growth)
			}
		}
	}

	photos, err := ListFamilyPhotos(ctx, ListFamilyPhotosRequest{
		DateFrom: from, DateTo: to, Limit: dashboardRecentPhotos,
	})
	recent.Photos = photos.Photos
	return
}

func onThisDay(ctx *vbeam.Context, today time.Time, items []FamilyTimelineItem) (years []DashboardYear, err error) {
	years = []DashboardYear{}
	for yearsAgo := 1; yearsAgo <= onThisDayMaxYears; yearsAgo++ {
		day := today.AddDate(-yearsAgo, 0, 0)
		from := dayOf(day.AddDate(0, 0, -onThisDaySpreadDays))
		to := dayOf(day.AddDate(0, 0, onThisDaySpreadDays))

		year := DashboardYear{YearsAgo: yearsAgo, Photos: []Image{}, Milestones: []Milestone{}}
		for _, item := range items {
			for _, milestone := range item.Milestones {
				if d := dayOf(milestone.MilestoneDate); d >= from && d <= to {
					year.Milestones = append(year.Milestones, milestone)
				}
			}
		}

		var photos ListFamilyPhotosResponse
		photos, err = ListFamilyPhotos(ctx, ListFamilyPhotosRequest{
			DateFrom: from, DateTo: to, Limit: onThisDayPhotos,
		})
		if err != nil {
			return
		}
		for _, photo := range photos.Photos {
			year.Photos = append(year.Photos, photo.Image)
		}

		if len(year.Photos) > 0 || len(year.Milestones) > 0 {
			years = append(years, year)
		}
	}
	return
}

func dayWithin(day time.Time, start, end time.Time) bool {
	d := dayOf(day)
	if d < dayOf(start) {
		return false
	}
	return end.IsZero() || d <= dayOf(end)
}

func eventEnd(event Event) time.Time {
	if event.EndDate.IsZero() || event.EndDate.Before(event.StartDate) {
		return event.StartDate
	}
	return event.EndDate
}

// pickSeasonEvent is the event running today, else the next one, else the
// most recent one.
func pickSeasonEvent(events []Event, today time.Time) (*Event, string) {
	var next, last *Event
	for i := range events {
		event := &events[i]
		if dayWithin(today, event.StartDate, eventEnd(*event)) {
			return event, "now"
		}
		if dayOf(event.StartDate) > dayOf(today) {
			if next == nil || event.StartDate.Before(next.StartDate) {
				next = event
			}
		} else if last == nil || eventEnd(*event).After(eventEnd(*last)) {
			last = event
		}
	}
	if next != nil {
		return next, "next"
	}
	if last != nil {
		return last, "last"
	}
	return nil, ""
}

func inResultWindow(event Event, today time.Time) bool {
	return dayWithin(today, event.StartDate, eventEnd(event).AddDate(0, 0, resultWindowDays))
}

func seasonsInProgress(tx *vbolt.Tx, user User, today time.Time) []DashboardSeason {
	seasons := []DashboardSeason{}
	for _, familyId := range familiesVisibleTo(tx, user) {
		canContribute := CanAccessFamily(tx, user, familyId, AccessContribute)
		for _, season := range GetFamilySeasons(tx, familyId) {
			if !dayWithin(today, season.StartDate, season.EndDate) {
				continue
			}
			entry := DashboardSeason{
				Season:       seasonSummary(tx, season),
				ActivityName: GetActivityById(tx, season.ActivityId).Name,
			}
			if event, timing := pickSeasonEvent(GetSeasonEvents(tx, season.Id), today); event != nil {
				summary := eventSummary(*event)
				entry.Event = &summary
				entry.EventTiming = timing
				entry.CanAddResults = canContribute && inResultWindow(*event, today)
			}
			seasons = append(seasons, entry)
		}
	}
	sort.SliceStable(seasons, func(i, j int) bool {
		return seasons[i].Season.StartDate.Before(seasons[j].Season.StartDate)
	})
	return seasons
}

type ListOpenEventsRequest struct {
	Today string `json:"today"`
}

type OpenEvent struct {
	Event        EventSummary `json:"event"`
	ActivityName string       `json:"activityName"`
}

type ListOpenEventsResponse struct {
	Events []OpenEvent `json:"events"`
}

// ListOpenEvents is the events whose results can be entered now: running
// today, or ended within the last few days.
func ListOpenEvents(ctx *vbeam.Context, req ListOpenEventsRequest) (resp ListOpenEventsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	today := dashboardToday(req.Today)
	resp.Events = []OpenEvent{}
	for _, familyId := range familiesVisibleTo(ctx.Tx, user) {
		if !CanAccessFamily(ctx.Tx, user, familyId, AccessContribute) {
			continue
		}
		for _, event := range GetFamilyEvents(ctx.Tx, familyId) {
			if !inResultWindow(event, today) {
				continue
			}
			season := GetSeasonById(ctx.Tx, event.SeasonId)
			resp.Events = append(resp.Events, OpenEvent{
				Event:        eventSummary(event),
				ActivityName: GetActivityById(ctx.Tx, season.ActivityId).Name,
			})
		}
	}
	sort.SliceStable(resp.Events, func(i, j int) bool {
		return resp.Events[i].Event.StartDate.After(resp.Events[j].Event.StartDate)
	})
	return
}

func firstName(name string) string {
	if fields := strings.Fields(name); len(fields) > 0 {
		return fields[0]
	}
	return name
}

func monthsBetween(from, to time.Time) int {
	months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	return months
}

func measureNudge(person Person, growth []GrowthData, today time.Time) (rankedNudge, bool) {
	if person.IsPregnancy || person.Birthday.IsZero() {
		return rankedNudge{}, false
	}
	age := monthsBetween(person.Birthday, today)
	if age < 0 || age >= measureNudgeMaxAge {
		return rankedNudge{}, false
	}

	var last time.Time
	for _, g := range growth {
		if g.MeasurementDate.After(last) {
			last = g.MeasurementDate
		}
	}
	if last.IsZero() {
		return rankedNudge{}, false
	}

	gap := measureNudgeChildMonths
	if age < measureNudgeInfantAge {
		gap = measureNudgeInfantMonths
	}
	if monthsBetween(last, today) < gap {
		return rankedNudge{}, false
	}

	since := last.UTC().Format("January")
	if last.UTC().Year() != today.Year() {
		since = last.UTC().Format("January 2006")
	}
	return rankedNudge{DashboardNudge{
		Kind:     "measure",
		Key:      fmt.Sprintf("measure:%d:%s", person.Id, dayOf(last)),
		Text:     fmt.Sprintf("%s hasn't been measured since %s", firstName(person.Name), since),
		PersonId: person.Id,
	}, 2}, true
}

func birthdayNudge(person Person, today time.Time) (rankedNudge, bool) {
	if person.IsPregnancy || person.Birthday.IsZero() {
		return rankedNudge{}, false
	}
	birthday := person.Birthday.UTC()
	next := time.Date(today.Year(), birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
	if dayOf(next) < dayOf(today) {
		next = next.AddDate(1, 0, 0)
	}
	days := int(next.Sub(today).Hours() / 24)
	if days > birthdayLookaheadDays {
		return rankedNudge{}, false
	}

	when := "on " + next.Weekday().String()
	priority := 3
	switch days {
	case 0:
		when = "today"
		priority = 0
	case 1:
		when = "tomorrow"
	}
	return rankedNudge{DashboardNudge{
		Kind:     "birthday",
		Key:      fmt.Sprintf("birthday:%d:%d", person.Id, next.Year()),
		Text:     fmt.Sprintf("%s turns %d %s", firstName(person.Name), next.Year()-birthday.Year(), when),
		PersonId: person.Id,
	}, priority}, true
}

func faceNudge(photoCount int) (rankedNudge, bool) {
	if photoCount == 0 {
		return rankedNudge{}, false
	}
	text := fmt.Sprintf("%d photos have faces to review", photoCount)
	if photoCount == 1 {
		text = "1 photo has faces to review"
	}
	return rankedNudge{DashboardNudge{
		Kind:  "faces",
		Key:   fmt.Sprintf("faces:%d", photoCount),
		Text:  text,
		Count: photoCount,
	}, 1}, true
}

func photosWithFacesToReview(tx *vbolt.Tx, user User) int {
	photos := make(map[int]bool)
	for _, familyId := range familiesVisibleTo(tx, user) {
		if !CanAccessFamily(tx, user, familyId, AccessContribute) {
			continue
		}
		for _, face := range GetFamilyFaces(tx, familyId) {
			if face.Status == FaceUnknown || face.Status == FaceAuto {
				photos[face.PhotoId] = true
			}
		}
	}
	return len(photos)
}

func dashboardNudges(tx *vbolt.Tx, user User, today time.Time, items []FamilyTimelineItem) []DashboardNudge {
	var nudges []rankedNudge

	if cfg.EnableFaceTagging {
		if nudge, ok := faceNudge(photosWithFacesToReview(tx, user)); ok {
			nudges = append(nudges, nudge)
		}
	}

	for _, item := range items {
		if nudge, ok := birthdayNudge(item.Person, today); ok {
			nudges = append(nudges, nudge)
		}
		if !CanAccessFamily(tx, user, item.Person.FamilyId, AccessContribute) {
			continue
		}
		if nudge, ok := measureNudge(item.Person, item.GrowthData, today); ok {
			nudges = append(nudges, nudge)
		}
	}

	sort.SliceStable(nudges, func(i, j int) bool { return nudges[i].priority < nudges[j].priority })
	result := make([]DashboardNudge, 0, len(nudges))
	for _, nudge := range nudges {
		result = append(result, nudge.DashboardNudge)
	}
	return result
}
