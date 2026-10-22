package backend

import (
	"context"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

const playbackContentType = "video/mp4"

type VideoAttempt struct {
	Time       time.Time `json:"time"`
	ImageId    int       `json:"imageId"`
	Outcome    string    `json:"outcome"`
	DurationMs int       `json:"durationMs"`
	Reason     string    `json:"reason"`
}

type videoWorker struct {
	*backlogWorker[int]
	db      *vbolt.DB
	tools   mediaTools
	scratch string
	ctx     context.Context
	cancel  context.CancelFunc

	statsMu   sync.Mutex
	processed int
	rejected  int
	failed    int
	lastError string
	lastErrAt time.Time
	recent    []VideoAttempt
}

var globalVideoWorker *videoWorker

var videoBeforePublish func(imageId int)

var videoUnavailableReason = "Video processing has not started."

func InitializeVideoWorker(db *vbolt.DB) {
	if globalVideoWorker != nil {
		return
	}
	if !cfg.EnableVideo {
		videoUnavailableReason = "Video is turned off in this build."
		videoProcessingAvailable = func() bool { return false }
		return
	}
	if activeMediaStore() == nil {
		videoUnavailableReason = "Media storage is not available."
		if mediaStoreProblem != "" {
			videoUnavailableReason += " " + mediaStoreProblem
		}
		videoProcessingAvailable = func() bool { return false }
		return
	}
	tools, err := findMediaTools()
	if err != nil {
		videoUnavailableReason = "ffmpeg and ffprobe were not found on this server."
		videoProcessingAvailable = func() bool { return false }
		LogWarn(LogCategoryWorker, "Video processing disabled: ffmpeg not found", map[string]interface{}{"error": err.Error()})
		return
	}

	scratch := filepath.Join(cfg.MediaDir, ".scratch")
	os.RemoveAll(scratch)
	if err := os.MkdirAll(scratch, 0750); err != nil {
		videoUnavailableReason = "The video scratch directory could not be created."
		videoProcessingAvailable = func() bool { return false }
		LogErrorSimple(LogCategoryWorker, "Video processing disabled: no scratch directory", map[string]interface{}{"error": err.Error()})
		return
	}

	w := newVideoWorker(db, tools, scratch)
	globalVideoWorker = w
	videoUnavailableReason = ""
	videoProcessingAvailable = func() bool { return w.isRunning() }
	w.run(func() []int { return videosNeedingProcessing(db) })
	LogInfo(LogCategoryWorker, "Video worker started", map[string]interface{}{"ffmpeg": tools.ffmpeg})
}

func newVideoWorker(db *vbolt.DB, tools mediaTools, scratch string) *videoWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &videoWorker{db: db, tools: tools, scratch: scratch, ctx: ctx, cancel: cancel}
	w.backlogWorker = newBacklogWorker("Video worker", w.process)
	return w
}

func QueueVideoProcessing(imageId int) {
	if globalVideoWorker != nil {
		globalVideoWorker.add(imageId)
	}
}

func StopVideoWorker(ctx context.Context) bool {
	if globalVideoWorker == nil {
		return true
	}
	globalVideoWorker.cancel()
	return globalVideoWorker.stopWait(ctx)
}

func videosNeedingProcessing(db *vbolt.DB) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
			if image.IsVideo() && image.Status == 1 {
				ids = append(ids, image.Id)
			}
			return true
		})
	})
	if len(ids) > 0 {
		LogInfo(LogCategoryWorker, "Queued videos left processing", map[string]interface{}{"count": len(ids)})
	}
	return
}

func (w *videoWorker) record(imageId int, start time.Time, outcome, reason string) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	switch outcome {
	case "ready":
		w.processed++
	case "rejected":
		w.rejected++
	case "failed":
		w.failed++
		w.lastError, w.lastErrAt = reason, time.Now()
	}
	w.recent = append(w.recent, VideoAttempt{
		Time: time.Now(), ImageId: imageId, Outcome: outcome,
		DurationMs: int(time.Since(start).Milliseconds()), Reason: reason,
	})
	if len(w.recent) > maxRecentPhotoAttempts {
		w.recent = w.recent[len(w.recent)-maxRecentPhotoAttempts:]
	}
}

