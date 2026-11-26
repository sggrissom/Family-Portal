package backend

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// An Everything archive is uploaded straight to the media store, then
// imported by a background job. A records file stays a normal ImportData call.

type ArchiveImportState int

const (
	ArchiveImportUploading ArchiveImportState = iota
	ArchiveImportQueued
	ArchiveImportStaging
	ArchiveImportImporting
	ArchiveImportDone
	ArchiveImportFailed
)

var archiveImportStateNames = map[ArchiveImportState]string{
	ArchiveImportUploading: "uploading",
	ArchiveImportQueued:    "queued",
	ArchiveImportStaging:   "staging",
	ArchiveImportImporting: "importing",
	ArchiveImportDone:      "done",
	ArchiveImportFailed:    "failed",
}

func (s ArchiveImportState) running() bool {
	return s == ArchiveImportQueued || s == ArchiveImportStaging || s == ArchiveImportImporting
}

type ArchiveImport struct {
	Id            int
	FamilyId      int
	UserId        int
	Key           string
	Filename      string
	DeclaredBytes int
	State         ArchiveImportState
	// Imported is set once a real import commits; the upload is then used up.
	Imported bool
	// Discarded is set once the uploaded zip is deleted.
	Discarded bool

	PreviewOnly      bool
	DryRun           bool
	ImportMilestones bool
	ImportActivities bool
	FilterFamilyIds  []int
	FilterPersonIds  []int

	Records    int
	Media      int
	Staged     int
	ResultJson string
	Error      string

	CreatedAt time.Time
	ExpiresAt time.Time
	UpdatedAt time.Time
}

func PackArchiveImport(self *ArchiveImport, buf *vpack.Buffer) {
	version := vpack.Version(2, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.Key, buf)
	vpack.String(&self.Filename, buf)
	vpack.Int(&self.DeclaredBytes, buf)
	vpack.IntEnum(&self.State, buf)
	vpack.Bool(&self.Imported, buf)
	vpack.Bool(&self.PreviewOnly, buf)
	vpack.Bool(&self.DryRun, buf)
	vpack.Bool(&self.ImportMilestones, buf)
	vpack.Bool(&self.ImportActivities, buf)
	vpack.Slice(&self.FilterFamilyIds, vpack.Int, buf)
	vpack.Slice(&self.FilterPersonIds, vpack.Int, buf)
	vpack.Int(&self.Records, buf)
	vpack.Int(&self.Media, buf)
	vpack.String(&self.ResultJson, buf)
	vpack.String(&self.Error, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.ExpiresAt, buf)
	vpack.Time(&self.UpdatedAt, buf)
	if version >= 2 {
		vpack.Bool(&self.Discarded, buf)
		vpack.Int(&self.Staged, buf)
	}
}

var ArchiveImportBkt = vbolt.Bucket(&cfg.Info, "archive_imports", vpack.FInt, PackArchiveImport)
var ArchiveImportByFamilyIndex = vbolt.Index(&cfg.Info, "archive_import_by_family", vpack.FInt, vpack.FInt)
var ArchiveImportByKeyIndex = vbolt.Index(&cfg.Info, "archive_import_by_key", vpack.String, vpack.FInt)

const maxArchiveImportBytes = 40 << 30

// Long enough to carry the largest archive over a home connection.
const archiveUploadTTL = 12 * time.Hour

func writeArchiveImport(tx *vbolt.Tx, job *ArchiveImport) {
	job.UpdatedAt = time.Now()
	vbolt.Write(tx, ArchiveImportBkt, job.Id, job)
	vbolt.SetTargetSingleTerm(tx, ArchiveImportByFamilyIndex, job.Id, job.FamilyId)
	vbolt.SetTargetSingleTerm(tx, ArchiveImportByKeyIndex, job.Id, job.Key)
}

