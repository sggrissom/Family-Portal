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
)

func newTestLocalStore(t *testing.T) *localMediaStore {
	t.Helper()
	store, err := newLocalMediaStore(filepath.Join(t.TempDir(), "media"), []byte("test-media-signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func sendUpload(store *localMediaStore, inst UploadInstructions, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(inst.Method, inst.URL, bytes.NewReader(body))
	for name, value := range inst.Headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	store.serveUpload(rec, req)
	return rec
}

func TestLocalMediaUploadStoresExactlyTheSignedBytes(t *testing.T) {
	store := newTestLocalStore(t)
	key := "videos/abc123/original.mov"
	body := []byte("pretend this is a quicktime file")

	inst, err := store.UploadURL(key, int64(len(body)), "video/quicktime", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Method != http.MethodPut || inst.Headers["Content-Type"] != "video/quicktime" || !strings.HasPrefix(inst.URL, localMediaUploadPrefix) {
		t.Fatalf("instructions = %+v", inst)
	}
	if rec := sendUpload(store, inst, body); rec.Code != http.StatusOK {
		t.Fatalf("upload status %d: %s", rec.Code, rec.Body)
	}

	obj, err := store.Stat(context.Background(), key)
	if err != nil || obj.Size != int64(len(body)) {
		t.Fatalf("Stat = %+v, %v", obj, err)
	}
	r, err := store.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("stored %q", got)
	}
}

func TestLocalMediaUploadRejectsBadRequests(t *testing.T) {
	store := newTestLocalStore(t)
	key := "videos/abc123/original.mov"
	body := []byte("0123456789")
	inst, _ := store.UploadURL(key, int64(len(body)), "video/mp4", time.Hour)

	tamper := func(param, value string) UploadInstructions {
		u, _ := url.Parse(inst.URL)
		q := u.Query()
		q.Set(param, value)
		u.RawQuery = q.Encode()
		out := inst
		out.URL = u.String()
		return out
	}

	cases := []struct {
		name string
		inst UploadInstructions
		body []byte
		want int
	}{
		{"bigger size", tamper("size", "999999"), body, http.StatusForbidden},
		{"other key", UploadInstructions{Method: http.MethodPut, URL: strings.Replace(inst.URL, "abc123", "def456", 1), Headers: inst.Headers}, body, http.StatusForbidden},
		{"forged sig", tamper("sig", "AAAA"), body, http.StatusForbidden},
		{"wrong type", UploadInstructions{Method: http.MethodPut, URL: inst.URL, Headers: map[string]string{"Content-Type": "text/html"}}, body, http.StatusBadRequest},
		{"too long", inst, append(body, 'x'), http.StatusRequestEntityTooLarge},
		{"too short", inst, body[:5], http.StatusBadRequest},
	}
	for _, c := range cases {
		if rec := sendUpload(store, c.inst, c.body); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, rec.Code, c.want, rec.Body)
		}
		if _, err := store.Stat(context.Background(), key); !errors.Is(err, ErrMediaNotFound) {
			t.Errorf("%s: left an object behind (%v)", c.name, err)
		}
	}

	store.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if rec := sendUpload(store, inst, body); rec.Code != http.StatusForbidden {
		t.Errorf("expired: status %d", rec.Code)
	}

	entries, _ := os.ReadDir(filepath.Join(store.root, "videos", "abc123"))
	for _, e := range entries {
		t.Errorf("partial file left behind: %s", e.Name())
	}
}

func TestLocalMediaFailedReuploadKeepsTheEarlierObject(t *testing.T) {
	store := newTestLocalStore(t)
	key := "videos/abc123/original.mov"
	body := []byte("complete upload")
	inst, _ := store.UploadURL(key, int64(len(body)), "video/mp4", time.Hour)
	if rec := sendUpload(store, inst, body); rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	if rec := sendUpload(store, inst, body[:3]); rec.Code != http.StatusBadRequest {
		t.Fatalf("short re-upload status %d", rec.Code)
	}
	if obj, err := store.Stat(context.Background(), key); err != nil || obj.Size != int64(len(body)) {
		t.Fatalf("earlier object damaged: %+v %v", obj, err)
	}
}

func TestLocalMediaUploadGate(t *testing.T) {
	store := newTestLocalStore(t)
	released := 0
	store.gate = func(key string, size int64) (func(), *AppError) {
		if strings.Contains(key, "closed") {
			return nil, NewAppError(ErrCodeConflict, "This upload has already finished.")
		}
		return func() { released++ }, nil
	}
	body := []byte("data")
	closed, _ := store.UploadURL("videos/closed/original.mov", 4, "video/mp4", time.Hour)
	if rec := sendUpload(store, closed, body); rec.Code != http.StatusConflict {
		t.Fatalf("gated upload status %d", rec.Code)
	}
	open, _ := store.UploadURL("videos/open/original.mov", 4, "video/mp4", time.Hour)
	sendUpload(store, open, body[:2])
	sendUpload(store, open, body)
	if released != 2 {
		t.Fatalf("release called %d times, want 2", released)
	}
}

func TestLocalMediaPlaybackSupportsRangesAndExpires(t *testing.T) {
	store := newTestLocalStore(t)
	key := "videos/abc123/playback.mp4"
	body := []byte("0123456789abcdefghij")
	if err := store.Put(context.Background(), key, bytes.NewReader(body), int64(len(body)), "video/mp4"); err != nil {
		t.Fatal(err)
	}

	link, expires, err := store.PlaybackURL(key, MediaURLOptions{ContentType: "video/mp4", TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if until := time.Until(expires); until < 59*time.Minute || until > time.Hour {
		t.Fatalf("expiry %v away", until)
	}

	req := httptest.NewRequest(http.MethodGet, link, nil)
	req.Header.Set("Range", "bytes=10-14")
	rec := httptest.NewRecorder()
	store.serveObject(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "abcde" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("range: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}

	download, _, _ := store.PlaybackURL(key, MediaURLOptions{ContentType: "video/mp4", DownloadName: "first steps.mov"})
	rec = httptest.NewRecorder()
	store.serveObject(rec, httptest.NewRequest(http.MethodGet, download, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "first steps.mov") {
		t.Fatalf("download: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}

	rec = httptest.NewRecorder()
	store.serveObject(rec, httptest.NewRequest(http.MethodGet, strings.Replace(download, "dl=", "dl=x", 1), nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tampered download name: %d", rec.Code)
	}

	store.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	rec = httptest.NewRecorder()
	store.serveObject(rec, httptest.NewRequest(http.MethodGet, link, nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expired link: %d", rec.Code)
	}
}

func TestLocalMediaKeysAndDeletes(t *testing.T) {
	store := newTestLocalStore(t)
	for _, key := range []string{"../etc/passwd", "videos/../../x", "/abs/path", "videos", "Videos/ABC/x", "videos/a/../b"} {
		if _, err := store.UploadURL(key, 1, "video/mp4", time.Hour); err == nil {
			t.Errorf("UploadURL accepted %q", key)
		}
		if _, err := store.Open(context.Background(), key); err == nil {
			t.Errorf("Open accepted %q", key)
		}
	}

	key := "videos/abc123/original.mov"
	if err := store.Put(context.Background(), key, strings.NewReader("x"), 1, "video/mp4"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatalf("deleting a missing object: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.root, "videos", "abc123")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty object directory left behind: %v", err)
	}
	if _, err := os.Stat(store.root); err != nil {
		t.Fatalf("store root removed: %v", err)
	}
}
