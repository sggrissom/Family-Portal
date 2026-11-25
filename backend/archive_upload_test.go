package backend

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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
