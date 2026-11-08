package backend

import (
	"hash/fnv"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
)

func RegisterMemoryMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetMemories)
}

const (
	MemoryMoment    = "moment"
	MemoryQuote     = "quote"
	MemoryArtwork   = "artwork"
	MemoryInterview = "interview"
	MemoryEvent     = "event"
	MemoryPhoto     = "photo"

	memoryDefaultLimit = 12
	memoryMaxLimit     = 40
	memoryRecentDays   = 45
	recentlyAddedDays  = 14
	recentlyAddedMax   = 8
)

type GetMemoriesRequest struct {
	// Today is the device's calendar date, YYYY-MM-DD.
	Today string `json:"today"`
	// Seed fixes the selection; the same seed and Seen give the same memories.
	Seed int `json:"seed"`
	// Seen holds keys shown on earlier visits, which go to the back.
	Seen  []string `json:"seen"`
	Limit int      `json:"limit"`
	// Kinds and PersonId narrow the candidates; empty means all.
	Kinds    []string `json:"kinds"`
	PersonId int      `json:"personId"`
	// RecentlyAdded also lists older records entered in the last two weeks.
	RecentlyAdded bool `json:"recentlyAdded"`
}

type Memory struct {
	Key         string     `json:"key"`
	Kind        string     `json:"kind"`
	PersonIds   []int      `json:"personIds"`
	PhotoIds    []int      `json:"photoIds"`
	Text        string     `json:"text"`
	Context     string     `json:"context"`
	Category    string     `json:"category"`
	Date        time.Time  `json:"date"`
	RecordTime  RecordTime `json:"recordTime"`
	CreatedAt   time.Time  `json:"createdAt"`
	MilestoneId int        `json:"milestoneId,omitempty"`
	InterviewId int        `json:"interviewId,omitempty"`
	PhotoId     int        `json:"photoId,omitempty"`
	EventId     int        `json:"eventId,omitempty"`
}

type GetMemoriesResponse struct {
	Memories      []Memory `json:"memories"`
	RecentlyAdded []Memory `json:"recentlyAdded"`
}

type memoryCandidate struct {
	Memory
	weight float64
}

func GetMemories(ctx *vbeam.Context, req GetMemoriesRequest) (resp GetMemoriesResponse, err error) {
	if _, authErr := GetAuthUser(ctx); authErr != nil {
		err = ErrAuthFailure
		return
	}

	timeline, err := GetFamilyTimeline(ctx, GetFamilyTimelineRequest{IncludeActivities: true})
	if err != nil {
		return
	}

	today := dashboardToday(req.Today)
	candidates := memoryCandidates(timeline)
	if req.PersonId != 0 {
		candidates = filterMemories(candidates, func(c memoryCandidate) bool {
			return containsInt(c.PersonIds, req.PersonId)
		})
	}
	if len(req.Kinds) > 0 {
		candidates = filterMemories(candidates, func(c memoryCandidate) bool {
			return slices.Contains(req.Kinds, c.Kind)
		})
	}

	limit := req.Limit
	if limit <= 0 {
		limit = memoryDefaultLimit
	}
	limit = min(limit, memoryMaxLimit)

	cutoff := dayOf(today.AddDate(0, 0, -memoryRecentDays))
	older := filterMemories(candidates, func(c memoryCandidate) bool {
		return c.RecordTime.Resolved && c.RecordTime.End <= cutoff
	})
	resp.Memories = pickMemories(older, req.Seed, req.Seen, limit)

	resp.RecentlyAdded = []Memory{}
	if req.RecentlyAdded {
		resp.RecentlyAdded = recentlyAdded(candidates, today)
	}
	return
}

func filterMemories(candidates []memoryCandidate, keep func(memoryCandidate) bool) []memoryCandidate {
	kept := make([]memoryCandidate, 0, len(candidates))
	for _, c := range candidates {
		if keep(c) {
			kept = append(kept, c)
		}
	}
	return kept
}

func milestoneMemoryKind(m Milestone) string {
	switch {
	case m.InterviewId != 0:
		return MemoryInterview
	case m.Category == "quote":
		return MemoryQuote
	case m.Category == "artwork":
		return MemoryArtwork
	}
	return MemoryMoment
}

