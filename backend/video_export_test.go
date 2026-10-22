package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func (fx videoFixture) storedVideo(t *testing.T, filename string, status int) Image {
	t.Helper()
	video := fx.addVideoAt(t, filename, featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		video.Status = status
		video.FileSize = len(filename)
		id := strings.ToLower(strings.TrimSuffix(filename, ".MOV"))
		video.OriginalKey = "videos/" + strings.ReplaceAll(id, "_", "") + "/original.mov"
		video.PlaybackKey = "videos/" + strings.ReplaceAll(id, "_", "") + "/playback.mp4"
		vbolt.Write(tx, ImagesBkt, video.Id, &video)
		vbolt.TxCommit(tx)
	})
	if err := fx.store.Put(context.Background(), video.OriginalKey, strings.NewReader(filename), int64(len(filename)), "video/quicktime"); err != nil {
		t.Fatal(err)
	}
	return video
}

func TestExportListsVideoOriginals(t *testing.T) {
	fx := setupVideoFixture(t)
	ready := fx.storedVideo(t, "IMG_0001.MOV", 0)
	fx.tagPerson(t, ready, fx.alice)
	failedKept := fx.storedVideo(t, "IMG_0002.MOV", 2)
	processing := fx.storedVideo(t, "IMG_0003.MOV", 1)
	fx.addPhotoAt(t, "still.jpg", featuresEpoch)

	resp, err := callAs(t, fx.resultsFixture, ExportData, ExportDataRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var data ExportDataStructure
	if err := json.Unmarshal([]byte(resp.JsonData), &data); err != nil {
		t.Fatal(err)
	}
	if data.TotalVideos != 2 || len(data.Videos) != 2 {
		t.Fatalf("data-only export lists %d videos, want the ready one and the kept failure", len(data.Videos))
	}
	byId := map[int]ExportVideo{}
	for _, v := range data.Videos {
		byId[v.Id] = v
	}
	if _, ok := byId[processing.Id]; ok {
		t.Fatal("a video still processing was exported")
	}
	got := byId[ready.Id]
	if got.FileName != "2025-06-01_"+strconv.Itoa(ready.Id)+"_IMG_0001.MOV" || !got.Available || len(got.PersonIds) != 1 || got.FileSize != len("IMG_0001.MOV") {
		t.Fatalf("exported %+v", got)
	}
	if byId[failedKept.Id].Available {
		t.Fatal("a failed video is marked available")
	}
}

func TestVideoOriginalsDownloadAfterManifestLinksExpire(t *testing.T) {
	fx := setupVideoFixture(t)
	video := fx.storedVideo(t, "IMG_0001.MOV", 0)

	manifest, err := callAs(t, fx.resultsFixture, ListVideoOriginals, ListVideoOriginalsRequest{})
	if err != nil || len(manifest.Videos) != 1 {
		t.Fatalf("manifest = %+v, %v", manifest, err)
	}
	link := manifest.Videos[0].DownloadUrl

	rec := httptest.NewRecorder()
	fx.store.serveObject(rec, httptest.NewRequest(http.MethodGet, link, nil))
	if rec.Body.String() != "IMG_0001.MOV" {
		t.Fatalf("manifest link served %d %q", rec.Code, rec.Body.String())
	}

	fx.store.now = func() time.Time { return time.Now().Add(mediaPlaybackTTL + time.Hour) }
	rec = httptest.NewRecorder()
	fx.store.serveObject(rec, httptest.NewRequest(http.MethodGet, link, nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expired manifest link: %d", rec.Code)
	}

	fresh, err := callAs(t, fx.resultsFixture, GetVideoOriginalLink, GetVideoOriginalLinkRequest{Id: video.Id})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	fx.store.serveObject(rec, httptest.NewRequest(http.MethodGet, fresh.DownloadUrl, nil))
	if !bytes.Equal(rec.Body.Bytes(), []byte("IMG_0001.MOV")) || !strings.Contains(rec.Header().Get("Content-Disposition"), fresh.FileName) {
		t.Fatalf("fresh link: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Disposition"))
	}
}

func TestVideoOriginalLinksRespectAccess(t *testing.T) {
	fx := setupVideoFixture(t)
	video := fx.storedVideo(t, "IMG_0001.MOV", 0)

	var stranger User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		stranger = AddUserTx(tx, CreateAccountRequest{Name: "Stranger", Email: "stranger-export@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	var linkErr, listErr error
	var listed ListVideoOriginalsResponse
	fx.asUser(t, stranger, func(ctx *vbeam.Context) {
		_, linkErr = GetVideoOriginalLink(ctx, GetVideoOriginalLinkRequest{Id: video.Id})
	})
	fx.asUser(t, stranger, func(ctx *vbeam.Context) {
		listed, listErr = ListVideoOriginals(ctx, ListVideoOriginalsRequest{FamilyId: fx.familyId})
	})
	if !errors.Is(linkErr, ErrVideoNotFound) {
		t.Fatalf("stranger got a link: %v", linkErr)
	}
	if listErr == nil && len(listed.Videos) > 0 {
		t.Fatal("stranger listed another family's videos")
	}
}

func TestConsistencyCheckFindsMissingVideoFiles(t *testing.T) {
	fx := setupVideoFixture(t)
	present := fx.storedVideo(t, "IMG_0001.MOV", 0)
	gone := fx.storedVideo(t, "IMG_0002.MOV", 0)
	fx.store.Delete(context.Background(), gone.OriginalKey)
	fx.store.Put(context.Background(), present.PlaybackKey, strings.NewReader("x"), 1, "video/mp4")
	fx.store.Put(context.Background(), gone.PlaybackKey, strings.NewReader("x"), 1, "video/mp4")

	var checked int
	var missing []MissingOriginal
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { checked, missing = scanVideoObjects(tx, fx.store) })
	if checked != 2 || len(missing) != 1 || missing[0].ImageId != gone.Id {
		t.Fatalf("checked %d, missing %+v", checked, missing)
	}
}
