package backend

import (
	"bytes"
	"context"
	"errors"
	"family/cfg"
	"family/vision"
	"fmt"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const (
	videoFramesVersion = 1
	frameThumbWidth    = 320
	framesTimeout      = 10 * time.Minute
	framesScratchBytes = 64 << 20
)

type VideoFrame struct {
	OffsetMs int
	Vector   []float32
}

type VideoFrames struct {
	ImageId    int
	FamilyId   int
	Model      string
	Version    int
	Frames     []VideoFrame
	ThumbBytes int
}

func PackVideoFrame(self *VideoFrame, buf *vpack.Buffer) {
	vpack.Int(&self.OffsetMs, buf)
	packFloat32Slice(&self.Vector, buf)
}

func PackVideoFrames(self *VideoFrames, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.ImageId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Model, buf)
	vpack.Int(&self.Version, buf)
	vpack.Slice(&self.Frames, PackVideoFrame, buf)
	vpack.Int(&self.ThumbBytes, buf)
}

var VideoFramesBkt = vbolt.Bucket(&cfg.Info, "video_frames", vpack.FInt, PackVideoFrames)
var VideoFramesByFamilyIndex = vbolt.Index(&cfg.Info, "video_frames_by_family", vpack.FInt, vpack.FInt)

func GetVideoFrames(tx *vbolt.Tx, imageId int) (f VideoFrames, ok bool) {
	ok = vbolt.Read(tx, VideoFramesBkt, imageId, &f)
	return
}

func videoFramesOutdated(f VideoFrames, found bool) bool {
	return !found || f.Model != clipModelId || f.Version < videoFramesVersion
}

func videoNeedsFrames(tx *vbolt.Tx, image Image) bool {
	if !image.IsVideo() || image.Status != 0 || image.PlaybackKey == "" {
		return false
	}
	f, found := GetVideoFrames(tx, image.Id)
	return videoFramesOutdated(f, found)
}

func writeVideoFramesTx(tx *vbolt.Tx, f *VideoFrames) {
	vbolt.Write(tx, VideoFramesBkt, f.ImageId, f)
	vbolt.SetTargetSingleTerm(tx, VideoFramesByFamilyIndex, f.ImageId, f.FamilyId)
	invalidateFamilyFrames(f.FamilyId)
}

func deleteVideoFramesTx(tx *vbolt.Tx, imageId int) {
	if f, ok := GetVideoFrames(tx, imageId); ok {
		invalidateFamilyFrames(f.FamilyId)
	}
	vbolt.Delete(tx, VideoFramesBkt, imageId)
	vbolt.DeleteTargetTerms(tx, VideoFramesByFamilyIndex, imageId)
}

// videoFramesDir holds a video's frame thumbnails, named by offset. It sits
// in a dot directory so media migration leaves it alone, like .scratch.
func videoFramesDir(image Image) string {
	id := strings.TrimSuffix(filepath.Base(image.FilePath), filepath.Ext(image.FilePath))
	if id == "" || id == "." {
		return ""
	}
	return filepath.Join(cfg.MediaDir, ".frames", id)
}

func frameThumbPath(image Image, offsetMs int) string {
	dir := videoFramesDir(image)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, strconv.Itoa(offsetMs)+".jpg")
}

func deleteVideoFrameFiles(image Image) {
	if dir := videoFramesDir(image); dir != "" && image.IsVideo() {
		os.RemoveAll(dir)
	}
}

// familyFrames caches each family's frame vectors by video, like
// familyVectors does for photo embeddings.
var familyFrames = struct {
	sync.Mutex
	byFamily map[int]map[int][]VideoFrame
}{byFamily: map[int]map[int][]VideoFrame{}}

func invalidateFamilyFrames(familyId int) {
	familyFrames.Lock()
	delete(familyFrames.byFamily, familyId)
	familyFrames.Unlock()
}

func framesForFamily(tx *vbolt.Tx, familyId int) map[int][]VideoFrame {
	familyFrames.Lock()
	cached, ok := familyFrames.byFamily[familyId]
	familyFrames.Unlock()
	if ok {
		return cached
	}
	var ids []int
	vbolt.ReadTermTargets(tx, VideoFramesByFamilyIndex, familyId, &ids, vbolt.Window{})
	var records []VideoFrames
	vbolt.ReadSlice(tx, VideoFramesBkt, ids, &records)
	loaded := map[int][]VideoFrame{}
	for _, f := range records {
		if f.Model == clipModelId && len(f.Frames) > 0 {
			loaded[f.ImageId] = f.Frames
		}
	}
	familyFrames.Lock()
	familyFrames.byFamily[familyId] = loaded
	familyFrames.Unlock()
	return loaded
}