var memoryKindWeight = map[string]float64{
	MemoryQuote:     6,
	MemoryArtwork:   5,
	MemoryInterview: 4,
	MemoryMoment:    3,
	MemoryEvent:     3,
	MemoryPhoto:     1.5,
}

func memoryWeight(m Memory) float64 {
	weight := memoryKindWeight[m.Kind]
	if m.Kind == MemoryMoment && (m.Category == "first" || m.Category == "achievement") {
		weight += 1
	}
	if len(m.PhotoIds) > 0 {
		weight += 3
	}
	if len([]rune(strings.TrimSpace(m.Text))) >= 40 {
		weight += 1
	}
	if m.Kind != MemoryInterview && strings.TrimSpace(m.Context) != "" {
		weight += 1
	}
	return weight
}

func memoryCandidates(timeline GetFamilyTimelineResponse) []memoryCandidate {
	var memories []Memory
	inMilestones := map[int]bool{}
	photos := map[int]*Memory{}
	var photoOrder []int

	for _, item := range timeline.People {
		for _, m := range item.Milestones {
			if strings.TrimSpace(m.Description) == "" {
				continue
			}
			memories = append(memories, Memory{
				Key:         "m" + strconv.Itoa(m.Id),
				Kind:        milestoneMemoryKind(m),
				PersonIds:   []int{m.PersonId},
				PhotoIds:    append([]int{}, m.PhotoIds...),
				Text:        m.Description,
				Context:     m.Context,
				Category:    m.Category,
				Date:        m.MilestoneDate,
				RecordTime:  m.RecordTime,
				CreatedAt:   m.CreatedAt,
				MilestoneId: m.Id,
				InterviewId: m.InterviewId,
			})
			for _, id := range m.PhotoIds {
				inMilestones[id] = true
			}
		}
		for _, image := range item.Photos {
			if image.Status != PhotoReady {
				continue
			}
			if existing, ok := photos[image.Id]; ok {
				existing.PersonIds = append(existing.PersonIds, item.Person.Id)
				continue
			}
			photos[image.Id] = &Memory{
				Key:        "p" + strconv.Itoa(image.Id),
				Kind:       MemoryPhoto,
				PersonIds:  []int{item.Person.Id},
				PhotoIds:   []int{image.Id},
				Text:       strings.TrimSpace(image.Title),
				Context:    strings.TrimSpace(image.Description),
				Date:       image.PhotoDate,
				RecordTime: image.RecordTime,
				CreatedAt:  image.CreatedAt,
				PhotoId:    image.Id,
			}
			photoOrder = append(photoOrder, image.Id)
		}
	}

	for _, id := range photoOrder {
		if !inMilestones[id] {
			memories = append(memories, *photos[id])
		}
	}

	events := map[int]*Memory{}
	var eventOrder []int
	for _, a := range timeline.Appearances {
		event := a.Detail.Event
		memory, ok := events[event.Id]
		if !ok {
			memory = &Memory{
				Key:        "e" + strconv.Itoa(event.Id),
				Kind:       MemoryEvent,
				PersonIds:  []int{},
				PhotoIds:   []int{},
				Text:       event.Name,
				Date:       event.StartDate,
				RecordTime: exactRecordTime(event.StartDate, ""),
				CreatedAt:  a.Detail.Appearance.CreatedAt,
				EventId:    event.Id,
			}
			events[event.Id] = memory
			eventOrder = append(eventOrder, event.Id)
		}
		for _, id := range a.PersonIds {
			if !containsInt(memory.PersonIds, id) {
				memory.PersonIds = append(memory.PersonIds, id)
			}
		}
		for _, id := range a.Detail.PhotoIds {
			if !containsInt(memory.PhotoIds, id) {
				memory.PhotoIds = append(memory.PhotoIds, id)
			}
		}
		if memory.Context == "" {
			memory.Context = strings.TrimSpace(a.Detail.Appearance.Notes)
		}
	}
	for _, id := range eventOrder {
		memory := events[id]
		if len(memory.PhotoIds) > 0 || memory.Context != "" {
			memories = append(memories, *memory)
		}
	}

	candidates := make([]memoryCandidate, len(memories))
	for i, m := range memories {
		candidates[i] = memoryCandidate{Memory: m, weight: memoryWeight(m)}
	}
	return candidates
}

