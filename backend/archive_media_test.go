package backend

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
)

func (fx *archiveFixture) uploadPhoto(t *testing.T, user User, filename string, data []byte) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	form.WriteField("familyId", fmt.Sprint(user.FamilyId))
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="photo"; filename=%q`, filename))
	header.Set("Content-Type", "image/png")
	part, _ := form.CreatePart(header)
	part.Write(data)
	form.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload-photo", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, user))
	rec := httptest.NewRecorder()
	uploadPhotoHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body.String())
	}
	for _, job := range drainPhotoJobs() {
		t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	}
}

func familyPhotoCount(contents archiveContents) (n int) {
	for _, row := range contents["images"] {
		if !isVideoRow(row) {
			n++
		}
	}
	return
}

func TestImportLinksPhotosTheFamilyAlreadyUploaded(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	photos := familyPhotoCount(readArchiveContents(t, fx.db, fx.familyId))

	restorer := fx.newUser(t, "restorer")
	fx.uploadPhoto(t, restorer, "sandcastle-from-my-phone.png", createTestImage(40, 30))
	resp := fx.importBundle(t, restorer, bundle, importEverything)

	if got := familyPhotoCount(readArchiveContents(t, fx.db, restorer.FamilyId)); got != photos {
		t.Errorf("the family has %d photos after the import; want the archive's %d, the uploaded one linked", got, photos)
	}
	if resp.ImportedPhotos != photos-1 || resp.SkippedPhotos != 1 {
		t.Errorf("imported %d and skipped %d photos; want %d and 1", resp.ImportedPhotos, resp.SkippedPhotos, photos-1)
	}
}

func TestImportRecognisesMediaStoredBeforeImagesHadAHash(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	before := readArchiveContents(t, fx.db, fx.familyId)

	if hashed := BackfillMediaContentHashes(fx.db); hashed != len(before["images"]) {
		t.Errorf("backfilled %d hashes; want one for each of the %d photos and videos", hashed, len(before["images"]))
	}
	fx.importBundle(t, fx.owner, bundle, importEverything)

	after := readArchiveContents(t, fx.db, fx.familyId)
	for _, bucket := range []string{"images", "photo_person", "photo_tags"} {
		for _, d := range diffArchiveContents(archiveContents{bucket: before[bucket]}, archiveContents{bucket: after[bucket]}) {
			t.Errorf("importing the family's own archive changed its media: %s", d)
		}
	}
}
