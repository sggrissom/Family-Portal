package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"family/cfg"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

// limitArchiveDisk leaves budget bytes above the disk floor, less whatever
// the import has staged so far.
func limitArchiveDisk(t *testing.T, budget int64) {
	t.Helper()
	root := t.TempDir()
	previousRoot, previousFree := archiveStagingRoot, freeDiskBytes
	archiveStagingRoot = root
	freeDiskBytes = func(string) (int64, error) {
		var staged int64
		filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if info, infoErr := d.Info(); infoErr == nil {
					staged += info.Size()
				}
			}
			return nil
		})
		return cfg.MinFreeDiskBytes + budget - staged, nil
	}
	t.Cleanup(func() { archiveStagingRoot, freeDiskBytes = previousRoot, previousFree })
}

func bundleMediaSizes(t *testing.T, bundle []byte, prefix string) (sizes []int64) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		if strings.HasPrefix(zf.Name, prefix) {
			sizes = append(sizes, int64(zf.UncompressedSize64))
		}
	}
	return
}

func stagedBytes(t *testing.T, bundle []byte) int64 {
	t.Helper()
	return sum(bundleMediaSizes(t, bundle, "photos/")) + sum(bundleMediaSizes(t, bundle, "videos/"))
}

func sum(sizes []int64) (total int64) {
	for _, n := range sizes {
		total += n
	}
	return
}

func (fx *archiveFixture) importWithDisk(t *testing.T, user User, bundle []byte, budget int64, options ImportDataRequest) ImportDataResponse {
	t.Helper()
	importId := fx.uploadArchive(t, user, bundle)
	limitArchiveDisk(t, budget)
	status, err := runArchiveImport(t, user, importId, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range drainPhotoJobs() {
		t.Cleanup(func() { deletePhotoFiles(Image{FilePath: job.FilePath}) })
	}
	if status.State != "done" || status.Result == nil {
		t.Fatalf("import finished as %+v", status)
	}
	return *status.Result
}

func TestArchivePhotosNeedRoomOnDiskOnlyOnce(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	photos := bundleMediaSizes(t, bundle, "photos/")
	restorer := fx.newUser(t, "restorer")

	resp := fx.importWithDisk(t, restorer, bundle, stagedBytes(t, bundle), importEverything)
	if resp.ImportedPhotos != len(photos) {
		t.Errorf("imported %d of %d photos with room to stage them; warnings %v", resp.ImportedPhotos, len(photos), resp.Warnings)
	}
}

func TestArchiveImportLowOnDiskStillLinksPhotosTheFamilyHas(t *testing.T) {
	fx := setupArchiveFixture(t)
	bundle := fx.exportBundle(t, fx.owner, "everything")
	reference := fx.newUser(t, "reference")
	fx.importBundle(t, reference, bundle, importEverything)

	restorer := fx.newUser(t, "restorer")
	fx.importBundle(t, restorer, bundle, ImportDataRequest{ImportActivities: true})
	resp := fx.importWithDisk(t, restorer, bundle, 0, importEverything)
	if resp.ImportedPhotos != 0 {
		t.Errorf("imported %d photos with no room on disk", resp.ImportedPhotos)
	}

	want, got := readArchiveContents(t, fx.db, reference.FamilyId), readArchiveContents(t, fx.db, restorer.FamilyId)
	for _, bucket := range []string{"images", "milestones", "milestone_photos"} {
		for _, d := range diffArchiveContents(archiveContents{bucket: want[bucket]}, archiveContents{bucket: got[bucket]}) {
			t.Errorf("with no room on disk, the family's own photos were not linked: %s", d)
		}
	}
}

func TestArchiveVideosShareTheDiskTheyNeed(t *testing.T) {
	fx := setupArchiveFixture(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, video := range familyVideos(t, fx.db, fx.familyId) {
			rc, err := fx.store.Open(context.Background(), video.OriginalKey)
			if err != nil {
				t.Fatal(err)
			}
			var data bytes.Buffer
			data.ReadFrom(rc)
			rc.Close()
			second := video
			second.Id = vbolt.NextIntId(tx, ImagesBkt)
			second.OriginalKey = "videos/archivetest2/original.mov"
			second.OriginalFilename = "IMG_0004.MOV"
			second.CreatedAt = archiveSeedTime.Add(time.Hour)
			upper := bytes.ToUpper(data.Bytes())
			if err := fx.store.Put(context.Background(), second.OriginalKey, bytes.NewReader(upper), int64(len(upper)), second.MimeType); err != nil {
				t.Fatal(err)
			}
			writeImage(tx, &second)
			vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, second.Id, fx.familyId)
		}
		vbolt.TxCommit(tx)
	})
	bundle := fx.exportBundle(t, fx.owner, "everything")
	videos := bundleMediaSizes(t, bundle, "videos/")
	if len(videos) != 2 || videos[0] != videos[1] {
		t.Fatalf("the archive holds videos of %v bytes; want two of one size", videos)
	}

	restorer := fx.newUser(t, "restorer")
	roomForOne := stagedBytes(t, bundle) + int64(videoReservationBytes(int(videos[0]))) + videos[0]
	resp := fx.importWithDisk(t, restorer, bundle, roomForOne, importEverything)
	if resp.ImportedVideos != 1 {
		t.Errorf("imported %d videos with room for one; warnings %v", resp.ImportedVideos, resp.Warnings)
	}
}

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
