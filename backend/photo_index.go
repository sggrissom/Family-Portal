package backend

import (
	"bytes"
	"family/cfg"
	"math"
	"slices"
	"sort"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Both indexes order a term's photos by photo date (whole seconds), then by id,
// so a page of photos is a vbolt.Window over the index rather than a sort of
// everything the family has.
var ImageByFamilyDateIndex = vbolt.IndexExt(&cfg.Info, "image_by_family_date", vpack.FInt, packDateKey, vpack.FInt)
var ImageByPersonDateIndex = vbolt.IndexExt(&cfg.Info, "image_by_person_date", vpack.FInt, packDateKey, vpack.FInt)

// The approximate indexes hold only photos whose date is not an exact day, so
// a date filter can find those that start before the window but overlap it.
var ImageApproxByFamilyIndex = vbolt.Index(&cfg.Info, "image_approx_by_family", vpack.FInt, vpack.FInt)
var ImageApproxByPersonIndex = vbolt.Index(&cfg.Info, "image_approx_by_person", vpack.FInt, vpack.FInt)

// PhotoWordIndex maps each word of a photo's title and description to the
// photo. Words are packed without a length prefix, so a cursor can seek to
// every word that starts with a search term.
var PhotoWordIndex = vbolt.Index(&cfg.Info, "photo_word", packIndexWord, vpack.FInt)

func packIndexWord(word *string, buf *vpack.Buffer) {
	if buf.Writing {
		buf.WriteBytes(append([]byte(*word), 0)...)
		return
	}
	end := bytes.IndexByte(buf.Data[buf.Pos:], 0)
	if end < 0 {
		buf.Error = true
		return
	}
	*word = string(buf.Data[buf.Pos : buf.Pos+end])
	buf.Pos += end + 1
}

func photosWithWordPrefix(tx *vbolt.Tx, prefix string, out map[int]bool) {
	seek := append([]byte{vbolt.IndexTermPrefix}, prefix...)
	cursor := vbolt.TxRawBucket(tx, PhotoWordIndex.Name).Cursor()
	for key, _ := cursor.Seek(seek); bytes.HasPrefix(key, seek); key, _ = cursor.Next() {
		var id int
		vpack.FromBytesInto(key[len(key)-8:], &id, vpack.FInt)
		out[id] = true
	}
}

// vpack.UnixTimeKey writes the seconds as an unsigned int, which puts every
// pre-1970 photo after the modern ones. Flipping the sign bit keeps the bytes
// in time order.
func packDateKey(seconds *int64, buf *vpack.Buffer) {
	u := uint64(*seconds) ^ (1 << 63)
	vpack.FUInt64(&u, buf)
	*seconds = int64(u ^ (1 << 63))
}

// ReindexPhotoDates brings the date and word indexes in line with the stored
// photo and its people. Call it after any write to the photo, its date, or who
// is in it.
func ReindexPhotoDates(tx *vbolt.Tx, photoId int) {
	image := GetImageById(tx, photoId)
	if image.Id == 0 {
		vbolt.DeleteTargetTerms(tx, ImageByFamilyDateIndex, photoId)
		vbolt.DeleteTargetTerms(tx, ImageByPersonDateIndex, photoId)
		vbolt.DeleteTargetTerms(tx, PhotoWordIndex, photoId)
		vbolt.DeleteTargetTerms(tx, ImageApproxByFamilyIndex, photoId)
		vbolt.DeleteTargetTerms(tx, ImageApproxByPersonIndex, photoId)
		return
	}
	words := searchFieldWords(image.Title, image.Description)
	if image.IsVideo() {
		words = append(words, transcriptWords(tx, image.Id)...)
	}
	vbolt.SetTargetTermsPlain(tx, PhotoWordIndex, image.Id, words)

	seconds := image.PhotoDate.Unix()
	vbolt.SetTargetSingleTermExt(tx, ImageByFamilyDateIndex, image.Id, seconds, image.FamilyId)

	photoPersons := GetPhotoPersonsByPhoto(tx, image.Id)
	personIds := make([]int, 0, len(photoPersons))
	for _, photoPerson := range photoPersons {
		personIds = append(personIds, photoPerson.PersonId)
	}
	vbolt.SetTargetTermsUniform(tx, ImageByPersonDateIndex, image.Id, personIds, seconds)

	if image.RecordTime.ExactDay() {
		vbolt.DeleteTargetTerms(tx, ImageApproxByFamilyIndex, image.Id)
		vbolt.DeleteTargetTerms(tx, ImageApproxByPersonIndex, image.Id)
	} else {
		vbolt.SetTargetSingleTerm(tx, ImageApproxByFamilyIndex, image.Id, image.FamilyId)
		vbolt.SetTargetTermsPlain(tx, ImageApproxByPersonIndex, image.Id, personIds)
	}
}

func approxPhotoIds(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], term int) (ids []int) {
	vbolt.ReadTermTargets(tx, index, term, &ids, vbolt.Window{})
	return
}

