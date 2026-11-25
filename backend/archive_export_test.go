package backend

import (
	"archive/zip"
	"bytes"
	"slices"
	"strings"
	"testing"
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

func TestArchiveExportSizesStayInTheFamily(t *testing.T) {
	fx := setupArchiveFixture(t)
	outsider := fx.newUser(t, "outsider")
	if _, err := callProc(t, outsider, GetArchiveExportSizes, GetArchiveExportSizesRequest{FamilyId: fx.familyId}); err == nil {
		t.Error("another family's export sizes were returned; want access denied")
	}
}
