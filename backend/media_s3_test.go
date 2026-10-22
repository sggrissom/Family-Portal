package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

func newFakeS3Store(t *testing.T) *s3MediaStore {
	t.Helper()
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	server := httptest.NewServer(faker.Server())
	t.Cleanup(server.Close)
	if err := backend.CreateBucket("family-media"); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(server.URL)
	store, err := newS3MediaStore(s3Config{
		Endpoint: u.Host, Region: "auto", Bucket: "family-media",
		AccessKey: "test-key", SecretKey: "test-secret", Insecure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// sendInstructions does what a client does with upload instructions: an HTTP
// request to the URL, with exactly the given headers and no credentials.
func sendInstructions(t *testing.T, base string, inst UploadInstructions, body []byte) int {
	t.Helper()
	target := inst.URL
	if strings.HasPrefix(target, "/") {
		target = base + target
	}
	req, err := http.NewRequest(inst.Method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range inst.Headers {
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func fetchRange(t *testing.T, base, link, byteRange string) (int, []byte) {
	t.Helper()
	if strings.HasPrefix(link, "/") {
		link = base + link
	}
	req, _ := http.NewRequest(http.MethodGet, link, nil)
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

type storeUnderTest struct {
	name  string
	store MediaStore
	base  string
}

func bothStores(t *testing.T) []storeUnderTest {
	local := newTestLocalStore(t)
	mux := http.NewServeMux()
	mux.HandleFunc(localMediaUploadPrefix, local.serveUpload)
	mux.HandleFunc(localMediaObjectPrefix, local.serveObject)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return []storeUnderTest{
		{"local", local, server.URL},
		{"s3", newFakeS3Store(t), ""},
	}
}

func TestMediaStoreContract(t *testing.T) {
	for _, s := range bothStores(t) {
		t.Run(s.name, func(t *testing.T) {
			ctx := context.Background()
			body := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
			src := "uploads/abc123/source.mov"

			inst, err := s.store.UploadURL(src, int64(len(body)), "video/quicktime", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			if inst.Method != http.MethodPut || inst.Headers["Content-Type"] != "video/quicktime" || inst.ExpiresAt.IsZero() {
				t.Fatalf("instructions %+v", inst)
			}
			if code := sendInstructions(t, s.base, inst, body); code < 200 || code > 299 {
				t.Fatalf("upload status %d", code)
			}
			if obj, err := s.store.Stat(ctx, src); err != nil || obj.Size != int64(len(body)) {
				t.Fatalf("Stat = %+v, %v", obj, err)
			}

			dst := "videos/abc123/original.mov"
			if err := moveMediaObject(ctx, s.store, src, dst, "video/quicktime"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.store.Stat(ctx, src); !errors.Is(err, ErrMediaNotFound) {
				t.Fatalf("source still present after move: %v", err)
			}
			r, err := s.store.Open(ctx, dst)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(r)
			r.Close()
			if !bytes.Equal(got, body) {
				t.Fatalf("moved object = %q", got)
			}

			link, expires, err := s.store.PlaybackURL(dst, MediaURLOptions{ContentType: "video/quicktime", DownloadName: "clip.mov"})
			if err != nil || time.Until(expires) < 5*time.Hour {
				t.Fatalf("PlaybackURL %v, expires in %v", err, time.Until(expires))
			}
			code, part := fetchRange(t, s.base, link, "bytes=10-14")
			if code != http.StatusPartialContent || string(part) != "abcde" {
				t.Fatalf("range read %d %q", code, part)
			}

			if err := s.store.Put(ctx, "videos/abc123/playback.mp4", bytes.NewReader(body[:10]), 10, "video/mp4"); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{dst, "videos/abc123/playback.mp4", "videos/never/existed.mov"} {
				if err := s.store.Delete(ctx, key); err != nil {
					t.Fatalf("Delete(%s) = %v", key, err)
				}
			}
			if _, err := s.store.Open(ctx, dst); !errors.Is(err, ErrMediaNotFound) {
				t.Fatalf("Open after delete = %v", err)
			}
			if err := moveMediaObject(ctx, s.store, "uploads/gone/source.mov", "videos/gone/original.mov", "video/mp4"); !errors.Is(err, ErrMediaNotFound) {
				t.Fatalf("moving a missing object = %v", err)
			}
			if _, err := s.store.UploadURL("../escape", 1, "video/mp4", time.Hour); err == nil {
				t.Fatal("accepted an invalid key")
			}
		})
	}
}

func TestVideoLifecycleOnTheBucket(t *testing.T) {
	fx := setupVideoFixture(t)
	bucket := newFakeS3Store(t)
	mediaStore = bucket

	body := []byte("not really a video, but the transfer path does not care")
	begun, err := fx.begin(t, len(body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(begun.Upload.URL, "/") {
		t.Fatalf("bucket upload URL is relative: %s", begun.Upload.URL)
	}
	if code := sendInstructions(t, "", begun.Upload, body); code != http.StatusOK {
		t.Fatalf("PUT %d", code)
	}
	done, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: begun.UploadId})
	if err != nil {
		t.Fatal(err)
	}
	video := imageForTest(t, fx.db, done.Image.Id)

	if _, err := callAs(t, fx.resultsFixture, DeletePhoto, DeletePhotoRequest{Id: video.Id}); err != nil {
		t.Fatal(err)
	}
	if r := sweepMedia(fx.db, bucket, time.Now()); r.Deleted != 1 {
		t.Fatalf("sweep = %+v", r)
	}
	if _, err := bucket.Stat(context.Background(), video.OriginalKey); !errors.Is(err, ErrMediaNotFound) {
		t.Fatal("deleting the video left its object in the bucket")
	}
}

func TestMigrateLocalMedia(t *testing.T) {
	local := newTestLocalStore(t)
	bucket := newFakeS3Store(t)
	ctx := context.Background()
	files := map[string]string{
		"videos/aaa/original.mov": "original bytes",
		"videos/aaa/playback.mp4": "playback",
		"uploads/bbb/source.mp4":  "pending upload",
	}
	for key, content := range files {
		if err := local.Put(ctx, key, strings.NewReader(content), int64(len(content)), "video/mp4"); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(local.root, ".scratch", "job"), 0750)
	os.WriteFile(filepath.Join(local.root, ".scratch", "job", "playback.mp4"), []byte("scratch"), 0640)
	os.WriteFile(filepath.Join(local.root, "videos", "aaa", ".partial-123"), []byte("partial"), 0640)

	result, err := MigrateLocalMedia(ctx, local.root, bucket, false)
	if err != nil || result.Copied != 3 || result.Failed != 0 {
		t.Fatalf("first run = %+v, %v", result, err)
	}
	for key, content := range files {
		r, err := bucket.Open(ctx, key)
		if err != nil {
			t.Fatalf("%s not in bucket: %v", key, err)
		}
		got, _ := io.ReadAll(r)
		r.Close()
		if string(got) != content {
			t.Fatalf("%s = %q", key, got)
		}
	}
	if _, err := bucket.Stat(ctx, ".scratch/job/playback.mp4"); err == nil {
		t.Fatal("scratch files were copied")
	}

	result, err = MigrateLocalMedia(ctx, local.root, bucket, true)
	if err != nil || result.Copied != 0 || result.Skipped != 3 || result.LocalDeleted != 3 {
		t.Fatalf("second run = %+v, %v", result, err)
	}
	if _, err := local.Stat(ctx, "videos/aaa/original.mov"); !errors.Is(err, ErrMediaNotFound) {
		t.Fatal("local copy kept after a verified migration with deleteLocal")
	}
}

func TestS3ConfigIsAllOrNothing(t *testing.T) {
	if _, err := newS3MediaStore(s3Config{Endpoint: "x.r2.cloudflarestorage.com", Bucket: "b"}); err == nil {
		t.Fatal("accepted settings without credentials")
	}
	if _, err := newS3MediaStore(s3Config{
		Endpoint: "https://x.r2.cloudflarestorage.com", Region: "auto", Bucket: "b", AccessKey: "a", SecretKey: "s",
	}); err == nil || !strings.Contains(err.Error(), "without https://") {
		t.Fatalf("accepted an endpoint with a scheme: %v", err)
	}
}

func TestVideoWorkerOnTheBucket(t *testing.T) {
	fx := setupVideoWorker(t)
	bucket := newFakeS3Store(t)
	mediaStore = bucket
	clip := makeClip(t, fx.worker.tools, "clip.mp4",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p")

	begun, err := fx.begin(t, len(clip))
	if err != nil {
		t.Fatal(err)
	}
	if code := sendInstructions(t, "", begun.Upload, clip); code != http.StatusOK {
		t.Fatalf("PUT %d", code)
	}
	done, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: begun.UploadId})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deletePhotoFiles(imageForTest(t, fx.db, done.Image.Id)) })

	if err := fx.worker.process(done.Image.Id); err != nil {
		t.Fatal(err)
	}
	got := imageForTest(t, fx.db, done.Image.Id)
	if got.Status != 0 {
		t.Fatalf("status %d %q", got.Status, got.ProcessingError)
	}
	for _, key := range []string{got.OriginalKey, got.PlaybackKey} {
		if _, err := bucket.Stat(context.Background(), key); err != nil {
			t.Fatalf("%s not in the bucket: %v", key, err)
		}
	}
	entries, _ := os.ReadDir(fx.worker.scratch)
	if len(entries) != 0 {
		t.Fatalf("scratch not cleaned: %d entries", len(entries))
	}
}

func TestCSPAllowsTheMediaBucket(t *testing.T) {
	rec := httptest.NewRecorder()
	addSecurityHeaders(rec)
	if strings.Contains(rec.Header().Get("Content-Security-Policy"), "r2.cloudflarestorage.com") {
		t.Fatal("bucket allowed without a bucket configured")
	}

	mediaOrigin = s3Config{Endpoint: "acct.r2.cloudflarestorage.com"}.origin()
	t.Cleanup(func() { mediaOrigin = "" })
	rec = httptest.NewRecorder()
	addSecurityHeaders(rec)
	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{
		"media-src 'self' blob: https://acct.r2.cloudflarestorage.com;",
		"connect-src 'self' ws: wss: https://acct.r2.cloudflarestorage.com;",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP %q lacks %q", csp, directive)
		}
	}
}
