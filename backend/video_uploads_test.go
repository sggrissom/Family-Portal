package backend

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type videoFixture struct {
	resultsFixture
	store *localMediaStore
}

func setupVideoFixture(t *testing.T) videoFixture {
	t.Helper()
	fx := videoFixture{resultsFixture: setupResultsFixture(t)}
	store, err := newLocalMediaStore(filepath.Join(t.TempDir(), "media"), []byte("video-upload-test-key"))
	if err != nil {
		t.Fatal(err)
	}
	store.gate = videoUploadGate(fx.db)
	fx.store = store

	previousStore, previousFree := mediaStore, freeDiskBytes
	mediaStore = store
	freeDiskBytes = func(string) (int64, error) { return 1 << 50, nil }
	t.Cleanup(func() { mediaStore, freeDiskBytes = previousStore, previousFree })

	fx.setVideoEnabled(t, fx.familyId, true)
	return fx
}

func (fx videoFixture) setVideoEnabled(t *testing.T, familyId int, enabled bool) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		family := GetFamily(tx, familyId)
		family.VideoEnabled = enabled
		vbolt.Write(tx, FamiliesBkt, family.Id, &family)
		vbolt.TxCommit(tx)
	})
}

func (fx videoFixture) begin(t *testing.T, size int) (BeginVideoUploadResponse, error) {
	t.Helper()
	return callAs(t, fx.resultsFixture, BeginVideoUpload, BeginVideoUploadRequest{
		PersonIds: []int{fx.alice.Id}, Filename: "IMG_0001.MOV", ContentType: "video/quicktime",
		Size: size, FallbackDate: "2026-06-01",
	})
}

func (fx videoFixture) put(t *testing.T, inst UploadInstructions, body []byte) int {
	t.Helper()
	return sendUpload(fx.store, inst, body).Code
}

func (fx videoFixture) usage(t *testing.T) (used int64) {
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { used = FamilyStorageUsage(tx, fx.familyId) })
	return
}

func TestVideoUploadLifecycle(t *testing.T) {
	fx := setupVideoFixture(t)
	body := bytes.Repeat([]byte("v"), 4096)

	begun, err := fx.begin(t, len(body))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fx.usage(t), int64(videoReservationBytes(len(body))); got != want {
		t.Fatalf("usage while uploading = %d, want reservation %d", got, want)
	}

	if _, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: begun.UploadId}); !errors.Is(err, ErrVideoUploadMissing) {
		t.Fatalf("completing before the upload arrived: %v", err)
	}

	if code := fx.put(t, begun.Upload, body); code != http.StatusOK {
		t.Fatalf("PUT status %d", code)
	}

	done, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: begun.UploadId})
	if err != nil {
		t.Fatal(err)
	}
	image := done.Image
	if !image.IsVideo() || image.Status != 1 || image.FileSize != len(body) || image.MimeType != "video/quicktime" {
		t.Fatalf("created row %+v", image)
	}
	if image.Title != "Video from Jun 1, 2026" || image.PhotoDate.Format(dateOnlyLayout) != "2026-06-01" {
		t.Fatalf("title %q date %v", image.Title, image.PhotoDate)
	}

	again, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: begun.UploadId})
	if err != nil || again.Image.Id != image.Id {
		t.Fatalf("repeated completion = %d, %v; want %d", again.Image.Id, err, image.Id)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if n := len(GetFamilyImages(tx, fx.familyId)); n != 1 {
			t.Fatalf("%d rows after repeated completion", n)
		}
		people := GetPhotoPeople(tx, image.Id)
		if len(people) != 1 || people[0].Id != fx.alice.Id {
			t.Fatalf("tagged people = %+v", people)
		}
	})

	stored := imageForTest(t, fx.db, image.Id)
	if got, want := fx.usage(t), imageStoredBytes(stored); got != want || got <= int64(len(body)) {
		t.Fatalf("usage after completion = %d, want the row's own %d", got, want)
	}

	if code := fx.put(t, begun.Upload, body); code != http.StatusConflict {
		t.Fatalf("re-upload after completion: status %d", code)
	}
}

func imageForTest(t *testing.T, db *vbolt.DB, id int) (image Image) {
	t.Helper()
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) { image = GetImageById(tx, id) })
	return
}

