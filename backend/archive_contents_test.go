package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

// archiveRow is one stored row flattened to field paths, with every id
// replaced by a content key, so rows from two families can be compared.
type archiveRow map[string]string

func (r archiveRow) String() string {
	parts := make([]string, 0, len(r))
	for k, v := range r {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, " ") + "}"
}

type archiveContents map[string][]archiveRow

func familyRows[T any](tx *vbolt.Tx, bkt *vbolt.BucketInfo[int, T], idx *vbolt.IndexInfo[int, int, uint16], familyId int) []any {
	var ids []int
	vbolt.ReadTermTargets(tx, idx, familyId, &ids, vbolt.Window{})
	var rows []T
	vbolt.ReadSlice(tx, bkt, ids, &rows)
	out := make([]any, len(rows))
	for i := range rows {
		out[i] = rows[i]
	}
	return out
}

func asRows[T any](rows []T) []any {
	out := make([]any, len(rows))
	for i := range rows {
		out[i] = rows[i]
	}
	return out
}

// archiveReaders lists a family's rows in each archived bucket.
var archiveReaders = map[string]func(tx *vbolt.Tx, familyId int) []any{
	"families": func(tx *vbolt.Tx, familyId int) []any { return []any{GetFamily(tx, familyId)} },
	"people":   func(tx *vbolt.Tx, familyId int) []any { return asRows(GetFamilyOwnPeople(tx, familyId)) },
	"relation": func(tx *vbolt.Tx, familyId int) []any {
		return asRows(relationsAmong(tx, GetFamilyOwnPeople(tx, familyId)))
	},
	"interview_question_lists": func(tx *vbolt.Tx, familyId int) []any {
		var list InterviewQuestionList
		if !vbolt.Read(tx, InterviewQuestionListBkt, familyId, &list) {
			return nil
		}
		return []any{list}
	},
	"video_transcripts": func(tx *vbolt.Tx, familyId int) []any {
		var out []any
		for _, img := range GetFamilyImages(tx, familyId) {
			if t, ok := GetVideoTranscriptRecord(tx, img.Id); ok {
				out = append(out, t)
			}
		}
		return out
	},
	"growth_data":    func(tx *vbolt.Tx, f int) []any { return familyRows(tx, GrowthDataBkt, GrowthDataByFamilyIndex, f) },
	"tags":           func(tx *vbolt.Tx, f int) []any { return familyRows(tx, TagBkt, TagByFamilyIndex, f) },
	"milestones":     func(tx *vbolt.Tx, f int) []any { return familyRows(tx, MilestoneBkt, MilestoneByFamilyIndex, f) },
	"milestone_tags": func(tx *vbolt.Tx, f int) []any { return familyRows(tx, MilestoneTagBkt, MilestoneTagByFamilyIndex, f) },
	"milestone_photos": func(tx *vbolt.Tx, f int) []any {
		return familyRows(tx, MilestonePhotoBkt, MilestonePhotoByFamilyIndex, f)
	},
	"images":           func(tx *vbolt.Tx, f int) []any { return familyRows(tx, ImagesBkt, ImageByFamilyIndex, f) },
	"photo_person":     func(tx *vbolt.Tx, f int) []any { return familyRows(tx, PhotoPersonBkt, PhotoPersonByFamilyIndex, f) },
	"photo_tags":       func(tx *vbolt.Tx, f int) []any { return familyRows(tx, PhotoTagBkt, PhotoTagByFamilyIndex, f) },
	"photo_faces":      func(tx *vbolt.Tx, f int) []any { return familyRows(tx, PhotoFaceBkt, FaceByFamilyIndex, f) },
	"family_places":    func(tx *vbolt.Tx, f int) []any { return familyRows(tx, FamilyPlaceBkt, FamilyPlaceByFamilyIndex, f) },
	"activities":       func(tx *vbolt.Tx, f int) []any { return familyRows(tx, ActivityBkt, ActivityByFamilyIndex, f) },
	"seasons":          func(tx *vbolt.Tx, f int) []any { return familyRows(tx, SeasonBkt, SeasonByFamilyIndex, f) },
	"activity_events":  func(tx *vbolt.Tx, f int) []any { return familyRows(tx, EventBkt, EventByFamilyIndex, f) },
	"activity_entries": func(tx *vbolt.Tx, f int) []any { return familyRows(tx, EntryBkt, EntryByFamilyIndex, f) },
	"entry_members":    func(tx *vbolt.Tx, f int) []any { return familyRows(tx, EntryMemberBkt, EntryMemberByFamilyIndex, f) },
	"appearances":      func(tx *vbolt.Tx, f int) []any { return familyRows(tx, AppearanceBkt, AppearanceByFamilyIndex, f) },
	"appearance_photos": func(tx *vbolt.Tx, f int) []any {
		return familyRows(tx, AppearancePhotoBkt, AppearancePhotoByFamilyIndex, f)
	},
	"activity_results":      func(tx *vbolt.Tx, f int) []any { return familyRows(tx, ResultBkt, ResultByFamilyIndex, f) },
	"activity_event_photos": func(tx *vbolt.Tx, f int) []any { return familyRows(tx, EventPhotoBkt, EventPhotoByFamilyIndex, f) },
	"interviews":            func(tx *vbolt.Tx, f int) []any { return familyRows(tx, InterviewBkt, InterviewByFamilyIndex, f) },
	"interview_questions": func(tx *vbolt.Tx, f int) []any {
		return familyRows(tx, InterviewQuestionBkt, InterviewQuestionByFamilyIndex, f)
	},
	"books": func(tx *vbolt.Tx, f int) []any { return familyRows(tx, BookBkt, BookByFamilyIndex, f) },
}