// mediaIdOf recovers the random id shared by a video's keys and poster files.
func mediaIdOf(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

type videoOutputs struct {
	playbackKey string
	posterPath  string
}

func (o videoOutputs) discard(store MediaStore) {
	if o.playbackKey != "" {
		store.Delete(context.Background(), o.playbackKey)
	}
	if o.posterPath != "" {
		deletePhotoFiles(Image{FilePath: o.posterPath})
	}
}

func (w *videoWorker) process(imageId int) error {
	start := time.Now()
	var image Image
	vbolt.WithReadTx(w.db, func(tx *vbolt.Tx) { image = GetImageById(tx, imageId) })
	if image.Id == 0 || !image.IsVideo() || image.Status != 1 {
		return nil
	}
	store := activeMediaStore()
	if store == nil {
		return errRetryLater
	}
	mediaId := mediaIdOf(image.OriginalKey)
	if mediaId == "" {
		w.fail(image, start, "The uploaded file is missing.", false)
		return nil
	}

	release, diskErr := diskBudget.reserve(cfg.MediaDir, int64(videoReservationBytes(image.FileSize)), cfg.MinFreeDiskBytes)
	if diskErr != nil {
		LogWarn(LogCategoryWorker, "Video waiting for disk space", map[string]interface{}{"imageId": imageId})
		return errRetryLater
	}
	defer release()

	workDir, err := os.MkdirTemp(w.scratch, mediaId+"-")
	if err != nil {
		return errRetryLater
	}
	defer os.RemoveAll(workDir)

	if strings.HasPrefix(image.OriginalKey, "uploads/") {
		finalKey := "videos/" + mediaId + "/original" + path.Ext(image.OriginalKey)
		if err := moveMediaObject(w.ctx, store, image.OriginalKey, finalKey, image.MimeType); err != nil {
			if errors.Is(err, ErrMediaNotFound) {
				w.fail(image, start, "The uploaded file is missing.", false)
				return nil
			}
			log.Printf("[VIDEO] Could not move video %d into place: %v", imageId, err)
			return errRetryLater
		}
		gone := false
		vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
			current := GetImageById(tx, imageId)
			if current.Id == 0 {
				gone = true
				return
			}
			current.OriginalKey = finalKey
			vbolt.Write(tx, ImagesBkt, current.Id, &current)
			vbolt.TxCommit(tx)
		})
		if gone {
			store.Delete(context.Background(), finalKey)
			return nil
		}
		image.OriginalKey = finalKey
	}

	source, err := localCopyOf(w.ctx, store, image.OriginalKey, workDir)
	if err != nil {
		if errors.Is(err, ErrMediaNotFound) {
			w.fail(image, start, "The uploaded file is missing.", false)
			return nil
		}
		return errRetryLater
	}

	probe, err := w.tools.probe(w.ctx, source)
	if w.ctx.Err() != nil {
		return nil
	}
	if err != nil {
		w.reject(image, start, "This file couldn't be read as a video.", err)
		return nil
	}
	if reason := probe.rejection(); reason != "" {
		w.reject(image, start, reason, nil)
		return nil
	}

	remuxPath := filepath.Join(workDir, "playback.mp4")
	if err := w.tools.remux(w.ctx, source, remuxPath, probe); err != nil {
		if w.ctx.Err() != nil {
			return nil
		}
		w.fail(image, start, "This video couldn't be prepared for playback.", true, err)
		return nil
	}
	posterFile := filepath.Join(workDir, "poster.jpg")
	if err := w.tools.poster(w.ctx, source, posterFile, probe); err != nil {
		if w.ctx.Err() != nil {
			return nil
		}
		w.fail(image, start, "A preview image couldn't be made for this video.", true, err)
		return nil
	}

	posterData, err := os.ReadFile(posterFile)
	if err != nil {
		return errRetryLater
	}
	variants, _, _, err := ProcessAndSaveMultipleSizes(posterData, "image/jpeg")
	if err != nil || len(variants) == 0 {
		w.fail(image, start, "A preview image couldn't be made for this video.", true, err)
		return nil
	}
	outputs := videoOutputs{posterPath: "photos/" + mediaId + ".jpg"}
	posterBytes, err := writeImageVariants(outputs.posterPath, variants)
	if err == nil {
		err = os.WriteFile(originalPathIn(cfg.StaticDir, outputs.posterPath), posterData, 0644)
		posterBytes += int64(len(posterData))
	}
	if err != nil {
		outputs.discard(store)
		return errRetryLater
	}

	remuxInfo, err := os.Stat(remuxPath)
	if err != nil {
		outputs.discard(store)
		return errRetryLater
	}
	outputs.playbackKey = "videos/" + mediaId + "/playback.mp4"
	if err := putFile(w.ctx, store, outputs.playbackKey, remuxPath, playbackContentType); err != nil {
		outputs.discard(store)
		if w.ctx.Err() != nil {
			return nil
		}
		log.Printf("[VIDEO] Could not store the playback file for video %d: %v", imageId, err)
		return errRetryLater
	}

	if videoBeforePublish != nil {
		videoBeforePublish(imageId)
	}
	published := false
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, imageId)
		if current.Id == 0 || current.Status != 1 {
			return
		}
		if probe.HasCreatedAt && current.PhotoDate.Equal(image.PhotoDate) {
			if current.Title == generateDefaultVideoTitle(current.OriginalFilename, current.PhotoDate) {
				current.Title = generateDefaultVideoTitle(current.OriginalFilename, probe.CreatedAt)
			}
			current.PhotoDate = probe.CreatedAt
		}
		current.Status = 0
		current.ProcessingError = ""
		current.FilePath = outputs.posterPath
		current.PlaybackKey = outputs.playbackKey
		current.Width, current.Height = probe.Width, probe.Height
		current.DurationMs = int(probe.Duration / time.Millisecond)
		current.VideoCodec = probe.VideoMime
		current.AudioCodec = probe.AudioMime
		current.DerivedBytes = int(remuxInfo.Size() + posterBytes)
		vbolt.Write(tx, ImagesBkt, current.Id, &current)
		ReindexPhotoDates(tx, current.Id)
		vbolt.TxCommit(tx)
		published = true
	})
	if !published {
		log.Printf("[VIDEO] Video %d was deleted while processing; discarding its outputs", imageId)
		outputs.discard(store)
		return nil
	}

	QueuePhotoEmbedding(imageId)
	w.record(imageId, start, "ready", "")
	log.Printf("[VIDEO] Video %d ready in %v (%s, %s)", imageId, time.Since(start), probe.VideoMime, probe.Duration)
	return nil
}