// mediaSimilarity scores a photo by its embedding and a video by the better
// of its poster and its best frame.
func mediaSimilarity(tx *vbolt.Tx, familyId, id int, query []float32) (score float64, frameAt int, fromFrame, known bool) {
	if v, ok := vectorsForFamily(tx, familyId)[id]; ok {
		score, known = float64(vision.Dot(query, v)), true
	}
	if frames, ok := framesForFamily(tx, familyId)[id]; ok {
		if s, at, ok := bestFrame(frames, query); ok && (!known || s > score) {
			score, frameAt, fromFrame, known = s, at, true, true
		}
	}
	return
}

// bestFrame is the frame closest to query, or ok false without frames.
func bestFrame(frames []VideoFrame, query []float32) (score float64, offsetMs int, ok bool) {
	for _, f := range frames {
		if s := float64(vision.Dot(query, f.Vector)); !ok || s > score {
			score, offsetMs, ok = s, f.OffsetMs, true
		}
	}
	return
}

type videoAnalysisWorker struct {
	*backlogWorker[int]
	db      *vbolt.DB
	tools   mediaTools
	client  *visionClient
	scratch string
	ctx     context.Context
	cancel  context.CancelFunc
}

var globalVideoAnalysisWorker *videoAnalysisWorker

func InitializeVideoAnalysisWorker(db *vbolt.DB) {
	if globalVideoAnalysisWorker != nil || globalVideoWorker == nil || globalVisionClient == nil {
		return
	}
	scratch := filepath.Join(globalVideoWorker.scratch, "analysis")
	if err := os.MkdirAll(scratch, 0750); err != nil {
		LogErrorSimple(LogCategoryWorker, "Video analysis disabled: no scratch directory", map[string]interface{}{"error": err.Error()})
		return
	}
	w := newVideoAnalysisWorker(db, globalVideoWorker.tools, globalVisionClient, scratch)
	globalVideoAnalysisWorker = w
	w.run(func() []int { return videosNeedingAnalysis(db) })
	LogInfo(LogCategoryWorker, "Video analysis worker started", nil)
}

func newVideoAnalysisWorker(db *vbolt.DB, tools mediaTools, client *visionClient, scratch string) *videoAnalysisWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &videoAnalysisWorker{db: db, tools: tools, client: client, scratch: scratch, ctx: ctx, cancel: cancel}
	w.backlogWorker = newBacklogWorker("Video analysis worker", w.process)
	return w
}

func QueueVideoAnalysis(imageId int) {
	if globalVideoAnalysisWorker != nil {
		globalVideoAnalysisWorker.add(imageId)
	}
}

func StopVideoAnalysisWorker(ctx context.Context) bool {
	if globalVideoAnalysisWorker == nil {
		return true
	}
	globalVideoAnalysisWorker.cancel()
	return globalVideoAnalysisWorker.stopWait(ctx)
}

func VideoAnalysisQueueLength() int {
	if globalVideoAnalysisWorker == nil {
		return 0
	}
	return globalVideoAnalysisWorker.length()
}

func videosNeedingAnalysis(db *vbolt.DB) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
			if videoNeedsFrames(tx, image) {
				ids = append(ids, image.Id)
			}
			return true
		})
	})
	if len(ids) > 0 {
		LogInfo(LogCategoryWorker, "Queued videos for analysis", map[string]interface{}{"count": len(ids)})
	}
	return
}

