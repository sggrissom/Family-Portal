package backend

import (
	"family/cfg"
	"fmt"
	"math/rand"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type storeIndex struct {
	name  string
	pairs func(tx *vbolt.Tx) []string
}

func indexOf[K, T, P comparable](idx *vbolt.IndexInfo[K, T, P]) storeIndex {
	return storeIndex{name: idx.Name, pairs: func(tx *vbolt.Tx) (out []string) {
		vbolt.IterateAllTerms(tx, idx, func(term T, target K, priority P) bool {
			out = append(out, fmt.Sprintf("%v|%v|%v", term, target, priority))
			return true
		})
		sort.Strings(out)
		return
	}}
}

func imageStoreIndexes() []storeIndex {
	return []storeIndex{
		indexOf(ImageByFamilyIndex),
		indexOf(ImageByAnchorIndex),
		indexOf(ImageByFamilyDateIndex),
		indexOf(ImageByPersonDateIndex),
		indexOf(ImageApproxByFamilyIndex),
		indexOf(ImageApproxByPersonIndex),
		indexOf(PhotoWordIndex),
		indexOf(PhotoPersonByPhotoIndex),
		indexOf(PhotoPersonByPersonIndex),
		indexOf(PhotoPersonByFamilyIndex),
	}
}

func snapshotImageIndexes(tx *vbolt.Tx) map[string][]string {
	out := map[string][]string{}
	for _, idx := range imageStoreIndexes() {
		out[idx.name] = idx.pairs(tx)
	}
	return out
}

func wipeImageIndexes(t *testing.T, tx *vbolt.Tx) {
	t.Helper()
	for _, idx := range imageStoreIndexes() {
		bkt := vbolt.TxRawBucket(tx, idx.name)
		var keys [][]byte
		bkt.ForEach(func(k, _ []byte) error {
			keys = append(keys, slices.Clone(k))
			return nil
		})
		for _, k := range keys {
			if err := bkt.Delete(k); err != nil {
				t.Fatalf("wipe %s: %v", idx.name, err)
			}
		}
	}
}

type storeSim struct {
	t        *testing.T
	tx       *vbolt.Tx
	rng      *rand.Rand
	familyId int
	people   []Person
	images   []int
	log      []string
}

var storeSimWords = []string{"beach", "zoo", "picnic", "birthday", "snow", "garden", "grandma", "bikes"}

func (s *storeSim) words(n int) string {
	out := make([]string, 0, n)
	for range n {
		out = append(out, storeSimWords[s.rng.Intn(len(storeSimWords))])
	}
	return strings.Join(out, " ")
}

func (s *storeSim) person() Person {
	return s.people[s.rng.Intn(len(s.people))]
}

func (s *storeSim) image() Image {
	return GetImageById(s.tx, s.images[s.rng.Intn(len(s.images))])
}

func (s *storeSim) recordTime(image *Image) {
	switch s.rng.Intn(4) {
	case 0:
		image.RecordTime = RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisCalendar,
			Precision: PrecisionYear, From: fmt.Sprint(2018 + s.rng.Intn(7)), Source: SourceFamily}
	case 1:
		image.RecordTime = RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisAge,
			Precision: PrecisionYear, AnchorPersonId: s.person().Id, AgeYears: s.rng.Intn(4), Source: SourceFamily}
	default:
		day := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, s.rng.Intn(2500))
		setExactDate(&image.RecordTime, &image.PhotoDate, day, SourceFamily, 0)
	}
	if err := validateRecordTime(image.RecordTime, false); err != nil {
		s.t.Fatalf("simulated record time is invalid: %+v: %v", image.RecordTime, err)
	}
}

