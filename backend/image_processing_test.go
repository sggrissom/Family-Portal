package backend

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

func createTestImageWithSize(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestCalculateDimensions(t *testing.T) {
	testCases := []struct {
		name                          string
		width, height                 int
		maxWidth, maxHeight           int
		expectedWidth, expectedHeight int
	}{
		{
			name:  "No resize needed",
			width: 100, height: 80,
			maxWidth: 200, maxHeight: 200,
			expectedWidth: 100, expectedHeight: 80,
		},
		{
			name:  "Resize by width",
			width: 800, height: 600,
			maxWidth: 400, maxHeight: 600,
			expectedWidth: 400, expectedHeight: 300,
		},
		{
			name:  "Resize by height",
			width: 600, height: 800,
			maxWidth: 600, maxHeight: 400,
			expectedWidth: 300, expectedHeight: 400,
		},
		{
			name:  "Square image resize",
			width: 1000, height: 1000,
			maxWidth: 500, maxHeight: 500,
			expectedWidth: 500, expectedHeight: 500,
		},
		{
			name:  "Portrait aspect ratio",
			width: 400, height: 600,
			maxWidth: 200, maxHeight: 200,
			expectedWidth: 133, expectedHeight: 200,
		},
		{
			name:  "Landscape aspect ratio",
			width: 600, height: 400,
			maxWidth: 200, maxHeight: 200,
			expectedWidth: 200, expectedHeight: 133,
		},
		{
			name:  "Very wide image",
			width: 1200, height: 300,
			maxWidth: 400, maxHeight: 400,
			expectedWidth: 400, expectedHeight: 100,
		},
		{
			name:  "Very tall image",
			width: 300, height: 1200,
			maxWidth: 400, maxHeight: 400,
			expectedWidth: 100, expectedHeight: 400,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newWidth, newHeight := calculateDimensions(tc.width, tc.height, tc.maxWidth, tc.maxHeight)

			if newWidth != tc.expectedWidth {
				t.Errorf("Expected width %d, got %d", tc.expectedWidth, newWidth)
			}
			if newHeight != tc.expectedHeight {
				t.Errorf("Expected height %d, got %d", tc.expectedHeight, newHeight)
			}

			originalRatio := float64(tc.width) / float64(tc.height)
			newRatio := float64(newWidth) / float64(newHeight)
			tolerance := 0.01

			if newWidth < tc.maxWidth && newHeight < tc.maxHeight {
				if tc.width > tc.maxWidth || tc.height > tc.maxHeight {
					if newWidth != tc.maxWidth && newHeight != tc.maxHeight {
						t.Error("At least one dimension should hit the maximum")
					}
				}
			}

			if math.Abs(originalRatio-newRatio) > tolerance {
				t.Errorf("Aspect ratio not preserved: original %f, new %f", originalRatio, newRatio)
			}
		})
	}
}

