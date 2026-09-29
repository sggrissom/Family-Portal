package backend

import (
	"family/cfg"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

var featuresEpoch = time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)

func (fx resultsFixture) addPhotoAt(t *testing.T, filename string, at time.Time) Image {
	t.Helper()
	photo := fx.addPhoto(t, fx.familyId, filename)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		photo.PhotoDate = at
		vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		ReindexPhotoDates(tx, photo.Id)
		vbolt.TxCommit(tx)
	})
	return photo
}

func (fx resultsFixture) recordFeatures(t *testing.T, photo Image, hash uint64, quality float64) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		f := PhotoFeatures{
			PhotoId: photo.Id, FamilyId: photo.FamilyId,
			Hash: hash, HashVersion: hashAnalyzerVersion,
			Quality: quality, QualityVersion: qualityAnalyzerVersion,
			PlaceVersion: placeAnalyzerVersion,
		}
		linkSimilarTx(tx, &f, photo.PhotoDate)
		writeFeaturesTx(tx, &f)
		vbolt.TxCommit(tx)
	})
}

func (fx resultsFixture) groupIdOf(t *testing.T, photo Image) int {
	t.Helper()
	var f PhotoFeatures
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { f, _ = GetPhotoFeatures(tx, photo.Id) })
	return f.GroupId
}

const baseHash = uint64(0xF0F0_A5A5_3C3C_0FF0)

func flipBits(h uint64, n int) uint64 {
	return flipBitsFrom(h, 0, n)
}

func flipBitsFrom(h uint64, first, n int) uint64 {
	for i := first; i < first+n; i++ {
		h ^= 1 << (i * 5 % 64)
	}
	return h
}

func TestSimilarPhotosGroupByBurstAndDuplicate(t *testing.T) {
	fx := setupResultsFixture(t)

	burstA := fx.addPhotoAt(t, "a.jpg", featuresEpoch)
	burstB := fx.addPhotoAt(t, "b.jpg", featuresEpoch.Add(40*time.Second))
	later := fx.addPhotoAt(t, "later.jpg", featuresEpoch.Add(3*time.Hour))
	reupload := fx.addPhotoAt(t, "reupload.jpg", featuresEpoch.AddDate(0, 2, 0))
	unrelated := fx.addPhotoAt(t, "unrelated.jpg", featuresEpoch.Add(30*time.Second))

	fx.recordFeatures(t, burstA, baseHash, 0.4)
	fx.recordFeatures(t, burstB, flipBits(baseHash, 9), 0.8)
	fx.recordFeatures(t, later, flipBitsFrom(baseHash, 20, 9), 0.5)
	fx.recordFeatures(t, reupload, flipBits(baseHash, 2), 0.3)
	fx.recordFeatures(t, unrelated, ^baseHash, 0.9)

	group := fx.groupIdOf(t, burstA)
	if group == 0 {
		t.Fatal("burst photo was not grouped")
	}
	if g := fx.groupIdOf(t, burstB); g != group {
		t.Errorf("burst partner group = %d, want %d", g, group)
	}
	if g := fx.groupIdOf(t, reupload); g != group {
		t.Errorf("near-identical reupload group = %d, want %d", g, group)
	}
	if g := fx.groupIdOf(t, later); g != 0 {
		t.Errorf("a loosely similar photo hours later was grouped (%d)", g)
	}
	if g := fx.groupIdOf(t, unrelated); g != 0 {
		t.Errorf("an unrelated photo in the burst window was grouped (%d)", g)
	}
}

func TestLinkingMergesGroupsItBridges(t *testing.T) {
	fx := setupResultsFixture(t)
	month := featuresEpoch.AddDate(0, 1, 0)

	first := fx.addPhotoAt(t, "first.jpg", featuresEpoch)
	dup := fx.addPhotoAt(t, "dup.jpg", featuresEpoch.Add(time.Hour))
	second := fx.addPhotoAt(t, "second.jpg", month)
	other := fx.addPhotoAt(t, "other.jpg", month.Add(time.Minute))
	fx.recordFeatures(t, first, baseHash, 0.5)
	fx.recordFeatures(t, dup, flipBitsFrom(baseHash, 40, 1), 0.5)
	fx.recordFeatures(t, second, flipBits(baseHash, 10), 0.5)
	fx.recordFeatures(t, other, flipBits(baseHash, 11), 0.5)
	if g1, g2 := fx.groupIdOf(t, first), fx.groupIdOf(t, second); g1 == 0 || g2 == 0 || g1 == g2 {
		t.Fatalf("setup: expected two separate groups, got %d and %d", g1, g2)
	}

	bridge := fx.addPhotoAt(t, "bridge.jpg", month.Add(30*time.Second))
	fx.recordFeatures(t, bridge, flipBits(baseHash, 1), 0.5)

	want := fx.groupIdOf(t, first)
	for _, p := range []Image{dup, second, other, bridge} {
		if got := fx.groupIdOf(t, p); got != want {
			t.Errorf("photo %s group = %d, want merged group %d", p.OriginalFilename, got, want)
		}
	}
}

