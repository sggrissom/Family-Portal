package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"go.hasen.dev/vbolt"
)

func zipEntryNames(t *testing.T, bundle []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, zf := range zr.File {
		names = append(names, zf.Name)
	}
	slices.Sort(names)
	return names
}

func TestEverythingArchiveHoldsPhotosAndVideos(t *testing.T) {
	fx := setupArchiveFixture(t)
	names := zipEntryNames(t, fx.exportBundle(t, fx.owner, "everything"))

	var photos, videos int
	for _, name := range names {
		switch {
		case strings.HasPrefix(name, "photos/"):
			photos++
		case strings.HasPrefix(name, "videos/"):
			videos++
		}
	}
	if !slices.Contains(names, "data.json") || photos == 0 || videos == 0 {
		t.Errorf("the Everything archive holds %v; want data.json, photos and videos", names)
	}

	legacy := zipEntryNames(t, fx.exportBundle(t, fx.owner, "with_photos"))
	if !slices.Equal(legacy, names) {
		t.Errorf("the old with_photos mode exports %v; want the Everything archive %v", legacy, names)
	}
}

func TestEverythingArchiveReportsMissingOriginals(t *testing.T) {
	fx := setupArchiveFixture(t)
	videos := familyVideos(t, fx.db, fx.familyId)
	if len(videos) == 0 {
		t.Fatal("expected an exported video")
	}
	if err := fx.store.Delete(context.Background(), videos[0].OriginalKey); err != nil {
		t.Fatal(err)
	}

	bundle := fx.exportBundle(t, fx.owner, "everything")
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	report, err := zr.Open(incompleteArchiveEntry)
	if err != nil {
		t.Fatal(err)
	}
	defer report.Close()
	body, err := io.ReadAll(report)
	if err != nil || !strings.Contains(string(body), "videos/"+videoExportFileName(videos[0])) {
		t.Fatalf("missing video not in the report: %s, %v", body, err)
	}
	resp := fx.importBundle(t, fx.newUser(t, "restorer"), bundle, importEverything)
	if !strings.Contains(strings.Join(resp.Warnings, " "), incompleteArchiveEntry) {
		t.Fatalf("missing export warning: %v", resp.Warnings)
	}
}

func withinPercent(got, want, percent int) bool {
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return diff*100 <= want*percent
}

func TestArchiveExportSizesEstimateTheDownloads(t *testing.T) {
	fx := setupArchiveFixture(t)
	records := len(fx.exportRecords(t, fx.owner))
	everything := len(fx.exportBundle(t, fx.owner, "everything"))

	sizes, err := callProc(t, fx.owner, GetArchiveExportSizes, GetArchiveExportSizesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !withinPercent(sizes.RecordsBytes, records, 10) {
		t.Errorf("Records is estimated at %d bytes; the download is %d", sizes.RecordsBytes, records)
	}
	if !withinPercent(sizes.EverythingBytes, everything, 25) {
		t.Errorf("the Everything archive is estimated at %d bytes; the download is %d", sizes.EverythingBytes, everything)
	}
}

func TestArchiveExportSizesCountTheVideosTheArchiveHolds(t *testing.T) {
	fx := setupArchiveFixture(t)
	check := func(when string) {
		t.Helper()
		bundle := fx.exportBundle(t, fx.owner, "everything")
		videos := 0
		for _, name := range zipEntryNames(t, bundle) {
			if strings.HasPrefix(name, "videos/") {
				videos++
			}
		}
		sizes, err := callProc(t, fx.owner, GetArchiveExportSizes, GetArchiveExportSizesRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if sizes.Videos != videos {
			t.Errorf("%s: the sizes count %d videos; the archive holds %d", when, sizes.Videos, videos)
		}
		if !withinPercent(sizes.EverythingBytes, len(bundle), 25) {
			t.Errorf("%s: the Everything archive is estimated at %d bytes; the download is %d", when, sizes.EverythingBytes, len(bundle))
		}
	}
	store := mediaStore
	mediaStore = nil
	check("with no media store")
	mediaStore = store

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, image := range GetFamilyImages(tx, fx.familyId) {
			if image.IsVideo() {
				image.Status = PhotoFailed
				saveImage(tx, &image)
			}
		}
		vbolt.TxCommit(tx)
	})
	check("with a failed video that kept its original")
}

func TestArchiveExportSizesStayInTheFamily(t *testing.T) {
	fx := setupArchiveFixture(t)
	outsider := fx.newUser(t, "outsider")
	if _, err := callProc(t, outsider, GetArchiveExportSizes, GetArchiveExportSizesRequest{FamilyId: fx.familyId}); err == nil {
		t.Error("another family's export sizes were returned; want access denied")
	}
}
