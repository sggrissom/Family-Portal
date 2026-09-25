package backend

import (
	"image"
	"testing"

	"go.hasen.dev/vbolt"
)

func faceVec(axis int, nudges ...float32) []float32 {
	v := make([]float32, 128)
	v[axis] = 1
	for i, n := range nudges {
		v[(axis+1+i)%128] += n
	}
	return v
}

func (fx resultsFixture) setProfileDescriptor(t *testing.T, person Person, d []float32) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		p := GetPersonById(tx, person.Id)
		p.FaceDescriptor = d
		vbolt.Write(tx, PeopleBkt, p.Id, &p)
		vbolt.TxCommit(tx)
	})
}

func (fx resultsFixture) recordFaces(t *testing.T, photo Image, faces ...DetectedFace) int {
	t.Helper()
	var tagged int
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		tagged = RecordPhotoFacesTx(tx, photo.Id, faces)
		vbolt.TxCommit(tx)
	})
	return tagged
}

func (fx resultsFixture) facesOn(t *testing.T, photo Image) (faces []PhotoFace, tags []PhotoPerson) {
	t.Helper()
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		faces = GetPhotoFacesTx(tx, photo.Id)
		tags = GetPhotoPersonsByPhoto(tx, photo.Id)
	})
	return
}

func box(l, t, r, b float64) FaceBox { return FaceBox{Left: l, Top: t, Right: r, Bottom: b} }

func faceAt(faces []PhotoFace, b FaceBox) PhotoFace {
	for _, f := range faces {
		if f.Box == b {
			return f
		}
	}
	return PhotoFace{}
}

func TestRecordPhotoFacesTagsKnownFacesAndKeepsStrangers(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	photo := fx.addPhoto(t, fx.familyId, "party.jpg")

	aliceBox, strangerBox := box(0.1, 0.1, 0.2, 0.2), box(0.5, 0.1, 0.6, 0.2)
	tagged := fx.recordFaces(t, photo,
		DetectedFace{Descriptor: faceVec(0, 0.2), Box: aliceBox},
		DetectedFace{Descriptor: faceVec(40), Box: strangerBox},
	)
	if tagged != 1 {
		t.Fatalf("tagged = %d, want 1", tagged)
	}

	faces, tags := fx.facesOn(t, photo)
	if len(faces) != 2 {
		t.Fatalf("stored %d faces, want both detections kept", len(faces))
	}
	if f := faceAt(faces, aliceBox); f.PersonId != fx.alice.Id || f.Status != FaceAuto {
		t.Errorf("alice's face = person %d status %d, want auto-assigned to alice", f.PersonId, f.Status)
	}
	if f := faceAt(faces, strangerBox); f.PersonId != 0 || f.Status != FaceUnknown {
		t.Errorf("stranger's face = person %d status %d, want unknown", f.PersonId, f.Status)
	}
	if len(tags) != 1 || tags[0].PersonId != fx.alice.Id || !tags[0].AutoTagged {
		t.Errorf("photo tags = %+v, want one auto tag for alice", tags)
	}
}

func TestAmbiguousMatchIsLeftForAPerson(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	fx.setProfileDescriptor(t, fx.bob, faceVec(0, 0, 0.02))
	photo := fx.addPhoto(t, fx.familyId, "siblings.jpg")

	if tagged := fx.recordFaces(t, photo, DetectedFace{Descriptor: faceVec(0, 0.3)}); tagged != 0 {
		t.Fatalf("tagged = %d for a face equally close to two siblings, want 0", tagged)
	}
}

func TestOneFacePerPersonPerPhoto(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	photo := fx.addPhoto(t, fx.familyId, "twins.jpg")

	near, nearer := box(0, 0, 0.1, 0.1), box(0.5, 0, 0.6, 0.1)
	fx.recordFaces(t, photo,
		DetectedFace{Descriptor: faceVec(0, 0.3), Box: near},
		DetectedFace{Descriptor: faceVec(0, 0.1), Box: nearer},
	)
	faces, _ := fx.facesOn(t, photo)
	if faceAt(faces, nearer).PersonId != fx.alice.Id || faceAt(faces, near).PersonId != 0 {
		t.Errorf("want only the closer face assigned to alice, got %+v", faces)
	}
}

