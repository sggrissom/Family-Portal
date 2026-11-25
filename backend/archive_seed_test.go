package backend

import (
	"bytes"
	"context"
	"family/cfg"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type archiveFixture struct {
	db       *vbolt.DB
	store    *localMediaStore
	owner    User
	familyId int
}

var archiveSeedTime = time.Date(2025, 5, 4, 9, 30, 0, 0, time.UTC)

func setupArchiveFixture(t *testing.T) *archiveFixture {
	t.Helper()
	db := vbolt.Open(filepath.Join(t.TempDir(), "archive.db"))
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })

	store, err := newLocalMediaStore(filepath.Join(t.TempDir(), "media"), []byte("archive-test-media-key"))
	if err != nil {
		t.Fatal(err)
	}

	previousDb, previousStore, previousFree, previousWorker := appDb, mediaStore, freeDiskBytes, globalPhotoWorker
	appDb, mediaStore = db, store
	freeDiskBytes = func(string) (int64, error) { return 1 << 50, nil }
	globalPhotoWorker = &PhotoWorker{db: db, jobQueue: make(chan PhotoProcessingJob, 100)}
	jwtKey = []byte("family-archive-test-secret-key-32b")
	t.Cleanup(func() {
		appDb, mediaStore, freeDiskBytes, globalPhotoWorker = previousDb, previousStore, previousFree, previousWorker
	})

	fx := &archiveFixture{db: db, store: store}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		fx.owner = AddUserTx(tx, CreateAccountRequest{Name: "Owner", Email: "owner@archive.test"}, nil)
		fx.familyId = fx.owner.FamilyId
		seedArchiveFamily(t, tx, fx)
		vbolt.TxCommit(tx)
	})
	return fx
}

func (fx *archiveFixture) newUser(t *testing.T, name string) User {
	t.Helper()
	var user User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		user = AddUserTx(tx, CreateAccountRequest{Name: name, Email: name + "@archive.test"}, nil)
		vbolt.TxCommit(tx)
	})
	return user
}

func (fx *archiveFixture) call(t *testing.T, user User, fn func(ctx *vbeam.Context)) {
	t.Helper()
	token := testAuthToken(t, user)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fn(&vbeam.Context{Tx: tx, Token: token})
		vbolt.TxCommit(tx)
	})
}