func TestBeginVideoUploadRefusals(t *testing.T) {
	fx := setupVideoFixture(t)

	if _, err := callAs(t, fx.resultsFixture, BeginVideoUpload, BeginVideoUploadRequest{
		Filename: "notes.txt", ContentType: "text/plain", Size: 10,
	}); err == nil {
		t.Error("accepted a text file")
	}
	if _, err := fx.begin(t, maxVideoBytes+1); err == nil {
		t.Error("accepted a file over the size cap")
	}
	if _, err := callAs(t, fx.resultsFixture, BeginVideoUpload, BeginVideoUploadRequest{
		PersonIds: []int{fx.outsiderPerson(t).Id}, Filename: "a.mp4", ContentType: "video/mp4", Size: 10,
	}); err == nil {
		t.Error("accepted a person from another family")
	}

	fx.setVideoEnabled(t, fx.familyId, false)
	if _, err := fx.begin(t, 10); !errors.Is(err, ErrVideoDisabled) {
		t.Errorf("family without video: %v", err)
	}
	support, _ := callAs(t, fx.resultsFixture, GetVideoSupport, GetVideoSupportRequest{})
	if support.Enabled || support.Reason == "" {
		t.Errorf("support while disabled = %+v", support)
	}

	fx.setVideoEnabled(t, fx.familyId, true)
	support, _ = callAs(t, fx.resultsFixture, GetVideoSupport, GetVideoSupportRequest{})
	if !support.Enabled || support.MaxBytes != maxVideoBytes {
		t.Errorf("support while enabled = %+v", support)
	}

	mediaStore = nil
	if _, err := fx.begin(t, 10); !errors.Is(err, ErrVideoUnavailable) {
		t.Errorf("without a store: %v", err)
	}
}

func TestConcurrentVideoUploadsCannotShareQuota(t *testing.T) {
	fx := setupVideoFixture(t)
	admitted := 0
	for range 6 {
		if _, err := fx.begin(t, maxVideoBytes); err == nil {
			admitted++
		}
	}
	if admitted != 4 {
		t.Fatalf("admitted %d one-gigabyte uploads against a 10 GB quota, want 4", admitted)
	}

	writeSizedImage(t, fx.db, fx.familyId, 100)
	if err := func() *AppError {
		var quotaErr *AppError
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { quotaErr = CheckStorageAccepts(tx, fx.familyId, 2<<30) })
		return quotaErr
	}(); err == nil {
		t.Fatal("a photo upload ignored outstanding video reservations")
	}
}