func TestProcessImage(t *testing.T) {
	testImageData := createTestImageWithSize(400, 300)

	t.Run("JPEG compression", func(t *testing.T) {
		reader := bytes.NewReader(testImageData)

		result, width, height, err := ProcessImage(reader, "image/png", MediumSize, "jpeg")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected non-empty result")
		}

		if width != 400 {
			t.Errorf("Expected width 400, got %d", width)
		}
		if height != 300 {
			t.Errorf("Expected height 300, got %d", height)
		}
	})

	t.Run("PNG compression", func(t *testing.T) {
		reader := bytes.NewReader(testImageData)

		result, width, height, err := ProcessImage(reader, "image/png", MediumSize, "png")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected non-empty result")
		}

		if width != 400 || height != 300 {
			t.Errorf("Expected dimensions 400x300, got %dx%d", width, height)
		}
	})

	t.Run("Resize large image", func(t *testing.T) {
		largeImageData := createTestImageWithSize(1200, 800)
		reader := bytes.NewReader(largeImageData)

		result, width, height, err := ProcessImage(reader, "image/png", ThumbnailSize, "jpeg")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected non-empty result")
		}

		if width > ThumbnailSize.MaxWidth {
			t.Errorf("Width %d exceeds max width %d", width, ThumbnailSize.MaxWidth)
		}
		if height > ThumbnailSize.MaxHeight {
			t.Errorf("Height %d exceeds max height %d", height, ThumbnailSize.MaxHeight)
		}

		expectedWidth := 300
		expectedHeight := 200
		if width != expectedWidth || height != expectedHeight {
			t.Errorf("Expected dimensions %dx%d, got %dx%d", expectedWidth, expectedHeight, width, height)
		}
	})

	t.Run("Invalid image data", func(t *testing.T) {
		invalidData := []byte("not an image")
		reader := bytes.NewReader(invalidData)

		_, _, _, err := ProcessImage(reader, "image/png", MediumSize, "jpeg")
		if err == nil {
			t.Error("Expected error for invalid image data")
		}

		if !strings.Contains(err.Error(), "failed to decode image") {
			t.Errorf("Expected decode error, got %v", err)
		}
	})

	t.Run("Default to JPEG for unknown format", func(t *testing.T) {
		reader := bytes.NewReader(testImageData)

		result, _, _, err := ProcessImage(reader, "image/png", SmallSize, "unknown_format")
		if err != nil {
			t.Errorf("Expected no error with unknown format, got %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected non-empty result")
		}
	})
}

func TestProcessAndSaveMultipleSizes(t *testing.T) {
	testImageData := createTestImageWithSize(800, 600)

	t.Run("Generate multiple sizes and formats", func(t *testing.T) {
		results, width, height, err := ProcessAndSaveMultipleSizes(testImageData, "image/png")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if width <= 0 || height <= 0 {
			t.Errorf("Expected positive dimensions, got %dx%d", width, height)
		}

		if len(results) == 0 {
			t.Error("Expected multiple results")
		}

		expectedCombinations := []string{
			"thumb_jpeg", "thumb_webp", "thumb_avif",
			"medium_jpeg", "medium_webp", "medium_avif",
			"large_jpeg", "large_webp", "large_avif",
			"xlarge_jpeg", "xlarge_webp", "xlarge_avif",
		}

		foundCombinations := 0
		for _, combo := range expectedCombinations {
			if data, exists := results[combo]; exists && len(data) > 0 {
				foundCombinations++
			}
		}

		if foundCombinations != 12 {
			t.Errorf("Expected 12 size/format combinations, got %d", foundCombinations)
		}

		for key, data := range results {
			if len(data) == 0 {
				t.Errorf("Result for %s is empty", key)
			}
		}
	})

	t.Run("Handle invalid image data gracefully", func(t *testing.T) {
		invalidData := []byte("not an image")

		results, width, height, err := ProcessAndSaveMultipleSizes(invalidData, "image/png")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected at least fallback result")
		}

		if width == 0 && height == 0 {
			if _, exists := results["large_jpeg"]; !exists {
				t.Error("Expected fallback result when processing fails")
			}
		}
	})
}

