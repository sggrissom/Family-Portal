package backend

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestImportedFacesSurviveTheAnalysisThatFollows(t *testing.T) {
	fx := setupArchiveFixture(t)
	source := readArchiveContents(t, fx.db, fx.familyId)

	bundle := fx.exportBundle(t, fx.owner, "everything")
	restorer := fx.newUser(t, "restorer")
	if rec := postImportBundle(t, restorer, bundle, importEverything); rec.Code != http.StatusOK {
		t.Fatalf("import = %d %s", rec.Code, rec.Body.String())
	}
	jobs := drainPhotoJobs()
	for _, job := range jobs {
		t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	}

	startFakeFaceDaemon(t, recognizeResponse{Source: "data", Faces: []recognizedFace{
		{Descriptor: make([]float32, 128), Rect: [4]int{20, 0, 40, 30}},
	}})
	for _, job := range jobs {
		globalPhotoWorker.processPhotoJob(job)
	}
	waitFor(t, func() bool {
		analysed := true
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			for _, img := range GetFamilyImages(tx, restorer.FamilyId) {
				if !img.IsVideo() && (img.Status != PhotoReady || img.AnalysisStatus != AnalysisDone) {
					analysed = false
				}
			}
		})
		return analysed && globalAnalysisWorker.length() == 0
	})
	StopAnalysisWorker(context.Background())

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, img := range GetFamilyImages(tx, restorer.FamilyId) {
			if !img.IsVideo() && !hasModernVariant(img.FilePath) {
				t.Errorf("imported photo %q has no resized variants", img.Title)
			}
		}
	})
	readArchiveContents(t, fx.db, fx.familyId) // fails if the import took the source photos' files
	restored := readArchiveContents(t, fx.db, restorer.FamilyId)
	for _, face := range source["photo_faces"] {
		found := false
		for _, got := range restored["photo_faces"] {
			found = found || got.String() == face.String()
		}
		if !found {
			t.Errorf("imported face %s was overwritten; the family now has %v", face, restored["photo_faces"])
		}
	}
}

// startFakeFaceDaemon runs the analysis worker against a daemon that answers
// every request with response.
func startFakeFaceDaemon(t *testing.T, response recognizeResponse) {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "face.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(response)
	})}
	go server.Serve(listener)

	previous := globalAnalysisWorker
	globalAnalysisWorker = nil
	retryPauseMin, retryPauseMax = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() {
		StopAnalysisWorker(context.Background())
		globalAnalysisWorker = previous
		retryPauseMin, retryPauseMax = 5*time.Second, 5*time.Minute
		server.Close()
	})
	initializeAnalysisWorker(appDb, socket)
}
