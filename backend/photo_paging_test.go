package backend

import (
	"family/cfg"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type pagingFixture struct {
	db     *vbolt.DB
	token  string
	kid    Person
	sister Person
	photos []Image
}

// Seven photos, one a day from 2024-01-01; even ids are of the kid, odd ids of
// the sister, and every third photo carries tag 50. Photo 4 failed processing.
func setupPagingFixture(t *testing.T) (pagingFixture, func()) {
	t.Helper()

	testDBPath := "test_photo_paging.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	previousDb := appDb
	appDb = db
	jwtKey = []byte("photo-paging-test-secret-key-32-bytes")
	cleanup := func() {
		appDb = previousDb
		db.Close()
		os.Remove(testDBPath)
	}

	fx := pagingFixture{db: db}
	var user User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		user = AddUserTx(tx, CreateAccountRequest{Name: "Me", Email: "paging@example.com"}, hash)

		var err error
		fx.kid, err = AddPersonTx(tx, AddPersonRequest{Name: "Mia", Birthdate: "2020-01-01"}, user.FamilyId)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}
		fx.sister, err = AddPersonTx(tx, AddPersonRequest{Name: "Ada", Birthdate: "2018-01-01"}, user.FamilyId)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}

		start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 1; i <= 7; i++ {
			image := Image{
				Id:        i,
				FamilyId:  user.FamilyId,
				Title:     "photo",
				PhotoDate: start.AddDate(0, 0, i-1),
			}
			if i == 4 {
				image.Status = 2
			}
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
			vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, image.FamilyId)

			if i%2 == 0 {
				AddPersonToPhoto(tx, i, fx.kid.Id, user.FamilyId)
			} else {
				AddPersonToPhoto(tx, i, fx.sister.Id, user.FamilyId)
			}
			if i%3 == 0 {
				addTagToPhoto(tx, i, 50, user.FamilyId)
			}
			ReindexPhotoDates(tx, i)
			fx.photos = append(fx.photos, image)
		}

		vbolt.TxCommit(tx)
	})

	token, err := generateJwtTokenString(user)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}
	fx.token = token

	return fx, cleanup
}

func (fx pagingFixture) listPhotos(t *testing.T, req ListFamilyPhotosRequest) ListFamilyPhotosResponse {
	t.Helper()
	var resp ListFamilyPhotosResponse
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		resp, err = ListFamilyPhotos(&vbeam.Context{Tx: tx, Token: fx.token}, req)
		if err != nil {
			t.Fatalf("ListFamilyPhotos() error = %v", err)
		}
	})
	return resp
}

func photoIds(photos []PhotoWithPeople) []int {
	ids := make([]int, 0, len(photos))
	for _, photo := range photos {
		ids = append(ids, photo.Image.Id)
	}
	return ids
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListFamilyPhotosPagesNewestFirst(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	var seen []int
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		resp := fx.listPhotos(t, ListFamilyPhotosRequest{Limit: 4, Cursor: cursor})
		seen = append(seen, photoIds(resp.Photos)...)
		cursor = resp.NextCursor
		if cursor == "" {
			break
		}
	}

	if want := []int{7, 6, 5, 3, 2, 1}; !equalInts(seen, want) {
		t.Errorf("paged ids = %v, want %v", seen, want)
	}
}

func TestListFamilyPhotosWithoutLimitReturnsEverything(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	resp := fx.listPhotos(t, ListFamilyPhotosRequest{})
	if len(resp.Photos) != 6 || resp.NextCursor != "" {
		t.Errorf("got %d photos, cursor %q; want 6 and no cursor", len(resp.Photos), resp.NextCursor)
	}
	for _, photo := range resp.Photos {
		if len(photo.People) != 1 {
			t.Errorf("photo %d has %d people, want 1", photo.Image.Id, len(photo.People))
		}
	}
}

