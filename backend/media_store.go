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

var mediaStoreProblem string

// InitializeMediaStore picks the bucket when MEDIA_S3_* is set and local disk
// otherwise. Incomplete bucket settings leave video unavailable rather than
// quietly filling the local disk.
func InitializeMediaStore() {
	if mediaStore != nil {
		return
	}
	var store MediaStore
	var err error
	if c, configured := s3ConfigFromEnv(); configured {
		var s3 *s3MediaStore
		s3, err = newS3MediaStore(c)
		if err == nil {
			store = s3
			mediaOrigin = c.origin()
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s3.checkReachable(ctx); err != nil {
					LogErrorSimple(LogCategorySystem, "Media bucket is not reachable", map[string]interface{}{
						"bucket": c.Bucket, "error": err.Error(),
					})
				}
			}()
		}
	} else {
		store, err = newLocalMediaStore(localMediaRoot(), mediaSigningKey())
	}
	if err != nil {
		mediaStoreProblem = err.Error()
		LogErrorSimple(LogCategorySystem, "Media storage is unavailable; video is disabled", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	mediaStore = store
	LogInfo(LogCategorySystem, "Media storage ready", map[string]interface{}{"backend": store.Name()})
}
