package backend

import (
	"math"
	"slices"
	"sort"
	"sync"

	"go.hasen.dev/vbolt"
)

// familyPhotoText is what search needs of a family's photos. photos are slim
// copies, newest first, holding only Id, FamilyId, Status, PhotoDate, Title
// and Description; the maps are keyed by photo id.
type familyPhotoText struct {
	builtAt int
	photos  []Image
	index   map[int]int
	words   map[int][]string
	people  map[int][]int
	places  map[int]PhotoPlace
}

type dbPhotoText struct {
	validFrom int
	families  map[int]familyPhotoText
}

// photoTextCache keeps each family's photo text between searches. A photo
// write records its transaction id, and a cache only counts once it was
// built from a snapshot that includes that write, so a search racing an
// uncommitted write can't store what it read.
var photoTextCache = struct {
	sync.Mutex
	byDB map[*vbolt.DB]*dbPhotoText
}{byDB: map[*vbolt.DB]*dbPhotoText{}}

func photoTextState(db *vbolt.DB) *dbPhotoText {
	state, ok := photoTextCache.byDB[db]
	if !ok {
		state = &dbPhotoText{families: map[int]familyPhotoText{}}
		photoTextCache.byDB[db] = state
	}
	return state
}

// notePhotoWrite must run in any transaction that creates or deletes a
// photo, or changes its status, date, title, description, people, location
// features, or the family's named places.
func notePhotoWrite(tx *vbolt.Tx) {
	photoTextCache.Lock()
	defer photoTextCache.Unlock()
	state := photoTextState(tx.DB())
	state.validFrom = max(state.validFrom, tx.ID())
	clear(state.families)
}

func familyPhotoTexts(tx *vbolt.Tx, familyId int) familyPhotoText {
	photoTextCache.Lock()
	state := photoTextState(tx.DB())
	cached, ok := state.families[familyId]
	usable := ok && cached.builtAt >= state.validFrom
	photoTextCache.Unlock()
	if usable {
		return cached
	}

	built := buildFamilyPhotoText(tx, familyId)

	photoTextCache.Lock()
	defer photoTextCache.Unlock()
	state = photoTextState(tx.DB())
	if !tx.Writable() && built.builtAt >= state.validFrom {
		state.families[familyId] = built
	}
	return built
}

func buildFamilyPhotoText(tx *vbolt.Tx, familyId int) familyPhotoText {
	built := familyPhotoText{
		builtAt: tx.ID(),
		index:   map[int]int{},
		words:   map[int][]string{},
		people:  map[int][]int{},
		places:  map[int]PhotoPlace{},
	}
	stream := newPhotoStream(ImageByFamilyDateIndex, familyId, photoKey{seconds: math.MaxInt64, id: math.MaxInt})
	for stream.peek(tx) {
		image := stream.pop()
		built.index[image.Id] = len(built.photos)
		built.photos = append(built.photos, Image{
			Id: image.Id, FamilyId: image.FamilyId, Status: image.Status,
			PhotoDate: image.PhotoDate, Title: image.Title, Description: image.Description,
		})
		built.words[image.Id] = searchFieldWords(image.Title, image.Description)
		for _, pp := range GetPhotoPersonsByPhoto(tx, image.Id) {
			built.people[image.Id] = append(built.people[image.Id], pp.PersonId)
		}
	}
	familyPlaces := GetFamilyPlaces(tx, familyId)
	for _, features := range familyFeatures(tx, familyId) {
		if place, ok := resolvePlace(familyPlaces, features); ok {
			built.places[features.PhotoId] = place
		}
	}
	return built
}

// familyTexts is familyPhotoTexts read once per listing.
func (l photoListing) familyTexts(familyId int) familyPhotoText {
	texts, ok := l.texts[familyId]
	if !ok {
		texts = familyPhotoTexts(l.tx, familyId)
		l.texts[familyId] = texts
	}
	return texts
}