func TestListFamilyPhotosFilters(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	cases := []struct {
		name string
		req  ListFamilyPhotosRequest
		want []int
	}{
		{"person", ListFamilyPhotosRequest{PersonIds: []int{fx.kid.Id}}, []int{6, 2}},
		{"either person", ListFamilyPhotosRequest{PersonIds: []int{fx.kid.Id, fx.sister.Id}}, []int{7, 6, 5, 3, 2, 1}},
		{"tag", ListFamilyPhotosRequest{TagIds: []int{50}}, []int{6, 3}},
		{"person and tag", ListFamilyPhotosRequest{PersonIds: []int{fx.sister.Id}, TagIds: []int{50}}, []int{3}},
		{"date range", ListFamilyPhotosRequest{DateFrom: "2024-01-02", DateTo: "2024-01-05"}, []int{5, 3, 2}},
		{"reversed range", ListFamilyPhotosRequest{DateFrom: "2024-01-05", DateTo: "2024-01-02"}, []int{5, 3, 2}},
		{"legacy person", ListFamilyPhotosRequest{PersonId: fx.sister.Id}, []int{7, 5, 3, 1}},
		{"unknown tag", ListFamilyPhotosRequest{TagIds: []int{999}}, []int{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := fx.listPhotos(t, tc.req)
			if got := photoIds(resp.Photos); !equalInts(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListFamilyPhotosRejectsBadInput(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	for _, req := range []ListFamilyPhotosRequest{
		{Cursor: "garbage"},
		{Cursor: "12_x"},
		{DateFrom: "01/02/2024"},
	} {
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			if _, err := ListFamilyPhotos(&vbeam.Context{Tx: tx, Token: fx.token}, req); err == nil {
				t.Errorf("ListFamilyPhotos(%+v) succeeded, want an error", req)
			}
		})
	}
}

func TestFamilyTimelineWindow(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, date := range []string{"2022-06-01", "2024-01-03"} {
			if _, err := AddMilestoneTx(tx, AddMilestoneRequest{
				PersonId: fx.kid.Id, Description: "walked", Category: "motor",
				InputType: "date", MilestoneDate: &date,
			}, fx.kid.FamilyId); err != nil {
				t.Fatalf("AddMilestoneTx: %v", err)
			}
		}
		date := "2023-03-03"
		if _, err := AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId: fx.kid.Id, MeasurementType: "height", Value: 90, Unit: "cm",
			InputType: "date", MeasurementDate: &date,
		}, fx.kid.FamilyId); err != nil {
			t.Fatalf("AddGrowthDataTx: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	timeline := func(req GetFamilyTimelineRequest) (kid FamilyTimelineItem, years []int) {
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
			resp, err := GetFamilyTimeline(&vbeam.Context{Tx: tx, Token: fx.token}, req)
			if err != nil {
				t.Fatalf("GetFamilyTimeline() error = %v", err)
			}
			years = resp.Years
			for _, item := range resp.People {
				if item.Person.Id == fx.kid.Id {
					kid = item
				}
			}
		})
		return
	}

	kid, years := timeline(GetFamilyTimelineRequest{})
	if len(kid.Milestones) != 2 || len(kid.GrowthData) != 1 || len(kid.Photos) != 3 {
		t.Errorf("full timeline: %d milestones, %d growth, %d photos; want 2, 1, 3",
			len(kid.Milestones), len(kid.GrowthData), len(kid.Photos))
	}
	if want := []int{2024, 2023, 2022}; !equalInts(years, want) {
		t.Errorf("years = %v, want %v", years, want)
	}

	kid, years = timeline(GetFamilyTimelineRequest{From: "2024-01-01", To: "2024-12-31"})
	if len(kid.Milestones) != 1 || len(kid.GrowthData) != 0 || len(kid.Photos) != 3 {
		t.Errorf("2024 window: %d milestones, %d growth, %d photos; want 1, 0, 3",
			len(kid.Milestones), len(kid.GrowthData), len(kid.Photos))
	}
	if kid.GrowthData == nil {
		t.Error("an empty window should send growthData as [], not null")
	}
	if want := []int{2024, 2023, 2022}; !equalInts(years, want) {
		t.Errorf("windowed years = %v, want %v", years, want)
	}

	kid, _ = timeline(GetFamilyTimelineRequest{SkipMilestones: true, SkipPhotos: true})
	if kid.Milestones != nil || kid.Photos != nil || len(kid.GrowthData) != 1 {
		t.Errorf("growth only: milestones %v, photos %v, %d growth", kid.Milestones, kid.Photos, len(kid.GrowthData))
	}
}