func TestAssigningAFaceTeachesTheMatcher(t *testing.T) {
	fx := setupResultsFixture(t)
	first := fx.addPhoto(t, fx.familyId, "first.jpg")
	second := fx.addPhoto(t, fx.familyId, "second.jpg")
	fx.recordFaces(t, first, DetectedFace{Descriptor: faceVec(7)})
	fx.recordFaces(t, second, DetectedFace{Descriptor: faceVec(7, 0.15)})

	faces, _ := fx.facesOn(t, first)
	resp, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: []int{faces[0].Id}, PersonId: fx.carol.Id})
	if err != nil {
		t.Fatalf("AssignFaces() error = %v", err)
	}
	if resp.Assigned != 1 || resp.AutoTagged != 1 {
		t.Errorf("resp = %+v, want 1 assigned and the other photo auto-tagged", resp)
	}

	faces, tags := fx.facesOn(t, first)
	if faces[0].Status != FaceConfirmed || len(tags) != 1 || tags[0].AutoTagged {
		t.Errorf("first photo: face %+v tags %+v, want a confirmed face and a manual tag", faces[0], tags)
	}
	faces, tags = fx.facesOn(t, second)
	if faces[0].PersonId != fx.carol.Id || faces[0].Status != FaceAuto || len(tags) != 1 || !tags[0].AutoTagged {
		t.Errorf("second photo: face %+v tags %+v, want carol auto-tagged", faces[0], tags)
	}
}

func TestRejectedMatchStaysRejectedThroughReanalysis(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	photo := fx.addPhoto(t, fx.familyId, "lookalike.jpg")
	detection := DetectedFace{Descriptor: faceVec(0, 0.2), Box: box(0.1, 0.1, 0.3, 0.3)}
	fx.recordFaces(t, photo, detection)

	faces, _ := fx.facesOn(t, photo)
	if _, err := callAs(t, fx, RejectFaces, FaceIdsRequest{FaceIds: []int{faces[0].Id}}); err != nil {
		t.Fatalf("RejectFaces() error = %v", err)
	}
	faces, tags := fx.facesOn(t, photo)
	if faces[0].PersonId != 0 || len(tags) != 0 {
		t.Fatalf("after reject: face %+v tags %+v, want unassigned and untagged", faces[0], tags)
	}

	if tagged := fx.recordFaces(t, photo, detection); tagged != 0 {
		t.Errorf("reanalysis re-tagged a rejected match")
	}
}

func TestRemovingATagRejectsTheFace(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	photo := fx.addPhoto(t, fx.familyId, "untag.jpg")
	fx.recordFaces(t, photo, DetectedFace{Descriptor: faceVec(0, 0.2)})

	if _, err := callAs(t, fx, RemovePersonFromPhotoProc, RemovePersonFromPhotoRequest{PhotoId: photo.Id, PersonId: fx.alice.Id}); err != nil {
		t.Fatalf("RemovePersonFromPhotoProc() error = %v", err)
	}
	faces, _ := fx.facesOn(t, photo)
	if faces[0].PersonId != 0 || !containsInt(faces[0].RejectedPersonIds, fx.alice.Id) {
		t.Errorf("face = %+v, want unassigned with alice rejected", faces[0])
	}
}

func TestDismissedFacesLeaveTheReview(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhoto(t, fx.familyId, "crowd.jpg")
	fx.recordFaces(t, photo, DetectedFace{Descriptor: faceVec(3)}, DetectedFace{Descriptor: faceVec(90)})

	faces, _ := fx.facesOn(t, photo)
	if _, err := callAs(t, fx, DismissFaces, FaceIdsRequest{FaceIds: []int{faces[0].Id}}); err != nil {
		t.Fatalf("DismissFaces() error = %v", err)
	}
	review, err := callAs(t, fx, GetFaceReview, GetFaceReviewRequest{})
	if err != nil {
		t.Fatalf("GetFaceReview() error = %v", err)
	}
	if review.UnknownCount != 1 || len(review.Groups) != 1 {
		t.Errorf("review has %d unknown in %d groups, want the one undismissed face", review.UnknownCount, len(review.Groups))
	}
}