func TestListFamilyPhotosCollapsesSimilarToBestPhoto(t *testing.T) {
	fx := setupResultsFixture(t)

	soft := fx.addPhotoAt(t, "soft.jpg", featuresEpoch)
	crisp := fx.addPhotoAt(t, "crisp.jpg", featuresEpoch.Add(10*time.Second))
	blink := fx.addPhotoAt(t, "blink.jpg", featuresEpoch.Add(20*time.Second))
	alone := fx.addPhotoAt(t, "alone.jpg", featuresEpoch.Add(time.Hour))
	fx.recordFeatures(t, soft, baseHash, 0.3)
	fx.recordFeatures(t, crisp, flipBits(baseHash, 4), 0.9)
	fx.recordFeatures(t, blink, flipBits(baseHash, 6), 0.5)
	fx.recordFeatures(t, alone, ^baseHash, 0.5)

	ids := func(resp ListFamilyPhotosResponse) (out []int) {
		for _, p := range resp.Photos {
			out = append(out, p.Image.Id)
		}
		return
	}

	all, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Photos) != 4 {
		t.Errorf("uncollapsed list has %d photos, want 4", len(all.Photos))
	}

	collapsed, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{CollapseSimilar: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(collapsed); !slices.Equal(got, []int{alone.Id, crisp.Id}) {
		t.Fatalf("collapsed ids = %v, want [alone crisp] = [%d %d]", got, alone.Id, crisp.Id)
	}
	cover := collapsed.Photos[1]
	if !slices.Equal(cover.Similar, []int{soft.Id, blink.Id}) {
		t.Errorf("cover similar = %v, want [%d %d]", cover.Similar, soft.Id, blink.Id)
	}

	tag := fx.createTag(t, "Blinks")
	fx.tagPhoto(t, blink, tag)
	filtered, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{CollapseSimilar: true, TagIds: []int{tag}})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(filtered); !slices.Equal(got, []int{blink.Id}) {
		t.Errorf("a filtered-out cover should not hide the matching member: got %v, want [%d]", got, blink.Id)
	}
}

func (fx resultsFixture) createTag(t *testing.T, name string) int {
	t.Helper()
	resp, err := callAs(t, fx, CreateTag, CreateTagRequest{Name: name, Color: "#336699"})
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	return resp.Tag.Id
}

func (fx resultsFixture) tagPhoto(t *testing.T, photo Image, tagId int) {
	t.Helper()
	if _, err := callAs(t, fx, UpdatePhotoTags, UpdatePhotoTagsRequest{PhotoId: photo.Id, TagIds: []int{tagId}}); err != nil {
		t.Fatalf("UpdatePhotoTags() error = %v", err)
	}
}

func TestNearestCity(t *testing.T) {
	city, ok := NearestCity(37.7793, -122.4193)
	if !ok || city.Name != "San Francisco" || city.Region != "California" {
		t.Errorf("SF city hall = %+v, %v", city, ok)
	}
	if city.Label() != "San Francisco, California" {
		t.Errorf("label = %q", city.Label())
	}
	if _, ok := NearestCity(-48.8767, -123.3933); ok {
		t.Error("Point Nemo should be nowhere near a city")
	}
	if city, ok := NearestCity(-17.7134, 178.065); !ok || city.Country != "Fiji" {
		t.Errorf("near the antimeridian = %+v, %v", city, ok)
	}
}

func TestResolvePlacePrefersTheTightestFamilyPlace(t *testing.T) {
	sf, _ := NearestCity(37.7793, -122.4193)
	features := PhotoFeatures{HasLocation: true, Latitude: 37.7793, Longitude: -122.4193, CityId: sf.Id}
	places := []FamilyPlace{
		{Id: 1, Name: "The Bay", Latitude: 37.80, Longitude: -122.40, RadiusMeters: 20000},
		{Id: 2, Name: "Grandma's", Latitude: 37.7794, Longitude: -122.4192, RadiusMeters: 200},
		{Id: 3, Name: "Cabin", Latitude: 38.5, Longitude: -120.0, RadiusMeters: 500},
	}
	place, ok := resolvePlace(places, features)
	if !ok || place.Name != "Grandma's" || place.Key != "f2" {
		t.Errorf("place = %+v", place)
	}
	place, _ = resolvePlace(places[2:], features)
	if place.Name != "San Francisco, California" || place.Key != cityPlaceKey(sf.Id) {
		t.Errorf("fallback place = %+v", place)
	}
	if _, ok := resolvePlace(places, PhotoFeatures{}); ok {
		t.Error("a photo without a location resolved to a place")
	}
}