func (s *storeSim) step() {
	op := s.rng.Intn(100)
	if len(s.images) == 0 {
		op = 0
	}
	switch {
	case op < 30:
		image := Image{FamilyId: s.familyId, Title: s.words(2), Description: s.words(3), Status: PhotoProcessing}
		if s.rng.Intn(3) == 0 {
			image.MediaKind = MediaVideo
		}
		s.recordTime(&image)
		createImage(s.tx, &image)
		s.images = append(s.images, image.Id)
		s.log = append(s.log, fmt.Sprintf("create %d video=%v", image.Id, image.IsVideo()))
	case op < 50:
		image := s.image()
		switch s.rng.Intn(4) {
		case 0:
			image.Title = s.words(2)
		case 1:
			image.Description = s.words(4)
		case 2:
			s.recordTime(&image)
		default:
			image.Status = PhotoReady
		}
		saveImage(s.tx, &image)
		s.log = append(s.log, fmt.Sprintf("save %d", image.Id))
	case op < 65:
		image, person := s.image(), s.person()
		AddPersonToPhoto(s.tx, image.Id, person.Id, image.FamilyId)
		s.log = append(s.log, fmt.Sprintf("add %d to %d", person.Id, image.Id))
	case op < 73:
		image, person := s.image(), s.person()
		RemovePersonFromPhoto(s.tx, image.Id, person.Id)
		s.log = append(s.log, fmt.Sprintf("remove %d from %d", person.Id, image.Id))
	case op < 78:
		if pps := GetPhotoPersonsByPhoto(s.tx, s.image().Id); len(pps) > 0 {
			pp, to := pps[s.rng.Intn(len(pps))], s.person()
			movePhotoPerson(s.tx, pp, to.Id)
			s.log = append(s.log, fmt.Sprintf("move row %d to %d", pp.Id, to.Id))
		}
	case op < 81:
		if pps := GetPhotoPersonsByPhoto(s.tx, s.image().Id); len(pps) > 0 {
			pp := pps[s.rng.Intn(len(pps))]
			setPhotoPersonAutoTagged(s.tx, pp, !pp.AutoTagged)
			s.log = append(s.log, fmt.Sprintf("autotag row %d", pp.Id))
		}
	case op < 84:
		if pps := GetPhotoPersonsByPhoto(s.tx, s.image().Id); len(pps) > 0 {
			pp := pps[s.rng.Intn(len(pps))]
			deletePhotoPerson(s.tx, pp)
			s.log = append(s.log, fmt.Sprintf("delete row %d", pp.Id))
		}
	case op < 93:
		image := s.image()
		if !image.IsVideo() {
			return
		}
		status := TranscriptDone
		if s.rng.Intn(4) == 0 {
			status = TranscriptRemoved
		}
		writeVideoTranscriptTx(s.tx, &VideoTranscript{ImageId: image.Id, FamilyId: image.FamilyId, Status: status,
			Segments: []TranscriptSegment{{StartMs: 0, EndMs: 1000, Text: s.words(5)}}})
		s.log = append(s.log, fmt.Sprintf("transcript %d status=%d", image.Id, status))
	default:
		i := s.rng.Intn(len(s.images))
		deleteImage(s.tx, s.images[i])
		s.log = append(s.log, fmt.Sprintf("delete %d", s.images[i]))
		s.images = slices.Delete(s.images, i, i+1)
	}
}

func TestImageStoreIndexesMatchARebuild(t *testing.T) {
	for seed := int64(1); seed <= 25; seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			db := vbolt.Open(filepath.Join(t.TempDir(), "store.db"))
			defer db.Close()
			vbolt.InitBuckets(db, &cfg.Info)

			vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
				hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
				user := AddUserTx(tx, CreateAccountRequest{Name: "Me", Email: "store@example.com"}, hash)
				s := &storeSim{t: t, tx: tx, rng: rand.New(rand.NewSource(seed)), familyId: user.FamilyId}
				for _, p := range []AddPersonRequest{{Name: "Mia", Birthdate: "2019-03-04"}, {Name: "Ada", Birthdate: "2016-11-20"}, {Name: "Leo", Birthdate: "2021-06-01"}} {
					person, err := AddPersonTx(tx, p, user.FamilyId)
					if err != nil {
						t.Fatalf("AddPersonTx: %v", err)
					}
					s.people = append(s.people, person)
				}

				for range 80 {
					s.step()
				}

				kept := snapshotImageIndexes(tx)
				wipeImageIndexes(t, tx)
				RebuildImageIndexes(tx)
				rebuilt := snapshotImageIndexes(tx)

				for _, idx := range imageStoreIndexes() {
					if !slices.Equal(kept[idx.name], rebuilt[idx.name]) {
						t.Errorf("%s drifted from a rebuild\nkept:    %v\nrebuilt: %v", idx.name, kept[idx.name], rebuilt[idx.name])
					}
				}
				if t.Failed() {
					t.Logf("operations:\n%s", strings.Join(s.log, "\n"))
				}
			})
		})
	}
}
