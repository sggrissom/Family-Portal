package backend

import (
	"context"
	"errors"
	"family/cfg"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

type VideoUploadState int

const (
	VideoUploadPending VideoUploadState = iota
	VideoUploadCompleted
	VideoUploadAbandoned
)

type VideoUpload struct {
	Id               int
	FamilyId         int
	UserId           int
	Key              string
	ContentType      string
	OriginalFilename string
	Title            string
	Description      string
	PersonIds        []int
	DeclaredBytes    int
	FallbackDate     time.Time
	State            VideoUploadState
	ImageId          int
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

func PackVideoUpload(self *VideoUpload, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.Key, buf)
	vpack.String(&self.ContentType, buf)
	vpack.String(&self.OriginalFilename, buf)
	vpack.String(&self.Title, buf)
	vpack.String(&self.Description, buf)
	vpack.Slice(&self.PersonIds, vpack.Int, buf)
	vpack.Int(&self.DeclaredBytes, buf)
	vpack.Time(&self.FallbackDate, buf)
	vpack.IntEnum(&self.State, buf)
	vpack.Int(&self.ImageId, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.ExpiresAt, buf)
}

var VideoUploadBkt = vbolt.Bucket(&cfg.Info, "video_uploads", vpack.FInt, PackVideoUpload)
var VideoUploadByFamilyIndex = vbolt.Index(&cfg.Info, "video_upload_by_family", vpack.FInt, vpack.FInt)
var VideoUploadByKeyIndex = vbolt.Index(&cfg.Info, "video_upload_by_key", vpack.String, vpack.FInt)

// Room for the poster and its resized variants on top of the remux.
const videoPosterAllowance = 8 << 20

// An upload reserves its original and a same-sized remux; the reservation is
// held until completion turns it into the Image row's own accounting.
func videoReservationBytes(size int) int {
	return 2*size + videoPosterAllowance
}

func (u VideoUpload) reservedBytes() int64 {
	if u.State != VideoUploadPending {
		return 0
	}
	return int64(videoReservationBytes(u.DeclaredBytes))
}

func writeVideoUpload(tx *vbolt.Tx, upload *VideoUpload) {
	vbolt.Write(tx, VideoUploadBkt, upload.Id, upload)
	vbolt.SetTargetSingleTerm(tx, VideoUploadByFamilyIndex, upload.Id, upload.FamilyId)
	vbolt.SetTargetSingleTerm(tx, VideoUploadByKeyIndex, upload.Id, upload.Key)
}

func deleteVideoUploadTx(tx *vbolt.Tx, uploadId int) {
	vbolt.Delete(tx, VideoUploadBkt, uploadId)
	vbolt.SetTargetSingleTerm(tx, VideoUploadByFamilyIndex, uploadId, -1)
	vbolt.SetTargetSingleTerm(tx, VideoUploadByKeyIndex, uploadId, "")
}

func getVideoUpload(tx *vbolt.Tx, id int) (upload VideoUpload) {
	vbolt.Read(tx, VideoUploadBkt, id, &upload)
	return
}

func videoUploadByKey(tx *vbolt.Tx, key string) (upload VideoUpload) {
	var ids []int
	vbolt.ReadTermTargets(tx, VideoUploadByKeyIndex, key, &ids, vbolt.Window{})
	if len(ids) > 0 {
		upload = getVideoUpload(tx, ids[0])
	}
	return
}

func familyVideoUploads(tx *vbolt.Tx, familyId int) (uploads []VideoUpload) {
	var ids []int
	vbolt.ReadTermTargets(tx, VideoUploadByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, VideoUploadBkt, ids, &uploads)
	return
}

var videoExtensions = map[string]string{
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/x-m4v":     ".m4v",
}

func videoContentType(contentType, filename string) (string, bool) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if _, ok := videoExtensions[contentType]; ok {
		return contentType, true
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp4":
		return "video/mp4", true
	case ".mov":
		return "video/quicktime", true
	case ".m4v":
		return "video/x-m4v", true
	}
	return "", false
}

var ErrVideoDisabled = errors.New("Video uploads are not turned on for this family.")
var ErrVideoUnavailable = errors.New("Video uploads are not available on this server right now.")