func TestGetOptimalImageFormat(t *testing.T) {
	testCases := []struct {
		acceptHeader string
		expected     string
	}{
		{
			acceptHeader: "text/html,application/xhtml+xml,image/avif,image/webp,*/*",
			expected:     "avif",
		},
		{
			acceptHeader: "text/html,application/xhtml+xml,image/webp,*/*",
			expected:     "webp",
		},
		{
			acceptHeader: "text/html,application/xhtml+xml,image/png,image/jpeg,*/*",
			expected:     "jpeg",
		},
		{
			acceptHeader: "image/webp,image/avif,*/*",
			expected:     "avif",
		},
		{
			acceptHeader: "image/png,image/webp,*/*",
			expected:     "webp",
		},
		{
			acceptHeader: "text/html,*/*",
			expected:     "jpeg",
		},
		{
			acceptHeader: "",
			expected:     "jpeg",
		},
		{
			acceptHeader: "application/json",
			expected:     "jpeg",
		},
	}

	for _, tc := range testCases {
		t.Run("Accept: "+tc.acceptHeader, func(t *testing.T) {
			result := GetOptimalImageFormat(tc.acceptHeader)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestGetImageMimeType(t *testing.T) {
	testCases := []struct {
		format   string
		expected string
	}{
		{"webp", "image/webp"},
		{"avif", "image/avif"},
		{"png", "image/png"},
		{"jpeg", "image/jpeg"},
		{"jpg", "image/jpeg"},
		{"gif", "image/gif"},
		{"unknown", "image/jpeg"},
		{"", "image/jpeg"},
	}

	for _, tc := range testCases {
		t.Run("Format: "+tc.format, func(t *testing.T) {
			result := GetImageMimeType(tc.format)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestImageSizeDefinitions(t *testing.T) {
	sizes := []ImageSize{
		ThumbnailSize, MediumSize,
		LargeSize, XLargeSize,
	}

	for _, size := range sizes {
		t.Run("Size: "+size.Name, func(t *testing.T) {
			if size.Name == "" {
				t.Error("Size name should not be empty")
			}
			if size.MaxWidth <= 0 {
				t.Error("Max width should be positive")
			}
			if size.MaxHeight <= 0 {
				t.Error("Max height should be positive")
			}
			if size.Quality <= 0 || size.Quality > 100 {
				t.Errorf("Quality should be between 1 and 100, got %d", size.Quality)
			}
		})
	}

	t.Run("Sizes in ascending order", func(t *testing.T) {
		if ThumbnailSize.MaxWidth >= MediumSize.MaxWidth {
			t.Error("Thumbnail size should be smaller than medium")
		}
		if MediumSize.MaxWidth >= LargeSize.MaxWidth {
			t.Error("Medium size should be smaller than large")
		}
		if LargeSize.MaxWidth >= XLargeSize.MaxWidth {
			t.Error("Large size should be smaller than xlarge")
		}
	})

	t.Run("Quality increases with size", func(t *testing.T) {
		if ThumbnailSize.Quality > LargeSize.Quality {
			t.Error("Thumbnail size quality should not exceed large size quality")
		}
		if MediumSize.Quality > XLargeSize.Quality {
			t.Error("Medium quality should not exceed xlarge quality")
		}
	})
}

func TestImageSizeStruct(t *testing.T) {
	customSize := ImageSize{
		Name:      "custom",
		MaxWidth:  400,
		MaxHeight: 300,
		Quality:   70,
	}

	if customSize.Name != "custom" {
		t.Errorf("Expected name 'custom', got '%s'", customSize.Name)
	}
	if customSize.MaxWidth != 400 {
		t.Errorf("Expected max width 400, got %d", customSize.MaxWidth)
	}
	if customSize.MaxHeight != 300 {
		t.Errorf("Expected max height 300, got %d", customSize.MaxHeight)
	}
	if customSize.Quality != 70 {
		t.Errorf("Expected quality 70, got %d", customSize.Quality)
	}
}

func TestCompressFunctions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	t.Run("JPEG compression", func(t *testing.T) {
		result, width, height, err := compressJPEG(img, 80)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected non-empty result")
		}
		if width != 100 || height != 100 {
			t.Errorf("Expected dimensions 100x100, got %dx%d", width, height)
		}
	})

	t.Run("PNG compression", func(t *testing.T) {
		result, width, height, err := compressPNG(img)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if len(result) == 0 {
			t.Error("Expected non-empty result")
		}
		if width != 100 || height != 100 {
			t.Errorf("Expected dimensions 100x100, got %dx%d", width, height)
		}
	})

	t.Run("Different JPEG quality levels", func(t *testing.T) {
		lowQuality, _, _, err1 := compressJPEG(img, 30)
		highQuality, _, _, err2 := compressJPEG(img, 95)

		if err1 != nil || err2 != nil {
			t.Fatalf("Expected no errors, got %v, %v", err1, err2)
		}

		if len(lowQuality) == 0 || len(highQuality) == 0 {
			t.Error("Expected non-empty results for both quality levels")
		}
	})
}
