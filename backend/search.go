package backend

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

const (
	searchPeopleLimit   = 8
	searchMomentLimit   = 20
	searchMaxMoments    = 100
	searchPhotoLimit    = 12
	searchOtherLimit    = 6
	searchMaxQueryRunes = 200
)

type SearchRequest struct {
	Query  string   `json:"query"`
	Ignore []string `json:"ignore,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

type ActivityHit struct {
	Kind   string    `json:"kind"`
	Id     int       `json:"id"`
	Name   string    `json:"name"`
	Detail string    `json:"detail"`
	Date   time.Time `json:"date"`
}

const (
	ActivityHitActivity = "activity"
	ActivityHitSeason   = "season"
	ActivityHitEvent    = "event"
)

type SearchResponse struct {
	Query       string            `json:"query"`
	Understood  SearchUnderstood  `json:"understood"`
	People      []Person          `json:"people"`
	Moments     []Milestone       `json:"moments"`
	MomentCount int               `json:"momentCount"`
	Interviews  []Interview       `json:"interviews"`
	Photos      []PhotoWithPeople `json:"photos"`
	MorePhotos  bool              `json:"morePhotos"`
	Tags        []Tag             `json:"tags"`
	Places      []PhotoPlace      `json:"places"`
	Activities  []ActivityHit     `json:"activities"`
	Books       []BookSummary     `json:"books"`
	SearchMode  string            `json:"searchMode"`
}

func RegisterSearchMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, Search)
}

func isSearchSeparator(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// searchTerms is the free text of a search as folded words, without filler.
func searchTerms(text string) []string {
	var terms []string
	for _, word := range strings.FieldsFunc(foldText(text), isSearchSeparator) {
		if !searchFillerWords[word] {
			terms = append(terms, word)
		}
	}
	return terms
}

func searchFieldWords(fields ...string) []string {
	var words []string
	for _, field := range fields {
		words = append(words, strings.FieldsFunc(foldText(field), isSearchSeparator)...)
	}
	return words
}

// termsMatch reports whether every term starts some word.
func termsMatch(terms []string, words []string) bool {
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		found := false
		for _, word := range words {
			if strings.HasPrefix(word, term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func emptySearchResponse(query string) SearchResponse {
	return SearchResponse{
		Query:      query,
		Understood: SearchUnderstood{PersonIds: []int{}, TagIds: []int{}},
		People:     []Person{},
		Moments:    []Milestone{},
		Interviews: []Interview{},
		Photos:     []PhotoWithPeople{},
		Tags:       []Tag{},
		Places:     []PhotoPlace{},
		Activities: []ActivityHit{},
		Books:      []BookSummary{},
		SearchMode: "semantic",
	}
}

func Search(ctx *vbeam.Context, req SearchRequest) (resp SearchResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	query := strings.TrimSpace(req.Query)
	if runes := []rune(query); len(runes) > searchMaxQueryRunes {
		query = strings.TrimSpace(string(runes[:searchMaxQueryRunes]))
	}
	resp = emptySearchResponse(query)
	if query == "" {
		return
	}

	limit := searchMomentLimit
	if req.Limit > 0 {
		limit = min(req.Limit, searchMaxMoments)
	}

	s := newSearcher(ctx.Tx, user)
	parsed := parseSearchQuery(query, s.people, s.tags, time.Now(), req.Ignore)
	terms := searchTerms(parsed.Text)
	resp.Understood = parsed.SearchUnderstood
	if len(terms) == 0 && !parsed.hasFilters() {
		return
	}

	resp.People = s.matchPeople(parsed, terms)
	var momentsSemantic, photosSemantic bool
	resp.Moments, resp.MomentCount, momentsSemantic = s.matchMoments(parsed, terms, limit)
	resp.Interviews = interviewsOfAnswers(ctx.Tx, resp.Moments)
	var photoPlaces map[int]PhotoPlace
	if len(terms) > 0 {
		photoPlaces = photoPlacesMatchingATerm(ctx.Tx, user, terms)
	}
	resp.Photos, resp.MorePhotos, photosSemantic = s.matchPhotos(parsed, terms, photoPlaces)
	if len(terms) > 0 {
		resp.Tags = s.matchTags(parsed, terms)
		resp.Places = s.matchPlaces(terms, photoPlaces)
		resp.Activities = s.matchActivities(terms)
		resp.Books = s.matchBooks(terms)
	} else {
		resp.Tags = s.understoodTags(parsed)
	}
	if parsed.Text != "" && (!momentsSemantic || !photosSemantic) {
		resp.SearchMode = "text"
	}
	return
}

type searcher struct {
	tx       *vbolt.Tx
	user     User
	people   []Person
	tags     []Tag
	families []int
	persons  map[int]Person
}

func newSearcher(tx *vbolt.Tx, user User) *searcher {
	s := &searcher{tx: tx, user: user, persons: map[int]Person{}}
	s.families = familiesVisibleTo(tx, user)
	s.people = GetVisiblePeople(tx, user)
	seen := map[int]bool{}
	for _, p := range s.people {
		seen[p.Id] = true
	}
	for _, p := range linkedPeopleVisibleTo(tx, user, ScopePeople) {
		if !seen[p.Id] {
			seen[p.Id] = true
			s.people = append(s.people, p)
		}
	}
	labelPeopleFor(tx, user, s.people)
	for _, p := range s.people {
		s.persons[p.Id] = p
	}
	s.tags = getVisibleTags(tx, user)
	return s
}

func (s *searcher) person(id int) Person {
	p, ok := s.persons[id]
	if !ok {
		p = GetPersonById(s.tx, id)
		s.persons[id] = p
	}
	return p
}

func (s *searcher) ageMatches(parsed parsedSearch, personId int, at time.Time) bool {
	birthday := s.person(personId).Birthday
	if birthday.IsZero() {
		return false
	}
	months := ageInMonths(birthday, at)
	return months >= *parsed.AgeFromMonth && months < *parsed.AgeToMonth
}

func (p parsedSearch) hasFilters() bool {
	return len(p.PersonIds) > 0 || p.AgeFromMonth != nil || p.Year != 0 || p.Month != 0 || p.Category != ""
}

func dateMatches(parsed parsedSearch, at time.Time) bool {
	return (parsed.Year == 0 || at.Year() == parsed.Year) && (parsed.Month == 0 || int(at.Month()) == parsed.Month)
}

func hasOneOfEach(have []int, groups [][]int) bool {
	present := map[int]bool{}
	for _, id := range have {
		present[id] = true
	}
	for _, group := range groups {
		found := false
		for _, id := range group {
			if present[id] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *searcher) matchPeople(parsed parsedSearch, terms []string) []Person {
	out := []Person{}
	added := map[int]bool{}
	add := func(p Person) {
		if added[p.Id] || len(out) >= searchPeopleLimit {
			return
		}
		added[p.Id] = true
		p.Age = calculatePersonAge(p.Birthday, p.IsPregnancy)
		out = append(out, p)
	}
	for _, id := range parsed.PersonIds {
		add(s.person(id))
	}
	if len(terms) > 0 {
		for _, p := range s.people {
			if termsMatch(terms, searchFieldWords(p.Name, p.Relationship)) {
				add(p)
			}
		}
	}
	return out
}

// matchMoments returns the visible moments passing the understood filters
// that match the text: word matches first, those matching in the
// description ahead of the rest, then moments close in meaning. With no
// text, every moment passing the filters, newest first.
func (s *searcher) matchMoments(parsed parsedSearch, terms []string, limit int) (out []Milestone, total int, semantic bool) {
	tagNames := map[int]string{}
	for _, tag := range s.tags {
		tagNames[tag.Id] = tag.Name
	}
	momentTags := map[int][]int{}
	tagsOf := func(m Milestone) []int {
		ids, ok := momentTags[m.Id]
		if !ok {
			ids = GetMilestoneTagIds(s.tx, m.Id)
			momentTags[m.Id] = ids
		}
		return ids
	}

	var candidates []Milestone
	for _, m := range visibleMilestones(s.tx, s.user) {
		if len(parsed.PersonIds) > 0 && !hasOneOfEach([]int{m.PersonId}, [][]int{parsed.PersonIds}) {
			continue
		}
		if parsed.AgeFromMonth != nil && !s.ageMatches(parsed, m.PersonId, m.MilestoneDate) {
			continue
		}
		if !dateMatches(parsed, m.MilestoneDate) || (parsed.Category != "" && m.Category != parsed.Category) {
			continue
		}
		candidates = append(candidates, m)
	}
	newestFirst := func(ms []Milestone) {
		sort.SliceStable(ms, func(i, j int) bool {
			if !ms[i].MilestoneDate.Equal(ms[j].MilestoneDate) {
				return ms[i].MilestoneDate.After(ms[j].MilestoneDate)
			}
			return ms[i].Id > ms[j].Id
		})
	}

	semantic = true
	var ranked []Milestone
	if len(terms) == 0 {
		ranked = candidates
		newestFirst(ranked)
	} else {
		var inDescription, elsewhere []Milestone
		hit := map[int]bool{}
		for _, m := range candidates {
			if termsMatch(terms, searchFieldWords(m.Description)) {
				inDescription = append(inDescription, m)
				hit[m.Id] = true
				continue
			}
			fields := []string{m.Description, m.Context, m.Category}
			for _, id := range tagsOf(m) {
				fields = append(fields, tagNames[id])
			}
			if termsMatch(terms, searchFieldWords(fields...)) {
				elsewhere = append(elsewhere, m)
				hit[m.Id] = true
			}
		}
		newestFirst(inDescription)
		newestFirst(elsewhere)
		ranked = append(inDescription, elsewhere...)

		var scores map[int]float64
		if len(candidates) > 0 {
			scores = semanticMilestoneScores(s.tx, parsed.Text, candidates)
			semantic = scores != nil
		}
		var close []Milestone
		for _, m := range candidates {
			if !hit[m.Id] && scores[m.Id] >= milestoneSearchMinScore {
				close = append(close, m)
			}
		}
		sort.SliceStable(close, func(i, j int) bool { return scores[close[i].Id] > scores[close[j].Id] })
		ranked = append(ranked, close...)
	}

	total = len(ranked)
	out = ranked[:min(limit, total)]
	if out == nil {
		out = []Milestone{}
	}
	return
}

func (s *searcher) photoListing(parsed parsedSearch) photoListing {
	var personIds []int
	if len(parsed.personGroups) > 0 {
		personIds = parsed.personGroups[0]
	}
	dateFrom, dateTo := "", ""
	if parsed.Year != 0 {
		from := time.Date(parsed.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(1, 0, -1)
		if parsed.Month != 0 {
			from = time.Date(parsed.Year, time.Month(parsed.Month), 1, 0, 0, 0, 0, time.UTC)
			to = from.AddDate(0, 1, -1)
		}
		dateFrom, dateTo = from.Format(dateOnlyLayout), to.Format(dateOnlyLayout)
	}
	return newPhotoListing(s.tx, s.user, personIds, nil, "", dateFrom, dateTo)
}

// photoAgeMatches checks the age against the named people in the photo, or
// anyone in it when no one is named.
func (s *searcher) photoAgeMatches(parsed parsedSearch, image Image, present []int) bool {
	named := map[int]bool{}
	for _, id := range parsed.PersonIds {
		named[id] = true
	}
	for _, id := range present {
		if (len(named) == 0 || named[id]) && s.ageMatches(parsed, id, image.PhotoDate) {
			return true
		}
	}
	return false
}

func (s *searcher) matchPhotos(parsed parsedSearch, terms []string, places map[int]PhotoPlace) (out []PhotoWithPeople, more bool, semantic bool) {
	l := s.photoListing(parsed)
	keep := func(image Image) bool {
		if !l.passes(image) || !dateMatches(parsed, image.PhotoDate) {
			return false
		}
		if len(parsed.personGroups) == 0 && parsed.AgeFromMonth == nil {
			return true
		}
		var present []int
		for _, pp := range GetPhotoPersonsByPhoto(s.tx, image.Id) {
			present = append(present, pp.PersonId)
		}
		if !hasOneOfEach(present, parsed.personGroups) {
			return false
		}
		return parsed.AgeFromMonth == nil || s.photoAgeMatches(parsed, image, present)
	}

	var candidates []Image
	if parsed.Text == "" {
		l.read(l.newest, func(image Image) bool {
			if keep(image) {
				candidates = append(candidates, image)
			}
			return len(candidates) <= searchPhotoLimit
		})
		semantic = true
	} else {
		l.read(l.newest, func(image Image) bool {
			if keep(image) {
				candidates = append(candidates, image)
			}
			return true
		})
		var mode string
		candidates, mode = rankPhotoQuery(l, candidates, parsed.Text, photoTextMatcher(l, terms, places))
		semantic = mode == "semantic"
	}

	more = len(candidates) > searchPhotoLimit
	return photosWithPeople(s.tx, candidates[:min(len(candidates), searchPhotoLimit)], nil), more, semantic
}

func (s *searcher) understoodTags(parsed parsedSearch) []Tag {
	out := []Tag{}
	for _, tag := range s.tags {
		if hasOneOfEach([]int{tag.Id}, [][]int{parsed.TagIds}) && len(out) < searchOtherLimit {
			out = append(out, tag)
		}
	}
	return out
}

func (s *searcher) matchTags(parsed parsedSearch, terms []string) []Tag {
	out := s.understoodTags(parsed)
	for _, tag := range s.tags {
		if len(out) >= searchOtherLimit {
			break
		}
		if termsMatch(terms, searchFieldWords(tag.Name)) && !hasOneOfEach([]int{tag.Id}, [][]int{parsed.TagIds}) {
			out = append(out, tag)
		}
	}
	return out
}

// matchPlaces looks through the family's named places and the places its
// photos were taken. Locations stay with the families that own the photos.
func (s *searcher) matchPlaces(terms []string, photoPlaces map[int]PhotoPlace) []PhotoPlace {
	out := []PhotoPlace{}
	seen := map[string]bool{}
	add := func(place PhotoPlace) {
		if place.Key == "" || seen[place.Key] {
			return
		}
		seen[place.Key] = true
		if termsMatch(terms, searchFieldWords(place.Name)) {
			out = append(out, place)
		}
	}
	for _, familyId := range s.families {
		for _, fp := range GetFamilyPlaces(s.tx, familyId) {
			add(PhotoPlace{Key: familyPlaceKey(fp.Id), Name: fp.Name, FamilyPlaceId: fp.Id, Latitude: fp.Latitude, Longitude: fp.Longitude})
		}
	}
	for _, place := range photoPlaces {
		add(place)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Key[0] == 'f') != (out[j].Key[0] == 'f') {
			return out[i].Key[0] == 'f'
		}
		return out[i].Name < out[j].Name
	})
	return out[:min(len(out), searchOtherLimit)]
}

func (s *searcher) matchActivities(terms []string) []ActivityHit {
	var activities, seasons, events []ActivityHit
	for _, familyId := range s.families {
		names := map[int]string{}
		for _, a := range GetFamilyActivities(s.tx, familyId) {
			names[a.Id] = a.Name
			if termsMatch(terms, searchFieldWords(a.Name)) {
				activities = append(activities, ActivityHit{Kind: ActivityHitActivity, Id: a.Id, Name: a.Name})
			}
		}
		seasonNames := map[int]string{}
		for _, season := range GetFamilySeasons(s.tx, familyId) {
			seasonNames[season.Id] = season.Name
			if termsMatch(terms, searchFieldWords(season.Name)) {
				seasons = append(seasons, ActivityHit{Kind: ActivityHitSeason, Id: season.Id, Name: season.Name, Detail: names[season.ActivityId], Date: season.StartDate})
			}
		}
		for _, event := range GetFamilyEvents(s.tx, familyId) {
			if termsMatch(terms, searchFieldWords(event.Name, event.Host, event.Location)) {
				detail := event.Location
				if detail == "" {
					detail = seasonNames[event.SeasonId]
				}
				events = append(events, ActivityHit{Kind: ActivityHitEvent, Id: event.Id, Name: event.Name, Detail: detail, Date: event.StartDate})
			}
		}
	}
	sort.SliceStable(activities, func(i, j int) bool { return activities[i].Name < activities[j].Name })
	newest := func(hits []ActivityHit) {
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].Date.After(hits[j].Date) })
	}
	newest(seasons)
	newest(events)
	out := append(append(append([]ActivityHit{}, activities...), seasons...), events...)
	return out[:min(len(out), searchOtherLimit)]
}

func (s *searcher) matchBooks(terms []string) []BookSummary {
	out := []BookSummary{}
	for _, familyId := range s.families {
		for _, book := range GetFamilyBooks(s.tx, familyId) {
			if !termsMatch(terms, searchFieldWords(book.Title)) {
				continue
			}
			if summary, ok := bookSummaryFor(s.tx, s.user, book); ok {
				out = append(out, summary)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out[:min(len(out), searchOtherLimit)]
}
