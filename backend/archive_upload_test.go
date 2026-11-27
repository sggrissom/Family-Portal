package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	bundle := fx.exportBundle(t, fx.owner, "everything")
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
	bundle := fx.exportBundle(t, fx.owner, "records")
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
	bundle := fx.exportBundle(t, fx.owner, "everything")
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
	bundle := fx.exportBundle(t, fx.owner, "everything")
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

}

func TestArchiveImportThatFailsCanBeRetriedFromTheSameUpload(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)
	paused, release, _ := pauseArchiveStaging(t, errors.New("The server ran out of room while unpacking."))

	if _, err := callProc(t, restorer, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: importId, Options: importEverything}); err != nil {
		t.Fatal(err)
	}
	<-paused
	release()
	waitForArchiveImports()
	if !fx.uploadStored(t, importId) {
		t.Fatal("a failed import deleted the upload it could be retried from")
	}

	status, err := runArchiveImport(t, restorer, importId, importEverything)
	if err != nil {
		t.Fatal(err)
	}
	drainPhotoJobs()
	if status.State != "done" || status.Result == nil || status.Result.ImportedPhotos == 0 {
		t.Fatalf("the retry finished as %+v", status)
	}
	if fx.uploadStored(t, importId) {
		t.Error("the uploaded archive is still stored after the retry imported it")
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
	bundle := fx.exportBundle(t, fx.owner, "everything")
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
	bundle := fx.exportBundle(t, fx.owner, "everything")
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
	bundle := fx.exportBundle(t, fx.owner, "records")
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

func TestAKeptArchiveUploadCountsTowardTheFamilyQuota(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	restorer := fx.newUser(t, "restorer")
	usage := func() (used int64) {
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { used = FamilyStorageUsage(tx, restorer.FamilyId) })
		return
	}
	importId := fx.uploadArchive(t, restorer, bundle)
	if _, err := runArchiveImport(t, restorer, importId, ImportDataRequest{PreviewOnly: true}); err != nil {
		t.Fatal(err)
	}
	if used := usage(); used != int64(len(bundle)) {
		t.Errorf("a previewed upload kept on the server counts %d bytes; want its %d", used, len(bundle))
	}

	if _, err := runArchiveImport(t, restorer, importId, importEverything); err != nil {
		t.Fatal(err)
	}
	drainPhotoJobs()
	var media int64
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, image := range GetFamilyImages(tx, restorer.FamilyId) {
			media += imageStoredBytes(image)
		}
	})
	if used := usage(); used != media {
		t.Errorf("after the import the family uses %d bytes; want only its %d bytes of media", used, media)
	}
}

func useStagingRoot(t *testing.T) string {
	t.Helper()
	dir, previous := t.TempDir(), archiveStagingRoot
	archiveStagingRoot = dir
	t.Cleanup(func() { archiveStagingRoot = previous })
	return dir
}