func TestListFamilyPhotosPagesThroughSmallWindows(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	for limit := 1; limit <= 7; limit++ {
		var seen []int
		cursor := ""
		for pages := 0; pages < 10; pages++ {
			resp := fx.listPhotos(t, ListFamilyPhotosRequest{Limit: limit, Cursor: cursor, TagIds: []int{50}})
			seen = append(seen, photoIds(resp.Photos)...)
			if cursor = resp.NextCursor; cursor == "" {
				break
			}
		}
		if want := []int{6, 3}; !equalInts(seen, want) {
			t.Errorf("limit %d: ids = %v, want %v", limit, seen, want)
		}
	}
}

// A photo of two people sits under both person terms; paging by either person
// or both must show it once.
func TestListFamilyPhotosSharedPhotoOnce(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		AddPersonToPhoto(tx, 6, fx.sister.Id, fx.kid.FamilyId)
		vbolt.TxCommit(tx)
	})

	resp := fx.listPhotos(t, ListFamilyPhotosRequest{PersonIds: []int{fx.kid.Id, fx.sister.Id}, Limit: 2})
	if want := []int{7, 6}; !equalInts(photoIds(resp.Photos), want) {
		t.Errorf("first page = %v, want %v", photoIds(resp.Photos), want)
	}
	resp = fx.listPhotos(t, ListFamilyPhotosRequest{PersonIds: []int{fx.kid.Id, fx.sister.Id}, Limit: 2, Cursor: resp.NextCursor})
	if want := []int{5, 3}; !equalInts(photoIds(resp.Photos), want) {
		t.Errorf("second page = %v, want %v", photoIds(resp.Photos), want)
	}
}

// Old scanned photos predate 1970; a plain unsigned time key would sort them
// after this year's.
func TestPhotoDateIndexOrdersPre1970AndFollowsEdits(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		old := Image{Id: 8, FamilyId: fx.kid.FamilyId, PhotoDate: time.Date(1958, 5, 5, 0, 0, 0, 0, time.UTC)}
		vbolt.Write(tx, ImagesBkt, old.Id, &old)
		ReindexPhotoDates(tx, old.Id)

		moved := GetImageById(tx, 1)
		moved.PhotoDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		vbolt.Write(tx, ImagesBkt, moved.Id, &moved)
		ReindexPhotoDates(tx, moved.Id)

		vbolt.TxCommit(tx)
	})

	resp := fx.listPhotos(t, ListFamilyPhotosRequest{})
	if want := []int{1, 7, 6, 5, 3, 2, 8}; !equalInts(photoIds(resp.Photos), want) {
		t.Errorf("ids = %v, want %v", photoIds(resp.Photos), want)
	}

	resp = fx.listPhotos(t, ListFamilyPhotosRequest{PersonIds: []int{fx.sister.Id}})
	if want := []int{1, 7, 5, 3}; !equalInts(photoIds(resp.Photos), want) {
		t.Errorf("sister ids = %v, want %v", photoIds(resp.Photos), want)
	}
}

func TestDeletedPhotoLeavesDateIndexes(t *testing.T) {
	fx, cleanup := setupPagingFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deletePhotoRecordTx(tx, GetImageById(tx, 7))
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, index := range []*vbolt.IndexInfo[int, int, int64]{ImageByFamilyDateIndex, ImageByPersonDateIndex} {
			vbolt.IterateTarget(tx, index, 7, func(term int, _ int64) bool {
				t.Errorf("%s still maps photo 7 to %d", index.Name, term)
				return true
			})
		}
	})
}