// reject drops a file that will never play: its original is deleted and its
// storage released, and the row keeps the reason to show the uploader.
func (w *videoWorker) reject(image Image, start time.Time, reason string, cause error) {
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		if current.Id == 0 {
			queueMediaDeletionTx(tx, image.OriginalKey)
			vbolt.TxCommit(tx)
			return
		}
		queueMediaDeletionTx(tx, current.OriginalKey)
		current.OriginalKey = ""
		current.Status = 2
		current.ProcessingError = reason
		current.DerivedBytes = 0
		vbolt.Write(tx, ImagesBkt, current.Id, &current)
		vbolt.TxCommit(tx)
	})
	kickMediaJanitor()
	detail := reason
	if cause != nil {
		detail += " (" + cause.Error() + ")"
	}
	w.record(image.Id, start, "rejected", detail)
	LogInfo(LogCategoryWorker, "Video rejected", map[string]interface{}{"imageId": image.Id, "reason": detail})
}

// fail marks a video that could not be processed but keeps its original, so
// an admin can retry it once the cause is fixed.
func (w *videoWorker) fail(image Image, start time.Time, reason string, keepOriginal bool, causes ...error) {
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		if current.Id == 0 {
			return
		}
		if !keepOriginal {
			current.OriginalKey = ""
		}
		current.Status = 2
		current.ProcessingError = reason
		current.DerivedBytes = 0
		vbolt.Write(tx, ImagesBkt, current.Id, &current)
		vbolt.TxCommit(tx)
	})
	detail := reason
	for _, cause := range causes {
		if cause != nil {
			detail += " (" + cause.Error() + ")"
		}
	}
	w.record(image.Id, start, "failed", detail)
	LogErrorSimple(LogCategoryWorker, "Video processing failed", map[string]interface{}{"imageId": image.Id, "reason": detail})
}

