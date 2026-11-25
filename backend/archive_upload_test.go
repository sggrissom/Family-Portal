package backend

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func (fx *archiveFixture) uploadArchive(t *testing.T, user User, bundle []byte) int {
	t.Helper()
	useArchiveUploadStore(t)
	begun, err := callProc(t, user, BeginArchiveImport, BeginArchiveImportRequest{Filename: "family.zip", Size: len(bundle)})
	if err != nil {
		t.Fatal(err)
	}
	if put := sendUpload(fx.store, begun.Upload, bundle); put.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", put.Code, put.Body.String())
	}
	return begun.ImportId
}

func TestEverythingArchiveImportsInTheBackground(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)

	status, err := runArchiveImport(t, restorer, importId, importEverything)
	if err != nil {
		t.Fatal(err)
	}
	drainPhotoJobs()
	if status.State != "done" || status.Result == nil {
		t.Fatalf("job finished as %+v", status)
	}
	restored := readArchiveContents(t, fx.db, restorer.FamilyId)
	if status.Result.ImportedPeople != len(restored["people"]) || status.Result.ImportedPhotos == 0 {
		t.Errorf("result %+v for %d people", status.Result, len(restored["people"]))
	}
	if status.Records == 0 || status.Media == 0 {
		t.Errorf("progress counts %d records, %d media", status.Records, status.Media)
	}

	other := fx.newUser(t, "other")
	if _, err := callProc(t, other, GetArchiveImportStatus, GetArchiveImportStatusRequest{ImportId: importId}); err == nil {
		t.Error("another user read the import's status")
	}
}

func TestArchiveImportCompletesOnlyAfterTheUploadArrives(t *testing.T) {
	fx := setupArchiveFixture(t)
	useArchiveUploadStore(t)
	bundle := fx.exportBundle(t, fx.owner, "data_only")
	restorer := fx.newUser(t, "restorer")
	begun, err := callProc(t, restorer, BeginArchiveImport, BeginArchiveImportRequest{Filename: "family.zip", Size: len(bundle)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: begun.ImportId, Options: importEverything}); err == nil {
		t.Fatal("completed before the archive was uploaded")
	}
	if put := sendUpload(fx.store, begun.Upload, bundle[:len(bundle)/2]); put.Code == http.StatusOK {
		t.Fatal("a short upload was accepted")
	}
	if n := len(readArchiveContents(t, fx.db, restorer.FamilyId)["people"]); n != 0 {
		t.Errorf("%d people imported without an archive", n)
	}
}

func TestArchiveImportPreviewAndDryRunWriteNothing(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)
	before := readArchiveContents(t, fx.db, restorer.FamilyId)

	preview, err := runArchiveImport(t, restorer, importId, ImportDataRequest{PreviewOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "done" || preview.Result == nil || len(preview.Result.AvailablePeople) == 0 {
		t.Fatalf("preview finished as %+v", preview)
	}

	dry := importEverything
	dry.DryRun = true
	dryRun, err := runArchiveImport(t, restorer, importId, dry)
	if err != nil {
		t.Fatal(err)
	}
	if len(drainPhotoJobs()) != 0 {
		t.Error("a dry run queued photo processing")
	}
	for _, d := range diffArchiveContents(before, readArchiveContents(t, fx.db, restorer.FamilyId)) {
		t.Errorf("the preview or dry run changed the family: %s", d)
	}

	real, err := runArchiveImport(t, restorer, importId, importEverything)
	if err != nil {
		t.Fatal(err)
	}
	drainPhotoJobs()
	if real.State != "done" || real.Result == nil || dryRun.Result == nil {
		t.Fatalf("dry run %+v, import %+v", dryRun, real)
	}
	d, r := dryRun.Result, real.Result
	if d.ImportedPeople != r.ImportedPeople || d.ImportedMeasurements != r.ImportedMeasurements ||
		d.ImportedMilestones != r.ImportedMilestones || d.ImportedPhotos != r.ImportedPhotos ||
		d.ImportedBooks != r.ImportedBooks || d.ImportedInterviews != r.ImportedInterviews {
		t.Errorf("dry run counted %+v, the import %+v", d, r)
	}

	if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: importId, Options: importEverything}); err == nil {
		t.Error("an archive was imported twice from one upload")
	}
}

