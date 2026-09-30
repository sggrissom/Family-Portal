package backend

import (
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestListFamilyPhotosDateToIncludesTheWholeDay(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		late := Image{Id: 8, FamilyId: fx.kid.FamilyId, Title: "photo", PhotoDate: time.Date(2024, 1, 5, 23, 59, 59, 0, time.UTC)}
		vbolt.Write(tx, ImagesBkt, late.Id, &late)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, late.Id, late.FamilyId)
		AddPersonToPhoto(tx, late.Id, fx.kid.Id, late.FamilyId)
		ReindexPhotoDates(tx, late.Id)
		vbolt.TxCommit(tx)
	})

	resp := fx.listPhotos(t, ListFamilyPhotosRequest{DateFrom: "2024-01-02", DateTo: "2024-01-05"})
	if want := []int{8, 5, 3, 2}; !equalInts(photoIds(resp.Photos), want) {
		t.Errorf("ids = %v, want %v", photoIds(resp.Photos), want)
	}
}

func TestListFamilyPhotosCursorStaysInsideTheDateWindow(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	outside := photoKeyOf(Image{Id: 7, PhotoDate: time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)}).String()
	resp := fx.listPhotos(t, ListFamilyPhotosRequest{DateTo: "2024-01-03", Cursor: outside})
	if want := []int{3, 2, 1}; !equalInts(photoIds(resp.Photos), want) {
		t.Errorf("a cursor past DateTo = %v, want %v", photoIds(resp.Photos), want)
	}

	var seen []int
	cursor := ""
	for range 10 {
		resp := fx.listPhotos(t, ListFamilyPhotosRequest{DateFrom: "2024-01-02", DateTo: "2024-01-06", PersonIds: []int{fx.kid.Id, fx.sister.Id}, Limit: 1, Cursor: cursor})
		seen = append(seen, photoIds(resp.Photos)...)
		if cursor = resp.NextCursor; cursor == "" {
			break
		}
	}
	if want := []int{6, 5, 3, 2}; !equalInts(seen, want) {
		t.Errorf("paged window = %v, want %v", seen, want)
	}
}

func TestSearchPagesThroughItsRankingWithFilters(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()
	previous := globalVisionClient
	globalVisionClient = nil
	t.Cleanup(func() { globalVisionClient = previous })
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, image := range fx.photos {
			image.Title = "Beach day"
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
		}
		vbolt.TxCommit(tx)
	})

	for _, tc := range []struct {
		name string
		req  ListFamilyPhotosRequest
		want []int
	}{
		{"everything", ListFamilyPhotosRequest{Query: "beach"}, []int{7, 6, 5, 3, 2, 1}},
		{"tag", ListFamilyPhotosRequest{Query: "beach", TagIds: []int{50}}, []int{6, 3}},
		{"legacy person", ListFamilyPhotosRequest{Query: "beach", PersonId: fx.kid.Id}, []int{6, 2}},
		{"named person", ListFamilyPhotosRequest{Query: "Ada at the beach"}, []int{7, 5, 3, 1}},
		{"dates", ListFamilyPhotosRequest{Query: "beach", DateFrom: "2024-01-02", DateTo: "2024-01-05"}, []int{5, 3, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen []int
			tc.req.Limit = 2
			for range 10 {
				resp := fx.listPhotos(t, tc.req)
				if resp.SearchMode != "text" {
					t.Fatalf("search mode = %q without a daemon", resp.SearchMode)
				}
				seen = append(seen, photoIds(resp.Photos)...)
				if tc.req.Cursor = resp.NextCursor; tc.req.Cursor == "" {
					break
				}
			}
			got := slices.Clone(seen)
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) || len(slices.Compact(slices.Clone(got))) != len(seen) {
				t.Errorf("paged search = %v, want each of %v once", seen, tc.want)
			}
		})
	}
}

func TestPlaceFilterHidesPhotosWhoseLocationIsNotYours(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()
	jwtKey = []byte("photo-listing-test-secret-key-32-bytes")

	city, _ := NearestCity(40.7608, -111.8910)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		f := PhotoFeatures{PhotoId: fx.alicePhoto.Id, FamilyId: fx.famA, HasLocation: true, Latitude: 40.7608, Longitude: -111.8910, CityId: city.Id, PlaceVersion: placeAnalyzerVersion}
		writeFeaturesTx(tx, &f)
		ReindexPhotoDates(tx, fx.alicePhoto.Id)
		vbolt.TxCommit(tx)
	})

	list := func(user User, req ListFamilyPhotosRequest) []int {
		t.Helper()
		token, err := generateJwtTokenString(user)
		if err != nil {
			t.Fatal(err)
		}
		var resp ListFamilyPhotosResponse
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = ListFamilyPhotos(&vbeam.Context{Tx: tx, Token: token}, req)
		})
		if err != nil {
			t.Fatal(err)
		}
		return photoIds(resp.Photos)
	}

	place := ListFamilyPhotosRequest{PlaceKey: cityPlaceKey(city.Id)}
	if got := list(fx.userA, place); !slices.Equal(got, []int{fx.alicePhoto.Id}) {
		t.Errorf("owner's place filter = %v, want [%d]", got, fx.alicePhoto.Id)
	}
	if got := list(fx.userB, ListFamilyPhotosRequest{}); !slices.Contains(got, fx.alicePhoto.Id) {
		t.Fatalf("precondition: linked household should see the photo, got %v", got)
	}
	if got := list(fx.userB, place); len(got) != 0 {
		t.Errorf("linked household's place filter = %v, want nothing", got)
	}
	place.PersonIds = []int{fx.alice.Id}
	if got := list(fx.userB, place); len(got) != 0 {
		t.Errorf("linked household's place filter by person = %v, want nothing", got)
	}
}