// videoUnavailable explains why a family cannot add video, or returns nil.
func videoUnavailable(tx *vbolt.Tx, familyId int) error {
	if !cfg.EnableVideo || activeMediaStore() == nil || !videoProcessingAvailable() {
		return ErrVideoUnavailable
	}
	if !GetFamily(tx, familyId).VideoEnabled {
		return ErrVideoDisabled
	}
	return nil
}

// Replaced by the video worker once it can find ffmpeg.
var videoProcessingAvailable = func() bool { return true }

func generateDefaultVideoTitle(originalFilename string, date time.Time) string {
	if !date.IsZero() {
		return fmt.Sprintf("Video from %s", date.Format("Jan 2, 2006"))
	}
	return strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
}

type GetVideoSupportRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type GetVideoSupportResponse struct {
	Enabled            bool     `json:"enabled"`
	Reason             string   `json:"reason"`
	MaxBytes           int      `json:"maxBytes"`
	MaxDurationSeconds int      `json:"maxDurationSeconds"`
	ContentTypes       []string `json:"contentTypes"`
}

func GetVideoSupport(ctx *vbeam.Context, req GetVideoSupportRequest) (resp GetVideoSupportResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	resp.MaxBytes = maxVideoBytes
	resp.MaxDurationSeconds = int(maxVideoDuration / time.Second)
	resp.ContentTypes = []string{"video/mp4", "video/quicktime", "video/x-m4v"}

	familyId, accessErr := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if accessErr != nil {
		resp.Reason = "You cannot add videos to that family."
		return
	}
	if reason := videoUnavailable(ctx.Tx, familyId); reason != nil {
		resp.Reason = reason.Error()
		return
	}
	resp.Enabled = true
	return
}