// archiveRefs names the bucket an id field points into. An id field that is
// neither here nor in archiveIgnored fails the comparison.
var archiveRefs = map[string]string{
	"PersonId": "people", "PersonIds": "people", "FromId": "people", "ToId": "people",
	"AnchorPersonId": "people", "RejectedPersonIds": "people",
	"PhotoId": "images", "PhotoIds": "images", "ImageId": "images",
	"ProfilePhotoId": "images", "CoverPhotoId": "images",
	"TagId": "tags", "TagIds": "tags",
	"MilestoneId": "milestones",
	"InterviewId": "interviews",
	"QuestionId":  "interview_questions", "QuestionIds": "interview_questions", "SkippedIds": "interview_questions",
	"ActivityId":   "activities",
	"SeasonId":     "seasons",
	"EventId":      "activity_events",
	"EntryId":      "activity_entries",
	"AppearanceId": "appearances",
}

// archiveIgnored are fields that are not family data: row identity, user
// accounts, storage locations (compared by content instead) and values the
// workers derive again after an import.
var archiveIgnored = map[string]bool{
	"Id": true, "FamilyId": true,
	"OwnerUserId": true, "CreatedBy": true, "EditedBy": true, "ChangedBy": true,
	"families.InviteCode": true, "families.Creation": true,
	"images.FilePath": true, "images.OriginalKey": true, "images.PlaybackKey": true,
	"images.Status": true, "images.AnalysisStatus": true, "images.Width": true, "images.Height": true,
	"images.DerivedBytes": true, "images.ContentHash": true, "images.ProcessingError": true, "images.VideoCodec": true, "images.AudioCodec": true,
	"people.Age": true, "people.FaceDescriptor": true, "photo_faces.Descriptor": true,
}

var bookItemBuckets = map[BookItemKind]string{
	BookItemMilestone: "milestones", BookItemPhoto: "images", BookItemInterview: "interviews",
}

type contentView struct {
	tx   *vbolt.Tx
	rows map[string][]any
	keys map[string]map[int]string
}

func readArchiveContents(t *testing.T, db *vbolt.DB, familyId int) (contents archiveContents) {
	t.Helper()
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		v := &contentView{tx: tx, rows: map[string][]any{}, keys: map[string]map[int]string{}}
		for bucket, read := range archiveReaders {
			v.rows[bucket] = read(tx, familyId)
		}
		v.indexKeys(t)
		contents = archiveContents{}
		for bucket, rows := range v.rows {
			for _, row := range rows {
				flat := archiveRow{}
				v.flatten(t, bucket, "", reflect.ValueOf(row), flat)
				if bucket == "images" {
					flat["Media"] = v.keys["images"][row.(Image).Id]
				}
				contents[bucket] = append(contents[bucket], flat)
			}
		}
	})
	return
}