func TestVideoUploadOwnershipAndCancel(t *testing.T) {
	fx := setupVideoFixture(t)
	begun, err := fx.begin(t, 8)
	if err != nil {
		t.Fatal(err)
	}

	var stranger User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		stranger = AddUserTx(tx, CreateAccountRequest{Name: "Stranger", Email: "stranger-video@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	var strangerErr error
	fx.asUser(t, stranger, func(ctx *vbeam.Context) {
		_, strangerErr = CompleteVideoUpload(ctx, CompleteVideoUploadRequest{UploadId: begun.UploadId})
	})
	if !errors.Is(strangerErr, ErrVideoUploadNotFound) {
		t.Fatalf("another user completed the upload: %v", strangerErr)
	}

	if _, err := callAs(t, fx.resultsFixture, CancelVideoUpload, CancelVideoUploadRequest{UploadId: begun.UploadId}); err != nil {
		t.Fatal(err)
	}
	if fx.usage(t) != 0 {
		t.Fatalf("cancelled upload still holds %d bytes", fx.usage(t))
	}
	if code := fx.put(t, begun.Upload, []byte("12345678")); code != http.StatusConflict {
		t.Fatalf("upload after cancel: status %d", code)
	}
}

func TestSweepExpiresAbandonedAndLateUploads(t *testing.T) {
	fx := setupVideoFixture(t)
	ctx := context.Background()

	abandoned, _ := fx.begin(t, 8)
	if code := fx.put(t, abandoned.Upload, []byte("12345678")); code != http.StatusOK {
		t.Fatal(code)
	}

	accepted, _ := fx.begin(t, 4)
	fx.put(t, accepted.Upload, []byte("1234"))
	done, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: accepted.UploadId})
	if err != nil {
		t.Fatal(err)
	}
	var sourceKey string
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		image := GetImageById(tx, done.Image.Id)
		sourceKey = image.OriginalKey
		image.OriginalKey = "videos/moved/original.mov"
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.TxCommit(tx)
	})
	if err := fx.store.Put(ctx, sourceKey, bytes.NewReader([]byte("late")), 4, "video/quicktime"); err != nil {
		t.Fatal(err)
	}

	abandonedKey := keyOf(t, fx, abandoned.UploadId)
	result := sweepMedia(fx.db, fx.store, time.Now())
	if result.Expired != 0 || result.LateRemoved != 0 {
		t.Fatalf("swept before anything expired: %+v", result)
	}

	later := time.Now().Add(mediaUploadTTL + videoCompletionGrace + time.Minute)
	result = sweepMedia(fx.db, fx.store, later)
	if result.Expired != 1 || result.LateRemoved != 1 {
		t.Fatalf("sweep = %+v", result)
	}
	for _, key := range []string{sourceKey, abandonedKey} {
		if _, err := fx.store.Stat(ctx, key); !errors.Is(err, ErrMediaNotFound) {
			t.Errorf("%s still stored after sweep", key)
		}
	}
	if _, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: abandoned.UploadId}); !errors.Is(err, ErrVideoUploadNotFound) && !errors.Is(err, ErrVideoUploadExpired) {
		t.Fatalf("completing an expired upload: %v", err)
	}
	if used, want := fx.usage(t), imageStoredBytes(imageForTest(t, fx.db, done.Image.Id)); used != want {
		t.Fatalf("usage after sweep = %d, want %d", used, want)
	}

	sweepMedia(fx.db, fx.store, time.Now().Add(completedUploadRetention+mediaUploadTTL+time.Hour))
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if n := len(familyVideoUploads(tx, fx.familyId)); n != 0 {
			t.Fatalf("%d upload sessions left after retention", n)
		}
	})
}

func keyOf(t *testing.T, fx videoFixture, uploadId int) (key string) {
	t.Helper()
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { key = getVideoUpload(tx, uploadId).Key })
	return
}

func TestDeletingAVideoRemovesItsStoredObjects(t *testing.T) {
	fx := setupVideoFixture(t)
	ctx := context.Background()
	video := fx.addVideoAt(t, "clip.mov", featuresEpoch)
	for _, key := range []string{video.OriginalKey, video.PlaybackKey} {
		if err := fx.store.Put(ctx, key, bytes.NewReader([]byte("x")), 1, "video/mp4"); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := callAs(t, fx.resultsFixture, DeletePhoto, DeletePhotoRequest{Id: video.Id}); err != nil {
		t.Fatal(err)
	}
	failing := failingDeleteStore{fx.store}
	if r := sweepMedia(fx.db, failing, time.Now()); r.DeleteFailed != 2 {
		t.Fatalf("failing sweep = %+v", r)
	}
	if r := sweepMedia(fx.db, fx.store, time.Now()); r.Deleted != 2 {
		t.Fatalf("retry sweep = %+v", r)
	}
	for _, key := range []string{video.OriginalKey, video.PlaybackKey} {
		if _, err := fx.store.Stat(ctx, key); !errors.Is(err, ErrMediaNotFound) {
			t.Errorf("%s survived the delete", key)
		}
	}
}

type failingDeleteStore struct{ MediaStore }

func (failingDeleteStore) Delete(context.Context, string) error {
	return errors.New("bucket unreachable")
}

func TestDiskReservationsAreShared(t *testing.T) {
	previous := freeDiskBytes
	t.Cleanup(func() { freeDiskBytes = previous })
	freeDiskBytes = func(string) (int64, error) { return 10 << 30, nil }

	budget := &diskReservations{}
	first, err := budget.reserve("/x", 5<<30, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := budget.reserve("/x", 5<<30, 1<<30); err == nil {
		t.Fatal("second reservation claimed the same free space")
	}
	first()
	first()
	if budget.reservedBytes() != 0 {
		t.Fatalf("release left %d reserved", budget.reservedBytes())
	}

	release, _ := diskBudget.reserve("/x", 8<<30, 1<<30)
	defer release()
	if CheckDiskHeadroom("/x", 2<<30, 1<<30) == nil {
		t.Fatal("CheckDiskHeadroom ignored outstanding reservations")
	}
}
