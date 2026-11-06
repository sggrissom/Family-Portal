package backend

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) recordVideoFaces(t *testing.T, video Image, frames ...FrameFaces) int {
	t.Helper()
	var tagged int
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		tagged = RecordVideoFacesTx(tx, video.Id, frames)
		vbolt.TxCommit(tx)
	})
	return tagged
}

func frameFaces(offsetMs int, faces ...DetectedFace) FrameFaces {
	return FrameFaces{OffsetMs: offsetMs, Faces: faces}
}

func TestVideoKeepsOneFacePerPerson(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	video := fx.addVideoAt(t, "party.mov", featuresEpoch)

	small, large := box(0.1, 0.1, 0.2, 0.2), box(0.1, 0.1, 0.4, 0.4)
	tagged := fx.recordVideoFaces(t, video,
		frameFaces(0, DetectedFace{Descriptor: faceVec(0, 0.1), Box: small}, DetectedFace{Descriptor: faceVec(40), Box: small}),
		frameFaces(5000, DetectedFace{Descriptor: faceVec(0, 0, 0.1), Box: large}, DetectedFace{Descriptor: faceVec(40, 0.05), Box: small}),
		frameFaces(10000, DetectedFace{Descriptor: faceVec(0, 0.05), Box: small}),
	)
	if tagged != 1 {
		t.Fatalf("tagged = %d, want alice once", tagged)
	}

	faces, tags := fx.facesOn(t, video)
	if len(faces) != 2 {
		t.Fatalf("stored %d faces, want one for alice and one for the stranger: %+v", len(faces), faces)
	}
	for _, f := range faces {
		switch f.PersonId {
		case fx.alice.Id:
			if f.Status != FaceAuto || f.FrameCount != 3 || f.FrameMs != 5000 || f.Box != large {
				t.Errorf("alice's face = %+v, want auto, seen in 3 frames, the large one at 5s", f)
			}
		case 0:
			if f.Status != FaceUnknown || f.FrameCount != 2 {
				t.Errorf("stranger's face = %+v, want unknown, seen in 2 frames", f)
			}
		default:
			t.Errorf("unexpected person on %+v", f)
		}
	}
	if len(tags) != 1 || tags[0].PersonId != fx.alice.Id || !tags[0].AutoTagged {
		t.Errorf("video tags = %+v, want one auto tag for alice", tags)
	}
}

func TestConfirmingVideoFaceGroupsKeepsEarlierAnswers(t *testing.T) {
	fx := setupResultsFixture(t)
	video := fx.addVideoAt(t, "angles.mov", featuresEpoch)
	crop := box(0.1, 0.1, 0.3, 0.3)
	fx.recordVideoFaces(t, video,
		frameFaces(0, DetectedFace{Descriptor: faceVec(0), Box: crop}),
		frameFaces(5000, DetectedFace{Descriptor: faceVec(40), Box: crop}),
		frameFaces(10000, DetectedFace{Descriptor: faceVec(80), Box: crop}),
		frameFaces(15000, DetectedFace{Descriptor: faceVec(120), Box: crop}),
	)

	faces, _ := fx.facesOn(t, video)
	if len(faces) != 4 {
		t.Fatalf("got %d face groups, want 4", len(faces))
	}
	for _, ids := range [][]int{{faces[0].Id}, {faces[1].Id}, {faces[2].Id, faces[3].Id}} {
		resp, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: ids, PersonId: fx.alice.Id})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Assigned != len(ids) || resp.Skipped != 0 {
			t.Errorf("AssignFaces(%v) = %+v, want every face confirmed", ids, resp)
		}
	}

	review, err := callAs(t, fx, GetFaceReview, GetFaceReviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if review.UnknownCount != 0 || review.AutoCount != 0 {
		t.Errorf("review still has %d unknown and %d automatic faces", review.UnknownCount, review.AutoCount)
	}
	faces, tags := fx.facesOn(t, video)
	for _, face := range faces {
		if face.PersonId != fx.alice.Id || face.Status != FaceConfirmed {
			t.Errorf("face %d lost its confirmation: person %d, status %d", face.Id, face.PersonId, face.Status)
		}
	}
	if len(tags) != 1 || tags[0].PersonId != fx.alice.Id || tags[0].AutoTagged {
		t.Errorf("video tags = %+v, want one confirmed tag for alice", tags)
	}
}