func BackfillPhotoDateIndexes(tx *vbolt.Tx) {
	var ids []int
	vbolt.IterateAll(tx, ImagesBkt, func(id int, _ Image) bool {
		ids = append(ids, id)
		return true
	})
	for _, id := range ids {
		ReindexPhotoDates(tx, id)
	}
}

// photoKey is a position in the newest-first order both indexes share.
type photoKey struct {
	seconds int64
	id      int
}

func photoKeyOf(image Image) photoKey {
	return photoKey{seconds: image.PhotoDate.Unix(), id: image.Id}
}

func (k photoKey) newerThan(other photoKey) bool {
	if k.seconds != other.seconds {
		return k.seconds > other.seconds
	}
	return k.id > other.id
}

// indexSeekKey builds the raw key vbolt stores for (term, priority, target),
// the same layout as vbolt's own term-target key, so a reverse Window can
// start from an arbitrary position rather than only from a key it handed out.
func indexSeekKey(term int, key photoKey) []byte {
	buf := vpack.NewWriter()
	buf.WriteBytes(vbolt.IndexTermPrefix)
	vpack.FInt(&term, buf)
	packDateKey(&key.seconds, buf)
	vpack.FInt(&key.id, buf)
	return buf.Data
}

// photoStream walks one index term newest first, a window at a time.
type photoStream struct {
	index  *vbolt.IndexInfo[int, int, int64]
	term   int
	cursor []byte
	queued []int
	head   Image
	done   bool
}

const photoStreamChunk = 64

func newPhotoStream(index *vbolt.IndexInfo[int, int, int64], term int, start photoKey) *photoStream {
	return &photoStream{index: index, term: term, cursor: indexSeekKey(term, start)}
}

// peek loads the stream's newest remaining photo into head.
func (s *photoStream) peek(tx *vbolt.Tx) bool {
	for s.head.Id == 0 {
		if len(s.queued) == 0 {
			if s.done {
				return false
			}
			window := vbolt.Window{Cursor: s.cursor, Limit: photoStreamChunk, Direction: vbolt.IterateReverse}
			s.cursor = vbolt.ReadTermTargets(tx, s.index, s.term, &s.queued, window)
			s.done = s.cursor == nil
			if len(s.queued) == 0 {
				s.done = true
				return false
			}
		}
		s.head = GetImageById(tx, s.queued[0])
		s.queued = s.queued[1:]
	}
	return true
}

func (s *photoStream) pop() Image {
	image := s.head
	s.head = Image{}
	return image
}

func (s *photoStream) headKey() photoKey {
	return photoKeyOf(s.head)
}

// imageStream is a newest-first source of photos with the key each sorts by.
type imageStream interface {
	peek(tx *vbolt.Tx) bool
	headKey() photoKey
	pop() Image
}

type keyedImage struct {
	image Image
	key   photoKey
}

// sliceStream serves photos already loaded and sorted newest first.
type sliceStream struct {
	items []keyedImage
}

func (s *sliceStream) peek(*vbolt.Tx) bool { return len(s.items) > 0 }
func (s *sliceStream) headKey() photoKey   { return s.items[0].key }
func (s *sliceStream) pop() Image {
	image := s.items[0].image
	s.items = s.items[1:]
	return image
}

