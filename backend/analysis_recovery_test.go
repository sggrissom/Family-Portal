package backend

import (
	"context"
	"encoding/json"
	"errors"
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

func TestFeaturesRetryAfterSourcesAreRestored(t *testing.T) {
	fx := setupResultsFixture(t)
	p := fx.addPhotoAt(t, "recovery-missing.jpg", featuresEpoch)
	analyzePhotoFeatures(fx.db, p.Id)
	if !slices.Contains(photosNeedingFeatures(fx.db), p.Id) {
		t.Fatal("unreadable photo marked fully analyzed")
	}
	// A display variant can be analyzed even while its original is missing;
	// the location analyzer must stay pending rather than claiming no GPS.
	medium := fx.photoWithMedium(t, "recovery-medium")
	analyzePhotoFeatures(fx.db, medium.Id)
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		f, _ := GetPhotoFeatures(tx, medium.Id)
		if f.HashVersion != hashAnalyzerVersion || f.QualityVersion != qualityAnalyzerVersion || f.PlaceVersion == placeAnalyzerVersion {
			t.Fatalf("unexpected analyzer completion: %+v", f)
		}
	})
	original, err := os.ReadFile("testdata/gps-temple-square.jpg")
	if err != nil {
		t.Fatal(err)
	}
	// The EXIF fixture contains metadata only, so restore a decodable display
	// variant separately from the original used by the location analyzer.
	variant, err := embeddingSourceBytes(medium)
	if err != nil {
		t.Fatal(err)
	}
	variantPath := filepath.Join(filepath.Dir(getOriginalPhotoPath(p)), "recovery-missing_medium.jpg")
	if err := os.WriteFile(variantPath, variant, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(variantPath) })
	for _, photo := range []Image{p, medium} {
		path := getOriginalPhotoPath(photo)
		if err := os.WriteFile(path, original, 0644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(path) })
		analyzePhotoFeatures(fx.db, photo.Id)
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			f, ok := GetPhotoFeatures(tx, photo.Id)
			if featuresOutdated(f, ok) || !f.HasLocation {
				t.Fatalf("restored photo not analyzed: %+v", f)
			}
		})
	}
}

func TestVisionRetriesTransientHTTPFailures(t *testing.T) {
	fx := setupResultsFixture(t)
	p := fx.photoWithMedium(t, "recovery-vision")
	m := fx.addMilestoneOn(t, fx.alice, "First steps", "development", featuresEpoch)
	for _, status := range []int{http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "temporary", status) }))
		client := &visionClient{http: &http.Client{Transport: rewriteTransport{server.URL, http.DefaultTransport}}}
		if _, err := callRecognize(client.http, recognizeRequest{}); !errors.Is(err, errRetryLater) {
			t.Errorf("face status %d: %v", status, err)
		}
		if err := embedPhoto(fx.db, client, p.Id); !errors.Is(err, errRetryLater) {
			t.Errorf("photo status %d: %v", status, err)
		}
		if err := embedMilestone(fx.db, client, m.Id); !errors.Is(err, errRetryLater) {
			t.Errorf("milestone status %d: %v", status, err)
		}
		fx.setEmbedding(t, p, axis(0))
		phraseVectors.Lock()
		phraseVectors.byText = map[string][]float32{}
		phraseVectors.Unlock()
		if err := suggestTagsForPhoto(fx.db, client, p.Id); !errors.Is(err, errRetryLater) {
			t.Errorf("suggestions status %d: %v", status, err)
		}
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) { deletePhotoEmbeddingTx(tx, p.Id); vbolt.TxCommit(tx) })
		server.Close()
	}
}

// Route the existing daemon client's requests to an HTTP test server.
type rewriteTransport struct {
	base      string
	transport http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	rewritten, err := http.NewRequestWithContext(req.Context(), req.Method, t.base+req.URL.Path, req.Body)
	if err != nil {
		return nil, err
	}
	clone.URL = rewritten.URL
	return t.transport.RoundTrip(clone)
}

func TestFaceWorkerRecoversWhenDaemonStartsLater(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.photoWithMedium(t, "recovery-face")
	previousWorker := globalAnalysisWorker
	socket := filepath.Join(t.TempDir(), "face.sock")
	globalAnalysisWorker = nil
	retryPauseMin, retryPauseMax = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() {
		StopAnalysisWorker(context.Background())
		globalAnalysisWorker = previousWorker
		retryPauseMin, retryPauseMax = 5*time.Second, 5*time.Minute
	})
	initializeAnalysisWorker(fx.db, socket)
	if !GetAnalysisWorkerStats().IsRunning {
		t.Fatal("worker must start without a daemon")
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(recognizeResponse{}) })}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	waitFor(t, func() bool {
		complete := false
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { complete = GetImageById(tx, photo.Id).AnalysisStatus == 2 })
		return complete
	})
}

func TestFaceFailuresRemainRetryableAndRequestsAreBounded(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.photoWithMedium(t, "recovery-face-error")
	worker := &photoAnalysisWorker{db: fx.db, client: newFaceClient(filepath.Join(t.TempDir(), "absent.sock"))}
	if err := worker.processAnalysisJob(PhotoAnalysisJob{ImageId: photo.Id}); !errors.Is(err, errRetryLater) {
		t.Fatalf("outage: %v", err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		p := GetImageById(tx, photo.Id)
		if p.AnalysisStatus != 0 {
			t.Fatalf("temporary outage recorded as status %d", p.AnalysisStatus)
		}
	})
	if !imageNeedsAnalysis(Image{AnalysisStatus: 3}) {
		t.Fatal("legacy failures must be eligible for startup recovery")
	}
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer server.Close()
	defer close(release)
	client := newFaceClient("")
	if client.Timeout == 0 {
		t.Fatal("face requests have no timeout")
	}
	client.Timeout = 20 * time.Millisecond
	client.Transport = rewriteTransport{server.URL, http.DefaultTransport}
	if _, err := callRecognize(client, recognizeRequest{}); !errors.Is(err, errRetryLater) {
		t.Fatalf("timeout must be retryable: %v", err)
	}
}
