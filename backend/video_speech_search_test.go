package backend

import (
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) setTranscript(t *testing.T, video Image, segments ...TranscriptSegment) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		writeVideoTranscriptTx(tx, &VideoTranscript{
			ImageId: video.Id, FamilyId: video.FamilyId, Model: "test", Version: transcriptVersion,
			Status: TranscriptDone, Segments: segments,
		})
		vbolt.TxCommit(tx)
	})
}

func (fx resultsFixture) speechVectors(t *testing.T, video Image) (v SpeechVectors, found bool) {
	t.Helper()
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { v, found = GetSpeechVectors(tx, video.Id) })
	return
}

func TestSpeechWindowsOverlapAndKeepWholeSegments(t *testing.T) {
	var segments []TranscriptSegment
	for at := 0; at < 70000; at += 10000 {
		segments = append(segments, TranscriptSegment{StartMs: at, EndMs: at + 9000, Text: string(rune('a' + at/10000))})
	}
	windows := speechWindows(VideoTranscript{Status: TranscriptDone, Segments: segments})
	var got []string
	for _, w := range windows {
		got = append(got, w.Text)
	}
	want := []string{"a b c", "c d e", "e f g"}
	if !slices.Equal(got, want) {
		t.Errorf("windows = %q, want %q", got, want)
	}
	if windows[1].StartMs != 20000 {
		t.Errorf("second window starts at %d, want 20000", windows[1].StartMs)
	}
	if speechWindows(VideoTranscript{Status: TranscriptRemoved, Segments: segments}) != nil {
		t.Error("a removed transcript has windows")
	}
}

func TestSearchFindsAVideoByWhatIsMeant(t *testing.T) {
	fx := setupResultsFixture(t)
	client := useFakeVision(t, map[string]int{
		"talking about dinosaurs":  3,
		"the T rex is my favorite": 3,
	})
	video := fx.addVideoAt(t, "museum.mov", featuresEpoch)
	fx.setEmbedding(t, video, axis(1))
	fx.setTranscript(t, video,
		TranscriptSegment{StartMs: 0, EndMs: 4000, Text: "the T rex is my favorite"},
		TranscriptSegment{StartMs: 40000, EndMs: 44000, Text: "can we get ice cream"})
	if ids := videosNeedingAnalysis(fx.db, true, false, false); !slices.Contains(ids, video.Id) {
		t.Fatalf("sweep %v is missing the transcribed video", ids)
	}
	if err := embedSpeech(fx.db, client, video.Id); err != nil {
		t.Fatal(err)
	}
	if v, found := fx.speechVectors(t, video); !found || len(v.Windows) != 2 {
		t.Fatalf("speech vectors = %+v, want two windows", v)
	}

	outsider := fx.addVideoAt(t, "theirs.mov", featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		outsider.FamilyId = fx.familyId + 999
		vbolt.Write(tx, ImagesBkt, outsider.Id, &outsider)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, outsider.Id, outsider.FamilyId)
		ReindexPhotoDates(tx, outsider.Id)
		vbolt.TxCommit(tx)
	})
	fx.setTranscript(t, outsider, TranscriptSegment{StartMs: 0, EndMs: 4000, Text: "the T rex is my favorite"})
	if err := embedSpeech(fx.db, client, outsider.Id); err != nil {
		t.Fatal(err)
	}

	resp, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "talking about dinosaurs", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{video.Id}) {
		t.Fatalf("results = %v, want only the family's video %d", got, video.Id)
	}
	if m := resp.Photos[0].Match; m == nil || m.Source != MatchTranscript || m.OffsetMs != 0 || m.Snippet != "the T rex is my favorite" {
		t.Errorf("match = %+v, want the line where it's said", m)
	}

	global, err := callAs(t, fx, Search, SearchRequest{Query: "talking about dinosaurs"})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(global.Photos); !slices.Equal(got, []int{video.Id}) {
		t.Errorf("global search photos = %v, want [%d]", got, video.Id)
	}

	if _, err := callAs(t, fx, UpdateVideoTranscript, UpdateVideoTranscriptRequest{
		Id: video.Id, Texts: []string{"we went swimming", "can we get ice cream"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, found := fx.speechVectors(t, video); found {
		t.Error("an edit left the old meaning searchable")
	}
	resp, _ = callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "talking about dinosaurs", Limit: 10})
	if len(resp.Photos) != 0 {
		t.Errorf("edited-away line still found: %v", photoIds(resp.Photos))
	}
	if err := embedSpeech(fx.db, client, video.Id); err != nil {
		t.Fatal(err)
	}
	if v, found := fx.speechVectors(t, video); !found || v.Windows[0].Text != "we went swimming" {
		t.Errorf("after the edit, vectors = %+v", v)
	}

	if _, err := callAs(t, fx, RemoveVideoTranscript, GetVideoTranscriptRequest{Id: video.Id}); err != nil {
		t.Fatal(err)
	}
	if _, found := fx.speechVectors(t, video); found {
		t.Error("a removed transcript kept its vectors")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if videoNeedsSpeechVectors(tx, GetImageById(tx, video.Id)) {
			t.Error("a removed transcript is queued for embedding")
		}
	})
}

func TestSuggestMilestonePhotosFindsTheClipWhereItWasSaid(t *testing.T) {
	fx := setupResultsFixture(t)
	client := useFakeVision(t, map[string]int{
		"First word: mama":     4,
		"mama mama":            4,
		"a photo of bath time": 2,
	})
	day := fx.alice.Birthday.AddDate(1, 0, 0)
	bath := fx.addPhotoAt(t, "bath.jpg", day)
	quiet := fx.addVideoAt(t, "quiet.mov", day.Add(time.Hour))
	clip := fx.addVideoAt(t, "mama.mov", day.Add(2*24*time.Hour))
	for _, p := range []Image{bath, quiet, clip} {
		fx.tagPerson(t, p, fx.alice)
	}
	fx.setTranscript(t, clip, TranscriptSegment{StartMs: 3000, EndMs: 5000, Text: "mama mama"})
	if err := embedSpeech(fx.db, client, clip.Id); err != nil {
		t.Fatal(err)
	}

	resp, err := callAs(t, fx, SuggestMilestonePhotos, SuggestMilestonePhotosRequest{
		PersonId: fx.alice.Id, Description: "First word: mama", RecordTime: exactRecordTime(day, SourceFamily),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Ranked || len(resp.PhotoIds) != 3 || resp.PhotoIds[0] != clip.Id {
		t.Errorf("suggested %v (ranked %v), want the clip %d first", resp.PhotoIds, resp.Ranked, clip.Id)
	}
}