func TestFaceReviewGroupsLookalikesAndSuggests(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.bob, faceVec(20, 0, 0, 0, 0.55))
	for i, d := range [][]float32{faceVec(20, 0.1), faceVec(20, 0, 0.1), faceVec(20, 0, 0, 0.1), faceVec(60)} {
		fx.recordFaces(t, fx.addPhoto(t, fx.familyId, string(rune('a'+i))+".jpg"), DetectedFace{Descriptor: d})
	}

	review, err := callAs(t, fx, GetFaceReview, GetFaceReviewRequest{})
	if err != nil {
		t.Fatalf("GetFaceReview() error = %v", err)
	}
	if len(review.Groups) != 2 {
		t.Fatalf("got %d groups, want the three lookalikes together and the stranger alone", len(review.Groups))
	}
	if len(review.Groups[0].Faces) != 3 {
		t.Errorf("largest group has %d faces, want 3", len(review.Groups[0].Faces))
	}
	if review.Groups[0].SuggestedPersonId != fx.bob.Id {
		t.Errorf("suggested person %d, want bob", review.Groups[0].SuggestedPersonId)
	}
	if review.Groups[1].SuggestedPersonId != 0 {
		t.Errorf("stranger got a suggestion")
	}
	if len(review.Families) != 1 || len(review.Families[0].People) != 3 {
		t.Errorf("families = %+v, want one family with three people to choose from", review.Families)
	}
}

func TestFacesStayInsideTheirFamily(t *testing.T) {
	fx := setupResultsFixture(t)
	outsiderFamily := fx.familyId + 999
	theirs := fx.addPhoto(t, outsiderFamily, "theirs.jpg")
	fx.recordFaces(t, theirs, DetectedFace{Descriptor: faceVec(5)})
	faces, _ := fx.facesOn(t, theirs)

	if _, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: []int{faces[0].Id}, PersonId: fx.alice.Id}); err != ErrFaceNotFound {
		t.Errorf("AssignFaces(other family) error = %v, want ErrFaceNotFound", err)
	}
	if _, err := callAs(t, fx, DismissFaces, FaceIdsRequest{FaceIds: []int{faces[0].Id}}); err != ErrFaceNotFound {
		t.Errorf("DismissFaces(other family) error = %v, want ErrFaceNotFound", err)
	}
	review, _ := callAs(t, fx, GetFaceReview, GetFaceReviewRequest{})
	if review.UnknownCount != 0 {
		t.Errorf("review shows %d faces from another family", review.UnknownCount)
	}
	photoFaces, _ := callAs(t, fx, GetPhotoFaces, GetPhotoFacesRequest{PhotoId: theirs.Id})
	if len(photoFaces.Faces) != 0 || photoFaces.CanLabel {
		t.Errorf("GetPhotoFaces leaked another family's faces")
	}

	ours := fx.addPhoto(t, fx.familyId, "ours.jpg")
	fx.recordFaces(t, ours, DetectedFace{Descriptor: faceVec(5)})
	ourFaces, _ := fx.facesOn(t, ours)
	var outsider Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		outsider, _ = AddPersonTx(tx, AddPersonRequest{Name: "Stranger", Birthdate: "2010-01-01"}, outsiderFamily)
		vbolt.TxCommit(tx)
	})
	if _, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: []int{ourFaces[0].Id}, PersonId: outsider.Id}); err == nil {
		t.Errorf("AssignFaces allowed naming a face as another family's person")
	}
}

func TestDeletingAPhotoDeletesItsFaces(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhoto(t, fx.familyId, "gone.jpg")
	fx.recordFaces(t, photo, DetectedFace{Descriptor: faceVec(1)})

	if _, err := callAs(t, fx, DeletePhoto, DeletePhotoRequest{Id: photo.Id}); err != nil {
		t.Fatalf("DeletePhoto() error = %v", err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if n := len(GetFamilyFaces(tx, fx.familyId)); n != 0 {
			t.Errorf("%d faces outlived their photo", n)
		}
	})
}