// seedArchiveFamily puts at least one row in every archived bucket, using the
// fields a person can set, so a round trip has something to lose.
func seedArchiveFamily(t *testing.T, tx *vbolt.Tx, fx *archiveFixture) {
	t.Helper()
	s := &seeder{tx: tx, now: archiveSeedTime, rng: rand.New(rand.NewSource(321))}
	familyId := fx.familyId
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	family := GetFamily(tx, familyId)
	family.Name = "The Archivists"
	family.VideoEnabled = true
	vbolt.Write(tx, FamiliesBkt, family.Id, &family)

	birthTime, birthPlace := "07:15", "St. Elsewhere"
	ada, err := AddPersonTx(tx, AddPersonRequest{
		Name: "Ada", Gender: int(Female), Birthdate: "2020-03-04",
		BirthDetails: BirthDetails{BirthTime: &birthTime, BirthPlace: &birthPlace},
	}, familyId)
	must(err)
	ben := s.person(familyId, "Ben", Male, "2018-06-01")
	s.pregnancy(familyId, "Bump", "2025-12-25")
	s.relate(ben, ada, RelationSibling)

	beach := s.tag(familyId, "beach", "#3366ff")
	s.measurement(ada, time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC), "height", 95.5, "cm")
	s.measurement(ada, time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC), "weight", 31.2, "lb")

	sandcastle := fx.seedPhoto(t, tx, s, "IMG_0001.png", 40, 30, "Sandcastle", []Person{ada}, beach)
	portrait := fx.seedPhoto(t, tx, s, "IMG_0002.png", 30, 40, "Portrait", []Person{ben})
	s.profile(ada, portrait, 40, 60, 1.5)

	for _, pp := range GetPhotoPersonsByPhoto(tx, portrait.Id) {
		pp.AutoTagged = true
		vbolt.Write(tx, PhotoPersonBkt, pp.Id, &pp)
	}
	sandcastle.AnalysisVersion = currentAnalysisVersion
	writeImage(tx, &sandcastle)
	face := PhotoFace{
		Id: vbolt.NextIntId(tx, PhotoFaceBkt), PhotoId: sandcastle.Id, FamilyId: familyId,
		PersonId: ada.Id, Status: FaceConfirmed, Box: FaceBox{Left: 0.1, Top: 0.2, Right: 0.4, Bottom: 0.6},
		RejectedPersonIds: []int{ben.Id}, CreatedAt: archiveSeedTime,
	}
	writeFace(tx, &face)

	place := FamilyPlace{
		Id: vbolt.NextIntId(tx, FamilyPlaceBkt), FamilyId: familyId, Name: "Grandma's",
		Latitude: 51.5, Longitude: -0.12, RadiusMeters: 250, CreatedAt: archiveSeedTime,
	}
	vbolt.Write(tx, FamilyPlaceBkt, place.Id, &place)
	vbolt.SetTargetSingleTerm(tx, FamilyPlaceByFamilyIndex, place.Id, familyId)

	video := fx.seedVideo(t, tx, "IMG_0003.MOV", []Person{ada}, beach)
	writeVideoTranscriptTx(tx, &VideoTranscript{
		ImageId: video.Id, FamilyId: familyId, Model: "base.en", Language: "en", Status: TranscriptDone,
		Segments: []TranscriptSegment{{StartMs: 0, EndMs: 1500, Text: "Look at the waves", Confidence: 0.9}},
		EditedBy: fx.owner.Id, EditedAt: archiveSeedTime, CreatedAt: archiveSeedTime,
	})

	rt := exactRecordTime(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), SourceFamily)
	firstSwim, err := AddMilestoneTx(tx, AddMilestoneRequest{
		PersonId: ada.Id, Description: "First swim", Category: "development", Context: "at the lake",
		RecordTime: &rt, TagIds: []int{beach.Id},
	}, familyId)
	must(err)
	firstSwim.Note = "She went in up to her knees"
	vbolt.Write(tx, MilestoneBkt, firstSwim.Id, &firstSwim)
	must(setMilestonePhotos(tx, firstSwim, []int{sandcastle.Id}))

	swim := s.activity(familyId, "Swim club", ActivityKindSport)
	season := s.season(swim, "Summer 2024", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 8, 31, 0, 0, 0, 0, time.UTC), "Saturday mornings")
	meet := s.event(season, "Lake meet", "Swim club", "The lake", time.Date(2024, 7, 13, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 13, 0, 0, 0, 0, time.UTC))
	relay := s.entry(season, "25m freestyle", "solo", "Freestyle", "Under 6", "Beginner", ada)
	heat := s.appearance(meet, relay, time.Date(2024, 7, 13, 10, 0, 0, 0, time.UTC), "Swam the whole way")
	s.result(heat, Result{Kind: "placement", Label: "2nd", Rank: seedInt(2), OutOf: seedInt(6), PersonId: &ada.Id})
	s.appearancePhotos(heat, sandcastle)
	must(setEventPhotosTx(tx, meet, []int{portrait.Id}))

	s.interviews(ada)
	list := familyQuestionList(tx, familyId)
	list.Cadence = CadenceSchoolStart
	list.CadenceMonth = 9
	list.DismissedKeys = append(list.DismissedKeys, "best-age")
	writeQuestionList(tx, list)
	position := 0
	for _, q := range familyQuestions(tx, familyId) {
		if q.DefaultKey == "best-age" {
			deleteInterviewQuestion(tx, q.Id)
		} else if !q.OneOff {
			q.Position = position
			position++
			writeInterviewQuestion(tx, q)
		}
	}
	interviews := personInterviews(tx, ada.Id)
	if s.err != nil || len(interviews) == 0 {
		t.Fatalf("seed: %v, %d interviews", s.err, len(interviews))
	}

	writeBookTx(tx, Book{
		Id: vbolt.NextIntId(tx, BookBkt), FamilyId: familyId, PersonId: ada.Id, PersonIds: []int{ada.Id},
		Preset: BookPresetCustom, Title: "Ada's summer", StartDate: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		EndDate: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC), CoverPhotoId: sandcastle.Id, Density: "detailed",
		Categories: []string{"milestones", "photos"}, Match: "any", Introduction: "A summer by the water",
		Items: []BookItem{
			{Kind: BookItemMilestone, SourceId: firstSwim.Id, PhotoId: sandcastle.Id, Caption: "In she goes", Pinned: true},
			{Kind: BookItemPhoto, SourceId: sandcastle.Id},
			{Kind: BookItemInterview, SourceId: interviews[0].Id},
		},
		Excluded: []BookItem{{Kind: BookItemPhoto, SourceId: portrait.Id}},
		Revision: 1, CreatedBy: fx.owner.Id, CreatedAt: archiveSeedTime, UpdatedAt: archiveSeedTime,
	})
	if s.err != nil {
		t.Fatal(s.err)
	}
}