// indexKeys gives every referenced row a key made of what identifies it to
// a person, in dependency order so a key can use its parents' keys.
func (v *contentView) indexKeys(t *testing.T) {
	key := func(bucket string, fn func(row any) (int, string)) {
		v.keys[bucket] = map[int]string{}
		for _, row := range v.rows[bucket] {
			id, k := fn(row)
			for other, seen := range v.keys[bucket] {
				// Photos with the same bytes are the same media; re-import duplicates them.
				if seen == k && bucket != "images" {
					t.Fatalf("%s rows %d and %d share the content key %q; make the seed tell them apart", bucket, other, id, k)
				}
			}
			v.keys[bucket][id] = k
		}
	}
	day := func(at time.Time) string { return at.UTC().Format("2006-01-02") }
	key("people", func(r any) (int, string) { p := r.(Person); return p.Id, p.Name + "@" + day(p.Birthday) })
	key("images", func(r any) (int, string) { img := r.(Image); return img.Id, v.mediaKey(t, img) })
	key("tags", func(r any) (int, string) { tag := r.(Tag); return tag.Id, strings.ToLower(tag.Name) })
	key("interview_questions", func(r any) (int, string) { q := r.(InterviewQuestion); return q.Id, q.Text })
	key("milestones", func(r any) (int, string) {
		m := r.(Milestone)
		return m.Id, v.keys["people"][m.PersonId] + "/" + m.RecordTime.Start + "/" + m.Description
	})
	key("interviews", func(r any) (int, string) {
		in := r.(Interview)
		return in.Id, v.keys["people"][in.PersonId] + "/" + day(in.Date) + "/" + in.Title
	})
	key("activities", func(r any) (int, string) { a := r.(Activity); return a.Id, a.Name })
	key("seasons", func(r any) (int, string) {
		s := r.(Season)
		return s.Id, v.keys["activities"][s.ActivityId] + "/" + s.Name
	})
	key("activity_events", func(r any) (int, string) { e := r.(Event); return e.Id, v.keys["seasons"][e.SeasonId] + "/" + e.Name })
	key("activity_entries", func(r any) (int, string) { e := r.(Entry); return e.Id, v.keys["seasons"][e.SeasonId] + "/" + e.Name })
	key("appearances", func(r any) (int, string) {
		a := r.(Appearance)
		return a.Id, v.keys["activity_events"][a.EventId] + "/" + v.keys["activity_entries"][a.EntryId] + "/" + a.OccurredAt.UTC().Format(time.RFC3339)
	})
}

func (v *contentView) mediaKey(t *testing.T, img Image) string {
	var r io.ReadCloser
	var err error
	kind := "photo"
	if img.IsVideo() {
		kind = "video"
		r, err = mediaStore.Open(context.Background(), img.OriginalKey)
	} else {
		r, err = os.Open(photoOriginalPath(img.FilePath))
	}
	if err != nil {
		t.Fatalf("%s %q has no readable original: %v", kind, img.Title, err)
	}
	defer r.Close()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		t.Fatal(err)
	}
	return kind + ":" + hex.EncodeToString(h.Sum(nil))[:16]
}

func (v *contentView) ref(t *testing.T, bucket, field string, id int) string {
	keys, ok := v.keys[bucket]
	if !ok {
		t.Fatalf("%s points into %s, which has no content key", field, bucket)
	}
	k, ok := keys[id]
	if !ok {
		t.Fatalf("%s %d is not a row of this family's %s", field, id, bucket)
	}
	return k
}