func getArchiveImport(tx *vbolt.Tx, id int) (job ArchiveImport) {
	vbolt.Read(tx, ArchiveImportBkt, id, &job)
	return
}

func archiveImportByKey(tx *vbolt.Tx, key string) (job ArchiveImport) {
	var ids []int
	vbolt.ReadTermTargets(tx, ArchiveImportByKeyIndex, key, &ids, vbolt.Window{})
	if len(ids) > 0 {
		job = getArchiveImport(tx, ids[0])
	}
	return
}

// A zip holds its space until it is deleted, except while its own import
// runs, which counts the media it writes instead.
func archiveImportReservedBytes(tx *vbolt.Tx, familyId int) (total int64) {
	var ids []int
	vbolt.ReadTermTargets(tx, ArchiveImportByFamilyIndex, familyId, &ids, vbolt.Window{})
	var jobs []ArchiveImport
	vbolt.ReadSlice(tx, ArchiveImportBkt, ids, &jobs)
	for _, job := range jobs {
		if !job.Discarded && !job.Imported && job.State != ArchiveImportImporting {
			total += int64(job.DeclaredBytes)
		}
	}
	return
}

func (job ArchiveImport) options() ImportDataRequest {
	return ImportDataRequest{
		FamilyId:         job.FamilyId,
		FilterFamilyIds:  job.FilterFamilyIds,
		FilterPersonIds:  job.FilterPersonIds,
		PreviewOnly:      job.PreviewOnly,
		ImportMilestones: job.ImportMilestones,
		ImportActivities: job.ImportActivities,
		DryRun:           job.DryRun,
	}
}

var ErrArchiveImportNotFound = errors.New("That import could not be found.")
var ErrArchiveImportUnavailable = errors.New("Archive imports are not available on this server right now.")

type BeginArchiveImportRequest struct {
	FamilyId int    `json:"familyId,omitempty"`
	Filename string `json:"filename"`
	Size     int    `json:"size"`
}

type BeginArchiveImportResponse struct {
	ImportId int                `json:"importId"`
	Upload   UploadInstructions `json:"upload"`
}

func BeginArchiveImport(ctx *vbeam.Context, req BeginArchiveImportRequest) (resp BeginArchiveImportResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if req.Size <= 0 {
		err = errors.New("Choose an archive to import.")
		return
	}
	if req.Size > maxArchiveImportBytes {
		err = fmt.Errorf("Archives can be at most %s.", formatBytes(maxArchiveImportBytes))
		return
	}
	store := activeMediaStore()
	if store == nil {
		err = ErrArchiveImportUnavailable
		return
	}

	vbeam.UseWriteTx(ctx)
	familyId, accessErr := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if accessErr != nil {
		err = errors.New("You cannot import into that family.")
		return
	}
	if quotaErr := CheckFamilyStorageQuota(ctx.Tx, familyId, int64(req.Size), cfg.FamilyStorageQuotaBytes); quotaErr != nil {
		err = errors.New(quotaErr.Message)
		return
	}
	if _, local := store.(*localMediaStore); local {
		if diskErr := CheckDiskHeadroom(cfg.MediaDir, int64(req.Size), cfg.MinFreeDiskBytes); diskErr != nil {
			err = errors.New(diskErr.Message)
			return
		}
	}

	mediaId, idErr := newMediaId()
	if idErr != nil {
		err = idErr
		return
	}
	now := time.Now()
	job := ArchiveImport{
		Id:            vbolt.NextIntId(ctx.Tx, ArchiveImportBkt),
		FamilyId:      familyId,
		UserId:        user.Id,
		Key:           "imports/" + mediaId + "/archive.zip",
		Filename:      filepath.Base(req.Filename),
		DeclaredBytes: req.Size,
		State:         ArchiveImportUploading,
		CreatedAt:     now,
		ExpiresAt:     now.Add(archiveUploadTTL),
	}
	inst, urlErr := store.UploadURL(job.Key, int64(job.DeclaredBytes), "application/zip", archiveUploadTTL)
	if urlErr != nil {
		err = urlErr
		return
	}
	writeArchiveImport(ctx.Tx, &job)
	resp.ImportId = job.Id
	resp.Upload = inst
	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryAPI, "Archive import upload started", map[string]interface{}{
		"userId": user.Id, "familyId": familyId, "importId": job.Id, "bytes": req.Size,
	})
	return
}