func TestArchiveImportThatFailsReportsItAndChangesNothing(t *testing.T) {
	fx := setupArchiveFixture(t)
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, []byte("this is not a zip file"))

	status, err := runArchiveImport(t, restorer, importId, importEverything)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "failed" || status.Error == "" {
		t.Fatalf("job finished as %+v", status)
	}
	if n := len(readArchiveContents(t, fx.db, restorer.FamilyId)["people"]); n != 0 {
		t.Errorf("a failed import left %d people", n)
	}
}

func TestArchiveUploadsMayExceedTheOldImportRequestCap(t *testing.T) {
	fx := setupArchiveFixture(t)
	useArchiveUploadStore(t)
	const oldCap = 2 << 30
	size := oldCap + 1<<20
	begun, err := callProc(t, fx.owner, BeginArchiveImport, BeginArchiveImportRequest{Filename: "family.zip", Size: size})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(begun.Upload.URL)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, u.RequestURI(), nil)
	if limit := requestBodyLimit(req); limit != 0 && limit < int64(size) {
		t.Errorf("the upload path caps bodies at %d, below the %d-byte archive", limit, size)
	}
}

func (fx *archiveFixture) uploadStored(t *testing.T, importId int) bool {
	t.Helper()
	var key string
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { key = getArchiveImport(tx, importId).Key })
	_, err := activeMediaStore().Stat(context.Background(), key)
	if err != nil && !errors.Is(err, ErrMediaNotFound) {
		t.Fatal(err)
	}
	return err == nil
}

func TestArchiveImportDeletesItsUploadWhenItEnds(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)

	if _, err := runArchiveImport(t, restorer, importId, ImportDataRequest{PreviewOnly: true}); err != nil {
		t.Fatal(err)
	}
	if !fx.uploadStored(t, importId) {
		t.Fatal("a preview deleted the archive it previewed")
	}
	if status, err := runArchiveImport(t, restorer, importId, importEverything); err != nil || status.State != "done" {
		t.Fatalf("import = %+v, %v", status, err)
	}
	drainPhotoJobs()
	if fx.uploadStored(t, importId) {
		t.Error("the uploaded archive is still stored after its import")
	}

	failing := fx.uploadArchive(t, restorer, []byte("this is not a zip file"))
	if status, _ := runArchiveImport(t, restorer, failing, importEverything); status.State != "failed" {
		t.Fatalf("import of a broken file = %+v", status)
	}
	if fx.uploadStored(t, failing) {
		t.Error("the uploaded archive is still stored after its import failed")
	}
	if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: failing, Options: importEverything}); err == nil {
		t.Error("an import restarted from an upload that was deleted")
	}
}

// pauseArchiveStaging stops the next import after it stages its first file,
// until the returned release is called. stagingDir is where it stages.
func pauseArchiveStaging(t *testing.T, fail error) (paused <-chan struct{}, release func(), stagingDir string) {
	t.Helper()
	stagingDir = t.TempDir()
	previousRoot, previousHook := archiveStagingRoot, archiveStagedHook
	archiveStagingRoot = stagingDir
	pausedCh, releaseCh := make(chan struct{}), make(chan struct{})
	var once bool
	archiveStagedHook = func(staged int) error {
		if once {
			return nil
		}
		once = true
		close(pausedCh)
		<-releaseCh
		return fail
	}
	var released bool
	release = func() {
		if !released {
			released = true
			close(releaseCh)
		}
	}
	t.Cleanup(func() {
		release()
		waitForArchiveImports()
		archiveStagingRoot, archiveStagedHook = previousRoot, previousHook
	})
	return pausedCh, release, stagingDir
}

func stagedFiles(t *testing.T, dir string) (n int) {
	t.Helper()
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return
}

