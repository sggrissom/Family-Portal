package backend

import (
	"slices"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func searchPhotosAsOwner(t *testing.T, fx resultsFixture, query string) []int {
	t.Helper()
	token, err := generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := ListFamilyPhotos(&vbeam.Context{Tx: tx, Token: token}, ListFamilyPhotosRequest{Query: query, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		ids = photoIds(resp.Photos)
	})
	return ids
}

func retitle(tx *vbolt.Tx, photo Image, title string) {
	photo = GetImageById(tx, photo.Id)
	photo.Title = title
	vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
	ReindexPhotoDates(tx, photo.Id)
}

func TestPhotoTextCacheFollowsCommittedEdits(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	photo := fx.addPhotoAt(t, "p.jpg", featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		retitle(tx, photo, "Pumpkin patch")
		vbolt.TxCommit(tx)
	})

	if got := searchPhotosAsOwner(t, fx, "pumpkin"); !slices.Equal(got, []int{photo.Id}) {
		t.Fatalf("pumpkin = %v", got)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		retitle(tx, photo, "Apple orchard")
		vbolt.TxCommit(tx)
	})
	if got := searchPhotosAsOwner(t, fx, "pumpkin"); len(got) != 0 {
		t.Errorf("an edited title still matched from the cache: %v", got)
	}
	if got := searchPhotosAsOwner(t, fx, "orchard"); !slices.Equal(got, []int{photo.Id}) {
		t.Errorf("orchard = %v", got)
	}

	markPhotoFailed(photo.Id)
	if got := searchPhotosAsOwner(t, fx, "orchard"); len(got) != 0 {
		t.Errorf("a failed photo still matched from the cache: %v", got)
	}
}

func TestPhotoTextCacheIgnoresReadsRacingAWrite(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhotoAt(t, "p.jpg", featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		retitle(tx, photo, "Before")
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		retitle(tx, photo, "After")
		reader, err := fx.db.Begin(false)
		if err != nil {
			t.Fatal(err)
		}
		if got := familyPhotoTexts(reader, fx.familyId).photos[0].Title; got != "Before" {
			t.Errorf("a reader that started before the commit saw %q", got)
		}
		reader.Rollback()
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := familyPhotoTexts(tx, fx.familyId).photos[0].Title; got != "After" {
			t.Errorf("after the commit the cache holds %q", got)
		}
	})
}

func globalPhotosAsOwner(t *testing.T, fx resultsFixture, query string) []int {
	t.Helper()
	token, err := generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp, err := Search(&vbeam.Context{Tx: tx, Token: token}, SearchRequest{Query: query})
		if err != nil {
			t.Fatal(err)
		}
		ids = photoIds(resp.Photos)
	})
	return ids
}

func TestPhotoTextCacheFollowsPeopleAndPlaces(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	photo := fx.addPhotoAt(t, "p.jpg", featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		retitle(tx, photo, "Orchard")
		writeFeaturesTx(tx, &PhotoFeatures{PhotoId: photo.Id, FamilyId: fx.familyId, HasLocation: true, Latitude: 36.16, Longitude: -86.78})
		vbolt.TxCommit(tx)
	})

	if got := globalPhotosAsOwner(t, fx, "alice orchard"); len(got) != 0 {
		t.Fatalf("alice orchard before tagging = %v", got)
	}
	fx.tagPerson(t, photo, fx.alice)
	if got := globalPhotosAsOwner(t, fx, "alice orchard"); !slices.Equal(got, []int{photo.Id}) {
		t.Errorf("a newly tagged person was missed from the cache: %v", got)
	}
	if got := searchPhotosAsOwner(t, fx, "alice orchard"); !slices.Equal(got, []int{photo.Id}) {
		t.Errorf("photos page: %v", got)
	}

	if got := globalPhotosAsOwner(t, fx, "grandmas"); len(got) != 0 {
		t.Fatalf("grandmas before naming the place = %v", got)
	}
	fx.as(t, func(ctx *vbeam.Context) {
		if _, err := SaveFamilyPlace(ctx, SaveFamilyPlaceRequest{PhotoId: photo.Id, Name: "Grandmas house"}); err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(ctx.Tx)
	})
	if got := globalPhotosAsOwner(t, fx, "grandmas"); !slices.Equal(got, []int{photo.Id}) {
		t.Errorf("a newly named place was missed from the cache: %v", got)
	}
}
