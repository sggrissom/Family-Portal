package backend

import (
	"encoding/json"
	"errors"
	"family/cfg"
	"image"
	"image/color"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"go.hasen.dev/vbolt"
)

type fakeVision struct {
	model  string
	status int
	calls  int
}

func startFakeVision(t *testing.T, fv *fakeVision) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "vision.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /embed/image", func(w http.ResponseWriter, r *http.Request) {
		fv.calls++
		var req struct {
			ImageData []byte `json:"image_data"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if fv.status != 0 {
			http.Error(w, "bad image", fv.status)
			return
		}
		if len(req.ImageData) == 0 {
			http.Error(w, "no image", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"model": fv.model, "embedding": []float32{0.6, 0.8}})
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return socket
}

func (fx resultsFixture) photoWithMedium(t *testing.T, name string) Image {
	t.Helper()
	photo := fx.addPhotoAt(t, name+".jpg", featuresEpoch)
	photosDir := filepath.Join(cfg.StaticDir, "photos")
	if err := os.MkdirAll(photosDir, 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	img.Set(0, 0, color.White)
	path := filepath.Join(photosDir, name+"_medium.jpg")
	writeTestJPEG(t, path, img)
	t.Cleanup(func() { os.Remove(path) })
	return photo
}

func TestEmbedPhotoStoresTheDaemonsVector(t *testing.T) {
	fx := setupResultsFixture(t)
	fv := &fakeVision{model: clipModelId}
	client := newVisionClient(startFakeVision(t, fv))
	photo := fx.photoWithMedium(t, "embed-ok")

	if err := embedPhoto(fx.db, client, photo.Id); err != nil {
		t.Fatalf("embedPhoto() = %v", err)
	}
	var e PhotoEmbedding
	var found bool
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { e, found = GetPhotoEmbedding(tx, photo.Id) })
	if !found || e.Model != clipModelId || len(e.Vector) != 2 || e.FamilyId != fx.familyId {
		t.Fatalf("stored embedding = %+v, %v", e, found)
	}
	if ids := photosNeedingEmbedding(fx.db); len(ids) != 0 {
		t.Errorf("still pending after embedding: %v", ids)
	}

	if err := embedPhoto(fx.db, client, photo.Id); err != nil || fv.calls != 1 {
		t.Errorf("a current embedding was recomputed (calls = %d, err = %v)", fv.calls, err)
	}

	if _, err := callAs(t, fx, DeletePhoto, DeletePhotoRequest{Id: photo.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := GetPhotoEmbedding(tx, photo.Id); found {
			t.Error("deleted photo kept its embedding")
		}
	})
}

func TestEmbedPhotoRetriesWhenDaemonIsDown(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.photoWithMedium(t, "embed-down")
	client := newVisionClient(filepath.Join(t.TempDir(), "missing.sock"))

	if err := embedPhoto(fx.db, client, photo.Id); !errors.Is(err, errRetryLater) {
		t.Errorf("embedPhoto() with no daemon = %v, want errRetryLater", err)
	}
}

func TestEmbedPhotoSkipsRejectedAndMismatchedResults(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.photoWithMedium(t, "embed-reject")

	rejecting := newVisionClient(startFakeVision(t, &fakeVision{status: http.StatusBadRequest}))
	if err := embedPhoto(fx.db, rejecting, photo.Id); err != nil {
		t.Errorf("a rejected image should be skipped, not retried: %v", err)
	}
	wrongModel := newVisionClient(startFakeVision(t, &fakeVision{model: "other-model"}))
	if err := embedPhoto(fx.db, wrongModel, photo.Id); err != nil {
		t.Errorf("a mismatched model should be skipped: %v", err)
	}
	if ids := photosNeedingEmbedding(fx.db); len(ids) != 1 {
		t.Errorf("pending = %v, want the photo still pending", ids)
	}
}
