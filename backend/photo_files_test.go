package backend

import (
	"family/cfg"
	"os"
	"path/filepath"
	"testing"
)

func TestPhotoFileNames(t *testing.T) {
	filePath := "photos/abc.png"
	base := filepath.Join(cfg.StaticDir, "photos", "abc")
	cases := map[string]string{
		photoOriginalPath(filePath):                     base + "_original.png",
		photoVariantPath(filePath, "large", "jpeg"):     base + ".jpg",
		photoVariantPath(filePath, "medium", "webp"):    base + "_medium.webp",
		photoVariantPath(filePath, "xxlarge", "avif"):   base + "_xxlarge.avif",
		filePathForOriginal(originalFileName(filePath)): filePath,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestWriteAndRemovePhotoFiles(t *testing.T) {
	filePath := "photos/files-test.jpg"
	written, err := writeImageVariants(filePath, map[string][]byte{
		"thumb_webp": []byte("a"),
		"large_jpeg": []byte("bb"),
		"not-a-key":  []byte("ignored"),
	})
	if err != nil || written != 3 {
		t.Fatalf("writeImageVariants = %d, %v", written, err)
	}
	if err := writePhotoOriginal(filePath, []byte("orig")); err != nil {
		t.Fatal(err)
	}
	if !hasModernVariant(filePath) {
		t.Error("a WebP variant should count as processed")
	}
	if got := photoSourcePath(filePath, "medium", "large"); got != photoVariantPath(filePath, "large", "jpeg") {
		t.Errorf("photoSourcePath = %q", got)
	}

	if err := removePhotoVariants(filePath); err != nil {
		t.Fatal(err)
	}
	if hasModernVariant(filePath) {
		t.Error("variants survived removePhotoVariants")
	}
	if _, err := os.Stat(photoOriginalPath(filePath)); err != nil {
		t.Errorf("removePhotoVariants removed the original: %v", err)
	}

	if err := deletePhotoFiles(Image{FilePath: filePath}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(photoOriginalPath(filePath)); !os.IsNotExist(err) {
		t.Errorf("original survived deletePhotoFiles: %v", err)
	}
}
