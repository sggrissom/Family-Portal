package backend

import (
	"family/cfg"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A photo's files sit beside Image.FilePath (relative to StaticDir). The
// upload is kept as <base>_original<ext>; each resized variant is
// <base>_<size>.<ext>, except the large size, which has no suffix.

// photoSizes are every size a variant may be stored or requested at.
var photoSizes = []ImageSize{SmallSize, ThumbnailSize, MediumSize, LargeSize, XLargeSize, XXLargeSize}

// photoFormats are the encodings variants are written in, best first.
var photoFormats = []string{"avif", "webp", "jpeg"}

func isPhotoSize(name string) bool {
	for _, size := range photoSizes {
		if size.Name == name {
			return true
		}
	}
	return false
}

func photoFormatFile(format string) (ext, contentType string) {
	switch format {
	case "avif":
		return ".avif", "image/avif"
	case "webp":
		return ".webp", "image/webp"
	case "png":
		return ".png", "image/png"
	default:
		return ".jpg", "image/jpeg"
	}
}

func photoBasePath(filePath string) string {
	full := filepath.Join(cfg.StaticDir, filePath)
	return strings.TrimSuffix(full, filepath.Ext(full))
}

func originalFileName(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "_original" + ext
}

func isOriginalFileName(name string) bool {
	return strings.Contains(name, "_original.")
}

// filePathForOriginal is the Image.FilePath an original's file name belongs to.
func filePathForOriginal(name string) string {
	ext := filepath.Ext(name)
	return "photos/" + strings.TrimSuffix(strings.TrimSuffix(name, ext), "_original") + ext
}

func originalPathIn(staticDir, filePath string) string {
	return filepath.Join(staticDir, filepath.Dir(filePath), originalFileName(filePath))
}

func photoOriginalPath(filePath string) string {
	return originalPathIn(cfg.StaticDir, filePath)
}

func photoVariantPath(filePath, size, format string) string {
	ext, _ := photoFormatFile(format)
	if size == LargeSize.Name {
		return photoBasePath(filePath) + ext
	}
	return photoBasePath(filePath) + "_" + size + ext
}

// photoSourcePath is the first JPEG variant on disk among sizes, or "".
func photoSourcePath(filePath string, sizes ...string) string {
	for _, size := range sizes {
		path := photoVariantPath(filePath, size, "jpeg")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func writePhotoOriginal(filePath string, data []byte) error {
	path := photoOriginalPath(filePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// writeImageVariants writes ProcessAndSaveMultipleSizes output beside
// filePath and returns the bytes written.
func writeImageVariants(filePath string, processedImages map[string][]byte) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(photoBasePath(filePath)), 0755); err != nil {
		return 0, fmt.Errorf("failed to create photos directory: %w", err)
	}

	var written int64
	for key, data := range processedImages {
		size, format, ok := strings.Cut(key, "_")
		if !ok {
			continue
		}
		path := photoVariantPath(filePath, size, format)
		if err := os.WriteFile(path, data, 0644); err != nil {
			return written, fmt.Errorf("failed to write file %s: %w", path, err)
		}
		written += int64(len(data))
	}
	return written, nil
}

// hasModernVariant reports whether any AVIF or WebP variant exists, which is
// how a processed photo is told from one that still needs processing.
func hasModernVariant(filePath string) bool {
	for _, size := range photoSizes {
		for _, format := range []string{"avif", "webp"} {
			if _, err := os.Stat(photoVariantPath(filePath, size.Name, format)); err == nil {
				return true
			}
		}
	}
	return false
}

// removePhotoVariants deletes every variant, keeping the original.
func removePhotoVariants(filePath string) error {
	var lastError error
	for _, size := range photoSizes {
		for _, format := range append(photoFormats, "png") {
			if err := os.Remove(photoVariantPath(filePath, size.Name, format)); err != nil && !os.IsNotExist(err) {
				lastError = err
			}
		}
	}
	return lastError
}

func deletePhotoFiles(photo Image) error {
	deleteVideoFrameFiles(photo)
	lastError := removePhotoVariants(photo.FilePath)
	if err := os.Remove(photoOriginalPath(photo.FilePath)); err != nil && !os.IsNotExist(err) {
		lastError = err
	}
	return lastError
}