func TestOtherWritesGoThroughWhileAnImportStagesMedia(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)
	paused, release, _ := pauseArchiveStaging(t, nil)

	if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: importId, Options: importEverything}); err != nil {
		t.Fatal(err)
	}
	<-paused

	wrote := make(chan error, 1)
	go func() {
		_, err := callProc(t, fx.owner, AddPerson, AddPersonRequest{Name: "Written during staging", Birthdate: "2020-01-01"})
		wrote <- err
	}()
	select {
	case err := <-wrote:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		release()
		t.Fatal("another family's write waited on the staging import")
	}

	status, err := callProc(t, restorer, GetArchiveImportStatus, GetArchiveImportStatusRequest{ImportId: importId})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "staging" || status.StagedMedia != 1 {
		t.Errorf("status while staging = %+v", status)
	}

	release()
	waitForArchiveImports()
	jobs := drainPhotoJobs()
	status, _ = callProc(t, restorer, GetArchiveImportStatus, GetArchiveImportStatusRequest{ImportId: importId})
	if status.State != "done" || status.Result == nil || status.Result.ImportedPhotos == 0 {
		t.Fatalf("import finished as %+v", status)
	}
	if r := status.Result; r.ImportedVideos == 0 || status.StagedMedia != r.ImportedPhotos+r.ImportedVideos {
		t.Errorf("staged %d files for %d photos and %d videos", status.StagedMedia, r.ImportedPhotos, r.ImportedVideos)
	}
	if len(jobs) != status.Result.ImportedPhotos {
		t.Errorf("%d photos queued for processing, %d imported", len(jobs), status.Result.ImportedPhotos)
	}
	for _, job := range jobs {
		if _, err := os.Stat(photoOriginalPath(job.FilePath)); err != nil {
			t.Errorf("photo %d is queued without its file: %v", job.ImageId, err)
		}
	}
	var images []Image
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { images = GetFamilyImages(tx, restorer.FamilyId) })
	for _, image := range images {
		if !image.IsVideo() {
			continue
		}
		if _, err := activeMediaStore().Stat(context.Background(), image.OriginalKey); err != nil {
			t.Errorf("video %d was imported without its file: %v", image.Id, err)
		}
	}
}

func TestArchiveImportThatFailsWhileStagingLeavesNothingBehind(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)
	before := readArchiveContents(t, fx.db, restorer.FamilyId)
	paused, release, stagingDir := pauseArchiveStaging(t, errors.New("The server ran out of room while unpacking."))

	if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: importId, Options: importEverything}); err != nil {
		t.Fatal(err)
	}
	<-paused
	if stagedFiles(t, stagingDir) == 0 {
		t.Fatal("nothing was staged before the failure")
	}
	release()
	waitForArchiveImports()

	status, err := callProc(t, restorer, GetArchiveImportStatus, GetArchiveImportStatusRequest{ImportId: importId})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "failed" || status.Error == "" {
		t.Fatalf("job finished as %+v", status)
	}
	for _, d := range diffArchiveContents(before, readArchiveContents(t, fx.db, restorer.FamilyId)) {
		t.Errorf("a failed import changed the family: %s", d)
	}
	if n := stagedFiles(t, stagingDir); n != 0 {
		t.Errorf("a failed import left %d staged files", n)
	}
	if len(drainPhotoJobs()) != 0 {
		t.Error("a failed import queued photo processing")
	}
}

func TestMediaJanitorExpiresAbandonedArchiveUploads(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "data_only")
	restorer := fx.newUser(t, "restorer")
	previewed := fx.uploadArchive(t, restorer, bundle)
	if _, err := runArchiveImport(t, restorer, previewed, ImportDataRequest{PreviewOnly: true}); err != nil {
		t.Fatal(err)
	}
	unfinished, err := callProc(t, restorer, BeginArchiveImport, BeginArchiveImportRequest{Filename: "family.zip", Size: len(bundle)})
	if err != nil {
		t.Fatal(err)
	}

	sweepMedia(fx.db, activeMediaStore(), time.Now().Add(23*time.Hour))
	if !fx.uploadStored(t, previewed) {
		t.Fatal("the janitor removed an upload within a day of its last use")
	}

	sweepMedia(fx.db, activeMediaStore(), time.Now().Add(25*time.Hour))
	if fx.uploadStored(t, previewed) {
		t.Error("an upload abandoned for over a day is still stored")
	}
	for _, id := range []int{previewed, unfinished.ImportId} {
		if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: id, Options: importEverything}); err == nil {
			t.Errorf("import %d started after the janitor expired it", id)
		}
	}
	var reserved int64
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { reserved = archiveImportReservedBytes(tx, restorer.FamilyId) })
	if reserved != 0 {
		t.Errorf("an expired upload still reserves %d bytes", reserved)
	}
}
