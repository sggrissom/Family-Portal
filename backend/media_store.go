package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

// UploadInstructions tell a client exactly how to send one object. Clients
// must use the method and headers as given and must not add the app's own
// credentials: for a bucket the URL is the whole authorization.
type UploadInstructions struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type MediaURLOptions struct {
	ContentType  string
	DownloadName string
	TTL          time.Duration
}

type MediaObject struct {
	Size int64
}

type MediaStore interface {
	Name() string
	UploadURL(key string, size int64, contentType string, ttl time.Duration) (UploadInstructions, error)
	PlaybackURL(key string, opts MediaURLOptions) (string, time.Time, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Stat(ctx context.Context, key string) (MediaObject, error)
	Delete(ctx context.Context, key string) error
}

var ErrMediaNotFound = errors.New("media object not found")

var errMediaKeyInvalid = errors.New("media key is invalid")

const (
	mediaUploadTTL   = 2 * time.Hour
	mediaPlaybackTTL = 6 * time.Hour
)

var mediaKeyPattern = regexp.MustCompile(`^[a-z0-9]+(/[a-z0-9][a-z0-9._-]*)+$`)

func validMediaKey(key string) bool {
	return len(key) <= 200 && mediaKeyPattern.MatchString(key) && !strings.Contains(key, "..")
}

func newMediaId() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

var mediaStore MediaStore

func activeMediaStore() MediaStore {
	return mediaStore
}

func InitializeMediaStore() {
	if mediaStore != nil {
		return
	}
	store, err := newLocalMediaStore(localMediaRoot(), mediaSigningKey())
	if err != nil {
		LogErrorSimple(LogCategorySystem, "Media storage is unavailable; video is disabled", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	mediaStore = store
	LogInfo(LogCategorySystem, "Media storage ready", map[string]interface{}{"backend": store.Name()})
}