func TestArchiveImportStoppedAfterItCommitsPlacesItsMediaOnRestart(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)
	stagingDir := useStagingRoot(t)
	archiveImportCommittedHook = func() {
		go StopArchiveImports(context.Background())
		for !archiveImportsStopping.Load() {
			time.Sleep(time.Millisecond)
		}
	}
	t.Cleanup(func() {
		archiveImportCommittedHook = nil
		archiveImportsStopping.Store(false)
	})

	status, err := runArchiveImport(t, restorer, importId, importEverything)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "placing" {
		t.Fatalf("a job stopped right after its commit finished as %+v; want it left placing", status)
	}
	if stagedFiles(t, stagingDir) == 0 {
		t.Fatal("the staged files were removed before they were placed")
	}

	archiveImportCommittedHook = nil
	InitializeArchiveImports(fx.db)
	waitForArchiveImports()
	jobs := drainPhotoJobs()
	for _, job := range jobs {
		t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	}
	status, _ = callProc(t, restorer, GetArchiveImportStatus, GetArchiveImportStatusRequest{ImportId: importId})
	if status.State != "done" || status.Result == nil || status.Result.ImportedPhotos == 0 {
		t.Fatalf("after the restart the job finished as %+v", status)
	}
	if len(jobs) != status.Result.ImportedPhotos {
		t.Errorf("%d photos queued after the restart; %d imported", len(jobs), status.Result.ImportedPhotos)
	}
	for _, job := range jobs {
		if _, err := os.Stat(photoOriginalPath(job.FilePath)); err != nil {
			t.Errorf("photo %d has no file after the restart: %v", job.ImageId, err)
		}
	}
	for _, video := range familyVideos(t, fx.db, restorer.FamilyId) {
		if _, err := activeMediaStore().Stat(context.Background(), video.OriginalKey); err != nil {
			t.Errorf("video %d has no file after the restart: %v", video.Id, err)
		}
	}
	if n := stagedFiles(t, stagingDir); n != 0 {
		t.Errorf("%d staged files left after the restart placed them", n)
	}
	if fx.uploadStored(t, importId) {
		t.Error("the upload is still stored after the restart finished its import")
	}
}

func TestArchiveRetriesFailedMediaPlacementWithoutMergingAgain(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	restorer := fx.newUser(t, "restorer")
	importId := fx.uploadArchive(t, restorer, bundle)
	stagingDir := useStagingRoot(t)

	var move stagedMove
	var setupErr error
	archiveImportCommittedHook = func() {
		var job ArchiveImport
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { job = getArchiveImport(tx, importId) })
		var followUp archiveImported
		if setupErr = json.Unmarshal([]byte(job.FollowUp), &followUp); setupErr != nil {
			return
		}
		if len(followUp.Photos) == 0 {
			setupErr = errors.New("expected an imported photo")
			return
		}
		move = followUp.Photos[0].File
		setupErr = os.MkdirAll(move.To, 0755)
	}
	t.Cleanup(func() { archiveImportCommittedHook = nil })

	status, err := runArchiveImport(t, restorer, importId, importEverything)
	if setupErr != nil {
		t.Fatal(setupErr)
	}
	if err != nil || status.State != "failed" || !strings.Contains(status.Error, "Click Import Data again") {
		t.Fatalf("expected a retryable placement failure, got %+v, %v", status, err)
	}
	if _, err := os.Stat(move.From); err != nil {
		t.Fatalf("staged original lost after placement failure: %v", err)
	}
	if !fx.uploadStored(t, importId) {
		t.Fatal("upload deleted after placement failure")
	}
	if n := expireArchiveUploads(fx.db, time.Now().Add(25*time.Hour)); n != 0 {
		t.Fatal("janitor expired a placement that can be retried")
	}
	InitializeArchiveImports(fx.db)
	waitForArchiveImports()
	if err := os.Remove(move.To); err != nil {
		t.Fatal(err)
	}

	// A retry must finish the committed import even if the filters change.
	status, err = runArchiveImport(t, restorer, importId, ImportDataRequest{FilterPersonIds: []int{-1}})
	if err != nil || status.State != "done" || status.Result == nil || status.Result.ImportedPhotos == 0 {
		t.Fatalf("placement retry finished as %+v, %v", status, err)
	}
	for _, job := range drainPhotoJobs() {
		t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	}
	assertArchiveMatches(t, readArchiveContents(t, fx.db, fx.familyId), readArchiveContents(t, fx.db, restorer.FamilyId))
	if stagedFiles(t, stagingDir) != 0 || fx.uploadStored(t, importId) {
		t.Fatal("successful placement retry left its staging or upload behind")
	}
}

