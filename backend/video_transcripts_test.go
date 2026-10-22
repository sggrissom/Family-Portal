package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.hasen.dev/vbolt"
)

const whisperJSON = `{"result":{"language":"en"},"transcription":[
 {"offsets":{"from":0,"to":2000},"text":" [MUSIC]","tokens":[{"text":" [MUSIC]","p":0.9}]},
 {"offsets":{"from":2000,"to":5000},"text":" The T-rex is my favorite dinosaur.","tokens":[
   {"text":"[_BEG_]","p":0.1},{"text":" The","p":0.9},{"text":" T","p":0.8},{"text":"-rex","p":0.7}]},
 {"offsets":{"from":5000,"to":6000},"text":" Thank you.","tokens":[{"text":" Thank","p":0.3},{"text":" you.","p":0.3}]},
 {"offsets":{"from":6000,"to":8000},"text":" Can we go to the museum?","tokens":[{"text":" Can","p":0.95}]}
]}`

func TestParseAndCleanWhisperOutput(t *testing.T) {
	language, segments, err := parseWhisperOutput([]byte(whisperJSON))
	if err != nil || language != "en" || len(segments) != 4 {
		t.Fatalf("parsed %q %d segments, %v", language, len(segments), err)
	}
	if c := segments[1].Confidence; c < 0.79 || c > 0.81 {
		t.Errorf("confidence %.3f, want the mean of the text tokens, 0.8", c)
	}
	cleaned := cleanTranscript(segments)
	if len(cleaned) != 2 || cleaned[0].Text != "The T-rex is my favorite dinosaur." || cleaned[1].StartMs != 6000 {
		t.Fatalf("cleaned = %+v", cleaned)
	}

	sure := []TranscriptSegment{{Text: "Thank you.", Confidence: 0.9}}
	if got := cleanTranscript(sure); len(got) != 1 {
		t.Error("dropped a confident thank-you")
	}
	loop := []TranscriptSegment{{Text: "Again."}, {Text: "again."}, {Text: "Again."}, {Text: "Again."}, {Text: "Done."}}
	if got := cleanTranscript(loop); len(got) != 3 || got[2].Text != "Done." {
		t.Errorf("repeats kept as %+v", got)
	}
	for _, text := range []string{"(laughs)", "♪ ♪", "...", "*applause*", ""} {
		if !isAnnotation(text) {
			t.Errorf("%q is not an annotation", text)
		}
	}
}

func TestTranscriptVTT(t *testing.T) {
	vtt := transcriptVTT(VideoTranscript{Segments: []TranscriptSegment{
		{StartMs: 2000, EndMs: 5500, Text: "Hi --> there"},
		{StartMs: 3723004, EndMs: 3725000, Text: "Later"},
	}})
	want := "WEBVTT\n\n1\n00:00:02.000 --> 00:00:05.500\nHi -> there\n\n2\n01:02:03.004 --> 01:02:05.000\nLater\n"
	if vtt != want {
		t.Errorf("vtt =\n%s\nwant\n%s", vtt, want)
	}
}

// fakeSpeech is a whisper-cli stand-in that writes canned output wherever
// -of points.
func fakeSpeech(t *testing.T, output string) *speechEngine {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "canned.json"), []byte(output), 0644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-of\" ]; then out=\"$2\"; fi\n  shift\ndone\ncp \"" +
		filepath.Join(dir, "canned.json") + "\" \"$out.json\"\n"
	if err := os.WriteFile(filepath.Join(dir, "whisper-cli"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, speechModelFile), []byte("model"), 0644); err != nil {
		t.Fatal(err)
	}
	e, err := findSpeechEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	previous := speech
	speech = e
	t.Cleanup(func() { speech = previous })
	return e
}

