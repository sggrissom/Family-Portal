package backend

import (
	"context"
	"errors"
	"family/cfg"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"go.hasen.dev/vbolt"
)

const analysisScratchBytes = 64 << 20

type videoAnalysisWorker struct {
	*backlogWorker[int]
	db      *vbolt.DB
	tools   mediaTools
	client  *visionClient
	faces   *http.Client
	speech  *speechEngine
	scratch string
	ctx     context.Context
	cancel  context.CancelFunc
}

var globalVideoAnalysisWorker *videoAnalysisWorker

func InitializeVideoAnalysisWorker(db *vbolt.DB) {
	InitializeSpeech()
	if globalVideoAnalysisWorker != nil || globalVideoWorker == nil || (globalVisionClient == nil && speech == nil) {
		return
	}
	scratch := filepath.Join(globalVideoWorker.scratch, "analysis")
	if err := os.MkdirAll(scratch, 0750); err != nil {
		LogErrorSimple(LogCategoryWorker, "Video analysis disabled: no scratch directory", map[string]interface{}{"error": err.Error()})
		return
	}
	w := newVideoAnalysisWorker(db, globalVideoWorker.tools, globalVisionClient, speech, scratch)
	if cfg.EnableFaceTagging {
		w.faces = newFaceClient(cfg.FaceAnalysisSocket)
	}
	globalVideoAnalysisWorker = w
	w.run(func() []int { return videosNeedingAnalysis(db, w.client != nil, w.faces != nil, w.speech != nil) })
	LogInfo(LogCategoryWorker, "Video analysis worker started", map[string]interface{}{
		"frames": w.client != nil, "faces": w.faces != nil, "transcripts": w.speech != nil,
	})
}

func newVideoAnalysisWorker(db *vbolt.DB, tools mediaTools, client *visionClient, speech *speechEngine, scratch string) *videoAnalysisWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &videoAnalysisWorker{db: db, tools: tools, client: client, speech: speech, scratch: scratch, ctx: ctx, cancel: cancel}
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

// videosNeedingAnalysis lists the videos missing a part the worker can
// produce: frames and transcript meaning with the vision daemon, faces with
// face tagging, and transcripts with a speech model.
func videosNeedingAnalysis(db *vbolt.DB, vision, faces, transcripts bool) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
			if (vision && (videoNeedsFrames(tx, image) || videoNeedsSpeechVectors(tx, image))) ||
				(faces && videoNeedsFaces(tx, image)) || (transcripts && videoNeedsTranscript(tx, image)) {
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

// process runs each analysis the video is missing over one local copy of its
// playback file, then embeds what its transcript says.
func (w *videoAnalysisWorker) process(imageId int) error {
	if err := w.analyzeMedia(imageId); err != nil || w.ctx.Err() != nil {
		return err
	}
	if w.client == nil {
		return nil
	}
	return embedSpeech(w.db, w.client, imageId)
}

func (w *videoAnalysisWorker) analyzeMedia(imageId int) error {
	var image Image
	var needFrames, needFaces, needTranscript bool
	vbolt.WithReadTx(w.db, func(tx *vbolt.Tx) {
		image = GetImageById(tx, imageId)
		needFrames = w.client != nil && videoNeedsFrames(tx, image) && videoFramesDir(image) != ""
		needFaces = w.faces != nil && (needFrames || videoNeedsFaces(tx, image))
		needTranscript = w.speech != nil && videoNeedsTranscript(tx, image)
	})
	if !needFrames && !needFaces && !needTranscript {
		return nil
	}
	store := activeMediaStore()
	if store == nil {
		return errRetryLater
	}
	release, diskErr := diskBudget.reserve(cfg.MediaDir, int64(image.FileSize)+analysisScratchBytes, cfg.MinFreeDiskBytes)
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
		LogWarn(LogCategoryWorker, "Video analysis found no playback file", map[string]interface{}{"videoId": imageId})
		return nil
	}
	if err != nil {
		return errRetryLater
	}

	var sampled sampledFrames
	if needFrames || needFaces {
		sampled = w.extractFrames(image, source, workDir)
		if w.ctx.Err() != nil {
			return nil
		}
	}
	if needFrames {
		if err := w.analyzeFrames(image, sampled); err != nil || w.ctx.Err() != nil {
			return err
		}
	}
	if needFaces {
		if err := w.analyzeFaces(image, sampled); err != nil || w.ctx.Err() != nil {
			return err
		}
	}
	if needTranscript {
		if err := w.analyzeSpeech(image, source, workDir); err != nil || w.ctx.Err() != nil {
			return err
		}
	}
	return nil
}

