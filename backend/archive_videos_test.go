package backend

import (
	"bytes"
	"context"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// startArchiveVideoWorkers runs the video and transcription workers as the
// site does, so an import's queued work actually happens.
func (fx *archiveFixture) startArchiveVideoWorkers(t *testing.T, tools mediaTools) (stop func()) {
	t.Helper()
	previousVideo, previousAnalysis := globalVideoWorker, globalVideoAnalysisWorker
	video := newVideoWorker(fx.db, tools, t.TempDir())
	analysis := newVideoAnalysisWorker(fx.db, tools, nil, fakeSpeech(t, whisperJSON), t.TempDir())
	globalVideoWorker, globalVideoAnalysisWorker = video, analysis
	video.run(nil)
	analysis.run(nil)
	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		video.stopWait(ctx)
		for analysis.length() > 0 && ctx.Err() == nil {
			time.Sleep(10 * time.Millisecond)
		}
		analysis.stopWait(ctx)
		video.cancel()
		analysis.cancel()
		globalVideoWorker, globalVideoAnalysisWorker = previousVideo, previousAnalysis
	}
	t.Cleanup(stop)
	return stop
}

func familyVideos(t *testing.T, db *vbolt.DB, familyId int) (videos []Image) {
	t.Helper()
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for _, img := range GetFamilyImages(tx, familyId) {
			if img.IsVideo() {
				videos = append(videos, img)
			}
		}
	})
	return
}

func TestVideoWorkerKeepsImportedTranscripts(t *testing.T) {
	fx := setupArchiveFixture(t)
	tools := requireFFmpeg(t)
	clip := makeClip(t, tools, "waves.mov",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30", "-f", "lavfi", "-i", "sine=frequency=440",
		"-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-f", "mov")
	seeded := familyVideos(t, fx.db, fx.familyId)[0]
	if err := fx.store.Put(context.Background(), seeded.OriginalKey, bytes.NewReader(clip), int64(len(clip)), seeded.MimeType); err != nil {
		t.Fatal(err)
	}
	bundle := fx.exportBundle(t, fx.owner, "with_photos")

	restorer := fx.newUser(t, "restorer")
	stopWorkers := fx.startArchiveVideoWorkers(t, tools)
	if resp := fx.importBundle(t, restorer, bundle, importEverything); resp.ImportedVideos != 1 {
		t.Fatalf("imported %d videos; want 1", resp.ImportedVideos)
	}

	deadline := time.Now().Add(30 * time.Second)
	var restored Image
	for {
		restored = familyVideos(t, fx.db, restorer.FamilyId)[0]
		if restored.Status != PhotoProcessing || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stopWorkers()
	if restored.Status != PhotoReady {
		t.Fatalf("the imported video is %d (%s); want it processed and ready", restored.Status, restored.ProcessingError)
	}
	t.Cleanup(func() { deletePhotoFiles(restored) })

	var got GetVideoTranscriptResponse
	var err error
	fx.call(t, restorer, func(ctx *vbeam.Context) {
		got, err = GetVideoTranscript(ctx, GetVideoTranscriptRequest{Id: restored.Id})
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TranscriptDone || got.Automatic || len(got.Segments) != 1 || got.Segments[0].Text != "Look at the waves" {
		t.Errorf("after the video worker ran, the transcript is %+v; want the imported edit", got)
	}
}