func (v *contentView) flatten(t *testing.T, bucket, path string, val reflect.Value, out archiveRow) {
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	if at, ok := val.Interface().(time.Time); ok {
		if !at.IsZero() {
			out[path] = at.UTC().Format(time.RFC3339Nano)
		}
		return
	}
	switch val.Kind() {
	case reflect.Struct:
		typ := val.Type()
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() || archiveIgnored[f.Name] || archiveIgnored[bucket+"."+f.Name] {
				continue
			}
			sub := f.Name
			if path != "" {
				sub = path + "." + f.Name
			}
			target, isRef := archiveRefs[f.Name]
			if item, ok := val.Interface().(BookItem); ok && f.Name == "SourceId" {
				target, isRef = bookItemBuckets[item.Kind]
				if !isRef {
					t.Fatalf("%s.%s: book item kind %d has no bucket in bookItemBuckets", bucket, sub, item.Kind)
				}
			}
			if isRef {
				v.flattenRef(t, sub, target, f.Name, val.Field(i), out)
				continue
			}
			if strings.HasSuffix(f.Name, "Id") || strings.HasSuffix(f.Name, "Ids") {
				t.Fatalf("%s.%s looks like an id; add it to archiveRefs or archiveIgnored", bucket, sub)
			}
			v.flatten(t, bucket, sub, val.Field(i), out)
		}
	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			v.flatten(t, bucket, fmt.Sprintf("%s.%d", path, i), val.Index(i), out)
		}
	case reflect.Map, reflect.Interface:
		t.Fatalf("%s.%s is a %s, which the archive comparison does not flatten yet", bucket, path, val.Kind())
	default:
		if !val.IsZero() {
			out[path] = fmt.Sprint(val.Interface())
		}
	}
}

func (v *contentView) flattenRef(t *testing.T, path, bucket, field string, val reflect.Value, out archiveRow) {
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	var keys []string
	if val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			keys = append(keys, v.ref(t, bucket, field, int(val.Index(i).Int())))
		}
		sort.Strings(keys)
	} else if id := int(val.Int()); id != 0 {
		keys = []string{v.ref(t, bucket, field, id)}
	}
	if len(keys) > 0 {
		out[path] = strings.Join(keys, ",")
	}
}

// archiveDrop is a known loss, masked on both sides before comparing. An
// empty Field drops the matching rows; otherwise that field and its
// sub-fields are removed. Bucket "*" applies to every bucket.
type archiveDrop struct {
	Bucket string
	Field  string
	Where  func(archiveRow) bool
	Reason string
}

func (c archiveContents) without(drops []archiveDrop) archiveContents {
	out := archiveContents{}
	for bucket, rows := range c {
		for _, row := range rows {
			keep := archiveRow{}
			for k, val := range row {
				keep[k] = val
			}
			dropped := false
			for _, d := range drops {
				if (d.Bucket != "*" && d.Bucket != bucket) || (d.Where != nil && !d.Where(row)) {
					continue
				}
				if d.Field == "" {
					dropped = true
					break
				}
				for k := range keep {
					if k == d.Field || strings.HasPrefix(k, d.Field+".") {
						delete(keep, k)
					}
				}
			}
			if !dropped {
				out[bucket] = append(out[bucket], keep)
			}
		}
	}
	return out
}

// diffArchiveContents lists rows one side has and the other lacks.
func diffArchiveContents(want, got archiveContents) []string {
	buckets := map[string]bool{}
	for b := range want {
		buckets[b] = true
	}
	for b := range got {
		buckets[b] = true
	}
	var diffs []string
	for b := range buckets {
		counts := map[string]int{}
		for _, row := range want[b] {
			counts[row.String()]++
		}
		for _, row := range got[b] {
			counts[row.String()]--
		}
		for row, n := range counts {
			for ; n > 0; n-- {
				diffs = append(diffs, b+": lost "+row)
			}
			for ; n < 0; n++ {
				diffs = append(diffs, b+": gained "+row)
			}
		}
	}
	sort.Strings(diffs)
	return diffs
}

// assertArchiveMatches fails on any difference the drops don't explain, and
// on any drop that no longer explains a difference.
func assertArchiveMatches(t *testing.T, want, got archiveContents, drops []archiveDrop) {
	t.Helper()
	for _, d := range diffArchiveContents(want.without(drops), got.without(drops)) {
		t.Errorf("unexpected difference: %s", d)
	}
	for i, d := range drops {
		others := append(append([]archiveDrop{}, drops[:i]...), drops[i+1:]...)
		if len(diffArchiveContents(want.without(others), got.without(others))) == 0 {
			t.Errorf("known drop %s %s (%s) no longer happens; remove it from the list", d.Bucket, d.Field, d.Reason)
		}
	}
}
