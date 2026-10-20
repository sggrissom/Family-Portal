package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// The S3 API is how both Cloudflare R2 and Backblaze B2 are reached, so one
// client covers either; which one is a matter of these settings.
var mediaS3EnvVars = []string{
	"MEDIA_S3_ENDPOINT",
	"MEDIA_S3_REGION",
	"MEDIA_S3_BUCKET",
	"MEDIA_S3_ACCESS_KEY_ID",
	"MEDIA_S3_SECRET_ACCESS_KEY",
}

type s3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Insecure  bool
}

func s3ConfigFromEnv() (s3Config, bool) {
	c := s3Config{
		Endpoint:  os.Getenv("MEDIA_S3_ENDPOINT"),
		Region:    os.Getenv("MEDIA_S3_REGION"),
		Bucket:    os.Getenv("MEDIA_S3_BUCKET"),
		AccessKey: os.Getenv("MEDIA_S3_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("MEDIA_S3_SECRET_ACCESS_KEY"),
	}
	return c, c.Endpoint != "" || c.Bucket != "" || c.AccessKey != "" || c.SecretKey != ""
}

func (c s3Config) problems() []string {
	var missing []string
	for name, value := range map[string]string{
		"MEDIA_S3_ENDPOINT": c.Endpoint, "MEDIA_S3_REGION": c.Region, "MEDIA_S3_BUCKET": c.Bucket,
		"MEDIA_S3_ACCESS_KEY_ID": c.AccessKey, "MEDIA_S3_SECRET_ACCESS_KEY": c.SecretKey,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if strings.Contains(c.Endpoint, "://") || strings.Contains(c.Endpoint, "/") {
		missing = append(missing, "MEDIA_S3_ENDPOINT (a host name, without https:// or a path)")
	}
	return missing
}

// mediaOrigin is where the browser fetches media from when it is not the app
// itself, so the content security policy can allow it.
var mediaOrigin string

func (c s3Config) origin() string {
	if c.Insecure {
		return "http://" + c.Endpoint
	}
	return "https://" + c.Endpoint
}

type s3MediaStore struct {
	client *minio.Client
	bucket string
}

func newS3MediaStore(c s3Config) (*s3MediaStore, error) {
	if problems := c.problems(); len(problems) > 0 {
		return nil, fmt.Errorf("incomplete S3 settings: %s", strings.Join(problems, ", "))
	}
	client, err := minio.New(c.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""),
		Secure:       !c.Insecure,
		Region:       c.Region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, err
	}
	return &s3MediaStore{client: client, bucket: c.Bucket}, nil
}

func (s *s3MediaStore) Name() string { return "s3:" + s.bucket }

func s3NotFound(err error) bool {
	code := minio.ToErrorResponse(err).Code
	return code == "NoSuchKey" || code == "NotFound" || minio.ToErrorResponse(err).StatusCode == http.StatusNotFound
}

func (s *s3MediaStore) UploadURL(key string, size int64, contentType string, ttl time.Duration) (UploadInstructions, error) {
	if !validMediaKey(key) {
		return UploadInstructions{}, errMediaKeyInvalid
	}
	headers := http.Header{"Content-Type": {contentType}}
	u, err := s.client.PresignHeader(context.Background(), http.MethodPut, s.bucket, key, ttl, nil, headers)
	if err != nil {
		return UploadInstructions{}, err
	}
	return UploadInstructions{
		URL:       u.String(),
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Now().Add(ttl).Truncate(time.Second),
	}, nil
}

func (s *s3MediaStore) PlaybackURL(key string, opts MediaURLOptions) (string, time.Time, error) {
	if !validMediaKey(key) {
		return "", time.Time{}, errMediaKeyInvalid
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = mediaPlaybackTTL
	}
	params := url.Values{}
	if opts.ContentType != "" {
		params.Set("response-content-type", opts.ContentType)
	}
	if opts.DownloadName != "" {
		params.Set("response-content-disposition", mime.FormatMediaType("attachment", map[string]string{"filename": opts.DownloadName}))
	}
	u, err := s.client.PresignedGetObject(context.Background(), s.bucket, key, ttl, params)
	if err != nil {
		return "", time.Time{}, err
	}
	return u.String(), time.Now().Add(ttl).Truncate(time.Second), nil
}

func (s *s3MediaStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if !validMediaKey(key) {
		return nil, errMediaKeyInvalid
	}
	if _, err := s.Stat(ctx, key); err != nil {
		return nil, err
	}
	return s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
}

func (s *s3MediaStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if !validMediaKey(key) {
		return errMediaKeyInvalid
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *s3MediaStore) Stat(ctx context.Context, key string) (MediaObject, error) {
	if !validMediaKey(key) {
		return MediaObject{}, errMediaKeyInvalid
	}
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if s3NotFound(err) {
			return MediaObject{}, ErrMediaNotFound
		}
		return MediaObject{}, err
	}
	return MediaObject{Size: info.Size}, nil
}

func (s *s3MediaStore) Delete(ctx context.Context, key string) error {
	if !validMediaKey(key) {
		return errMediaKeyInvalid
	}
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil && s3NotFound(err) {
		return nil
	}
	return err
}

// Move copies inside the bucket, so nothing large passes through the server.
func (s *s3MediaStore) Move(ctx context.Context, src, dst string) error {
	if !validMediaKey(src) || !validMediaKey(dst) {
		return errMediaKeyInvalid
	}
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dst},
		minio.CopySrcOptions{Bucket: s.bucket, Object: src})
	if err != nil {
		if s3NotFound(err) {
			return ErrMediaNotFound
		}
		return err
	}
	return s.Delete(ctx, src)
}

// checkReachable confirms the bucket answers with these credentials. A
// failure is logged, not fatal: video degrades and the app keeps running.
func (s *s3MediaStore) checkReachable(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("bucket does not exist")
	}
	return nil
}

// NewConfiguredS3Store builds the bucket store from MEDIA_S3_* and checks it
// answers, for tools that must not proceed against a bucket they cannot reach.
func NewConfiguredS3Store() (MediaStore, error) {
	c, configured := s3ConfigFromEnv()
	if !configured {
		return nil, errors.New("MEDIA_S3_* is not set")
	}
	store, err := newS3MediaStore(c)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.checkReachable(ctx); err != nil {
		return nil, fmt.Errorf("bucket %s: %w", c.Bucket, err)
	}
	return store, nil
}
