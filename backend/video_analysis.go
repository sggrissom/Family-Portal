package backend

import (
	"context"
	"errors"
	"family/cfg"
	"log"
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
	globalVideoAnalysisWorker = w
	w.run(func() []int { return videosNeedingAnalysis(db, w.client != nil, w.speech != nil) })
	LogInfo(LogCategoryWorker, "Video analysis worker started", map[string]interface{}{
		"frames": w.client != nil, "transcripts": w.speech != nil,
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

func videosNeedingAnalysis(db *vbolt.DB, frames, transcripts bool) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
			if (frames && videoNeedsFrames(tx, image)) || (transcripts && videoNeedsTranscript(tx, image)) {
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
// playback file.
func (w *videoAnalysisWorker) process(imageId int) error {
	var image Image
	var needFrames, needTranscript bool
	vbolt.WithReadTx(w.db, func(tx *vbolt.Tx) {
		image = GetImageById(tx, imageId)
		needFrames = w.client != nil && videoNeedsFrames(tx, image) && videoFramesDir(image) != ""
		needTranscript = w.speech != nil && videoNeedsTranscript(tx, image)
	})
	if !needFrames && !needTranscript {
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
		log.Printf("[VIDEO_ANALYSIS] Video %d has no playback file", imageId)
		return nil
	}
	if err != nil {
		return errRetryLater
	}

	if needFrames {
		if err := w.analyzeFrames(image, source, workDir); err != nil || w.ctx.Err() != nil {
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

func (w *videoAnalysisWorker) analyzeFrames(image Image, source, workDir string) error {
	frames, thumbs, err := w.sampleFrames(image, source, workDir)
	if w.ctx.Err() != nil || err != nil {
		return err
	}
	stored := false
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		if current.Id == 0 || current.Status != 0 || current.PlaybackKey != image.PlaybackKey {
			return
		}
		previous, _ := GetVideoFrames(tx, image.Id)
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
	QueuePhotoEmbedding(image.Id)
	log.Printf("[VIDEO_ANALYSIS] Video %d: %d frames", image.Id, len(frames))
	return nil
}

func (w *videoAnalysisWorker) analyzeSpeech(image Image, source, workDir string) error {
	t, err := w.speech.transcribe(w.ctx, w.tools, image, source, workDir)
	if w.ctx.Err() != nil {
		return nil
	}
	if err != nil {
		log.Printf("[VIDEO_ANALYSIS] Could not transcribe video %d: %v", image.Id, err)
		t.Status, t.Segments = TranscriptFailed, nil
	}
	vbolt.WithWriteTx(w.db, func(tx *vbolt.Tx) {
		current := GetImageById(tx, image.Id)
		existing, found := GetVideoTranscriptRecord(tx, image.Id)
		if current.Id == 0 || current.Status != 0 || current.PlaybackKey != image.PlaybackKey || (found && transcriptSettled(existing)) {
			return
		}
		writeVideoTranscriptTx(tx, &t)
		vbolt.TxCommit(tx)
	})
	log.Printf("[VIDEO_ANALYSIS] Video %d: transcript status %d, %d segments", image.Id, t.Status, len(t.Segments))
	return nil
}
