package backend

import (
	"errors"
	"family/vision"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.hasen.dev/vbolt"
)

// Calibrated on the seed photos with "a photo of <query>": subjects that are
// absent top out near 0.22, present ones score 0.23 to 0.36, and the window
// drops the tail of weak matches below a strong one.
const (
	searchMinScore  = 0.23
	searchWindow    = 0.05
	searchTextBonus = 0.05
	queryCacheSize  = 256
)

// familyVectors caches each family's embeddings in memory. Writes mark the
// family stale rather than patching the cache, and the next search reloads it.
var familyVectors = struct {
	sync.Mutex
	byFamily map[int]map[int][]float32
}{byFamily: map[int]map[int][]float32{}}

func invalidateFamilyVectors(familyId int) {
	familyVectors.Lock()
	delete(familyVectors.byFamily, familyId)
	familyVectors.Unlock()
}

func vectorsForFamily(tx *vbolt.Tx, familyId int) map[int][]float32 {
	familyVectors.Lock()
	cached, ok := familyVectors.byFamily[familyId]
	familyVectors.Unlock()
	if ok {
		return cached
	}
	loaded := map[int][]float32{}
	for _, e := range familyEmbeddings(tx, familyId) {
		if !embeddingOutdated(e, true) {
			loaded[e.PhotoId] = e.Vector
		}
	}
	familyVectors.Lock()
	familyVectors.byFamily[familyId] = loaded
	familyVectors.Unlock()
	return loaded
}

var queryVectors = struct {
	sync.Mutex
	byText map[string][]float32
}{byText: map[string][]float32{}}

func embedQuery(text string) ([]float32, error) {
	client := globalVisionClient
	if client == nil {
		return nil, errVisionUnavailable
	}
	key := strings.ToLower(strings.TrimSpace(text))
	queryVectors.Lock()
	cached, ok := queryVectors.byText[key]
	queryVectors.Unlock()
	if ok {
		return cached, nil
	}
	vectors, err := client.embedTexts("clip", []string{"a photo of " + key})
	if err != nil {
		if errors.Is(err, errVisionUnavailable) {
			noteVisionReachable(false)
		}
		return nil, err
	}
	noteVisionReachable(true)
	if len(vectors) != 1 {
		return nil, fmt.Errorf("vision daemon returned %d vectors", len(vectors))
	}
	queryVectors.Lock()
	if len(queryVectors.byText) >= queryCacheSize {
		queryVectors.byText = map[string][]float32{}
	}
	queryVectors.byText[key] = vectors[0]
	queryVectors.Unlock()
	return vectors[0], nil
}

func searchWords(text string) []string {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, ".,!?;:()[]{}\"")
		w = strings.TrimSuffix(strings.TrimSuffix(w, "'s"), "’s")
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

// peopleInQuery pulls the names of visible people out of a search. A first
// name shared by two visible people is left in the text.
func peopleInQuery(people []Person, query string) (personIds []int, rest string) {
	byFirst := map[string][]int{}
	for _, p := range people {
		if p.IsPregnancy {
			continue
		}
		if fields := strings.Fields(strings.ToLower(p.Name)); len(fields) > 0 {
			byFirst[fields[0]] = append(byFirst[fields[0]], p.Id)
		}
	}
	var kept []string
	seen := map[int]bool{}
	for _, word := range strings.Fields(query) {
		clean := searchWords(word)
		if len(clean) == 1 {
			if ids := byFirst[clean[0]]; len(ids) == 1 {
				if !seen[ids[0]] {
					seen[ids[0]] = true
					personIds = append(personIds, ids[0])
				}
				continue
			}
		}
		kept = append(kept, word)
	}
	rest = strings.Join(kept, " ")
	if len(searchWords(rest)) == 0 {
		rest = ""
	}
	return
}

var searchFillerWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "at": true, "in": true, "on": true,
	"of": true, "with": true, "to": true, "for": true, "photo": true, "photos": true,
	"picture": true, "pictures": true, "when": true, "was": true, "old": true,
}

func matchesATerm(terms []string, name string) bool {
	words := searchFieldWords(name)
	for _, term := range terms {
		if termsMatch([]string{term}, words) {
			return true
		}
	}
	return false
}