func TestOneFrameOfAPersonOnlySuggests(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	video := fx.addVideoAt(t, "blur.mov", featuresEpoch)

	if tagged := fx.recordVideoFaces(t, video,
		frameFaces(0, DetectedFace{Descriptor: faceVec(0, 0.1), Box: box(0, 0, 0.1, 0.1)}),
		frameFaces(5000),
	); tagged != 0 {
		t.Fatalf("tagged = %d from a single frame, want 0", tagged)
	}
	faces, tags := fx.facesOn(t, video)
	if len(faces) != 1 || faces[0].Status != FaceUnknown || len(tags) != 0 {
		t.Fatalf("faces = %+v tags = %+v, want one unknown face and no tag", faces, tags)
	}

	review, err := callAs(t, fx, GetFaceReview, GetFaceReviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Groups) != 1 || review.Groups[0].SuggestedPersonId != fx.alice.Id {
		t.Errorf("review groups = %+v, want the face suggested as alice", review.Groups)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if tagged := RematchFamilyFacesTx(tx, fx.familyId); tagged != 0 {
			t.Errorf("rematch tagged %d from a single frame", tagged)
		}
		vbolt.TxCommit(tx)
	})
}

func TestVideoFacesAreNotReferences(t *testing.T) {
	fx := setupResultsFixture(t)
	video := fx.addVideoAt(t, "clip.mov", featuresEpoch)
	fx.recordVideoFaces(t, video,
		frameFaces(0, DetectedFace{Descriptor: faceVec(30), Box: box(0, 0, 0.1, 0.1)}),
		frameFaces(5000, DetectedFace{Descriptor: faceVec(30, 0.05), Box: box(0, 0, 0.1, 0.1)}),
	)
	faces, _ := fx.facesOn(t, video)
	if _, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: []int{faces[0].Id}, PersonId: fx.bob.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, ref := range familyFaceReferences(tx, fx.familyId) {
			if ref.PersonId == fx.bob.Id {
				t.Errorf("a confirmed video face became a reference for bob: %d descriptors", len(ref.Descriptors))
			}
		}
	})

	photo := fx.addPhoto(t, fx.familyId, "bob.jpg")
	if tagged := fx.recordFaces(t, photo, DetectedFace{Descriptor: faceVec(30, 0.02)}); tagged != 0 {
		t.Errorf("a photo was tagged from a video face, tagged = %d", tagged)
	}
}

func TestVideoFacesKeepDecisionsThroughReanalysis(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.setProfileDescriptor(t, fx.alice, faceVec(0))
	video := fx.addVideoAt(t, "again.mov", featuresEpoch)
	frames := []FrameFaces{
		frameFaces(0, DetectedFace{Descriptor: faceVec(0, 0.1), Box: box(0, 0, 0.2, 0.2)}, DetectedFace{Descriptor: faceVec(50), Box: box(0.5, 0, 0.7, 0.2)}),
		frameFaces(4000, DetectedFace{Descriptor: faceVec(0, 0, 0.1), Box: box(0, 0, 0.2, 0.2)}, DetectedFace{Descriptor: faceVec(50, 0.05), Box: box(0.5, 0, 0.7, 0.2)}),
	}
	fx.recordVideoFaces(t, video, frames...)

	faces, _ := fx.facesOn(t, video)
	var alice, stranger PhotoFace
	for _, f := range faces {
		if f.PersonId == fx.alice.Id {
			alice = f
		} else {
			stranger = f
		}
	}
	if _, err := callAs(t, fx, RejectFaces, FaceIdsRequest{FaceIds: []int{alice.Id}}); err != nil {
		t.Fatal(err)
	}
	if _, err := callAs(t, fx, AssignFaces, AssignFacesRequest{FaceIds: []int{stranger.Id}, PersonId: fx.bob.Id}); err != nil {
		t.Fatal(err)
	}

	fx.recordVideoFaces(t, video, frames...)
	faces, tags := fx.facesOn(t, video)
	if len(faces) != 2 {
		t.Fatalf("after reanalysis %d faces, want 2", len(faces))
	}
	for _, f := range faces {
		if f.PersonId == fx.alice.Id {
			t.Errorf("a rejected match came back: %+v", f)
		}
		if f.PersonId == fx.bob.Id && f.Status != FaceConfirmed {
			t.Errorf("bob's confirmed face lost its status: %+v", f)
		}
	}
	for _, tag := range tags {
		if tag.PersonId == fx.alice.Id {
			t.Errorf("alice's rejected auto tag is back")
		}
	}
}