type CompleteArchiveImportRequest struct {
	ImportId int `json:"importId"`
	// Options are the import page's choices; their familyId is ignored, since
	// the target family was fixed when the upload began.
	Options ImportDataRequest `json:"options"`
}

// CompleteArchiveImport starts a job on an uploaded archive. A preview or dry
// run leaves the upload in place, so the same archive can then be imported.
func CompleteArchiveImport(ctx *vbeam.Context, req CompleteArchiveImportRequest) (resp ArchiveImportStatus, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	job := getArchiveImport(ctx.Tx, req.ImportId)
	if job.Id == 0 || job.UserId != user.Id {
		err = ErrArchiveImportNotFound
		return
	}
	switch {
	case job.State.running():
		return archiveImportStatus(job), nil
	case job.Imported:
		err = errors.New("This archive has already been imported.")
		return
	case job.Discarded:
		err = errors.New("This upload has been cleared away. Please upload the archive again.")
		return
	case job.State == ArchiveImportUploading && time.Now().After(job.ExpiresAt.Add(videoCompletionGrace)):
		err = errors.New("This upload expired before it finished. Please try again.")
		return
	}

	store := activeMediaStore()
	if store == nil {
		err = ErrArchiveImportUnavailable
		return
	}
	obj, statErr := store.Stat(context.Background(), job.Key)
	if errors.Is(statErr, ErrMediaNotFound) {
		err = errors.New("The archive hasn't finished uploading yet.")
		return
	}
	if statErr != nil {
		err = ErrArchiveImportUnavailable
		return
	}
	if obj.Size != int64(job.DeclaredBytes) {
		err = fmt.Errorf("The upload arrived with %s instead of the expected %s. Please try again.",
			formatBytes(obj.Size), formatBytes(int64(job.DeclaredBytes)))
		return
	}

	vbeam.UseWriteTx(ctx)
	db := ctx.Tx.DB()
	job = getArchiveImport(ctx.Tx, req.ImportId)
	if job.State.running() || job.Imported || job.Discarded {
		return archiveImportStatus(job), nil
	}
	o := req.Options
	job.PreviewOnly, job.DryRun = o.PreviewOnly, o.DryRun
	job.ImportMilestones, job.ImportActivities = o.ImportMilestones, o.ImportActivities
	job.FilterFamilyIds, job.FilterPersonIds = o.FilterFamilyIds, o.FilterPersonIds
	job.State = ArchiveImportQueued
	job.ResultJson, job.Error = "", ""
	writeArchiveImport(ctx.Tx, &job)
	resp = archiveImportStatus(job)
	vbolt.TxCommit(ctx.Tx)

	startArchiveImportJob(db, job.Id)
	return
}

type GetArchiveImportStatusRequest struct {
	ImportId int `json:"importId"`
}

type ArchiveImportStatus struct {
	ImportId    int                 `json:"importId"`
	State       string              `json:"state"`
	PreviewOnly bool                `json:"previewOnly"`
	DryRun      bool                `json:"dryRun"`
	Records     int                 `json:"records"`
	Media       int                 `json:"media"`
	StagedMedia int                 `json:"stagedMedia"`
	Result      *ImportDataResponse `json:"result"`
	Error       string              `json:"error"`
}