// photoTextMatcher reports whether a photo's title, description, tags or
// place name contain every search term as a word prefix. Only tags and
// places whose names match some term can make a difference, so those are
// gathered once up front.
func photoTextMatcher(l photoListing, terms []string, places map[int]PhotoPlace, words photoWords) func(*Image) bool {
	if len(terms) == 0 {
		return func(*Image) bool { return false }
	}
	extra := map[int][]string{}
	for _, tag := range getVisibleTags(l.tx, l.user) {
		if matchesATerm(terms, tag.Name) {
			for _, photoId := range GetTagPhotoIds(l.tx, tag.Id) {
				extra[photoId] = append(extra[photoId], tag.Name)
			}
		}
	}
	for photoId, place := range places {
		extra[photoId] = append(extra[photoId], place.Name)
	}
	return func(image *Image) bool {
		own, ok := words[image.FamilyId][image.Id]
		if !ok {
			own = searchFieldWords(image.Title, image.Description)
		}
		if more, found := extra[image.Id]; found {
			own = append(append([]string{}, own...), searchFieldWords(more...)...)
		}
		return termsMatch(terms, own)
	}
}

type scoredPhoto struct {
	index int
	score float64
}

// rankPhotos orders candidates by similarity to the query, keeping those
// within searchWindow of the best match and above searchMinScore, plus any
// text matches. Without a query vector (no daemon), only the text matches
// remain.
func rankPhotos(tx *vbolt.Tx, candidates []Image, queryVector []float32, textMatches func(*Image) bool) []Image {
	scored := make([]scoredPhoto, 0, len(candidates))
	textHit := make([]bool, len(candidates))
	best := 0.0
	for i := range candidates {
		image := &candidates[i]
		score := 0.0
		if queryVector != nil {
			if v, ok := vectorsForFamily(tx, image.FamilyId)[image.Id]; ok {
				score = float64(vision.Dot(queryVector, v))
			}
		}
		best = max(best, score)
		if textMatches(image) {
			textHit[i] = true
			score += searchTextBonus
		}
		scored = append(scored, scoredPhoto{i, score})
	}
	cutoff := max(searchMinScore, best-searchWindow)
	kept := scored[:0]
	for _, s := range scored {
		if textHit[s.index] || (queryVector != nil && s.score >= cutoff) {
			kept = append(kept, s)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].score > kept[j].score })
	out := make([]Image, len(kept))
	for i, s := range kept {
		out[i] = candidates[s.index]
	}
	return out
}

func parseSearchCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(strings.TrimPrefix(cursor, "s"))
	if err != nil || !strings.HasPrefix(cursor, "s") || offset < 0 {
		return 0, errors.New("Invalid search cursor")
	}
	return offset, nil
}

func hasEveryone(present []int, personIds []int) bool {
	for _, id := range personIds {
		if !slices.Contains(present, id) {
			return false
		}
	}
	return true
}

// searchPhotos ranks every photo in the listing that shows all the people
// named in the query, and pages through the ranking by offset.
func searchPhotos(l photoListing, query string, offset int, limit int) (page []Image, nextCursor string, matched []int, mode string) {
	matched, query = peopleInQuery(GetVisiblePeople(l.tx, l.user), query)
	if len(l.personIds) == 0 && len(matched) > 0 {
		l.personIds = matched[:1]
	}

	candidates, words := l.searchCandidates(func(image Image) bool {
		return l.passes(image) && (len(matched) == 0 || hasEveryone(l.photoPeople(image), matched))
	})
	if query != "" {
		terms := searchTerms(query)
		candidates, mode = rankPhotoQuery(l, candidates, query, photoTextMatcher(l, terms, l.cachedPlaces(terms), words))
	} else {
		mode = "semantic"
	}

	end := len(candidates)
	if limit > 0 && offset+limit < end {
		end = offset + limit
		nextCursor = fmt.Sprintf("s%d", end)
	}
	if offset < end {
		page = reloadImages(l.tx, candidates[offset:end])
	}
	return
}

func rankPhotoQuery(l photoListing, candidates []Image, query string, textMatches func(*Image) bool) ([]Image, string) {
	mode := "semantic"
	vector, err := embedQuery(query)
	if err != nil {
		mode = "text"
	}
	return rankPhotos(l.tx, candidates, vector, textMatches), mode
}