func memoryDraw(seed int, key string) float64 {
	h := fnv.New64a()
	h.Write([]byte(strconv.Itoa(seed) + ":" + key))
	return (float64(h.Sum64()>>11) + 0.5) / float64(1<<53)
}

func memoryYear(m Memory) string {
	if len(m.RecordTime.Start) >= 4 {
		return m.RecordTime.Start[:4]
	}
	return ""
}

func sharesPerson(a, b []int) bool {
	for _, id := range a {
		if containsInt(b, id) {
			return true
		}
	}
	return false
}

// pickMemories orders candidates by a weighted draw fixed by seed, then takes
// them greedily so neighbours differ in kind, person and year and no image
// or text shows twice. Constraints relax only when nothing else is left, and seen
// memories come only after every unseen one.
func pickMemories(candidates []memoryCandidate, seed int, seen []string, limit int) []Memory {
	seenKeys := map[string]bool{}
	for _, key := range seen {
		seenKeys[key] = true
	}

	type ranked struct {
		memoryCandidate
		rank float64
		seen bool
	}
	pool := make([]ranked, len(candidates))
	for i, c := range candidates {
		pool[i] = ranked{c, -math.Log(memoryDraw(seed, c.Key)) / c.weight, seenKeys[c.Key]}
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].rank < pool[j].rank })

	picked := []Memory{}
	taken := make([]bool, len(pool))
	usedPhotos := map[int]bool{}

	usedTexts := map[string]bool{}
	textOf := func(m Memory) string { return strings.ToLower(strings.TrimSpace(m.Text)) }

	fits := func(m Memory, strictness int) bool {
		for _, id := range m.PhotoIds {
			if usedPhotos[id] {
				return false
			}
		}
		if text := textOf(m); text != "" && usedTexts[text] {
			return false
		}
		if strictness == 0 || len(picked) == 0 {
			return true
		}
		last := picked[len(picked)-1]
		if last.Kind == m.Kind {
			return false
		}
		if strictness == 1 {
			return true
		}
		if memoryYear(last) == memoryYear(m) {
			return false
		}
		for i := len(picked) - 1; i >= 0 && i >= len(picked)-2; i-- {
			if sharesPerson(picked[i].PersonIds, m.PersonIds) {
				return false
			}
		}
		return true
	}

	for len(picked) < limit {
		chosen := -1
		for _, allowSeen := range []bool{false, true} {
			for strictness := 2; strictness >= 0 && chosen < 0; strictness-- {
				for i := range pool {
					if !taken[i] && (allowSeen || !pool[i].seen) && fits(pool[i].Memory, strictness) {
						chosen = i
						break
					}
				}
			}
		}
		if chosen < 0 {
			break
		}
		taken[chosen] = true
		m := pool[chosen].Memory
		for _, id := range m.PhotoIds {
			usedPhotos[id] = true
		}
		usedTexts[textOf(m)] = true
		picked = append(picked, m)
	}
	return picked
}

// recentlyAdded is what was entered in the last two weeks about an earlier
// time, which the dashboard's by-date Recent list would bury.
func recentlyAdded(candidates []memoryCandidate, today time.Time) []Memory {
	since := today.AddDate(0, 0, -(recentlyAddedDays - 1))
	recentFrom := dayOf(since)
	var added []Memory
	for _, c := range candidates {
		if c.CreatedAt.Before(since) {
			continue
		}
		if c.RecordTime.ExactDay() && dayOf(c.Date) >= recentFrom {
			continue
		}
		added = append(added, c.Memory)
	}
	sort.SliceStable(added, func(i, j int) bool { return added[i].CreatedAt.After(added[j].CreatedAt) })
	if len(added) > recentlyAddedMax {
		added = added[:recentlyAddedMax]
	}
	if added == nil {
		added = []Memory{}
	}
	return added
}
