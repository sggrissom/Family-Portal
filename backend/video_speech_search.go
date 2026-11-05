package backend

import (
	"errors"
	"family/cfg"
	"family/vision"
	"strings"
	"sync"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const (
	speechVectorsVersion = 1
	speechWindowMs       = 30000
	speechWindowStepMs   = 15000
	speechSnippetChars   = 160
)

// SpeechWindow is about thirty seconds of a transcript, embedded with the
// sentence model so a search can match what was meant rather than said.
type SpeechWindow struct {
	StartMs int
	Text    string
	Vector  []float32
}

type SpeechVectors struct {
	ImageId  int
	FamilyId int
	Model    string
	Version  int
	Windows  []SpeechWindow
}

func PackSpeechWindow(self *SpeechWindow, buf *vpack.Buffer) {
	vpack.Int(&self.StartMs, buf)
	vpack.String(&self.Text, buf)
	packFloat32Slice(&self.Vector, buf)
}

func PackSpeechVectors(self *SpeechVectors, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.ImageId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Model, buf)
	vpack.Int(&self.Version, buf)
	vpack.Slice(&self.Windows, PackSpeechWindow, buf)
}

var SpeechVectorsBkt = vbolt.Bucket(&cfg.Info, "speech_vectors", vpack.FInt, PackSpeechVectors)
var SpeechVectorsByFamilyIndex = vbolt.Index(&cfg.Info, "speech_vectors_by_family", vpack.FInt, vpack.FInt)

func GetSpeechVectors(tx *vbolt.Tx, imageId int) (v SpeechVectors, ok bool) {
	ok = vbolt.Read(tx, SpeechVectorsBkt, imageId, &v)
	return
}

func writeSpeechVectorsTx(tx *vbolt.Tx, v *SpeechVectors) {
	vbolt.Write(tx, SpeechVectorsBkt, v.ImageId, v)
	vbolt.SetTargetSingleTerm(tx, SpeechVectorsByFamilyIndex, v.ImageId, v.FamilyId)
	invalidateFamilySpeech(v.FamilyId)
}

func deleteSpeechVectorsTx(tx *vbolt.Tx, imageId int) {
	v, ok := GetSpeechVectors(tx, imageId)
	if !ok {
		return
	}
	vbolt.Delete(tx, SpeechVectorsBkt, imageId)
	vbolt.DeleteTargetTerms(tx, SpeechVectorsByFamilyIndex, imageId)
	invalidateFamilySpeech(v.FamilyId)
}

// speechWindows splits a transcript into overlapping windows of whole
// segments, each starting about fifteen seconds after the last.
func speechWindows(t VideoTranscript) (windows []SpeechWindow) {
	if t.Status != TranscriptDone {
		return nil
	}
	segments := t.Segments
	nextStart := 0
	for i, first := range segments {
		if first.StartMs < nextStart && i > 0 {
			continue
		}
		var texts []string
		last := i
		for j := i; j < len(segments) && (j == i || segments[j].EndMs-first.StartMs <= speechWindowMs); j++ {
			texts = append(texts, segments[j].Text)
			last = j
		}
		windows = append(windows, SpeechWindow{StartMs: first.StartMs, Text: strings.Join(texts, " ")})
		if last == len(segments)-1 {
			break
		}
		nextStart = first.StartMs + speechWindowStepMs
	}
	return
}

// speechVectorsOutdated is a done transcript whose windows aren't embedded
// as they now read.
func speechVectorsOutdated(tx *vbolt.Tx, imageId int) bool {
	t, ok := GetVideoTranscriptRecord(tx, imageId)
	if !ok || t.Status != TranscriptDone {
		return false
	}
	v, found := GetSpeechVectors(tx, imageId)
	if !found || v.Model != sentenceModelId || v.Version < speechVectorsVersion {
		return true
	}
	return !sameWindows(speechWindows(t), v.Windows)
}

// dropStaleSpeechVectorsTx removes vectors that no longer match what the
// transcript says, so an edited or removed line stops matching at once.
func dropStaleSpeechVectorsTx(tx *vbolt.Tx, t VideoTranscript) {
	if v, ok := GetSpeechVectors(tx, t.ImageId); ok && !sameWindows(speechWindows(t), v.Windows) {
		deleteSpeechVectorsTx(tx, t.ImageId)
	}
}