// photoPeople lists who is in a photo, from the cache for the user's own
// families.
func (l photoListing) photoPeople(image Image) []int {
	if l.own[image.FamilyId] {
		return l.familyTexts(image.FamilyId).people[image.Id]
	}
	var ids []int
	for _, pp := range GetPhotoPersonsByPhoto(l.tx, image.Id) {
		ids = append(ids, pp.PersonId)
	}
	return ids
}

// slimImage is the cached copy of an own-family photo, or the stored photo
// for anyone else's.
func (l photoListing) slimImage(id int) (Image, bool) {
	for _, familyId := range l.ownList {
		texts := l.familyTexts(familyId)
		if i, ok := texts.index[id]; ok {
			return texts.photos[i], true
		}
	}
	image := GetImageById(l.tx, id)
	return image, image.Id != 0
}

// photoWords holds the cached search words of photos, by family then photo.
type photoWords map[int]map[int][]string

// searchCandidates gathers every photo in the listing that keep accepts,
// newest first, for ranking against a query. Photos of the user's own
// families come from the cache as slim copies, with their search words;
// reload the ones that are returned.
func (l photoListing) searchCandidates(keep func(Image) bool) (photos []Image, words photoWords) {
	words = photoWords{}
	for _, familyId := range l.ownList {
		words[familyId] = l.familyTexts(familyId).words
	}
	inWindow := func(image *Image) bool {
		return image.PhotoDate.Unix() >= l.oldest && !photoKeyOf(*image).newerThan(l.newest)
	}

	sorted := true
	if len(l.personIds) > 0 {
		seen := map[int]bool{}
		for _, personId := range l.personIds {
			var ids []int
			vbolt.ReadTermTargets(l.tx, ImageByPersonDateIndex, personId, &ids, vbolt.Window{})
			for _, id := range ids {
				if seen[id] {
					continue
				}
				seen[id] = true
				if image, found := l.slimImage(id); found && inWindow(&image) && keep(image) {
					photos = append(photos, image)
				}
			}
		}
		sorted = false
	} else {
		for i, familyId := range l.ownList {
			texts := l.familyTexts(familyId)
			if i > 0 && len(texts.photos) > 0 && len(photos) > 0 {
				sorted = false
			}
			photos = slices.Grow(photos, len(texts.photos))
			for j := range texts.photos {
				image := &texts.photos[j]
				if inWindow(image) && keep(*image) {
					photos = append(photos, *image)
				}
			}
		}
		var linked []*photoStream
		for _, person := range linkedPeopleVisibleTo(l.tx, l.user, ScopePhotos) {
			linked = append(linked, newPhotoStream(ImageByPersonDateIndex, person.Id, l.newest))
		}
		mergePhotoStreams(l.tx, linked, l.newest, l.oldest, func(image Image) bool {
			if !l.own[image.FamilyId] && keep(image) {
				photos = append(photos, image)
				sorted = false
			}
			return true
		})
	}
	if !sorted {
		sort.SliceStable(photos, func(i, j int) bool { return photoKeyOf(photos[i]).newerThan(photoKeyOf(photos[j])) })
	}
	return photos, words
}

// cachedPlaces is the place of every located photo of the user's own
// families whose place name matches some term.
func (l photoListing) cachedPlaces(terms []string) map[int]PhotoPlace {
	out := map[int]PhotoPlace{}
	matches := map[string]bool{}
	for _, familyId := range l.ownList {
		for photoId, place := range l.familyTexts(familyId).places {
			match, seen := matches[place.Name]
			if !seen {
				match = matchesATerm(terms, place.Name)
				matches[place.Name] = match
			}
			if match {
				out[photoId] = place
			}
		}
	}
	return out
}

func reloadImages(tx *vbolt.Tx, slim []Image) []Image {
	out := make([]Image, 0, len(slim))
	for _, image := range slim {
		if full := GetImageById(tx, image.Id); full.Id != 0 {
			out = append(out, full)
		}
	}
	return out
}