func archiveImportStatus(job ArchiveImport) ArchiveImportStatus {
	status := ArchiveImportStatus{
		ImportId:    job.Id,
		State:       archiveImportStateNames[job.State],
		PreviewOnly: job.PreviewOnly,
		DryRun:      job.DryRun,
		Records:     job.Records,
		Media:       job.Media,
		StagedMedia: job.Staged,
		Error:       job.Error,
	}
	if job.ResultJson != "" {
		var result ImportDataResponse
		if json.Unmarshal([]byte(job.ResultJson), &result) == nil {
			status.Result = &result
		}
	}
	return status
}

func GetArchiveImportStatus(ctx *vbeam.Context, req GetArchiveImportStatusRequest) (resp ArchiveImportStatus, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	job := getArchiveImport(ctx.Tx, req.ImportId)
	if job.Id == 0 || job.UserId != user.Id {
		err = ErrArchiveImportNotFound
		return
	}
	return archiveImportStatus(job), nil
}

// Jobs run one at a time, so two large imports never compete for the lock.
var (
	archiveImportRunner sync.Mutex
	archiveImportJobs   sync.WaitGroup
)

func startArchiveImportJob(db *vbolt.DB, id int) {
	archiveImportJobs.Add(1)
	go func() {
		defer archiveImportJobs.Done()
		archiveImportRunner.Lock()
		defer archiveImportRunner.Unlock()
		runArchiveImportJob(db, id)
	}()
}

func waitForArchiveImports() {
	archiveImportJobs.Wait()
}

func StopArchiveImports(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		archiveImportJobs.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// InitializeArchiveImports restarts jobs a shutdown interrupted. An import
// writes its records and its done state in one transaction, so a job that
// had not finished left nothing behind and can run again.
func InitializeArchiveImports(db *vbolt.DB) {
	var ids []int
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ArchiveImportBkt, func(id int, job ArchiveImport) bool {
			if job.State.running() {
				ids = append(ids, id)
			}
			return true
		})
	})
	for _, id := range ids {
		setArchiveImportState(db, id, ArchiveImportQueued, nil)
		startArchiveImportJob(db, id)
	}
}

func setArchiveImportState(db *vbolt.DB, id int, state ArchiveImportState, update func(*ArchiveImport)) (job ArchiveImport) {
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		job = getArchiveImport(tx, id)
		if job.Id == 0 {
			return
		}
		job.State = state
		if update != nil {
			update(&job)
		}
		writeArchiveImport(tx, &job)
		vbolt.TxCommit(tx)
	})
	return
}

func failArchiveImport(db *vbolt.DB, id int, err error) {
	setArchiveImportState(db, id, ArchiveImportFailed, func(job *ArchiveImport) { job.Error = err.Error() })
	LogWarn(LogCategoryAPI, "Archive import failed", map[string]interface{}{"importId": id, "error": err.Error()})
}

func runArchiveImportJob(db *vbolt.DB, id int) {
	var job ArchiveImport
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) { job = getArchiveImport(tx, id) })
	if job.State != ArchiveImportQueued {
		return
	}
	req := job.options()
	writes := !req.PreviewOnly && !req.DryRun
	var staging *archiveStaging
	committed := false
	defer func() {
		if r := recover(); r != nil {
			failArchiveImport(db, id, fmt.Errorf("The import stopped unexpectedly: %v", r))
		}
		staging.cleanup()
		if committed {
			discardArchiveUpload(db, id)
		}
	}()
	job = setArchiveImportState(db, id, ArchiveImportStaging, nil)

	archive, closeArchive, err := openUploadedArchive(activeMediaStore(), job.Key, int64(job.DeclaredBytes))
	if err != nil {
		failArchiveImport(db, id, err)
		return
	}
	defer closeArchive()
	setArchiveImportState(db, id, ArchiveImportStaging, func(job *ArchiveImport) {
		job.Records = archiveRecordCount(archive.ImportDataStructure)
		job.Media = len(archive.media)
	})

	if !req.PreviewOnly {
		staging, err = stageArchiveMedia(archive, req, writes, archiveStagingProgress(db, id))
		if err != nil {
			failArchiveImport(db, id, err)
			return
		}
		archive.staged = staging.files
	}
	job = setArchiveImportState(db, id, ArchiveImportImporting, func(job *ArchiveImport) {
		if staging != nil {
			job.Staged = len(staging.files)
		}
	})

	var resp ImportDataResponse
	var done archiveImported
	withTx := vbolt.WithWriteTx
	if req.PreviewOnly {
		withTx = vbolt.WithReadTx
	}
	withTx(db, func(tx *vbolt.Tx) {
		user := GetUser(tx, job.UserId)
		var familyId int
		if familyId, err = ResolveActingFamily(tx, user, job.FamilyId, AccessContribute); err != nil {
			return
		}
		if resp, done, err = importFamilyArchive(tx, user, familyId, archive, req); err != nil || !writes {
			return
		}
		markArchiveImportDone(tx, &job, resp, true)
		vbolt.TxCommit(tx)
		committed = true
	})
	if err != nil {
		failArchiveImport(db, id, err)
		return
	}
	if !writes {
		setArchiveImportState(db, id, ArchiveImportDone, func(j *ArchiveImport) { markArchiveImportDone(nil, j, resp, false) })
		return
	}
	done.queue()
}

