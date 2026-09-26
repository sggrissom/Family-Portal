package backend

import (
	"family/cfg"

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