type mediaMover interface {
	Move(ctx context.Context, src, dst string) error
}

func moveMediaObject(ctx context.Context, store MediaStore, src, dst, contentType string) error {
	if mover, ok := store.(mediaMover); ok {
		return mover.Move(ctx, src, dst)
	}
	info, err := store.Stat(ctx, src)
	if err != nil {
		return err
	}
	r, err := store.Open(ctx, src)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := store.Put(ctx, dst, r, info.Size, contentType); err != nil {
		return err
	}
	return store.Delete(ctx, src)
}

type localPather interface {
	localPath(key string) (string, error)
}

// localCopyOf gives ffmpeg a file path: the object itself for local storage,
// otherwise a download into dir.
func localCopyOf(ctx context.Context, store MediaStore, key, dir string) (string, error) {
	if lp, ok := store.(localPather); ok {
		p, err := lp.localPath(key)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return "", ErrMediaNotFound
		}
		return p, nil
	}
	r, err := store.Open(ctx, key)
	if err != nil {
		return "", err
	}
	defer r.Close()
	dst := filepath.Join(dir, "source"+path.Ext(key))
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return dst, f.Close()
}

func putFile(ctx context.Context, store MediaStore, key, file, contentType string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return store.Put(ctx, key, f, info.Size(), contentType)
}

type VideoProcessingStats struct {
	Available      bool           `json:"available"`
	Reason         string         `json:"reason"`
	IsRunning      bool           `json:"isRunning"`
	QueueLength    int            `json:"queueLength"`
	Processed      int            `json:"processed"`
	Rejected       int            `json:"rejected"`
	Failed         int            `json:"failed"`
	LastError      string         `json:"lastError"`
	LastErrorAt    time.Time      `json:"lastErrorAt"`
	RecentAttempts []VideoAttempt `json:"recentAttempts"`
}

func GetVideoWorkerStats() VideoProcessingStats {
	stats := VideoProcessingStats{
		Available:      videoProcessingAvailable(),
		Reason:         videoUnavailableReason,
		RecentAttempts: []VideoAttempt{},
	}
	w := globalVideoWorker
	if w == nil {
		return stats
	}
	stats.IsRunning = w.isRunning()
	stats.QueueLength = w.length()
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	stats.Processed, stats.Rejected, stats.Failed = w.processed, w.rejected, w.failed
	stats.LastError, stats.LastErrorAt = w.lastError, w.lastErrAt
	for i := len(w.recent) - 1; i >= 0; i-- {
		stats.RecentAttempts = append(stats.RecentAttempts, w.recent[i])
	}
	return stats
}

func GetVideoProcessingStats(ctx *vbeam.Context, req Empty) (resp VideoProcessingStats, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	resp = GetVideoWorkerStats()
	return
}

type RetryFailedVideosResponse struct {
	Queued int `json:"queued"`
}

func RetryFailedVideos(ctx *vbeam.Context, req Empty) (resp RetryFailedVideosResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	if globalVideoWorker == nil || !globalVideoWorker.isRunning() {
		err = fmt.Errorf("Video processing is not running: %s", videoUnavailableReason)
		return
	}
	vbeam.UseWriteTx(ctx)
	var ids []int
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(_ int, image Image) bool {
		if image.IsVideo() && image.Status == 2 && image.OriginalKey != "" {
			ids = append(ids, image.Id)
		}
		return true
	})
	for _, id := range ids {
		image := GetImageById(ctx.Tx, id)
		image.Status = 1
		image.ProcessingError = ""
		image.DerivedBytes = image.FileSize + videoPosterAllowance
		vbolt.Write(ctx.Tx, ImagesBkt, id, &image)
	}
	vbolt.TxCommit(ctx.Tx)
	globalVideoWorker.add(ids...)
	resp.Queued = len(ids)
	return
}