func (fx *archiveFixture) seedPhoto(t *testing.T, tx *vbolt.Tx, s *seeder, filename string, w, h int, title string, people []Person, tags ...Tag) Image {
	t.Helper()
	data := createTestImage(w, h)
	unique, err := generateUniqueFilename(filename)
	if err != nil {
		t.Fatal(err)
	}
	photo := Image{
		Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: fx.familyId, OwnerUserId: fx.owner.Id,
		OriginalFilename: filename, MimeType: "image/png", FileSize: len(data), Width: w, Height: h,
		FilePath: "photos/" + unique, Title: title, Description: title + " by the sea",
		RecordTime: exactRecordTime(time.Date(2024, 7, 2, 15, 0, 0, 0, time.UTC), SourceMetadata),
		CreatedAt:  archiveSeedTime, Status: PhotoReady,
	}
	photo.PhotoDate = time.Date(2024, 7, 2, 15, 0, 0, 0, time.UTC)
	if err := writePhotoOriginal(photo.FilePath, data); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deletePhotoFiles(photo) })
	writeImage(tx, &photo)
	vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, photo.Id, fx.familyId)
	ReindexPhotoDates(tx, photo.Id)
	for _, person := range people {
		AddPersonToPhoto(tx, photo.Id, person.Id, fx.familyId)
	}
	for _, tag := range tags {
		addTagToPhoto(tx, photo.Id, tag.Id, fx.familyId)
	}
	return photo
}

func (fx *archiveFixture) seedVideo(t *testing.T, tx *vbolt.Tx, filename string, people []Person, tags ...Tag) Image {
	t.Helper()
	data := []byte("not really a quicktime movie, but its bytes are its own")
	video := Image{
		Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: fx.familyId, OwnerUserId: fx.owner.Id,
		OriginalFilename: filename, MimeType: "video/quicktime", FileSize: len(data), MediaKind: MediaVideo,
		Title: "Waves", DurationMs: 1500, Status: PhotoReady, CreatedAt: archiveSeedTime,
		OriginalKey: "videos/archivetest/original.mov",
		RecordTime:  exactRecordTime(time.Date(2024, 7, 2, 16, 0, 0, 0, time.UTC), SourceMetadata),
		PhotoDate:   time.Date(2024, 7, 2, 16, 0, 0, 0, time.UTC),
	}
	if err := fx.store.Put(context.Background(), video.OriginalKey, bytes.NewReader(data), int64(len(data)), video.MimeType); err != nil {
		t.Fatal(err)
	}
	writeImage(tx, &video)
	vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, video.Id, fx.familyId)
	ReindexPhotoDates(tx, video.Id)
	for _, person := range people {
		AddPersonToPhoto(tx, video.Id, person.Id, fx.familyId)
	}
	for _, tag := range tags {
		addTagToPhoto(tx, video.Id, tag.Id, fx.familyId)
	}
	return video
}
