package backend

import (
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func packImageV4(self *Image, buf *vpack.Buffer) {
	vpack.Version(4, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.OwnerUserId, buf)
	vpack.String(&self.OriginalFilename, buf)
	vpack.String(&self.MimeType, buf)
	vpack.Int(&self.FileSize, buf)
	vpack.Int(&self.Width, buf)
	vpack.Int(&self.Height, buf)
	vpack.String(&self.FilePath, buf)
	vpack.String(&self.Title, buf)
	vpack.String(&self.Description, buf)
	vpack.Time(&self.PhotoDate, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Int(&self.Status, buf)
	vpack.Int(&self.AnalysisStatus, buf)
	vpack.Int(&self.AnalysisVersion, buf)
}

func (fx resultsFixture) addVideoAt(t *testing.T, filename string, at time.Time) Image {
	t.Helper()
	video := fx.addPhotoAt(t, filename, at)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		video.MediaKind = MediaVideo
		video.MimeType = "video/quicktime"
		video.OriginalKey = "videos/" + filename
		video.PlaybackKey = "videos/" + filename + ".mp4"
		vbolt.Write(tx, ImagesBkt, video.Id, &video)
		vbolt.TxCommit(tx)
	})
	return video
}

func TestOldImageRowsDecodeAsPhotos(t *testing.T) {
	old := Image{
		Id: 7, FamilyId: 3, MimeType: "image/jpeg", FilePath: "photos/abc.jpg",
		Status: 0, AnalysisStatus: 2, AnalysisVersion: currentAnalysisVersion,
		PhotoDate: time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC),
	}
	data := vpack.ToBytes(&old, packImageV4)

	var decoded Image
	vpack.FromBytesInto(data, &decoded, PackImage)
	if decoded.IsVideo() || decoded.MediaKind != MediaPhoto {
		t.Fatalf("old row decoded as kind %d", decoded.MediaKind)
	}
	if decoded.FilePath != old.FilePath || decoded.AnalysisStatus != 2 || decoded.AnalysisVersion != currentAnalysisVersion {
		t.Fatalf("old fields not preserved: %+v", decoded)
	}

	video := Image{
		Id: 8, MediaKind: MediaVideo, DurationMs: 61500, VideoCodec: "hevc", AudioCodec: "aac",
		ProcessingError: "too long", OriginalKey: "videos/a/original.mov", PlaybackKey: "videos/a/playback.mp4",
		DerivedBytes: 1234,
	}
	var roundTrip Image
	vpack.FromBytesInto(vpack.ToBytes(&video, PackImage), &roundTrip, PackImage)
	if roundTrip.MediaKind != MediaVideo || roundTrip.DurationMs != 61500 || roundTrip.VideoCodec != "hevc" ||
		roundTrip.AudioCodec != "aac" || roundTrip.ProcessingError != "too long" ||
		roundTrip.OriginalKey != video.OriginalKey || roundTrip.PlaybackKey != video.PlaybackKey || roundTrip.DerivedBytes != 1234 {
		t.Fatalf("video fields not round-tripped: %+v", roundTrip)
	}
}

func TestVideosNeverNeedFaceAnalysis(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhotoAt(t, "still.jpg", featuresEpoch)
	video := fx.addVideoAt(t, "clip.mov", featuresEpoch)

	for _, status := range []int{0, 1, 3} {
		v := video
		v.AnalysisStatus = status
		if imageNeedsAnalysis(v) {
			t.Errorf("video with analysis status %d needs analysis", status)
		}
	}
	outdated := video
	outdated.AnalysisStatus = 2
	outdated.AnalysisVersion = currentAnalysisVersion - 1
	if imageNeedsAnalysis(outdated) {
		t.Error("video with an outdated analysis version needs analysis")
	}

	var queued []int
	for _, job := range photosNeedingAnalysis(fx.db) {
		queued = append(queued, job.ImageId)
	}
	if !slices.Contains(queued, photo.Id) || slices.Contains(queued, video.Id) {
		t.Fatalf("startup sweep queued %v, want photo %d and not video %d", queued, photo.Id, video.Id)
	}

	aw := &photoAnalysisWorker{db: fx.db}
	if err := aw.processAnalysisJob(PhotoAnalysisJob{ImageId: video.Id, FamilyId: video.FamilyId}); err != nil {
		t.Fatalf("processAnalysisJob(video) error = %v", err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetImageById(tx, video.Id).AnalysisStatus; got != 0 {
			t.Fatalf("worker touched the video's analysis status: %d", got)
		}
	})
}

func TestVideosStayOutOfPhotoDerivedWork(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhotoAt(t, "still.jpg", featuresEpoch)
	video := fx.addVideoAt(t, "clip.mov", featuresEpoch)

	if ids := photosNeedingFeatures(fx.db); !slices.Contains(ids, photo.Id) || slices.Contains(ids, video.Id) {
		t.Errorf("features sweep = %v", ids)
	}
	analyzePhotoFeatures(fx.db, video.Id)
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := GetPhotoFeatures(tx, video.Id); found {
			t.Error("features recorded for a video")
		}
	})
}

func TestVideosAreNotPortraitsOrProfilePhotos(t *testing.T) {
	fx := setupResultsFixture(t)
	born := fx.alice.Birthday
	photo := fx.addPhotoAt(t, "baby.jpg", born.AddDate(0, 2, 0))
	video := fx.addVideoAt(t, "first-steps.mov", born.AddDate(1, 0, 0))
	fx.tagPerson(t, photo, fx.alice)
	fx.tagPerson(t, video, fx.alice)
	fx.addFace(t, video, fx.alice, box(0.3, 0.2, 0.7, 0.6), FaceConfirmed)

	resp, err := callAs(t, fx, GetPersonPhotoInsights, GetPersonPhotoInsightsRequest{PersonId: fx.alice.Id})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range resp.GrowingUp {
		if p.PhotoId == video.Id {
			t.Fatal("video chosen for Growing Up")
		}
	}
	if resp.Header != nil && resp.Header.PhotoId == video.Id {
		t.Fatal("video chosen as the header portrait")
	}

	if _, err := callAs(t, fx, SetProfilePhoto, SetProfilePhotoRequest{PersonId: fx.alice.Id, PhotoId: video.Id}); err == nil {
		t.Fatal("SetProfilePhoto accepted a video")
	}
	if _, err := callAs(t, fx, SetProfilePhoto, SetProfilePhotoRequest{PersonId: fx.alice.Id, PhotoId: photo.Id}); err != nil {
		t.Fatalf("SetProfilePhoto(photo) error = %v", err)
	}
}

func TestVideosAreNotExportedAsPhotos(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhotoAt(t, "still.jpg", featuresEpoch)
	fx.addVideoAt(t, "clip.mov", featuresEpoch)

	var exported []ExportPhoto
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		exported = buildPhotoExportMetadata(tx, fx.familyId)
	})
	if len(exported) != 1 || exported[0].Id != photo.Id {
		t.Fatalf("exported %+v, want only photo %d", exported, photo.Id)
	}
}
