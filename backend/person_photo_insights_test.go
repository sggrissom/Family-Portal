package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) addFace(t *testing.T, photo Image, person Person, box FaceBox, status int) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		writeFace(tx, &PhotoFace{
			Id: vbolt.NextIntId(tx, PhotoFaceBkt), PhotoId: photo.Id, FamilyId: photo.FamilyId,
			PersonId: person.Id, Status: status, Box: box, CreatedAt: time.Now(),
		})
		vbolt.TxCommit(tx)
	})
}

func TestPersonPhotoInsightsPicksPortraitsAndCompanions(t *testing.T) {
	fx := setupResultsFixture(t)
	born := fx.alice.Birthday

	groupShot := fx.addPhotoAt(t, "group.jpg", born.AddDate(0, 3, 2))
	closeUp := fx.addPhotoAt(t, "closeup.jpg", born.AddDate(0, 3, 10))
	toddler := fx.addPhotoAt(t, "toddler.jpg", born.AddDate(1, 1, 0))
	fiveA := fx.addPhotoAt(t, "five-a.jpg", born.AddDate(5, 2, 0))
	fiveB := fx.addPhotoAt(t, "five-b.jpg", born.AddDate(5, 9, 0))
	for _, p := range []Image{groupShot, closeUp, toddler, fiveA, fiveB} {
		fx.tagPerson(t, p, fx.alice)
	}
	fx.tagPerson(t, groupShot, fx.bob)
	fx.tagPerson(t, groupShot, fx.carol)
	fx.tagPerson(t, fiveA, fx.bob)

	fx.addFace(t, groupShot, fx.alice, box(0.1, 0.1, 0.15, 0.15), FaceAuto)
	fx.addFace(t, closeUp, fx.alice, box(0.3, 0.2, 0.7, 0.6), FaceConfirmed)
	fx.addFace(t, fiveB, fx.alice, box(0.4, 0.4, 0.6, 0.6), FaceAuto)

	resp, err := callAs(t, fx, GetPersonPhotoInsights, GetPersonPhotoInsightsRequest{PersonId: fx.alice.Id})
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	var ages []int
	for _, p := range resp.GrowingUp {
		got = append(got, p.PhotoId)
		ages = append(ages, p.AgeMonths)
	}
	want := []int{closeUp.Id, toddler.Id, fiveB.Id}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("growing up = %v (ages %v), want %v", got, ages, want)
	}
	if ages[0] != 3 || ages[1] != 13 || ages[2] != 60 {
		t.Errorf("buckets = %v, want [3 13 60]", ages)
	}
	if resp.GrowingUp[0].Box.Right != 0.7 {
		t.Errorf("close-up should carry its face box: %+v", resp.GrowingUp[0].Box)
	}

	if len(resp.OftenWith) != 2 || resp.OftenWith[0].Person.Id != fx.bob.Id || resp.OftenWith[0].Count != 2 {
		t.Errorf("often with = %+v", resp.OftenWith)
	}
	if resp.Header != nil {
		t.Errorf("no recent photo, but header = %+v", resp.Header)
	}

	recent := fx.addPhotoAt(t, "recent.jpg", time.Now().AddDate(0, -2, 0))
	fx.tagPerson(t, recent, fx.alice)
	fx.addFace(t, recent, fx.alice, box(0.4, 0.3, 0.6, 0.5), FaceAuto)
	resp, _ = callAs(t, fx, GetPersonPhotoInsights, GetPersonPhotoInsightsRequest{PersonId: fx.alice.Id})
	if resp.Header == nil || resp.Header.PhotoId != recent.Id {
		t.Errorf("header = %+v, want the recent photo with a face", resp.Header)
	}
}
