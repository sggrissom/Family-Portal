package backend

import (
	"family/cfg"
	"math"
	"slices"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Both indexes order a term's photos by photo date (whole seconds), then by id,
// so a page of photos is a vbolt.Window over the index rather than a sort of
// everything the family has.
var ImageByFamilyDateIndex = vbolt.IndexExt(&cfg.Info, "image_by_family_date", vpack.FInt, packDateKey, vpack.FInt)
var ImageByPersonDateIndex = vbolt.IndexExt(&cfg.Info, "image_by_person_date", vpack.FInt, packDateKey, vpack.FInt)

// vpack.UnixTimeKey writes the seconds as an unsigned int, which puts every
// pre-1970 photo after the modern ones. Flipping the sign bit keeps the bytes
// in time order.
func packDateKey(seconds *int64, buf *vpack.Buffer) {
	u := uint64(*seconds) ^ (1 << 63)
	vpack.FUInt64(&u, buf)
	*seconds = int64(u ^ (1 << 63))
}

// ReindexPhotoDates brings both date indexes in line with the stored photo and
// its people. Call it after any write to the photo, its date, or who is in it.
func ReindexPhotoDates(tx *vbolt.Tx, photoId int) {
	image := GetImageById(tx, photoId)
	if image.Id == 0 {
		vbolt.DeleteTargetTerms(tx, ImageByFamilyDateIndex, photoId)
		vbolt.DeleteTargetTerms(tx, ImageByPersonDateIndex, photoId)
		return
	}

	seconds := image.PhotoDate.Unix()
	vbolt.SetTargetSingleTermExt(tx, ImageByFamilyDateIndex, image.Id, seconds, image.FamilyId)

	photoPersons := GetPhotoPersonsByPhoto(tx, image.Id)
	personIds := make([]int, 0, len(photoPersons))
	for _, photoPerson := range photoPersons {
		personIds = append(personIds, photoPerson.PersonId)
	}
	vbolt.SetTargetTermsUniform(tx, ImageByPersonDateIndex, image.Id, personIds, seconds)
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

// mergePhotoStreams yields photos from every stream newest first, each photo
// once, strictly older than start and no older than stop, until visit returns
// false.
func mergePhotoStreams(tx *vbolt.Tx, streams []*photoStream, start photoKey, stop int64, visit func(Image) bool) {
	seen := make(map[int]bool)
	for {
		var newest *photoStream
		for _, stream := range streams {
			if !stream.peek(tx) {
				continue
			}
			if newest == nil || photoKeyOf(stream.head).newerThan(photoKeyOf(newest.head)) {
				newest = stream
			}
		}
		if newest == nil {
			return
		}

		image := newest.pop()
		key := photoKeyOf(image)
		if key.seconds < stop {
			return
		}
		if seen[image.Id] || !start.newerThan(key) {
			continue
		}
		seen[image.Id] = true
		if !visit(image) {
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
	if dateTo != "" {
		end, _ := time.Parse(dateOnlyLayout, dateTo)
		l.newest = photoKey{seconds: end.AddDate(0, 0, 1).Unix() - 1, id: math.MaxInt}
	}
	if dateFrom != "" {
		begin, _ := time.Parse(dateOnlyLayout, dateFrom)
		l.oldest = begin.Unix()
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

// read visits the listing's photos newest first from start, which is clamped
// to the date window, until visit returns false.
func (l photoListing) read(start photoKey, visit func(Image) bool) {
	if start.newerThan(l.newest) {
		start = l.newest
	}
	var streams []*photoStream
	if len(l.personIds) > 0 {
		for _, personId := range l.personIds {
			streams = append(streams, newPhotoStream(ImageByPersonDateIndex, personId, start))
		}
	} else {
		for _, familyId := range familiesVisibleTo(l.tx, l.user) {
			streams = append(streams, newPhotoStream(ImageByFamilyDateIndex, familyId, start))
		}
		for _, person := range linkedPeopleVisibleTo(l.tx, l.user, ScopePhotos) {
			streams = append(streams, newPhotoStream(ImageByPersonDateIndex, person.Id, start))
		}
	}
	mergePhotoStreams(l.tx, streams, start, l.oldest, visit)
}

// passes is the filter for a photo already read from the listing's streams.
// Person streams are not limited to the user's families, so they need the
// access check that family streams get from their index.
func (l photoListing) passes(image Image) bool {
	if image.Status == 2 || (l.withTag != nil && !l.withTag[image.Id]) {
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

// shows reports whether a photo read from anywhere, such as a group's cover,
// would appear in this listing.
func (l photoListing) shows(image Image) bool {
	if image.PhotoDate.Unix() < l.oldest || photoKeyOf(image).newerThan(l.newest) || !l.passes(image) {
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
	similarTo = map[int][]int{}
	l.read(start, func(image Image) bool {
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
			nextCursor = photoKeyOf(page[limit-1]).String()
			return false
		}
		page = append(page, image)
		return true
	})
	return
}
