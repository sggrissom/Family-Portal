package backend

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type SearchUnderstood struct {
	PersonIds    []int  `json:"personIds"`
	AgeFromMonth *int   `json:"ageFromMonth,omitempty"`
	AgeToMonth   *int   `json:"ageToMonth,omitempty"`
	AgeLabel     string `json:"ageLabel,omitempty"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
	Category     string `json:"category,omitempty"`
	TagIds       []int  `json:"tagIds"`
	Text         string `json:"text"`
}

const (
	SearchIgnorePerson   = "person"
	SearchIgnoreAge      = "age"
	SearchIgnoreDate     = "date"
	SearchIgnoreCategory = "category"
	SearchIgnoreTag      = "tag"
)

// parsedSearch is the understood query plus the people named in it, one
// group per mention: a photo must show someone from every group.
type parsedSearch struct {
	SearchUnderstood
	personGroups [][]int
}

func foldText(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func searchToken(word string) string {
	w := strings.Trim(foldText(word), ".,!?;:()[]{}\"'“”‘’")
	w = strings.TrimSuffix(strings.TrimSuffix(w, "'s"), "’s")
	return strings.Trim(w, "'’")
}

var relationAliases = map[string]string{
	"mom": "mother", "mum": "mother", "mommy": "mother", "mummy": "mother", "mama": "mother", "momma": "mother",
	"dad": "father", "daddy": "father", "papa": "father",
	"grandma": "grandmother", "granny": "grandmother", "nana": "grandmother", "grandmom": "grandmother",
	"grandpa": "grandfather", "granddad": "grandfather", "grandad": "grandfather",
	"bro": "brother", "sis": "sister",
}

var searchMonths = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6, "july": 7,
	"august": 8, "september": 9, "sept": 9, "october": 10, "november": 11, "december": 12,
}

var searchCategoryWords = map[string]string{
	"development": "development",
	"behavior":    "behavior", "behaviour": "behavior",
	"health":      "health",
	"achievement": "achievement", "achievements": "achievement",
	"quote": "quote", "quotes": "quote", "saying": "quote", "sayings": "quote",
	"artwork": "artwork", "artworks": "artwork", "drawing": "artwork", "drawings": "artwork",
	"interview": "interview", "interviews": "interview",
}

var (
	ageYearUnits  = map[string]bool{"y": true, "yo": true, "y/o": true, "yr": true, "yrs": true, "year": true, "years": true}
	ageMonthUnits = map[string]bool{"m": true, "mo": true, "mos": true, "month": true, "months": true}
	ageNumberUnit = regexp.MustCompile(`^(\d{1,4})([a-z/]+)$`)
)

type searchPhrase struct {
	words []string
	ids   []int
	tag   bool
}

func searchPhrases(people []Person, tags []Tag) []searchPhrase {
	byKey := map[string]*searchPhrase{}
	var order []string
	add := func(name string, id int, tag bool) {
		words := strings.Fields(foldText(name))
		if len(words) == 0 {
			return
		}
		key := fmt.Sprintf("%t|%s", tag, strings.Join(words, " "))
		p, ok := byKey[key]
		if !ok {
			p = &searchPhrase{words: words, tag: tag}
			byKey[key] = p
			order = append(order, key)
		}
		if !slices.Contains(p.ids, id) {
			p.ids = append(p.ids, id)
		}
	}

	byRelation := map[string][]int{}
	for _, person := range people {
		if person.IsPregnancy {
			continue
		}
		names := strings.Fields(person.Name)
		if len(names) == 0 {
			continue
		}
		add(person.Name, person.Id, false)
		add(names[0], person.Id, false)
		if label := foldText(person.Relationship); label != "" && !slices.Contains(byRelation[label], person.Id) {
			byRelation[label] = append(byRelation[label], person.Id)
		}
	}
	for label, ids := range byRelation {
		if len(ids) != 1 {
			continue
		}
		add(label, ids[0], false)
		for alias, canonical := range relationAliases {
			if canonical == label {
				add(alias, ids[0], false)
			}
		}
	}
	for _, tag := range tags {
		add(tag.Name, tag.Id, true)
	}

	out := make([]searchPhrase, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	return out
}

func phraseAt(phrases []searchPhrase, tokens []string, used []bool, i int, ignore map[string]bool) (best searchPhrase, found bool) {
	for _, p := range phrases {
		if (p.tag && ignore[SearchIgnoreTag]) || (!p.tag && ignore[SearchIgnorePerson]) {
			continue
		}
		if i+len(p.words) > len(tokens) || len(p.words) < len(best.words) {
			continue
		}
		if len(p.words) == len(best.words) && (p.tag || !best.tag) {
			continue
		}
		matches := true
		for j, w := range p.words {
			if used[i+j] || tokens[i+j] != w {
				matches = false
				break
			}
		}
		if matches {
			best, found = p, true
		}
	}
	return
}

type searchAge struct {
	from, to int
	label    string
}

func ageOfYears(n int) searchAge {
	return searchAge{n * 12, n*12 + 12, fmt.Sprintf("age %d", n)}
}

func ageOfMonths(n int) searchAge {
	label := fmt.Sprintf("%d months", n)
	if n == 1 {
		label = "1 month"
	}
	return searchAge{n, n + 1, label}
}

// ageAt reads an age starting at token i and returns how many tokens it
// spans, or 0. A number with no unit is an age only after "age", or when a
// person is named and the number is under 25.
func ageAt(tokens []string, used []bool, i int, personNamed bool) (searchAge, int) {
	free := func(j int) bool { return j < len(tokens) && !used[j] }
	switch tokens[i] {
	case "newborn", "newborns":
		return searchAge{0, 1, "newborn"}, 1
	case "baby", "babies":
		return searchAge{0, 12, "baby"}, 1
	}

	start, explicit := i, false
	switch tokens[i] {
	case "age", "aged":
		explicit = true
		i++
	case "at":
		i++
	}
	if !free(i) {
		return searchAge{}, 0
	}

	var age searchAge
	end := i + 1
	if n, err := strconv.Atoi(tokens[i]); err == nil {
		switch {
		case free(end) && ageYearUnits[tokens[end]] && n <= 120:
			age = ageOfYears(n)
			end++
		case free(end) && ageMonthUnits[tokens[end]] && n <= 1500:
			age = ageOfMonths(n)
			end++
		case n < 25 && (explicit || personNamed):
			age = ageOfYears(n)
		default:
			return searchAge{}, 0
		}
	} else if m := ageNumberUnit.FindStringSubmatch(tokens[i]); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch {
		case ageYearUnits[m[2]] && n <= 120:
			age = ageOfYears(n)
		case ageMonthUnits[m[2]] && n <= 1500:
			age = ageOfMonths(n)
		default:
			return searchAge{}, 0
		}
	} else {
		return searchAge{}, 0
	}
	if free(end) && tokens[end] == "old" {
		end++
	}
	return age, end - start
}

func searchYear(token string) int {
	if len(token) != 4 {
		return 0
	}
	if n, err := strconv.Atoi(token); err == nil && n >= 1900 && n <= 2100 {
		return n
	}
	return 0
}

// parseSearchQuery pulls people, an age, a date, a category and tags out of
// a search, leaving the rest as free text. Kinds named in ignore are left
// in the text instead.
func parseSearchQuery(query string, people []Person, tags []Tag, now time.Time, ignore []string) parsedSearch {
	ignored := map[string]bool{}
	for _, kind := range ignore {
		ignored[kind] = true
	}
	raw := strings.Fields(query)
	tokens := make([]string, len(raw))
	for i, word := range raw {
		tokens[i] = searchToken(word)
	}
	used := make([]bool, len(tokens))
	out := parsedSearch{SearchUnderstood: SearchUnderstood{PersonIds: []int{}, TagIds: []int{}}}

	phrases := searchPhrases(people, tags)
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "" {
			continue
		}
		p, ok := phraseAt(phrases, tokens, used, i, ignored)
		if !ok {
			continue
		}
		for j := range p.words {
			used[i+j] = true
		}
		if p.tag {
			for _, id := range p.ids {
				if !slices.Contains(out.TagIds, id) {
					out.TagIds = append(out.TagIds, id)
				}
			}
		} else {
			out.personGroups = append(out.personGroups, p.ids)
			for _, id := range p.ids {
				if !slices.Contains(out.PersonIds, id) {
					out.PersonIds = append(out.PersonIds, id)
				}
			}
		}
		i += len(p.words) - 1
	}

	haveAge := false
	for i := 0; i < len(tokens); i++ {
		if used[i] {
			continue
		}
		token := tokens[i]
		next := ""
		if i+1 < len(tokens) && !used[i+1] {
			next = tokens[i+1]
		}

		if !ignored[SearchIgnoreDate] && out.Year == 0 {
			if (token == "this" || token == "last") && next == "year" {
				out.Year = now.Year()
				if token == "last" {
					out.Year--
				}
				used[i], used[i+1] = true, true
				continue
			}
			if year := searchYear(token); year != 0 {
				out.Year = year
				used[i] = true
				continue
			}
		}
		if !ignored[SearchIgnoreDate] && out.Month == 0 {
			if month := searchMonths[token]; month != 0 {
				out.Month = month
				used[i] = true
				if year := searchYear(next); year != 0 && out.Year == 0 {
					out.Year = year
					used[i+1] = true
				}
				continue
			}
		}
		if !ignored[SearchIgnoreAge] && !haveAge {
			if age, n := ageAt(tokens, used, i, len(out.PersonIds) > 0); n > 0 {
				from, to := age.from, age.to
				out.AgeFromMonth, out.AgeToMonth, out.AgeLabel = &from, &to, age.label
				haveAge = true
				for j := i; j < i+n; j++ {
					used[j] = true
				}
				continue
			}
		}
		if !ignored[SearchIgnoreCategory] && out.Category == "" {
			if category := searchCategoryWords[token]; category != "" {
				out.Category = category
				used[i] = true
				continue
			}
		}
	}

	var text []string
	for i, word := range raw {
		if !used[i] && tokens[i] != "" && !searchFillerWords[tokens[i]] {
			text = append(text, word)
		}
	}
	out.Text = strings.Join(text, " ")
	return out
}
