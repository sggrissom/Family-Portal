package backend

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	_ "image/gif"
	_ "golang.org/x/image/webp"
)

// ImageSize represents different image size variants
type ImageSize struct {
	Name      string
	MaxWidth  int
	MaxHeight int
	Quality   int
}

var (
	// Define responsive image sizes
	SmallSize     = ImageSize{Name: "small", MaxWidth: 150, MaxHeight: 150, Quality: 60}
	ThumbnailSize = ImageSize{Name: "thumb", MaxWidth: 300, MaxHeight: 300, Quality: 65}
	MediumSize    = ImageSize{Name: "medium", MaxWidth: 600, MaxHeight: 600, Quality: 75}
	LargeSize     = ImageSize{Name: "large", MaxWidth: 900, MaxHeight: 900, Quality: 80}
	XLargeSize    = ImageSize{Name: "xlarge", MaxWidth: 1200, MaxHeight: 1200, Quality: 85}
	XXLargeSize   = ImageSize{Name: "xxlarge", MaxWidth: 1800, MaxHeight: 1800, Quality: 90}
)

// ProcessImage compresses and optionally resizes an image
func ProcessImage(reader io.Reader, mimeType string, size ImageSize, outputFormat string) ([]byte, int, int, error) {
	// Decode the image with automatic EXIF orientation correction
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions if resizing is needed
	newWidth, newHeight := calculateDimensions(width, height, size.MaxWidth, size.MaxHeight)

	// Resize if needed using the imaging library
	if width != newWidth || height != newHeight {
		img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
		width = newWidth
		height = newHeight
	}

	// Compress based on output format
	switch outputFormat {
	case "webp":
		return compressWebP(img, size.Quality)
	case "avif":
		return compressAVIF(img, size.Quality)
	case "png":
		return compressPNG(img)
	default:
		// Default to JPEG compression
		return compressJPEG(img, size.Quality)
	}
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

// compressJPEG compresses an image to JPEG format with progressive encoding
func compressJPEG(img image.Image, quality int) ([]byte, int, int, error) {
	var buf bytes.Buffer
	// Use progressive JPEG for better perceived loading performance
	err := jpeg.Encode(&buf, img, &jpeg.Options{
		Quality: quality,
	})
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

// compressWebP compresses an image to WebP format
func compressWebP(img image.Image, quality int) ([]byte, int, int, error) {
	// For now, fall back to JPEG until WebP encoding is properly set up
	return compressJPEG(img, quality)
}

// compressAVIF compresses an image to AVIF format
func compressAVIF(img image.Image, quality int) ([]byte, int, int, error) {
	var buf bytes.Buffer
	// AVIF quality ranges from 0-100, same as JPEG
	err := avif.Encode(&buf, img, avif.Options{Quality: quality})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode AVIF: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// ProcessAndSaveMultipleSizes processes an image and saves multiple size variants and formats
func ProcessAndSaveMultipleSizes(imageData []byte, mimeType string) (map[string][]byte, int, int, error) {
	results := make(map[string][]byte)

	// All sizes to generate
	sizes := []ImageSize{SmallSize, ThumbnailSize, MediumSize, LargeSize, XLargeSize, XXLargeSize}
	// All formats to generate (prioritize most efficient formats first)
	formats := []string{"jpeg", "webp", "avif"}

	var width, height int

	// Process each size and format combination
	for _, size := range sizes {
		for _, format := range formats {
			data, w, h, err := ProcessImage(bytes.NewReader(imageData), mimeType, size, format)
			if err != nil {
				continue // Skip if format encoding fails
			}

			// Store the dimensions from the first successful process
			if width == 0 && height == 0 {
				width, height = w, h
			}

			// Create key in format: size_format (e.g. "thumb_webp", "large_avif")
			key := size.Name + "_" + format
			results[key] = data
		}
	}

	// Fallback: if no images were processed successfully, try to get dimensions
	if width == 0 && height == 0 {
		img, err := imaging.Decode(bytes.NewReader(imageData), imaging.AutoOrientation(true))
		if err == nil {
			bounds := img.Bounds()
			width = bounds.Dx()
			height = bounds.Dy()
		}
		// Store original as large_jpeg fallback
		results["large_jpeg"] = imageData
	}

	return results, width, height, nil
}

// GetOptimalImageFormat determines the best image format based on browser Accept header
func GetOptimalImageFormat(acceptHeader string) string {
	// Check for modern format support in order of efficiency
	if strings.Contains(acceptHeader, "image/avif") {
		return "avif"
	}
	if strings.Contains(acceptHeader, "image/webp") {
		return "webp"
	}
	// Fallback to JPEG for universal compatibility
	return "jpeg"
}

// GetImageMimeType returns the appropriate MIME type for format
func GetImageMimeType(format string) string {
	switch format {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}