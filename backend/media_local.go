package backend

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
)

const (
	localMediaUploadPrefix = "/api/media/upload/"
	localMediaObjectPrefix = "/api/media/object/"
)

// A localUploadGate decides whether a signed upload may proceed. The release
// func it returns runs once the body has been read, whether or not it landed.
type localUploadGate func(key string, size int64) (release func(), err *AppError)

type localMediaStore struct {
	root string
	key  []byte
	now  func() time.Time
	gate localUploadGate
}

func localMediaRoot() string {
	return filepath.Clean(cfg.MediaDir)
}

func mediaSigningKey() []byte {
	mac := hmac.New(sha256.New, jwtKey)
	mac.Write([]byte("family-media-url-v1"))
	return mac.Sum(nil)
}

func newLocalMediaStore(root string, key []byte) (*localMediaStore, error) {
	if len(key) == 0 {
		return nil, errors.New("no signing key")
	}
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &localMediaStore{root: root, key: key, now: time.Now}, nil
}

func (s *localMediaStore) Name() string { return "local" }

func (s *localMediaStore) path(key string) (string, error) {
	if !validMediaKey(key) {
		return "", errMediaKeyInvalid
	}
	return filepath.Join(s.root, filepath.FromSlash(key)), nil
}

func (s *localMediaStore) sign(fields ...string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(strings.Join(fields, "\n")))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *localMediaStore) UploadURL(key string, size int64, contentType string, ttl time.Duration) (UploadInstructions, error) {
	if !validMediaKey(key) {
		return UploadInstructions{}, errMediaKeyInvalid
	}
	expires := s.now().Add(ttl).Truncate(time.Second)
	exp := strconv.FormatInt(expires.Unix(), 10)
	sz := strconv.FormatInt(size, 10)
	q := url.Values{}
	q.Set("exp", exp)
	q.Set("size", sz)
	q.Set("ct", contentType)
	q.Set("sig", s.sign(http.MethodPut, key, exp, sz, contentType))
	return UploadInstructions{
		URL:       localMediaUploadPrefix + key + "?" + q.Encode(),
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: expires,
	}, nil
}

func (s *localMediaStore) PlaybackURL(key string, opts MediaURLOptions) (string, time.Time, error) {
	if !validMediaKey(key) {
		return "", time.Time{}, errMediaKeyInvalid
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = mediaPlaybackTTL
	}
	expires := s.now().Add(ttl).Truncate(time.Second)
	exp := strconv.FormatInt(expires.Unix(), 10)
	q := url.Values{}
	q.Set("exp", exp)
	q.Set("ct", opts.ContentType)
	if opts.DownloadName != "" {
		q.Set("dl", opts.DownloadName)
	}
	q.Set("sig", s.sign(http.MethodGet, key, exp, opts.ContentType, opts.DownloadName))
	return localMediaObjectPrefix + key + "?" + q.Encode(), expires, nil
}

func (s *localMediaStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrMediaNotFound
	}
	return f, err
}

func (s *localMediaStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	_, err = s.writeAtomically(p, body, size)
	return err
}

// writeAtomically renames the body into place only when it is exactly size
// bytes, or any length for a negative size.
func (s *localMediaStore) writeAtomically(p string, body io.Reader, limit int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".partial-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	keep := false
	defer func() {
		if !keep {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	reader := body
	if limit >= 0 {
		reader = io.LimitReader(body, limit+1)
	}
	n, err := io.Copy(tmp, reader)
	if err != nil {
		return n, err
	}
	if limit >= 0 && n > limit {
		return n, errMediaTooLarge
	}
	if limit >= 0 && n < limit {
		return n, fmt.Errorf("%w: received %d of %d bytes", errMediaIncomplete, n, limit)
	}
	if err := tmp.Sync(); err != nil {
		return n, err
	}
	if err := tmp.Close(); err != nil {
		return n, err
	}
	if err := os.Rename(tmpName, p); err != nil {
		return n, err
	}
	keep = true
	return n, nil
}

var errMediaTooLarge = errors.New("media body exceeds its declared size")

var errMediaIncomplete = errors.New("media body is shorter than its declared size")

func (s *localMediaStore) Stat(ctx context.Context, key string) (MediaObject, error) {
	p, err := s.path(key)
	if err != nil {
		return MediaObject{}, err
	}
	info, err := os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return MediaObject{}, ErrMediaNotFound
	}
	if err != nil {
		return MediaObject{}, err
	}
	return MediaObject{Size: info.Size()}, nil
}

func (s *localMediaStore) Delete(ctx context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for dir := filepath.Dir(p); dir != s.root && strings.HasPrefix(dir, s.root); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

func (s *localMediaStore) checkSignature(r *http.Request, fields ...string) bool {
	exp, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	if err != nil || s.now().Unix() > exp {
		return false
	}
	want := s.sign(fields...)
	return hmac.Equal([]byte(want), []byte(r.URL.Query().Get("sig")))
}

func (s *localMediaStore) serveUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, localMediaUploadPrefix)
	q := r.URL.Query()
	size, err := strconv.ParseInt(q.Get("size"), 10, 64)
	p, pathErr := s.path(key)
	if err != nil || pathErr != nil || size <= 0 ||
		!s.checkSignature(r, http.MethodPut, key, q.Get("exp"), q.Get("size"), q.Get("ct")) {
		RespondWithError(w, r, NewAppError(ErrCodeForbidden, "This upload link is invalid or has expired."), http.StatusForbidden)
		return
	}
	if r.Header.Get("Content-Type") != q.Get("ct") {
		RespondValidationError(w, r, "The upload was sent with the wrong content type.", r.Header.Get("Content-Type"))
		return
	}
	if r.ContentLength > size {
		RespondWithError(w, r, NewAppError(ErrCodeTooLarge, "This file is larger than the upload was set up for."), http.StatusRequestEntityTooLarge)
		return
	}

	if s.gate != nil {
		release, gateErr := s.gate(key, size)
		if gateErr != nil {
			RespondWithError(w, r, gateErr, statusForErrorCode(gateErr.Code))
			return
		}
		defer release()
	}

	_, err = s.writeAtomically(p, r.Body, size)
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.Is(err, errMediaTooLarge), errors.As(err, &maxBytesErr):
		RespondWithError(w, r, NewAppError(ErrCodeTooLarge, "This file is larger than the upload was set up for."), http.StatusRequestEntityTooLarge)
	case err != nil:
		RespondValidationError(w, r, "The upload did not finish. Please try again.", err.Error())
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (s *localMediaStore) serveObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, localMediaObjectPrefix)
	q := r.URL.Query()
	p, err := s.path(key)
	if err != nil || !s.checkSignature(r, http.MethodGet, key, q.Get("exp"), q.Get("ct"), q.Get("dl")) {
		http.Error(w, "This link is invalid or has expired.", http.StatusForbidden)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if ct := q.Get("ct"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if dl := q.Get("dl"); dl != "" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": dl}))
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

func RegisterMediaHandlers(app *vbeam.Application) {
	app.HandleFunc(localMediaUploadPrefix, func(w http.ResponseWriter, r *http.Request) {
		if s, ok := activeMediaStore().(*localMediaStore); ok {
			s.serveUpload(w, r)
			return
		}
		http.NotFound(w, r)
	})
	app.HandleFunc(localMediaObjectPrefix, func(w http.ResponseWriter, r *http.Request) {
		if s, ok := activeMediaStore().(*localMediaStore); ok {
			s.serveObject(w, r)
			return
		}
		http.NotFound(w, r)
	})
}
