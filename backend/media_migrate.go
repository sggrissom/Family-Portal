package backend

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type MediaMigrationResult struct {
	Copied       int
	Skipped      int
	Failed       int
	Bytes        int64
	LocalDeleted int
	Failures     []string
}

// MigrateLocalMedia copies every object under the local media root into dst,
// keeping its key. It is safe to run again: objects already in dst at the
// same size are skipped. With deleteLocal, a local file is removed only after
// dst reports it at the full size.
func MigrateLocalMedia(ctx context.Context, root string, dst MediaStore, deleteLocal bool) (result MediaMigrationResult, err error) {
	root = filepath.Clean(root)
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if !validMediaKey(key) {
			result.Failed++
			result.Failures = append(result.Failures, key+": not a media key")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		existing, statErr := dst.Stat(ctx, key)
		switch {
		case statErr == nil && existing.Size == info.Size():
			result.Skipped++
		case statErr != nil && !errors.Is(statErr, ErrMediaNotFound):
			result.Failed++
			result.Failures = append(result.Failures, key+": "+statErr.Error())
			return nil
		default:
			if err := putFile(ctx, dst, key, p, mediaContentType(key)); err != nil {
				result.Failed++
				result.Failures = append(result.Failures, key+": "+err.Error())
				return nil
			}
			if copied, err := dst.Stat(ctx, key); err != nil || copied.Size != info.Size() {
				result.Failed++
				result.Failures = append(result.Failures, key+": size differs after copy")
				return nil
			}
			result.Copied++
			result.Bytes += info.Size()
		}

		if deleteLocal {
			if err := os.Remove(p); err == nil {
				result.LocalDeleted++
			}
		}
		return nil
	})
	return result, walkErr
}

func mediaContentType(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".mov":
		return "video/quicktime"
	case ".m4v":
		return "video/x-m4v"
	case ".mp4":
		return "video/mp4"
	}
	return "application/octet-stream"
}
