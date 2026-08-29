package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) addPhoto(t *testing.T, familyId int, filename string) Image {
	t.Helper()

	var photo Image
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		photo = Image{
			Id: vbolt.NextIntId(tx, ImagesBkt), FamilyId: familyId,
			OwnerUserId: fx.owner.Id, OriginalFilename: filename,
			MimeType: "image/jpeg", FilePath: "photos/" + filename, CreatedAt: time.Now(),
		}
		vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, photo.Id, familyId)
		vbolt.TxCommit(tx)
	})
	return photo
}

func TestSetAppearancePhotosReplacesTheWholeSet(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	first := fx.addPhoto(t, fx.familyId, "one.jpg")
	second := fx.addPhoto(t, fx.familyId, "two.jpg")
	third := fx.addPhoto(t, fx.familyId, "three.jpg")

	resp, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id,
		PhotoIds:     []int{second.Id, first.Id, second.Id, 0},
	})
	if err != nil {
		t.Fatalf("SetAppearancePhotos() error = %v", err)
	}
	if len(resp.Appearance.PhotoIds) != 2 {
		t.Fatalf("photoIds = %v, want two", resp.Appearance.PhotoIds)
	}
	if resp.Appearance.PhotoIds[0] != second.Id || resp.Appearance.PhotoIds[1] != first.Id {
		t.Errorf("photoIds = %v, want the order they were listed in", resp.Appearance.PhotoIds)
	}

	resp, err = callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{third.Id},
	})
	if err != nil {
		t.Fatalf("SetAppearancePhotos(second) error = %v", err)
	}
	if len(resp.Appearance.PhotoIds) != 1 || resp.Appearance.PhotoIds[0] != third.Id {
		t.Fatalf("photoIds = %v, want just the third photo", resp.Appearance.PhotoIds)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilyAppearancePhotos(tx, fx.familyId)); got != 1 {
			t.Errorf("%d join rows survived the replacement, want 1", got)
		}
	})

	resp, err = callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{},
	})
	if err != nil {
		t.Fatalf("SetAppearancePhotos(empty) error = %v", err)
	}
	if len(resp.Appearance.PhotoIds) != 0 {
		t.Errorf("photoIds = %v after detaching everything", resp.Appearance.PhotoIds)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetImageById(tx, third.Id).Id == 0 {
			t.Error("detaching a photo deleted it")
		}
	})
}

func TestSetEventPhotosKeepsTheWeekendSeparateFromTheRoutine(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	venue := fx.addPhoto(t, fx.familyId, "venue.jpg")
	onstage := fx.addPhoto(t, fx.familyId, "onstage.jpg")

	if _, err := callAs(t, fx, SetEventPhotos, SetEventPhotosRequest{
		EventId: fx.event.Id, PhotoIds: []int{venue.Id},
	}); err != nil {
		t.Fatalf("SetEventPhotos() error = %v", err)
	}
	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{onstage.Id},
	}); err != nil {
		t.Fatalf("SetAppearancePhotos() error = %v", err)
	}

	detail, err := callAs(t, fx, GetEventDetail, GetEventDetailRequest{EventId: fx.event.Id})
	if err != nil {
		t.Fatalf("GetEventDetail() error = %v", err)
	}
	if len(detail.PhotoIds) != 1 || detail.PhotoIds[0] != venue.Id {
		t.Errorf("event photos = %v, want just the venue shot", detail.PhotoIds)
	}
	if len(detail.Appearances) != 1 {
		t.Fatalf("got %d performances, want 1", len(detail.Appearances))
	}
	if got := detail.Appearances[0].PhotoIds; len(got) != 1 || got[0] != onstage.Id {
		t.Errorf("performance photos = %v, want just the onstage shot", got)
	}

	history, err := callAs(t, fx, GetEntryHistory, GetEntryHistoryRequest{EntryId: fx.entry.Id})
	if err != nil {
		t.Fatalf("GetEntryHistory() error = %v", err)
	}
	if got := history.Appearances[0].PhotoIds; len(got) != 1 || got[0] != onstage.Id {
		t.Errorf("history photos = %v, want the onstage shot", got)
	}
}

func TestSetPhotosRefusesAnotherFamilysPhoto(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	ours := fx.addPhoto(t, fx.familyId, "ours.jpg")
	theirs := fx.addPhoto(t, fx.familyId+999, "theirs.jpg")

	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{ours.Id},
	}); err != nil {
		t.Fatalf("SetAppearancePhotos() error = %v", err)
	}

	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{ours.Id, theirs.Id},
	}); err == nil {
		t.Error("a photo from another family attached to a performance")
	}
	if _, err := callAs(t, fx, SetEventPhotos, SetEventPhotosRequest{
		EventId: fx.event.Id, PhotoIds: []int{theirs.Id},
	}); err == nil {
		t.Error("a photo from another family attached to a competition")
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		got := GetAppearancePhotoIds(tx, appearance.Id)
		if len(got) != 1 || got[0] != ours.Id {
			t.Errorf("a rejected call changed the stored set: %v", got)
		}
	})

	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{999999},
	}); err == nil {
		t.Error("a photo id that does not exist was accepted")
	}
}