func writeTestJPEG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzePhotoFeaturesReadsHashQualityAndPlace(t *testing.T) {
	fx := setupResultsFixture(t)
	photosDir := filepath.Join(cfg.StaticDir, "photos")
	if err := os.MkdirAll(photosDir, 0755); err != nil {
		t.Fatal(err)
	}

	withGPS := fx.addPhotoAt(t, "features-gps.jpg", featuresEpoch)
	original, err := os.ReadFile("testdata/gps-temple-square.jpg")
	if err != nil {
		t.Fatal(err)
	}
	originalPath := filepath.Join(photosDir, "features-gps_original.jpg")
	if err := os.WriteFile(originalPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(originalPath) })

	noGPS := fx.addPhotoAt(t, "features-plain.jpg", featuresEpoch.Add(time.Hour))
	pattern := image.NewNRGBA(image.Rect(0, 0, 300, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 300; x++ {
			pattern.Set(x, y, color.NRGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	for _, name := range []string{"features-gps_medium.jpg", "features-plain_medium.jpg", "features-plain_original.jpg"} {
		path := filepath.Join(photosDir, name)
		writeTestJPEG(t, path, pattern)
		t.Cleanup(func() { os.Remove(path) })
	}

	analyzePhotoFeatures(fx.db, withGPS.Id)
	analyzePhotoFeatures(fx.db, noGPS.Id)

	var gps, plain PhotoFeatures
	var needed []int
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		gps, _ = GetPhotoFeatures(tx, withGPS.Id)
		plain, _ = GetPhotoFeatures(tx, noGPS.Id)
	})
	needed = photosNeedingFeatures(fx.db)

	city, _ := CityById(gps.CityId)
	if !gps.HasLocation || city.Name != "Salt Lake City" {
		t.Errorf("gps photo: location %v at %.4f,%.4f resolved to %+v", gps.HasLocation, gps.Latitude, gps.Longitude, city)
	}
	if gps.Hash == 0 || gps.HashVersion != hashAnalyzerVersion || gps.Quality <= 0 {
		t.Errorf("gps photo hash/quality not recorded: %+v", gps)
	}
	if plain.HasLocation || plain.Hash == 0 || plain.PlaceVersion != placeAnalyzerVersion {
		t.Errorf("plain photo: %+v", plain)
	}
	if slices.Contains(needed, withGPS.Id) || slices.Contains(needed, noGPS.Id) {
		t.Errorf("analyzed photos still need analysis: %v", needed)
	}

	resp, err := callAs(t, fx, GetPhoto, GetPhotoRequest{Id: withGPS.Id})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Place == nil || resp.Place.Name != "Salt Lake City, Utah" {
		t.Errorf("GetPhoto place = %+v", resp.Place)
	}

	saved, err := callAs(t, fx, SaveFamilyPlace, SaveFamilyPlaceRequest{PhotoId: withGPS.Id, Name: "Temple Square"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Place.RadiusMeters != defaultPlaceRadius {
		t.Errorf("radius = %d, want default", saved.Place.RadiusMeters)
	}
	places, err := callAs(t, fx, ListPhotoPlaces, Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if len(places.Places) != 1 || places.Places[0].Name != "Temple Square" || places.Places[0].Count != 1 {
		t.Errorf("places = %+v", places.Places)
	}
	filtered, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{PlaceKey: places.Places[0].Key})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Photos) != 1 || filtered.Photos[0].Image.Id != withGPS.Id {
		t.Errorf("place filter returned %d photos", len(filtered.Photos))
	}
	if _, err := callAs(t, fx, SaveFamilyPlace, SaveFamilyPlaceRequest{PhotoId: noGPS.Id, Name: "Nowhere"}); err == nil {
		t.Error("saved a place from a photo without a location")
	}
}

func TestDeletingAPhotoDropsItsFeatures(t *testing.T) {
	fx := setupResultsFixture(t)
	a := fx.addPhotoAt(t, "del-a.jpg", featuresEpoch)
	b := fx.addPhotoAt(t, "del-b.jpg", featuresEpoch.Add(5*time.Second))
	fx.recordFeatures(t, a, baseHash, 0.5)
	fx.recordFeatures(t, b, baseHash, 0.9)

	if _, err := callAs(t, fx, DeletePhoto, DeletePhotoRequest{Id: b.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, ok := GetPhotoFeatures(tx, b.Id); ok {
			t.Error("deleted photo kept its features")
		}
		if _, grouped := groupOf(tx, fx.groupIdOf(t, a), map[int]photoGroup{}); grouped {
			t.Error("a group of one still counts as a group")
		}
	})
}