func (w *videoAnalysisWorker) process(imageId int) error {
	var image Image
	var needed bool
	vbolt.WithReadTx(w.db, func(tx *vbolt.Tx) {
		image = GetImageById(tx, imageId)
		needed = videoNeedsFrames(tx, image)
	})
	if !needed || videoFramesDir(image) == "" {
		return nil
	}
	store := activeMediaStore()
	if store == nil {
		return errRetryLater
	}
	release, diskErr := diskBudget.reserve(cfg.MediaDir, int64(image.FileSize)+framesScratchBytes, cfg.MinFreeDiskBytes)
	if diskErr != nil {
		return errRetryLater
	}
	defer release()

	workDir, err := os.MkdirTemp(w.scratch, strconv.Itoa(imageId)+"-")
	if err != nil {
		return errRetryLater
	}
	defer os.RemoveAll(workDir)

	source, err := localCopyOf(w.ctx, store, image.PlaybackKey, workDir)
	if errors.Is(err, ErrMediaNotFound) {
		log.Printf("[VIDEO_ANALYSIS] Video %d has no playback file", imageId)
		return nil
	}
	if err != nil {
		return errRetryLater
	}

	frames, thumbs, err := w.sampleFrames(image, source, workDir)
	if w.ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return err
	}

	stored := false
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, imageId)
		if current.Id == 0 || current.Status != 0 || current.PlaybackKey != image.PlaybackKey {
			return
		}
		previous, _ := GetVideoFrames(tx, imageId)
		current.DerivedBytes += thumbs - previous.ThumbBytes
		vbolt.Write(tx, ImagesBkt, current.Id, &current)
		writeVideoFramesTx(tx, &VideoFrames{
			ImageId: current.Id, FamilyId: current.FamilyId,
			Model: clipModelId, Version: videoFramesVersion,
			Frames: frames, ThumbBytes: thumbs,
		})
		vbolt.TxCommit(tx)
		stored = true
	})
	if !stored {
		deleteVideoFrameFiles(image)
		return nil
	}
	log.Printf("[VIDEO_ANALYSIS] Video %d: %d frames", imageId, len(frames))
	return nil
}

// sampleFrames embeds the video's chosen frames and leaves a thumbnail of
// each distinct one in the video's frames directory.
func (w *videoAnalysisWorker) sampleFrames(image Image, source, workDir string) ([]VideoFrame, int, error) {
	probe, err := w.tools.probe(w.ctx, source)
	if err != nil {
		log.Printf("[VIDEO_ANALYSIS] Could not probe video %d: %v", image.Id, err)
		return nil, 0, nil
	}
	convert := ""
	if probe.HDR {
		convert = hdrTonemap
	}
	_, stderr, err := w.tools.runLogged(w.ctx, framesTimeout, w.tools.ffmpeg,
		"-nostdin", "-hide_banner", "-v", "info", "-i", source, "-an",
		"-vf", vision.FrameSelectFilter(probe.Duration, convert),
		"-fps_mode", "vfr", "-q:v", "4", filepath.Join(workDir, "frame-%03d.jpg"))
	if err != nil {
		log.Printf("[VIDEO_ANALYSIS] Could not sample frames from video %d: %v", image.Id, err)
		return nil, 0, nil
	}
	times := vision.ParseFrameTimes(stderr)

	var offsets []int
	var vectors [][]float32
	var files []string
	for _, i := range vision.ThinFrames(len(times), vision.MaxFrames) {
		file := filepath.Join(workDir, fmt.Sprintf("frame-%03d.jpg", i+1))
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		vector, model, err := w.client.embedImage(data)
		if errors.Is(err, errVisionUnavailable) {
			noteVisionReachable(false)
			return nil, 0, errRetryLater
		}
		if errors.Is(err, errRetryLater) {
			return nil, 0, errRetryLater
		}
		if err != nil || model != clipModelId {
			log.Printf("[VIDEO_ANALYSIS] Frame %d of video %d not embedded: %v %s", i, image.Id, err, model)
			continue
		}
		noteVisionReachable(true)
		offsets = append(offsets, int(times[i]/time.Millisecond))
		vectors = append(vectors, vector)
		files = append(files, file)
	}

	dir := videoFramesDir(image)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, 0, errRetryLater
	}
	var frames []VideoFrame
	thumbs := 0
	for _, k := range vision.DistinctFrames(vectors) {
		n, err := writeFrameThumb(files[k], frameThumbPath(image, offsets[k]))
		if err != nil {
			return nil, 0, errRetryLater
		}
		thumbs += n
		frames = append(frames, VideoFrame{OffsetMs: offsets[k], Vector: vectors[k]})
	}
	return frames, thumbs, nil
}

func writeFrameThumb(src, dst string) (int, error) {
	img, err := imaging.Open(src)
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, imaging.Resize(img, frameThumbWidth, 0, imaging.Lanczos), &jpeg.Options{Quality: 80}); err != nil {
		return 0, err
	}
	return buf.Len(), os.WriteFile(dst, buf.Bytes(), 0640)
}

func serveVideoFrame(w http.ResponseWriter, r *http.Request, image Image, offset string) {
	offsetMs, err := strconv.Atoi(offset)
	if err != nil || !image.IsVideo() || offsetMs < 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	path := frameThumbPath(image, offsetMs)
	if path == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}