// startFakeFaces answers every recognize call with the same single face.
func startFakeFaces(t *testing.T, descriptor []float32) *http.Client {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "face.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /recognize", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(recognizeResponse{
			Faces:  []recognizedFace{{Descriptor: descriptor, Rect: [4]int{32, 24, 96, 72}}},
			Source: "data",
		})
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return newFaceClient(socket)
}

func TestVideoAnalysisFindsFacesInFrames(t *testing.T) {
	fx := setupVideoWorker(t)
	clip := makeClip(t, fx.worker.tools, "faces.mp4",
		"-f", "lavfi", "-i", "color=c=red:s=320x240:r=30:d=6",
		"-f", "lavfi", "-i", "color=c=blue:s=320x240:r=30:d=6",
		"-filter_complex", "[0][1]concat=n=2:v=1:a=0", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	video := fx.upload(t, "faces.mp4", "video/mp4", clip)
	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}

	aw := newVideoAnalysisWorker(fx.db, fx.worker.tools, startColorVision(t), nil, t.TempDir())
	aw.faces = startFakeFaces(t, faceVec(7))
	t.Cleanup(aw.cancel)
	if err := aw.process(video.Id); err != nil {
		t.Fatal(err)
	}

	var frames VideoFrames
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { frames, _ = GetVideoFrames(tx, video.Id) })
	if frames.FacesVersion != videoFacesVersion {
		t.Fatalf("faces version = %d, want %d", frames.FacesVersion, videoFacesVersion)
	}
	faces, _ := fx.facesOn(t, video)
	if len(faces) != 1 || faces[0].FrameCount != 2 {
		t.Fatalf("faces = %+v, want one face seen in both frames", faces)
	}
	if b := faces[0].Box; !hasFaceBoxForTest(b) {
		t.Errorf("box = %+v, want it relative to the frame", b)
	}
	if _, err := os.Stat(frameThumbPath(imageForTest(t, fx.db, video.Id), faces[0].FrameMs)); err != nil {
		t.Errorf("no frame thumbnail to crop the face from: %v", err)
	}
	if ids := videosNeedingAnalysis(fx.db, true, true, false); len(ids) != 0 {
		t.Errorf("still pending after analysis: %v", ids)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		frames.FacesVersion = 0
		writeVideoFramesTx(tx, &frames)
		vbolt.TxCommit(tx)
	})
	if ids := videosNeedingAnalysis(fx.db, true, true, false); len(ids) != 1 {
		t.Fatalf("an outdated faces version was not swept: %v", ids)
	}
	if ids := videosNeedingAnalysis(fx.db, true, false, false); len(ids) != 0 {
		t.Errorf("faces swept with face tagging off: %v", ids)
	}
}

func hasFaceBoxForTest(b FaceBox) bool {
	return 0 < b.Left && b.Left < b.Right && b.Right <= 1 && 0 < b.Top && b.Top < b.Bottom && b.Bottom <= 1
}