func TestMergingPeopleMovesTheirFaces(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhoto(t, fx.familyId, "merge.jpg")
	fx.recordFaces(t, photo, DetectedFace{Descriptor: faceVec(9)})
	faces, _ := fx.facesOn(t, photo)
	if _, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: []int{faces[0].Id}, PersonId: fx.bob.Id}); err != nil {
		t.Fatalf("AssignFaces() error = %v", err)
	}

	if _, err := callAs(t, fx, MergePeople, MergePeopleRequest{SourcePersonId: fx.bob.Id, TargetPersonId: fx.carol.Id}); err != nil {
		t.Fatalf("MergePeople() error = %v", err)
	}
	faces, _ = fx.facesOn(t, photo)
	if faces[0].PersonId != fx.carol.Id || faces[0].Status != FaceConfirmed {
		t.Errorf("face = %+v, want it confirmed as carol after the merge", faces[0])
	}
}

func TestToDetectedFacesNormalizesBoxes(t *testing.T) {
	got := toDetectedFaces(recognizeResponse{
		Faces: []recognizedFace{{Descriptor: faceVec(0), Rect: [4]int{100, 50, 300, 250}}},
	}, image.Point{X: 1000, Y: 500})
	want := box(0.1, 0.1, 0.3, 0.5)
	if len(got) != 1 || got[0].Box != want {
		t.Errorf("got %+v, want box %+v", got, want)
	}

	legacy := toDetectedFaces(recognizeResponse{Descriptors: [][]float32{faceVec(0), faceVec(1)}}, image.Point{})
	if len(legacy) != 2 || legacy[0].Box != (FaceBox{}) {
		t.Errorf("legacy daemon response = %+v, want two boxless faces", legacy)
	}
}

func TestPickProfileFacePrefersTheCropFocus(t *testing.T) {
	person := Person{Id: 4, ProfileCropX: 80, ProfileCropY: 50}
	detected := []DetectedFace{
		{Descriptor: faceVec(1), Box: box(0.1, 0.4, 0.2, 0.6)},
		{Descriptor: faceVec(2), Box: box(0.7, 0.4, 0.9, 0.6)},
	}
	if got := pickProfileFace(person, nil, detected); got[2] != 1 {
		t.Errorf("picked the face away from the crop focus")
	}
	known := []PhotoFace{{PersonId: 4, Descriptor: faceVec(3)}}
	if got := pickProfileFace(person, known, detected); got[3] != 1 {
		t.Errorf("ignored the face already assigned to the person")
	}
}

func TestClusteringDoesNotChainLookalikes(t *testing.T) {
	a := PhotoFace{Id: 1, PhotoId: 1, Descriptor: faceVec(0)}
	b := PhotoFace{Id: 2, PhotoId: 2, Descriptor: faceVec(0, 0.4)}
	c := PhotoFace{Id: 3, PhotoId: 3, Descriptor: faceVec(0, 0.8)}
	clusters := clusterFaces([]PhotoFace{a, b, c})
	if len(clusters) != 2 {
		t.Fatalf("got %d groups, want the far end of the chain kept apart", len(clusters))
	}
	if len(clusters[0].faces) != 2 || clusters[0].faces[0].Id != 1 || clusters[0].faces[1].Id != 2 {
		t.Errorf("first group = %+v, want faces 1 and 2", clusters[0].faces)
	}
}

func TestClusteringKeepsFacesFromOnePhotoApart(t *testing.T) {
	a := PhotoFace{Id: 1, PhotoId: 7, Descriptor: faceVec(0)}
	b := PhotoFace{Id: 2, PhotoId: 7, Descriptor: faceVec(0, 0.1)}
	if clusters := clusterFaces([]PhotoFace{a, b}); len(clusters) != 2 {
		t.Errorf("two faces in one photo were grouped as the same person")
	}
}