func TestVideoTranscriptsAreSearchableEditableAndRemovable(t *testing.T) {
	fx := setupVideoWorker(t)
	e := fakeSpeech(t, whisperJSON)
	clip := makeClip(t, fx.worker.tools, "talk.mp4",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30", "-f", "lavfi", "-i", "sine=frequency=440",
		"-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	video := fx.upload(t, "talk.mp4", "video/mp4", clip)
	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	silent := fx.upload(t, "silent.mp4", "video/mp4", makeClip(t, fx.worker.tools, "silent.mp4",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p"))
	if err := fx.worker.process(silent.Id); err != nil {
		t.Fatal(err)
	}

	aw := newVideoAnalysisWorker(fx.db, fx.worker.tools, nil, e, t.TempDir())
	t.Cleanup(aw.cancel)
	if ids := videosNeedingAnalysis(fx.db, false, true); len(ids) != 2 {
		t.Fatalf("sweep = %v, want both videos", ids)
	}
	for _, v := range []Image{video, silent} {
		if err := aw.process(v.Id); err != nil {
			t.Fatal(err)
		}
	}
	if ids := videosNeedingAnalysis(fx.db, false, true); len(ids) != 0 {
		t.Errorf("still pending: %v", ids)
	}

	got, err := callAs(t, fx.resultsFixture, GetVideoTranscript, GetVideoTranscriptRequest{Id: video.Id})
	if err != nil || got.Status != TranscriptDone || !got.Automatic || len(got.Segments) != 2 || got.CaptionUrl == "" {
		t.Fatalf("transcript = %+v, %v", got, err)
	}
	quiet, _ := callAs(t, fx.resultsFixture, GetVideoTranscript, GetVideoTranscriptRequest{Id: silent.Id})
	if quiet.Status != TranscriptNoSpeech || len(quiet.Segments) != 0 {
		t.Errorf("video without audio: %+v", quiet)
	}

	search := func(query string) []PhotoWithPeople {
		t.Helper()
		resp, err := callAs(t, fx.resultsFixture, ListFamilyPhotos, ListFamilyPhotosRequest{Query: query, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Photos
	}
	hits := search("dinosaur")
	if len(hits) != 1 || hits[0].Image.Id != video.Id {
		t.Fatalf("dinosaur hits = %v", photoIds(hits))
	}
	if m := hits[0].Match; m == nil || m.Source != MatchTranscript || m.OffsetMs != 2000 || !strings.Contains(m.Snippet, "T-rex") {
		t.Errorf("match = %+v", m)
	}
	global, err := callAs(t, fx.resultsFixture, Search, SearchRequest{Query: "museum"})
	if err != nil || len(global.Photos) != 1 || global.Photos[0].Match == nil || global.Photos[0].Match.OffsetMs != 6000 {
		t.Errorf("global museum = %+v, %v", global.Photos, err)
	}

	edited, err := callAs(t, fx.resultsFixture, UpdateVideoTranscript, UpdateVideoTranscriptRequest{
		Id: video.Id, Texts: []string{"The T-rex is my favorite dinosaur.", "Can we go to the aquarium?"},
	})
	if err != nil || edited.Automatic || edited.Segments[1].Text != "Can we go to the aquarium?" {
		t.Fatalf("edited = %+v, %v", edited, err)
	}
	if len(search("museum")) != 0 || len(search("aquarium")) != 1 {
		t.Error("search did not follow the edit")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if videoNeedsTranscript(tx, imageForTest(t, fx.db, video.Id)) {
			t.Error("an edited transcript would be transcribed again")
		}
	})
	if _, err := callAs(t, fx.resultsFixture, UpdateVideoTranscript, UpdateVideoTranscriptRequest{Id: video.Id, Texts: []string{"one"}}); err == nil {
		t.Error("accepted texts that don't line up with the segments")
	}

	removed, err := callAs(t, fx.resultsFixture, RemoveVideoTranscript, GetVideoTranscriptRequest{Id: video.Id})
	if err != nil || removed.Status != TranscriptRemoved || len(removed.Segments) != 0 {
		t.Fatalf("removed = %+v, %v", removed, err)
	}
	if len(search("dinosaur")) != 0 {
		t.Error("a removed transcript is still searchable")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if videoNeedsTranscript(tx, imageForTest(t, fx.db, video.Id)) {
			t.Error("a removed transcript would come back")
		}
	})

	if _, err := callAs(t, fx.resultsFixture, DeletePhoto, DeletePhotoRequest{Id: video.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if _, found := GetVideoTranscriptRecord(tx, video.Id); found {
			t.Error("deleted video kept its transcript")
		}
	})
}

// Set FAMILY_SPEECH_DIR to an install-speech.sh directory to run the real
// whisper-cli over its sample.
func TestRealWhisperTranscribesTheSample(t *testing.T) {
	dir := os.Getenv("FAMILY_SPEECH_DIR")
	if dir == "" {
		t.Skip("FAMILY_SPEECH_DIR not set")
	}
	e, err := findSpeechEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	fx := setupVideoWorker(t)
	clip := makeClip(t, fx.worker.tools, "jfk.mp4",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30", "-i", filepath.Join(dir, "bin", "jfk.wav"),
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	video := fx.upload(t, "jfk.mp4", "video/mp4", clip)
	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	aw := newVideoAnalysisWorker(fx.db, fx.worker.tools, nil, e, t.TempDir())
	t.Cleanup(aw.cancel)
	if err := aw.process(video.Id); err != nil {
		t.Fatal(err)
	}
	var tr VideoTranscript
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { tr, _ = GetVideoTranscriptRecord(tx, video.Id) })
	var text []string
	for _, s := range tr.Segments {
		text = append(text, s.Text)
	}
	if tr.Status != TranscriptDone || !strings.Contains(strings.ToLower(strings.Join(text, " ")), "fellow americans") {
		t.Fatalf("transcript %+v", tr)
	}
	t.Logf("%s: %+v", tr.Model, tr.Segments)
}