func TestSetPhotosCapsOneRequest(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	oversized := make([]int, maxPhotosPerSubject+1)
	for i := range oversized {
		oversized[i] = i + 1
	}
	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: oversized,
	}); err != ErrTooManyPhotos {
		t.Errorf("error = %v, want %v", err, ErrTooManyPhotos)
	}
}

func TestDeletingAPhotoClearsBothJoinTables(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)

	shared := fx.addPhoto(t, fx.familyId, "shared.jpg")
	kept := fx.addPhoto(t, fx.familyId, "kept.jpg")

	if _, err := callAs(t, fx, SetAppearancePhotos, SetAppearancePhotosRequest{
		AppearanceId: appearance.Id, PhotoIds: []int{shared.Id, kept.Id},
	}); err != nil {
		t.Fatalf("SetAppearancePhotos() error = %v", err)
	}
	if _, err := callAs(t, fx, SetEventPhotos, SetEventPhotosRequest{
		EventId: fx.event.Id, PhotoIds: []int{shared.Id},
	}); err != nil {
		t.Fatalf("SetEventPhotos() error = %v", err)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deletePhotoRecordTx(tx, GetImageById(tx, shared.Id))
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetAppearancePhotoIds(tx, appearance.Id); len(got) != 1 || got[0] != kept.Id {
			t.Errorf("performance photos = %v, want just the surviving photo", got)
		}
		if got := GetEventPhotoIds(tx, fx.event.Id); len(got) != 0 {
			t.Errorf("competition photos = %v, want none", got)
		}
		if got := len(GetFamilyAppearancePhotos(tx, fx.familyId)); got != 1 {
			t.Errorf("%d appearance-photo joins survived, want 1", got)
		}
		if got := len(GetFamilyEventPhotos(tx, fx.familyId)); got != 0 {
			t.Errorf("%d event-photo joins survived, want 0", got)
		}
	})
}

func TestLinkedHouseholdOnlyGetsPhotoIdsItCanLoad(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()
	jwtKey = []byte("activity-photos-test-secret-key-at-least-32")

	setLinkScopes(t, fx.familyLinkFixture, fx.linkAB, LinkScopes{People: true, Activities: true})

	asUserB := func(fn func(ctx *vbeam.Context)) {
		t.Helper()
		token, err := generateJwtTokenString(fx.userB)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			fn(&vbeam.Context{Tx: tx, Token: token})
		})
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetAppearancePhotoIds(tx, fx.groupAppr.Id); len(got) != 1 {
			t.Fatalf("the fixture's performance has %d photos, want 1", got)
		}
	})

	asUserB(func(ctx *vbeam.Context) {
		resp, err := GetEntryHistory(ctx, GetEntryHistoryRequest{EntryId: fx.groupEntry.Id})
		if err != nil {
			t.Fatalf("GetEntryHistory() error = %v", err)
		}
		if len(resp.Appearances) != 1 {
			t.Fatalf("got %d performances, want 1", len(resp.Appearances))
		}
		if got := resp.Appearances[0].PhotoIds; len(got) != 0 {
			t.Errorf("photoIds = %v, want none without the photos scope", got)
		}
	})

	setLinkScopes(t, fx.familyLinkFixture, fx.linkAB,
		LinkScopes{People: true, Activities: true, Photos: true})

	asUserB(func(ctx *vbeam.Context) {
		resp, err := GetEntryHistory(ctx, GetEntryHistoryRequest{EntryId: fx.groupEntry.Id})
		if err != nil {
			t.Fatalf("GetEntryHistory() error = %v", err)
		}
		if got := resp.Appearances[0].PhotoIds; len(got) != 1 || got[0] != fx.alicePhoto.Id {
			t.Errorf("photoIds = %v, want alice's photo once the link carries photos", got)
		}
	})

	asUserB(func(ctx *vbeam.Context) {
		if _, err := SetAppearancePhotos(ctx, SetAppearancePhotosRequest{
			AppearanceId: fx.groupAppr.Id, PhotoIds: []int{fx.alicePhoto.Id},
		}); err == nil {
			t.Error("a link granted writes on a shared performance's photos")
		}
	})
}