// archiveStagingProgress records how many files are staged, at most about
// once a second so the progress writes stay out of other writers' way.
func archiveStagingProgress(db *vbolt.DB, id int) func(staged int) error {
	var last time.Time
	return func(staged int) error {
		if time.Since(last) >= time.Second {
			last = time.Now()
			setArchiveImportState(db, id, ArchiveImportStaging, func(job *ArchiveImport) { job.Staged = staged })
		}
		if archiveStagedHook != nil {
			return archiveStagedHook(staged)
		}
		return nil
	}
}

// archiveStagedHook, when a test sets it, runs after each staged file.
var archiveStagedHook func(staged int) error

// discardArchiveUpload deletes the uploaded zip. A delete that fails is left
// to the media janitor to retry.
func discardArchiveUpload(db *vbolt.DB, id int) {
	var job ArchiveImport
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) { job = getArchiveImport(tx, id) })
	if job.Id == 0 || job.Discarded {
		return
	}
	var deleteErr error
	if store := activeMediaStore(); store != nil {
		deleteErr = store.Delete(context.Background(), job.Key)
	}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		job = getArchiveImport(tx, id)
		job.Discarded = true
		if deleteErr != nil && !errors.Is(deleteErr, ErrMediaNotFound) {
			queueMediaDeletionTx(tx, job.Key)
		}
		writeArchiveImport(tx, &job)
		vbolt.TxCommit(tx)
	})
}

// An upload left unused this long, after it began or after its last
// preview, is deleted by the media janitor.
const abandonedArchiveUploadAge = 24 * time.Hour

// expireArchiveUploads marks abandoned uploads discarded and queues their
// zips for deletion. An upload that never finished also frees its space.
func expireArchiveUploads(db *vbolt.DB, now time.Time) (expired int) {
	cutoff := now.Add(-abandonedArchiveUploadAge)
	abandoned := func(job ArchiveImport) bool {
		return job.Id != 0 && !job.Discarded && !job.State.running() && job.UpdatedAt.Before(cutoff)
	}
	var ids []int
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ArchiveImportBkt, func(id int, job ArchiveImport) bool {
			if abandoned(job) {
				ids = append(ids, id)
			}
			return true
		})
	})
	for _, id := range ids {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			job := getArchiveImport(tx, id)
			if !abandoned(job) {
				return
			}
			if job.State == ArchiveImportUploading {
				job.State = ArchiveImportFailed
				job.Error = "This upload expired before it finished. Please try again."
			}
			job.Discarded = true
			queueMediaDeletionTx(tx, job.Key)
			writeArchiveImport(tx, &job)
			vbolt.TxCommit(tx)
			expired++
		})
	}
	return
}