func (w *videoAnalysisWorker) analyzeFrames(image Image, sampled sampledFrames) error {
	frames, thumbs, err := w.embedFrames(image, sampled)
	if w.ctx.Err() != nil || err != nil {
		return err
	}
	stored := false
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		if current.Id == 0 || current.Status != PhotoReady || current.PlaybackKey != image.PlaybackKey {
			return
		}
		previous, _ := GetVideoFrames(tx, image.Id)
		current.DerivedBytes += thumbs - previous.ThumbBytes
		writeImage(tx, &current)
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
	QueuePhotoEmbedding(image.Id)
	return nil
}

// analyzeFaces looks for faces in each stored frame and records them against
// the video. The frames must be current; faces wait for them otherwise.
func (w *videoAnalysisWorker) analyzeFaces(image Image, sampled sampledFrames) error {
	var stored VideoFrames
	var found bool
	vbolt.WithReadTx(w.db, func(tx *vbolt.Tx) { stored, found = GetVideoFrames(tx, image.Id) })
	if videoFramesOutdated(stored, found) {
		return nil
	}
	var detected []FrameFaces
	for _, frame := range stored.Frames {
		file, ok := sampled.fileAt(frame.OffsetMs)
		if !ok {
			continue
		}
		faces, err := detectFrameFaces(w.faces, file)
		if errors.Is(err, errRetryLater) {
			return errRetryLater
		}
		if err != nil {
			LogWarn(LogCategoryWorker, "Video frame face detection failed", map[string]interface{}{"videoId": image.Id, "offsetMs": frame.OffsetMs, "error": err.Error()})
			continue
		}
		detected = append(detected, FrameFaces{OffsetMs: frame.OffsetMs, Faces: faces})
	}
	tagged := 0
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		frames, found := GetVideoFrames(tx, image.Id)
		if current.Id == 0 || current.Status != PhotoReady || current.PlaybackKey != image.PlaybackKey ||
			!found || frames.Version != stored.Version || frames.Model != stored.Model {
			return
		}
		tagged = RecordVideoFacesTx(tx, image.Id, detected)
		frames.FacesVersion = videoFacesVersion
		writeVideoFramesTx(tx, &frames)
		vbolt.TxCommit(tx)
	})
	LogInfo(LogCategoryWorker, "Video face analysis finished", map[string]interface{}{"videoId": image.Id, "frames": len(detected), "autoTagged": tagged})
	return nil
}

func (w *videoAnalysisWorker) analyzeSpeech(image Image, source, workDir string) error {
	t, err := w.speech.transcribe(w.ctx, w.tools, image, source, workDir)
	if w.ctx.Err() != nil {
		return nil
	}
	if err != nil {
		LogWarn(LogCategoryWorker, "Video transcription failed", map[string]interface{}{"videoId": image.Id, "error": err.Error()})
		t.Status, t.Segments = TranscriptFailed, nil
	}
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		existing, found := GetVideoTranscriptRecord(tx, image.Id)
		if current.Id == 0 || current.Status != PhotoReady || current.PlaybackKey != image.PlaybackKey || (found && transcriptSettled(existing)) {
			return
		}
		writeVideoTranscriptTx(tx, &t)
		vbolt.TxCommit(tx)
	})
	return nil
}