// mergePhotoStreams yields photos from every stream newest first, each photo
// once, strictly older than start and no older than stop, until visit returns
// false.
func mergePhotoStreams(tx *vbolt.Tx, streams []imageStream, start photoKey, stop int64, visit func(Image, photoKey) bool) {
	seen := make(map[int]bool)
	for {
		var newest imageStream
		for _, stream := range streams {
			if !stream.peek(tx) {
				continue
			}
			if newest == nil || stream.headKey().newerThan(newest.headKey()) {
				newest = stream
			}
		}
		if newest == nil {
			return
		}

		key := newest.headKey()
		image := newest.pop()
		if key.seconds < stop {
			return
		}
		if seen[image.Id] || !start.newerThan(key) {
			continue
		}
		seen[image.Id] = true
		if !visit(image, key) {
			return
		}
	}
}

const maxPhotoPageSize = 200

// photoListing is what browsing and searching share: which streams to read,
// the date window, and which photos a filter lets through.
type photoListing struct {
	tx        *vbolt.Tx
	user      User
	personIds []int
	withTag   map[int]bool
	placeKey  string
	places    *placeResolver
	newest    photoKey
	oldest    int64
	exactOnly bool
	// windowStart and windowEnd are the date filter as civil days, half-open;
	// empty when there is no filter.
	windowStart string
	windowEnd   string
}

func newPhotoListing(tx *vbolt.Tx, user User, personIds []int, tagIds []int, placeKey string, dateFrom string, dateTo string) photoListing {
	l := photoListing{
		tx:        tx,
		user:      user,
		personIds: personIds,
		placeKey:  placeKey,
		places:    newPlaceResolver(tx),
		newest:    photoKey{seconds: math.MaxInt64, id: math.MaxInt},
		oldest:    math.MinInt64,
	}
	l.windowStart, l.windowEnd = "0001-01-01", "9999-12-31"
	if dateTo != "" {
		end, _ := time.Parse(dateOnlyLayout, dateTo)
		l.newest = photoKey{seconds: end.AddDate(0, 0, 1).Unix() - 1, id: math.MaxInt}
		l.windowEnd = dayOf(end.AddDate(0, 0, 1))
	}
	if dateFrom != "" {
		begin, _ := time.Parse(dateOnlyLayout, dateFrom)
		l.oldest = begin.Unix()
		l.windowStart = dateFrom
	}
	if len(tagIds) > 0 {
		l.withTag = make(map[int]bool)
		for _, tagId := range tagIds {
			for _, photoId := range GetTagPhotoIds(tx, tagId) {
				l.withTag[photoId] = true
			}
		}
	}
	return l
}

// listingTerm is one index term the listing reads photos from, with the
// index of that term's approximately dated photos.
type listingTerm struct {
	index  *vbolt.IndexInfo[int, int, int64]
	approx *vbolt.IndexInfo[int, int, uint16]
	term   int
}

func (l photoListing) terms() (terms []listingTerm) {
	if len(l.personIds) > 0 {
		for _, personId := range l.personIds {
			terms = append(terms, listingTerm{ImageByPersonDateIndex, ImageApproxByPersonIndex, personId})
		}
		return
	}
	for _, familyId := range familiesVisibleTo(l.tx, l.user) {
		terms = append(terms, listingTerm{ImageByFamilyDateIndex, ImageApproxByFamilyIndex, familyId})
	}
	for _, person := range linkedPeopleVisibleTo(l.tx, l.user, ScopePhotos) {
		terms = append(terms, listingTerm{ImageByPersonDateIndex, ImageApproxByPersonIndex, person.Id})
	}
	return
}

// overlapping lists approximately dated photos that start before the date
// window but overlap it. The date indexes place them by their start, so a
// window scan alone would miss them; they sort at the window's oldest edge.
func (l photoListing) overlapping() []keyedImage {
	if l.oldest == math.MinInt64 {
		return nil
	}
	seen := map[int]bool{}
	var items []keyedImage
	for _, t := range l.terms() {
		for _, id := range approxPhotoIds(l.tx, t.approx, t.term) {
			if seen[id] {
				continue
			}
			seen[id] = true
			image := GetImageById(l.tx, id)
			if image.Id == 0 || image.PhotoDate.Unix() >= l.oldest || !image.RecordTime.Overlaps(l.windowStart, l.windowEnd) {
				continue
			}
			items = append(items, keyedImage{image, photoKey{seconds: l.oldest, id: image.Id}})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key.newerThan(items[j].key) })
	return items
}