func videoNeedsSpeechVectors(tx *vbolt.Tx, image Image) bool {
	return image.IsVideo() && image.Status == PhotoReady && speechVectorsOutdated(tx, image.Id)
}

// embedSpeech embeds the windows of a video's transcript. It needs no copy of
// the video, so it also runs after an edit.
func embedSpeech(db *vbolt.DB, client *visionClient, imageId int) error {
	var t VideoTranscript
	var outdated bool
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		t, _ = GetVideoTranscriptRecord(tx, imageId)
		outdated = speechVectorsOutdated(tx, imageId)
	})
	if !outdated {
		return nil
	}
	windows := speechWindows(t)
	texts := make([]string, len(windows))
	for i, w := range windows {
		texts[i] = w.Text
	}
	vectors, err := embedSentences(client, texts)
	if errors.Is(err, errVisionUnavailable) || errors.Is(err, errRetryLater) {
		return errRetryLater
	}
	if err != nil {
		LogWarn(LogCategoryWorker, "Transcript embedding failed", map[string]interface{}{"videoId": imageId, "error": err.Error()})
		return nil
	}
	for i := range windows {
		windows[i].Vector = vectors[i]
	}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		current, ok := GetVideoTranscriptRecord(tx, imageId)
		if !ok || current.Status != TranscriptDone || !sameWindows(speechWindows(current), windows) {
			return
		}
		writeSpeechVectorsTx(tx, &SpeechVectors{
			ImageId: imageId, FamilyId: current.FamilyId,
			Model: sentenceModelId, Version: speechVectorsVersion, Windows: windows,
		})
		vbolt.TxCommit(tx)
	})
	return nil
}

func sameWindows(a, b []SpeechWindow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].StartMs != b[i].StartMs || a[i].Text != b[i].Text {
			return false
		}
	}
	return true
}

// familySpeech caches each family's transcript windows by video, like
// familyFrames does for frames.
var familySpeech = struct {
	sync.Mutex
	byFamily map[int]map[int][]SpeechWindow
}{byFamily: map[int]map[int][]SpeechWindow{}}

func invalidateFamilySpeech(familyId int) {
	familySpeech.Lock()
	delete(familySpeech.byFamily, familyId)
	familySpeech.Unlock()
}

func speechForFamily(tx *vbolt.Tx, familyId int) map[int][]SpeechWindow {
	familySpeech.Lock()
	cached, ok := familySpeech.byFamily[familyId]
	familySpeech.Unlock()
	if ok {
		return cached
	}
	var ids []int
	vbolt.ReadTermTargets(tx, SpeechVectorsByFamilyIndex, familyId, &ids, vbolt.Window{})
	var records []SpeechVectors
	vbolt.ReadSlice(tx, SpeechVectorsBkt, ids, &records)
	loaded := map[int][]SpeechWindow{}
	for _, v := range records {
		if v.Model == sentenceModelId && len(v.Windows) > 0 {
			loaded[v.ImageId] = v.Windows
		}
	}
	familySpeech.Lock()
	familySpeech.byFamily[familyId] = loaded
	familySpeech.Unlock()
	return loaded
}

// spokenMatch is the window of a video's transcript closest in meaning to
// query, when it clears the threshold moment search uses.
func spokenMatch(tx *vbolt.Tx, familyId, imageId int, query []float32) (window SpeechWindow, score float64, ok bool) {
	if query == nil {
		return
	}
	for _, w := range speechForFamily(tx, familyId)[imageId] {
		if s := float64(vision.Dot(query, w.Vector)); s >= milestoneSearchMinScore && (!ok || s > score) {
			window, score, ok = w, s, true
		}
	}
	return
}

func familyHasSpeech(tx *vbolt.Tx, familyIds []int) bool {
	for _, id := range familyIds {
		if len(speechForFamily(tx, id)) > 0 {
			return true
		}
	}
	return false
}

// embedSpeechQuery embeds a search with the sentence model, or nil when the
// daemon can't.
func embedSpeechQuery(text string) []float32 {
	vectors, err := embedSentences(globalVisionClient, []string{text})
	if err != nil {
		return nil
	}
	return vectors[0]
}

func speechSnippet(text string) string {
	if len(text) <= speechSnippetChars {
		return text
	}
	cut := strings.LastIndexByte(text[:speechSnippetChars], ' ')
	if cut <= 0 {
		cut = speechSnippetChars
	}
	return text[:cut] + "…"
}
