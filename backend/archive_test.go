package backend

import (
	"context"
	"encoding/json"
	"errors"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestEveryBucketIsClassifiedForTheArchive(t *testing.T) {
	registered := map[string]bool{}
	for _, name := range cfg.Info.BucketList {
		registered[name] = true
		entry, ok := archiveBuckets[name]
		switch {
		case !ok:
			t.Errorf("bucket %q is not classified in archiveBuckets as archived, derived or excluded", name)
		case entry.Class != BucketArchived && entry.Reason == "":
			t.Errorf("bucket %q is derived or excluded without a reason", name)
		case entry.Class == BucketArchived && archiveReaders[name] == nil:
			t.Errorf("archived bucket %q has no reader in archiveReaders", name)
		}
	}
	for name := range archiveBuckets {
		if !registered[name] {
			t.Errorf("archiveBuckets classifies %q, which is not a registered bucket", name)
		}
	}
}

func TestArchiveSeedFillsEveryArchivedBucket(t *testing.T) {
	fx := setupArchiveFixture(t)
	contents := readArchiveContents(t, fx.db, fx.familyId)
	for name, entry := range archiveBuckets {
		if entry.Class == BucketArchived && len(contents[name]) == 0 {
			t.Errorf("the archive seed leaves %q empty", name)
		}
	}
}

func isVideoRow(row archiveRow) bool { return strings.HasPrefix(row["Media"], "video:") }

func pointsAtVideo(row archiveRow) bool { return strings.HasPrefix(row["PhotoId"], "video:") }

// Each later family archive ticket deletes the entries it fixes.
var knownArchiveDrops = []archiveDrop{
	{"images", "", isVideoRow, "videos and their files are not imported"},
	{"photo_person", "", pointsAtVideo, "videos and their files are not imported"},
	{"photo_tags", "", pointsAtVideo, "videos and their files are not imported"},
	{"video_transcripts", "", nil, "transcripts are not imported"},
	{"milestone_photos", "", nil, "moment-photo links are not exported"},
	{"photo_faces", "", nil, "faces are not exported"},
	{"images", "AnalysisVersion", nil, "faces are not exported"},
	{"family_places", "", nil, "places are not exported"},
	{"families", "Name", nil, "family settings are not exported"},
	{"families", "VideoEnabled", nil, "family settings are not exported"},
	{"interview_question_lists", "DismissedKeys", nil, "interview list settings are not exported"},
	{"milestones", "CreatedAt", nil, "createdAt is reset"},
	{"milestone_tags", "CreatedAt", nil, "createdAt is reset"},
	{"interview_questions", "CreatedAt", nil, "createdAt is reset"},
	{"books", "CreatedAt", nil, "createdAt is reset"},
	{"images", "CreatedAt", nil, "createdAt is reset"},
	{"photo_person", "CreatedAt", nil, "createdAt is reset"},
	{"photo_tags", "CreatedAt", nil, "createdAt is reset"},
	{"activities", "CreatedAt", nil, "createdAt is reset"},
	{"seasons", "CreatedAt", nil, "createdAt is reset"},
	{"activity_events", "CreatedAt", nil, "createdAt is reset"},
	{"activity_entries", "CreatedAt", nil, "createdAt is reset"},
	{"entry_members", "CreatedAt", nil, "createdAt is reset"},
	{"appearances", "CreatedAt", nil, "createdAt is reset"},
	{"appearance_photos", "CreatedAt", nil, "createdAt is reset"},
	{"activity_results", "CreatedAt", nil, "createdAt is reset"},
	{"activity_event_photos", "CreatedAt", nil, "createdAt is reset"},
	{"books", "UpdatedAt", nil, "book timestamps are reset"},
	{"books", "ReviewedAt", nil, "book timestamps are reset"},
	{"milestones", "Note", nil, "the moment note is not imported"},
	{"images", "OriginalFilename", nil, "the original filename is not exported"},
	{"photo_person", "AutoTagged", nil, "the auto-tagged flag is not exported"},
}

var knownReimportDrops = []archiveDrop{}

func TestFamilyArchiveRoundTrip(t *testing.T) {
	fx := setupArchiveFixture(t)
	source := readArchiveContents(t, fx.db, fx.familyId)

	bundle := fx.exportBundle(t, fx.owner, "with_photos")
	restorer := fx.newUser(t, "restorer")
	fx.importBundle(t, restorer, bundle, importEverything)
	restored := readArchiveContents(t, fx.db, restorer.FamilyId)
	assertArchiveMatches(t, source, restored, knownArchiveDrops)

	fx.importBundle(t, restorer, bundle, importEverything)
	reimported := readArchiveContents(t, fx.db, restorer.FamilyId)
	assertArchiveMatches(t, restored, reimported, knownReimportDrops)
}

func TestFormatV1ArchiveImports(t *testing.T) {
	fx := setupArchiveFixture(t)
	data, err := os.ReadFile("testdata/archive_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var legacy ImportDataStructure
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}

	moments := 0
	for _, m := range legacy.Milestones {
		if m.InterviewId == 0 {
			moments++
		}
	}

	restorer := fx.newUser(t, "restorer")
	var resp ImportDataResponse
	fx.call(t, restorer, func(ctx *vbeam.Context) {
		resp, err = ImportData(ctx, ImportDataRequest{
			JsonData: string(data), ImportMilestones: true, ImportActivities: true,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Errors) > 0 {
		t.Errorf("import errors: %v", resp.Errors)
	}
	if resp.ImportedPeople != len(legacy.People) ||
		resp.ImportedMeasurements != len(legacy.Heights)+len(legacy.Weights) ||
		resp.ImportedMilestones != moments ||
		resp.ImportedRelations != len(legacy.Relations) ||
		resp.ImportedBooks != len(legacy.Books) ||
		resp.ImportedInterviews != len(legacy.Interviews) ||
		resp.ImportedActivities.Activities != len(legacy.Activities) {
		t.Errorf("imported %+v from a fixture of %d people, %d measurements, %d moments, %d relations, %d books, %d interviews, %d activities",
			resp, len(legacy.People), len(legacy.Heights)+len(legacy.Weights), moments,
			len(legacy.Relations), len(legacy.Books), len(legacy.Interviews), len(legacy.Activities))
	}

	restored := readArchiveContents(t, fx.db, restorer.FamilyId)
	for _, bucket := range []string{"people", "growth_data", "milestones", "tags", "activities", "interviews", "books"} {
		if len(restored[bucket]) == 0 {
			t.Errorf("the v1 fixture imported nothing into %s", bucket)
		}
	}
}

func (fx *archiveFixture) exportBundle(t *testing.T, user User, mode string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/export-bundle?mode="+mode, nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, user))
	rec := httptest.NewRecorder()
	exportBundleHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

var importEverything = ImportDataRequest{ImportMilestones: true, ImportActivities: true}

func (fx *archiveFixture) importBundle(t *testing.T, user User, bundle []byte, options ImportDataRequest) ImportDataResponse {
	t.Helper()
	rec := postImportBundle(t, user, bundle, options)
	var resp ImportDataResponse
	if rec.Code != http.StatusOK {
		t.Fatalf("import = %d %s", rec.Code, rec.Body.String())
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	for _, job := range drainPhotoJobs() {
		t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	}
	return resp
}

// postImportBundle imports a zip the way the import page does: begin, PUT to
// the signed URL, complete, then wait for the job. It answers like an HTTP
// endpoint so callers can check the outcome by status code.
func postImportBundle(t *testing.T, user User, bundle []byte, options ImportDataRequest) *httptest.ResponseRecorder {
	t.Helper()
	useArchiveUploadStore(t)
	rec := httptest.NewRecorder()
	fail := func(err error) *httptest.ResponseRecorder {
		rec.WriteHeader(http.StatusBadRequest)
		rec.WriteString(err.Error())
		return rec
	}
	begun, err := callProc(t, user, BeginArchiveImport, BeginArchiveImportRequest{
		FamilyId: options.FamilyId, Filename: "family.zip", Size: len(bundle),
	})
	if err != nil {
		return fail(err)
	}
	if put := sendUpload(activeMediaStore().(*localMediaStore), begun.Upload, bundle); put.Code != http.StatusOK {
		return put
	}
	status, err := runArchiveImport(t, user, begun.ImportId, options)
	if err != nil {
		return fail(err)
	}
	if status.State != "done" {
		return fail(errors.New(status.Error))
	}
	rec.WriteHeader(http.StatusOK)
	json.NewEncoder(rec).Encode(status.Result)
	return rec
}

// runArchiveImport completes an uploaded archive and waits for its job.
func runArchiveImport(t *testing.T, user User, importId int, options ImportDataRequest) (ArchiveImportStatus, error) {
	t.Helper()
	if _, err := callProc(t, user, CompleteArchiveImport, CompleteArchiveImportRequest{ImportId: importId, Options: options}); err != nil {
		return ArchiveImportStatus{}, err
	}
	waitForArchiveImports()
	return callProc(t, user, GetArchiveImportStatus, GetArchiveImportStatusRequest{ImportId: importId})
}

// useArchiveUploadStore gives a test without a media store a local one whose
// uploads are gated the way the server gates them.
func useArchiveUploadStore(t *testing.T) {
	t.Helper()
	store, ok := activeMediaStore().(*localMediaStore)
	if !ok {
		var err error
		if store, err = newLocalMediaStore(filepath.Join(t.TempDir(), "media"), []byte("archive-upload-test-key")); err != nil {
			t.Fatal(err)
		}
		previousStore, previousFree := mediaStore, freeDiskBytes
		mediaStore = store
		freeDiskBytes = func(string) (int64, error) { return 1 << 50, nil }
		t.Cleanup(func() { mediaStore, freeDiskBytes = previousStore, previousFree })
	}
	store.gate = mediaUploadGate(appDb)
}

func callProc[Req any, Resp any](t *testing.T, user User, proc func(*vbeam.Context, Req) (Resp, error), req Req) (resp Resp, err error) {
	t.Helper()
	token := testAuthToken(t, user)
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		resp, err = proc(&vbeam.Context{Tx: tx, Token: token}, req)
		vbolt.TxCommit(tx)
	})
	return
}

func drainPhotoJobs() (jobs []PhotoProcessingJob) {
	for {
		select {
		case job := <-globalPhotoWorker.jobQueue:
			jobs = append(jobs, job)
		default:
			return
		}
	}
}