// read visits the listing's photos newest first from start, which is clamped
// to the date window, until visit returns false.
func (l photoListing) read(start photoKey, visit func(Image) bool) {
	l.readKeyed(start, func(image Image, _ photoKey) bool { return visit(image) })
}

func (l photoListing) readKeyed(start photoKey, visit func(Image, photoKey) bool) {
	if start.newerThan(l.newest) {
		start = l.newest
	}
	var streams []imageStream
	for _, t := range l.terms() {
		streams = append(streams, newPhotoStream(t.index, t.term, start))
	}
	if spill := l.overlapping(); len(spill) > 0 {
		streams = append(streams, &sliceStream{items: spill})
	}
	mergePhotoStreams(l.tx, streams, start, l.oldest, visit)
}

// keys lists the photos read would visit, newest first, without loading them.
func (l photoListing) keys() []photoKey {
	seen := map[int]bool{}
	var keys []photoKey
	for _, item := range l.overlapping() {
		seen[item.image.Id] = true
		keys = append(keys, item.key)
	}
	for _, t := range l.terms() {
		vbolt.IterateTerm(l.tx, t.index, t.term, func(id int, seconds int64) bool {
			key := photoKey{seconds: seconds, id: id}
			if !seen[id] && seconds >= l.oldest && l.newest.newerThan(key) {
				seen[id] = true
				keys = append(keys, key)
			}
			return true
		})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].newerThan(keys[j]) })
	return keys
}

// passes is the filter for a photo already read from the listing's streams.
// Person streams are not limited to the user's families, so they need the
// access check that family streams get from their index.
func (l photoListing) passes(image Image) bool {
	if image.Status == 2 || (l.withTag != nil && !l.withTag[image.Id]) {
		return false
	}
	if l.exactOnly && !image.RecordTime.ExactDay() {
		return false
	}
	if len(l.personIds) > 0 && !CanAccessPhoto(l.tx, l.user, image, AccessView) {
		return false
	}
	if l.placeKey != "" {
		if !canSeeLocation(l.tx, l.user, image) {
			return false
		}
		if place, ok := l.places.forPhoto(image); !ok || place.Key != l.placeKey {
			return false
		}
	}
	return true
}

func (l photoListing) inWindow(image Image) bool {
	if !image.RecordTime.ExactDay() && l.oldest != math.MinInt64 {
		return image.RecordTime.Overlaps(l.windowStart, l.windowEnd)
	}
	return image.PhotoDate.Unix() >= l.oldest && !photoKeyOf(image).newerThan(l.newest)
}

// shows reports whether a photo read from anywhere, such as a group's cover,
// would appear in this listing.
func (l photoListing) shows(image Image) bool {
	if !l.inWindow(image) || !l.passes(image) {
		return false
	}
	if len(l.personIds) == 0 {
		return true
	}
	for _, pp := range GetPhotoPersonsByPhoto(l.tx, image.Id) {
		if slices.Contains(l.personIds, pp.PersonId) {
			return true
		}
	}
	return false
}

// browsePhotos pages through the listing by date. With collapse, a photo
// whose similar-group cover is also listed is left out, and the cover carries
// the rest of its group.
func browsePhotos(l photoListing, start photoKey, limit int, collapse bool) (page []Image, similarTo map[int][]int, nextCursor string) {
	groups := map[int]photoGroup{}
	var last photoKey
	similarTo = map[int][]int{}
	l.readKeyed(start, func(image Image, key photoKey) bool {
		if !l.passes(image) {
			return true
		}
		if collapse {
			if features, ok := GetPhotoFeatures(l.tx, image.Id); ok {
				if group, grouped := groupOf(l.tx, features.GroupId, groups); grouped {
					if group.cover != image.Id {
						if l.shows(GetImageById(l.tx, group.cover)) {
							return true
						}
					} else {
						for _, id := range group.members {
							if id != image.Id {
								similarTo[image.Id] = append(similarTo[image.Id], id)
							}
						}
					}
				}
			}
		}
		if limit > 0 && len(page) == limit {
			nextCursor = last.String()
			return false
		}
		page = append(page, image)
		last = key
		return true
	})
	return
}
