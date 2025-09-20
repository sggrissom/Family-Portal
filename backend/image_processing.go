package backend

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	_ "image/gif"
)

// ImageSize represents different image size variants
type ImageSize struct {
	Name      string
	MaxWidth  int
	MaxHeight int
	Quality   int
}

var (
	// Define standard image sizes
	ThumbnailSize = ImageSize{Name: "thumb", MaxWidth: 400, MaxHeight: 400, Quality: 75}
	MediumSize    = ImageSize{Name: "medium", MaxWidth: 1024, MaxHeight: 1024, Quality: 85}
	LargeSize     = ImageSize{Name: "large", MaxWidth: 2048, MaxHeight: 2048, Quality: 90}
)

// ProcessImage compresses and optionally resizes an image
func ProcessImage(reader io.Reader, mimeType string, size ImageSize) ([]byte, int, int, error) {
	// Decode the image
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions if resizing is needed
	newWidth, newHeight := calculateDimensions(width, height, size.MaxWidth, size.MaxHeight)

	// If no resizing needed and format is already JPEG, just recompress
	if width == newWidth && height == newHeight && format == "jpeg" {
		return compressJPEG(img, size.Quality)
	}

	// For now, we'll just recompress without resizing
	// In production, you'd want to use a proper image resizing library like github.com/disintegration/imaging
	if format == "png" {
		return compressPNG(img)
	}

	// Default to JPEG compression for better file sizes
	return compressJPEG(img, size.Quality)
}

// calculateDimensions calculates new dimensions while maintaining aspect ratio
func calculateDimensions(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}

	aspectRatio := float64(width) / float64(height)

	newWidth := maxWidth
	newHeight := int(float64(maxWidth) / aspectRatio)

	if newHeight > maxHeight {
		newHeight = maxHeight
		newWidth = int(float64(maxHeight) * aspectRatio)
	}

	return newWidth, newHeight
}

// compressJPEG compresses an image to JPEG format
func compressJPEG(img image.Image, quality int) ([]byte, int, int, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// compressPNG compresses an image to PNG format
func compressPNG(img image.Image) ([]byte, int, int, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode PNG: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// ProcessAndSaveMultipleSizes processes an image and saves multiple size variants
func ProcessAndSaveMultipleSizes(imageData []byte, mimeType string) (map[string][]byte, int, int, error) {
	results := make(map[string][]byte)

	// Process original/large size
	largeData, width, height, err := ProcessImage(bytes.NewReader(imageData), mimeType, LargeSize)
	if err != nil {
		// Fall back to original if processing fails
		largeData = imageData
		// Try to get dimensions from original
		img, _, err := image.Decode(bytes.NewReader(imageData))
		if err == nil {
			bounds := img.Bounds()
			width = bounds.Dx()
			height = bounds.Dy()
		}
	}
	results["large"] = largeData

	// Process medium size
	mediumData, _, _, err := ProcessImage(bytes.NewReader(imageData), mimeType, MediumSize)
	if err == nil {
		results["medium"] = mediumData
	}

	// Process thumbnail
	thumbData, _, _, err := ProcessImage(bytes.NewReader(imageData), mimeType, ThumbnailSize)
	if err == nil {
		results["thumb"] = thumbData
	}

	return results, width, height, nil
}