func TestUploadedRecordsReadFromObjectStore(t *testing.T) {
	store := newFakeS3Store(t)
	ctx := context.Background()
	const key = "imports/example/records.json"
	const records = `{"formatVersion":1,"people":[{"Id":2,"Name":"Ada","Birthday":"2020-03-04T00:00:00Z"}]}`
	if err := store.Put(ctx, key, strings.NewReader(records), int64(len(records)), "application/json"); err != nil {
		t.Fatal(err)
	}

	archive, closeArchive, err := openUploadedArchive(store, key, int64(len(records)))
	defer closeArchive()
	if err != nil || len(archive.People) != 1 || archive.People[0].Name != "Ada" {
		t.Fatalf("uploaded records = %+v, %v", archive.People, err)
	}
}

func TestArchiveRejectsOversizedRecords(t *testing.T) {
	fx := setupArchiveFixture(t)
	_, err := callProc(t, fx.owner, BeginArchiveImport, BeginArchiveImportRequest{
		Filename: "family.JSON", Size: maxArchiveRecordsBytes + 1,
	})
	if !errors.Is(err, errArchiveRecordsTooLarge) {
		t.Fatalf("oversized JSON upload: %v", err)
	}

	var bundle bytes.Buffer
	zw := zip.NewWriter(&bundle)
	entry, err := zw.Create("data.json")
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 1<<20)
	for i := 0; i <= maxArchiveRecordsBytes/len(chunk); i++ {
		if _, err := entry.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(bundle.Bytes()), int64(bundle.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = readArchiveZip(zr); !errors.Is(err, errArchiveRecordsTooLarge) {
		t.Fatalf("oversized data.json in ZIP: %v", err)
	}
}

func TestArchiveImportsClearStagingACrashLeftBehind(t *testing.T) {
	fx := setupArchiveFixture(t)
	stagingDir := useStagingRoot(t)
	leftover := filepath.Join(stagingDir, "import-123")
	if err := os.MkdirAll(leftover, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leftover, "7.png"), []byte("half a photo"), 0644); err != nil {
		t.Fatal(err)
	}

	InitializeArchiveImports(fx.db)
	waitForArchiveImports()
	if n := stagedFiles(t, stagingDir); n != 0 {
		t.Errorf("%d staged files a crashed import left are still there after a restart", n)
	}
}

func TestARecordsFileTooBigForARequestImportsThroughTheUpload(t *testing.T) {
	fx := setupArchiveFixture(t)
	var records ImportDataStructure
	if err := json.Unmarshal([]byte(fx.exportRecords(t, fx.owner)), &records); err != nil {
		t.Fatal(err)
	}
	if len(records.Books) == 0 {
		t.Fatal("the seed has no book")
	}
	introduction := strings.Repeat("A long summer by the water. ", int(maxJSONRequestBytes/28)+1)
	records.Books[0].Introduction = introduction
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}

	restorer := fx.newUser(t, "restorer")
	useArchiveUploadStore(t)
	begun, err := callProc(t, restorer, BeginArchiveImport, BeginArchiveImportRequest{Filename: "family-records.json", Size: len(data)})
	if err != nil {
		t.Fatal(err)
	}
	if put := sendUpload(fx.store, begun.Upload, data); put.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", put.Code, put.Body.String())
	}
	preview, err := runArchiveImport(t, restorer, begun.ImportId, ImportDataRequest{PreviewOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "done" || preview.Result == nil || len(preview.Result.AvailablePeople) != len(records.People) {
		t.Fatalf("the preview finished as %+v", preview)
	}
	status, err := runArchiveImport(t, restorer, begun.ImportId, importEverything)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "done" || status.Result == nil || status.Result.ImportedPeople != len(records.People) {
		t.Fatalf("the import finished as %+v", status)
	}
	var books []Book
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, row := range archiveReaders["books"](tx, restorer.FamilyId) {
			books = append(books, row.(Book))
		}
	})
	if len(books) != 1 || books[0].Introduction != introduction {
		t.Errorf("imported %d books; want the one with its %d-byte introduction", len(books), len(introduction))
	}
}
