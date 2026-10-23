package backend

import (
	"bytes"
	"encoding/json"
	"family/vision"
	"image"
	_ "image/jpeg"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

// startColorVision embeds an image as its mean colour, so frames of
// different colours get different vectors.
func startColorVision(t *testing.T) *visionClient {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "vision.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /embed/image", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ImageData []byte `json:"image_data"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		img, _, err := image.Decode(bytes.NewReader(req.ImageData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var sum [3]float64
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				sum[0] += float64(r)
				sum[1] += float64(g)
				sum[2] += float64(bl)
			}
		}
		vector := vision.Normalize([]float32{float32(sum[0]), float32(sum[1]), float32(sum[2])})
		json.NewEncoder(w).Encode(map[string]any{"model": clipModelId, "embedding": vector})
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return newVisionClient(socket)
}

func TestVideoAnalysisKeepsOneFramePerScene(t *testing.T) {
	fx := setupVideoWorker(t)
	clip := makeClip(t, fx.worker.tools, "scenes.mp4",
		"-f", "lavfi", "-i", "color=c=red:s=320x240:r=30:d=6",
		"-f", "lavfi", "-i", "color=c=blue:s=320x240:r=30:d=6",
		"-filter_complex", "[0][1]concat=n=2:v=1:a=0", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	video := fx.upload(t, "scenes.mp4", "video/mp4", clip)
	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	before := imageForTest(t, fx.db, video.Id)

	aw := newVideoAnalysisWorker(fx.db, fx.worker.tools, startColorVision(t), nil, t.TempDir())
	t.Cleanup(aw.cancel)
	if ids := videosNeedingAnalysis(fx.db, true, false); len(ids) != 1 || ids[0] != video.Id {
		t.Fatalf("sweep = %v, want the published video", ids)
	}
	if err := aw.process(video.Id); err != nil {
		t.Fatal(err)
	}

	var frames VideoFrames
	var found bool
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { frames, found = GetVideoFrames(tx, video.Id) })
	if !found || len(frames.Frames) != 2 {
		t.Fatalf("frames = %+v, want one red and one blue", frames)
	}
	if first, cut := frames.Frames[0].OffsetMs, frames.Frames[1].OffsetMs; first != 0 || cut < 5900 || cut > 6100 {
		t.Fatalf("offsets %d and %d, want 0 and the cut at 6s", first, cut)
	}
	after := imageForTest(t, fx.db, video.Id)
	if frames.ThumbBytes <= 0 || after.DerivedBytes != before.DerivedBytes+frames.ThumbBytes {
		t.Fatalf("derived %d -> %d with %d thumb bytes", before.DerivedBytes, after.DerivedBytes, frames.ThumbBytes)
	}
	if ids := videosNeedingAnalysis(fx.db, true, false); len(ids) != 0 {
		t.Errorf("still pending after analysis: %v", ids)
	}

	blue := vision.Normalize([]float32{0, 0, 1})
	var cached []VideoFrame
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { cached = framesForFamily(tx, video.FamilyId)[video.Id] })
	if _, at, ok := bestFrame(cached, blue); !ok || at != frames.Frames[1].OffsetMs {
		t.Errorf("best blue frame at %d, want %d", at, frames.Frames[1].OffsetMs)
	}

	thumb := frameThumbPath(after, frames.Frames[1].OffsetMs)
	if _, err := os.Stat(thumb); err != nil {
		t.Fatalf("frame thumbnail missing: %v", err)
	}
	rec := httptest.NewRecorder()
	serveVideoFrame(rec, httptest.NewRequest(http.MethodGet, "/", nil), after, "999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown frame offset served %d", rec.Code)
	}

	if _, err := callAs(t, fx.resultsFixture, DeletePhoto, DeletePhotoRequest{Id: video.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := GetVideoFrames(tx, video.Id); found {
			t.Error("deleted video kept its frames")
		}
	})
	if _, err := os.Stat(filepath.Dir(thumb)); !os.IsNotExist(err) {
		t.Errorf("frame directory survived the delete: %v", err)
	}
}

func (fx resultsFixture) setFrames(t *testing.T, video Image, frames ...VideoFrame) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		writeVideoFramesTx(tx, &VideoFrames{
			ImageId: video.Id, FamilyId: video.FamilyId,
			Model: clipModelId, Version: videoFramesVersion, Frames: frames,
		})
		vbolt.TxCommit(tx)
	})
}

func TestSearchFindsAVideoByItsBestFrame(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalVisionClient
	globalVisionClient = startFakeTextVision(t, map[string][]float32{"cake": {1, 0, 0}})
	t.Cleanup(func() { globalVisionClient = previous })

	photo := fx.addPhotoAt(t, "cake.jpg", featuresEpoch)
	fx.setEmbedding(t, photo, []float32{1, 0, 0})
	video := fx.addVideoAt(t, "party.mov", featuresEpoch.Add(time.Hour))
	fx.setEmbedding(t, video, []float32{0, 0, 1})
	fx.setFrames(t, video,
		VideoFrame{OffsetMs: 0, Vector: []float32{0, 0, 1}},
		VideoFrame{OffsetMs: 42000, Vector: []float32{0.99, 0.141, 0}})
	other := fx.addVideoAt(t, "pool.mov", featuresEpoch.Add(2*time.Hour))
	fx.setEmbedding(t, other, []float32{0, 1, 0})

	resp, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "cake", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{photo.Id, video.Id}) {
		t.Fatalf("cake results = %v, want [photo video] = [%d %d]", got, photo.Id, video.Id)
	}
	if resp.Photos[0].Match != nil {
		t.Errorf("photo carries a match: %+v", resp.Photos[0].Match)
	}
	if m := resp.Photos[1].Match; m == nil || m.Source != MatchFrame || m.OffsetMs != 42000 {
		t.Errorf("video match = %+v, want the frame at 42s", m)
	}

	global, err := callAs(t, fx, Search, SearchRequest{Query: "cake"})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(global.Photos); !slices.Contains(got, video.Id) {
		t.Errorf("global search photos = %v, want the video", got)
	}
}
