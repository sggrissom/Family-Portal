package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestBundleImportedPhotosAreProcessed(t *testing.T) {
	fx := setupMatrixFixture(t)
	source := "photos/bundle-source.png"
	if err := writePhotoOriginal(source, createTestImage(120, 80)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deletePhotoFiles(Image{FilePath: source}) })
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fx.photo.Status = PhotoReady
		fx.photo.FilePath = source
		fx.photo.MimeType = "image/png"
		writeImage(tx, &fx.photo)
		vbolt.TxCommit(tx)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/export-bundle?mode=with_photos", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, fx.owner))
	rec := httptest.NewRecorder()
	exportBundleHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d %s", rec.Code, rec.Body.String())
	}
	bundle := rec.Body.Bytes()

	previous := globalPhotoWorker
	globalPhotoWorker = &PhotoWorker{db: fx.db, jobQueue: make(chan PhotoProcessingJob, 10)}
	t.Cleanup(func() { globalPhotoWorker = previous })

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "family.zip")
	io.Copy(part, bytes.NewReader(bundle))
	form.Close()
	req = httptest.NewRequest(http.MethodPost, "/api/import-bundle", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, fx.owner))
	rec = httptest.NewRecorder()
	importBundleHandler(rec, req)
	var resp ImportDataResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if rec.Code != http.StatusOK || resp.ImportedPhotos != 1 {
		t.Fatalf("import = %d %+v", rec.Code, resp)
	}

	if len(globalPhotoWorker.jobQueue) != 1 {
		t.Fatalf("%d photos queued for processing, want 1", len(globalPhotoWorker.jobQueue))
	}
	job := <-globalPhotoWorker.jobQueue
	t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	if job.FilePath == source {
		t.Fatal("the imported photo shares its files with the photo it was exported from")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetImageById(tx, job.ImageId).Status; got != PhotoProcessing {
			t.Errorf("imported photo status = %d before processing, want processing", got)
		}
	})

	globalPhotoWorker.processPhotoJob(job)

	var imported Image
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { imported = GetImageById(tx, job.ImageId) })
	if imported.Status != PhotoReady || imported.Width == 0 || imported.Height == 0 {
		t.Errorf("after processing: status %d, %dx%d", imported.Status, imported.Width, imported.Height)
	}
	if !hasModernVariant(imported.FilePath) {
		t.Error("the imported photo has no resized variants")
	}
	if _, err := os.Stat(photoOriginalPath(source)); err != nil {
		t.Errorf("the source photo's original is gone: %v", err)
	}
}