// markArchiveImportDone records the result, inside the import's own
// transaction when it wrote anything.
func markArchiveImportDone(tx *vbolt.Tx, job *ArchiveImport, resp ImportDataResponse, imported bool) {
	encoded, _ := json.Marshal(resp)
	job.State = ArchiveImportDone
	job.ResultJson = string(encoded)
	job.Imported = imported
	if tx != nil {
		writeArchiveImport(tx, job)
	}
}

// openUploadedArchive reads a zip from the media store. A local store's file
// is read in place; a bucket's is copied to a temporary file first, because
// a zip is read from its end.
func openUploadedArchive(store MediaStore, key string, size int64) (archive familyArchive, closeFn func(), err error) {
	closeFn = func() {}
	if store == nil {
		return archive, closeFn, ErrArchiveImportUnavailable
	}
	var file *os.File
	if local, ok := store.(*localMediaStore); ok {
		p, pathErr := local.localPath(key)
		if pathErr != nil {
			return archive, closeFn, pathErr
		}
		if file, err = os.Open(p); err != nil {
			return archive, closeFn, errors.New("The uploaded archive could not be found. Please upload it again.")
		}
		closeFn = func() { file.Close() }
	} else {
		body, openErr := store.Open(context.Background(), key)
		if openErr != nil {
			return archive, closeFn, errors.New("The uploaded archive could not be found. Please upload it again.")
		}
		defer body.Close()
		if file, err = os.CreateTemp("", "family-import-*.zip"); err != nil {
			return archive, closeFn, err
		}
		closeFn = func() { file.Close(); os.Remove(file.Name()) }
		if _, err = io.Copy(file, body); err != nil {
			closeFn()
			return archive, func() {}, err
		}
	}
	zr, zipErr := zip.NewReader(file, size)
	if zipErr != nil {
		closeFn()
		return archive, func() {}, errors.New("That file is not a valid Family Record archive. Check that you picked the right file.")
	}
	if archive, err = readArchiveZip(zr); err != nil {
		closeFn()
		return archive, func() {}, err
	}
	return archive, closeFn, nil
}

func archiveRecordCount(r ImportDataStructure) int {
	return len(r.People) + len(r.Relations) + len(r.Heights) + len(r.Weights) + len(r.Milestones) +
		len(r.Tags) + len(r.Photos) + len(r.Videos) + len(r.Activities) + len(r.Books) +
		len(r.InterviewQuestions) + len(r.Interviews)
}

// archiveImportGate admits a local upload only for an open import of exactly
// the signed size, and holds disk for it while the body streams.
func archiveImportGate(db *vbolt.DB) localUploadGate {
	return func(key string, size int64) (func(), *AppError) {
		var job ArchiveImport
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) { job = archiveImportByKey(tx, key) })
		if job.Id == 0 || job.State != ArchiveImportUploading || time.Now().After(job.ExpiresAt) {
			return nil, NewAppError(ErrCodeConflict, "This upload is no longer open. Please start it again.")
		}
		if int64(job.DeclaredBytes) != size {
			return nil, NewAppError(ErrCodeForbidden, "This upload link is invalid or has expired.")
		}
		return diskBudget.reserve(cfg.MediaDir, size, cfg.MinFreeDiskBytes)
	}
}

// mediaUploadGate routes each signed local upload to the session that owns
// its key.
func mediaUploadGate(db *vbolt.DB) localUploadGate {
	imports, videos := archiveImportGate(db), videoUploadGate(db)
	return func(key string, size int64) (func(), *AppError) {
		if strings.HasPrefix(key, "imports/") {
			return imports(key, size)
		}
		return videos(key, size)
	}
}

func RegisterArchiveImportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, BeginArchiveImport)
	vbeam.RegisterProc(app, CompleteArchiveImport)
	vbeam.RegisterProc(app, GetArchiveImportStatus)
}
