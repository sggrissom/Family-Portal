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
	_ "golang.org/x/image/webp"
	_ "image/gif"
)

type ImageSize struct {
	Name      string
	MaxWidth  int
	MaxHeight int
	Quality   int
}

var (
	SmallSize     = ImageSize{Name: "small", MaxWidth: 150, MaxHeight: 150, Quality: 60}
	ThumbnailSize = ImageSize{Name: "thumb", MaxWidth: 300, MaxHeight: 300, Quality: 65}
	MediumSize    = ImageSize{Name: "medium", MaxWidth: 600, MaxHeight: 600, Quality: 75}
	LargeSize     = ImageSize{Name: "large", MaxWidth: 900, MaxHeight: 900, Quality: 80}
	XLargeSize    = ImageSize{Name: "xlarge", MaxWidth: 1200, MaxHeight: 1200, Quality: 85}
	XXLargeSize   = ImageSize{Name: "xxlarge", MaxWidth: 1800, MaxHeight: 1800, Quality: 90}
)

func ProcessImage(reader io.Reader, mimeType string, size ImageSize, outputFormat string) ([]byte, int, int, error) {
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	newWidth, newHeight := calculateDimensions(width, height, size.MaxWidth, size.MaxHeight)

	if width != newWidth || height != newHeight {
		img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
		width = newWidth
		height = newHeight
	}

	switch outputFormat {
	case "webp":
		return compressWebP(img, size.Quality)
	case "avif":
		return compressAVIF(img, size.Quality)
	case "png":
		return compressPNG(img)
	default:
		return compressJPEG(img, size.Quality)
	}
}

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

func compressJPEG(img image.Image, quality int) ([]byte, int, int, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{
		Quality: quality,
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

func compressPNG(img image.Image) ([]byte, int, int, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode PNG: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

func compressWebP(img image.Image, quality int) ([]byte, int, int, error) {
	return compressJPEG(img, quality)
}

func compressAVIF(img image.Image, quality int) ([]byte, int, int, error) {
	var buf bytes.Buffer
	err := avif.Encode(&buf, img, avif.Options{Quality: quality})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode AVIF: %w", err)
	}

	bounds := img.Bounds()
	return buf.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

func ProcessAndSaveMultipleSizes(imageData []byte, mimeType string) (map[string][]byte, int, int, error) {
	results := make(map[string][]byte)

	sizes := []ImageSize{ThumbnailSize, MediumSize, LargeSize, XLargeSize}
	formats := []string{"jpeg", "webp", "avif"}

	var width, height int

	for _, size := range sizes {
		for _, format := range formats {
			data, w, h, err := ProcessImage(bytes.NewReader(imageData), mimeType, size, format)
			if err != nil {
				continue
			}

			if width == 0 && height == 0 {
				width, height = w, h
			}

			key := size.Name + "_" + format
			results[key] = data
		}
	}

	if width == 0 && height == 0 {
		img, err := imaging.Decode(bytes.NewReader(imageData), imaging.AutoOrientation(true))
		if err == nil {
			bounds := img.Bounds()
			width = bounds.Dx()
			height = bounds.Dy()
		}
		results["large_jpeg"] = imageData
	}

	return results, width, height, nil
}

func GetOptimalImageFormat(acceptHeader string) string {
	if strings.Contains(acceptHeader, "image/avif") {
		return "avif"
	}
	if strings.Contains(acceptHeader, "image/webp") {
		return "webp"
	}
	return "jpeg"
}

func GetImageMimeType(format string) string {
	switch format {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "jpeg", "jpg":
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}