type BeginVideoUploadRequest struct {
	FamilyId    int    `json:"familyId,omitempty"`
	PersonIds   []int  `json:"personIds"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// FallbackDate is the uploader's local day, used when the file carries no
	// creation date.
	FallbackDate string `json:"fallbackDate"`
}

type BeginVideoUploadResponse struct {
	UploadId int                `json:"uploadId"`
	Upload   UploadInstructions `json:"upload"`
}

func BeginVideoUpload(ctx *vbeam.Context, req BeginVideoUploadRequest) (resp BeginVideoUploadResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if req.Size <= 0 {
		err = errors.New("Choose a video to upload.")
		return
	}
	if req.Size > maxVideoBytes {
		err = fmt.Errorf("Videos can be at most %s.", formatBytes(maxVideoBytes))
		return
	}
	contentType, ok := videoContentType(req.ContentType, req.Filename)
	if !ok {
		err = errors.New("That file type isn't supported. Upload an MP4 or MOV video.")
		return
	}
	fallback, parseErr := time.Parse(dateOnlyLayout, req.FallbackDate)
	if parseErr != nil {
		fallback = wallClockUTC(time.Now().UTC())
	}

	vbeam.UseWriteTx(ctx)

	familyId, accessErr := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if accessErr != nil {
		err = errors.New("You cannot add videos to that family.")
		return
	}
	if err = videoUnavailable(ctx.Tx, familyId); err != nil {
		return
	}
	for _, personId := range req.PersonIds {
		person := GetPersonById(ctx.Tx, personId)
		if person.Id == 0 || !CanFamilyAccess(ctx.Tx, familyId, person.FamilyId, AccessContribute) {
			err = errors.New("One of the tagged people is not in your family.")
			return
		}
	}

	reservation := int64(videoReservationBytes(req.Size))
	if quotaErr := CheckFamilyStorageQuota(ctx.Tx, familyId, reservation, cfg.FamilyStorageQuotaBytes); quotaErr != nil {
		err = errors.New(quotaErr.Message)
		return
	}
	store := activeMediaStore()
	if _, local := store.(*localMediaStore); local {
		if diskErr := CheckDiskHeadroom(cfg.MediaDir, reservation, cfg.MinFreeDiskBytes); diskErr != nil {
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
	upload := VideoUpload{
		Id:               vbolt.NextIntId(ctx.Tx, VideoUploadBkt),
		FamilyId:         familyId,
		UserId:           user.Id,
		Key:              "uploads/" + mediaId + "/source" + videoExtensions[contentType],
		ContentType:      contentType,
		OriginalFilename: filepath.Base(req.Filename),
		Title:            strings.TrimSpace(req.Title),
		Description:      strings.TrimSpace(req.Description),
		PersonIds:        req.PersonIds,
		DeclaredBytes:    req.Size,
		FallbackDate:     fallback,
		State:            VideoUploadPending,
		CreatedAt:        now,
		ExpiresAt:        now.Add(mediaUploadTTL),
	}

	inst, urlErr := store.UploadURL(upload.Key, int64(upload.DeclaredBytes), contentType, mediaUploadTTL)
	if urlErr != nil {
		err = urlErr
		return
	}
	writeVideoUpload(ctx.Tx, &upload)
	resp.UploadId = upload.Id
	resp.Upload = inst
	vbolt.TxCommit(ctx.Tx)

	LogInfo(LogCategoryPhoto, "Video upload started", map[string]interface{}{
		"userId": user.Id, "familyId": familyId, "uploadId": upload.Id, "bytes": req.Size,
	})
	return
}

type CompleteVideoUploadRequest struct {
	UploadId int `json:"uploadId"`
}

type CompleteVideoUploadResponse struct {
	Image Image `json:"image"`
}

// The PUT is bounded by its URL's expiry; completion gets a little longer so
// an upload that finished at the last moment can still be accepted.
const videoCompletionGrace = 15 * time.Minute

type completion struct {
	image Image
	err   error
}

// completedVideoUpload settles a completion request that has nothing left to
// do: a retry of one already accepted, or one that can no longer be accepted.
func completedVideoUpload(tx *vbolt.Tx, upload VideoUpload) (bool, completion) {
	switch {
	case upload.State == VideoUploadCompleted:
		image := GetImageById(tx, upload.ImageId)
		if image.Id == 0 {
			return true, completion{err: ErrVideoUploadNotFound}
		}
		image.TagIds = GetPhotoTagIds(tx, image.Id)
		return true, completion{image: image}
	case upload.State == VideoUploadAbandoned, time.Now().After(upload.ExpiresAt.Add(videoCompletionGrace)):
		return true, completion{err: ErrVideoUploadExpired}
	}
	return false, completion{}
}

var ErrVideoUploadNotFound = errors.New("That upload could not be found.")
var ErrVideoUploadExpired = errors.New("This upload expired before it finished. Please try again.")
var ErrVideoUploadMissing = errors.New("The video hasn't finished uploading yet.")

func CompleteVideoUpload(ctx *vbeam.Context, req CompleteVideoUploadRequest) (resp CompleteVideoUploadResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	upload := getVideoUpload(ctx.Tx, req.UploadId)
	if upload.Id == 0 || upload.UserId != user.Id {
		err = ErrVideoUploadNotFound
		return
	}
	if done, doneErr := completedVideoUpload(ctx.Tx, upload); done {
		resp.Image, err = doneErr.image, doneErr.err
		return
	}

	store := activeMediaStore()
	if store == nil {
		err = ErrVideoUnavailable
		return
	}
	obj, statErr := store.Stat(context.Background(), upload.Key)
	if errors.Is(statErr, ErrMediaNotFound) {
		err = ErrVideoUploadMissing
		return
	}
	if statErr != nil {
		LogErrorSimple(LogCategoryPhoto, "Could not check an uploaded video", map[string]interface{}{
			"uploadId": upload.Id, "error": statErr.Error(),
		})
		err = ErrVideoUnavailable
		return
	}

	vbeam.UseWriteTx(ctx)
	upload = getVideoUpload(ctx.Tx, req.UploadId)
	if done, doneErr := completedVideoUpload(ctx.Tx, upload); done {
		resp.Image, err = doneErr.image, doneErr.err
		return
	}

	if obj.Size != int64(upload.DeclaredBytes) {
		upload.State = VideoUploadAbandoned
		writeVideoUpload(ctx.Tx, &upload)
		vbolt.TxCommit(ctx.Tx)
		kickMediaJanitor()
		err = fmt.Errorf("The upload arrived with %s instead of the expected %s. Please try again.",
			formatBytes(obj.Size), formatBytes(int64(upload.DeclaredBytes)))
		return
	}

	var people []Person
	for _, personId := range upload.PersonIds {
		person := GetPersonById(ctx.Tx, personId)
		if person.Id != 0 && CanFamilyAccess(ctx.Tx, upload.FamilyId, person.FamilyId, AccessContribute) {
			people = append(people, person)
		}
	}

	title := upload.Title
	if title == "" {
		title = generateDefaultVideoTitle(upload.OriginalFilename, upload.FallbackDate)
	}
	image := Image{
		Id:               vbolt.NextIntId(ctx.Tx, ImagesBkt),
		FamilyId:         upload.FamilyId,
		OwnerUserId:      user.Id,
		OriginalFilename: upload.OriginalFilename,
		MimeType:         upload.ContentType,
		FileSize:         upload.DeclaredBytes,
		Title:            title,
		Description:      upload.Description,
		PhotoDate:        upload.FallbackDate,
		CreatedAt:        time.Now(),
		Status:           1,
		MediaKind:        MediaVideo,
		OriginalKey:      upload.Key,
		DerivedBytes:     upload.DeclaredBytes + videoPosterAllowance,
	}
	vbolt.Write(ctx.Tx, ImagesBkt, image.Id, &image)
	vbolt.SetTargetSingleTerm(ctx.Tx, ImageByFamilyIndex, image.Id, image.FamilyId)
	ReindexPhotoDates(ctx.Tx, image.Id)
	for _, person := range people {
		AddPersonToPhoto(ctx.Tx, image.Id, person.Id, image.FamilyId)
	}

	upload.State = VideoUploadCompleted
	upload.ImageId = image.Id
	writeVideoUpload(ctx.Tx, &upload)

	resp.Image = image
	resp.Image.TagIds = []int{}
	vbolt.TxCommit(ctx.Tx)

	QueueVideoProcessing(image.Id)
	LogInfo(LogCategoryPhoto, "Video upload completed", map[string]interface{}{
		"userId": user.Id, "uploadId": upload.Id, "imageId": image.Id, "bytes": obj.Size,
	})
	return
}

type CancelVideoUploadRequest struct {
	UploadId int `json:"uploadId"`
}

func CancelVideoUpload(ctx *vbeam.Context, req CancelVideoUploadRequest) (resp Empty, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	upload := getVideoUpload(ctx.Tx, req.UploadId)
	if upload.Id == 0 || upload.UserId != user.Id {
		err = ErrVideoUploadNotFound
		return
	}
	if upload.State != VideoUploadPending {
		return
	}
	upload.State = VideoUploadAbandoned
	writeVideoUpload(ctx.Tx, &upload)
	vbolt.TxCommit(ctx.Tx)
	kickMediaJanitor()
	return
}

// videoUploadGate admits a local upload only for a pending, unexpired session
// of exactly the signed size, and holds disk for it while the body streams.
func videoUploadGate(db *vbolt.DB) localUploadGate {
	return func(key string, size int64) (func(), *AppError) {
		var upload VideoUpload
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			upload = videoUploadByKey(tx, key)
		})
		if upload.Id == 0 || upload.State != VideoUploadPending || time.Now().After(upload.ExpiresAt) {
			return nil, NewAppError(ErrCodeConflict, "This upload is no longer open. Please start it again.")
		}
		if int64(upload.DeclaredBytes) != size {
			return nil, NewAppError(ErrCodeForbidden, "This upload link is invalid or has expired.")
		}
		return diskBudget.reserve(cfg.MediaDir, size, cfg.MinFreeDiskBytes)
	}
}

type SetFamilyVideoEnabledRequest struct {
	FamilyId int  `json:"familyId"`
	Enabled  bool `json:"enabled"`
}

func SetFamilyVideoEnabled(ctx *vbeam.Context, req SetFamilyVideoEnabledRequest) (resp Empty, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	vbeam.UseWriteTx(ctx)
	family := GetFamily(ctx.Tx, req.FamilyId)
	if family.Id == 0 {
		err = errors.New("Family not found")
		return
	}
	family.VideoEnabled = req.Enabled
	vbolt.Write(ctx.Tx, FamiliesBkt, family.Id, &family)
	vbolt.TxCommit(ctx.Tx)
	LogInfo(LogCategoryAdmin, "Changed video availability for a family", map[string]interface{}{
		"familyId": family.Id, "enabled": req.Enabled,
	})
	return
}

func RegisterVideoUploadMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetVideoSupport)
	vbeam.RegisterProc(app, BeginVideoUpload)
	vbeam.RegisterProc(app, CompleteVideoUpload)
	vbeam.RegisterProc(app, CancelVideoUpload)
	vbeam.RegisterProc(app, SetFamilyVideoEnabled)
}

func QueueVideoProcessing(imageId int) {}
