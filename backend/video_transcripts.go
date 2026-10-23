package backend

import (
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const transcriptVersion = 1

type TranscriptStatus int

const (
	TranscriptDone TranscriptStatus = iota
	TranscriptNoSpeech
	TranscriptFailed
	TranscriptRemoved
)

type TranscriptSegment struct {
	StartMs    int     `json:"startMs"`
	EndMs      int     `json:"endMs"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

type VideoTranscript struct {
	ImageId   int                 `json:"imageId"`
	FamilyId  int                 `json:"familyId"`
	Model     string              `json:"model"`
	Version   int                 `json:"-"`
	Language  string              `json:"language"`
	Status    TranscriptStatus    `json:"status"`
	Segments  []TranscriptSegment `json:"segments"`
	EditedBy  int                 `json:"editedBy"`
	EditedAt  time.Time           `json:"editedAt"`
	CreatedAt time.Time           `json:"createdAt"`
}

func PackTranscriptSegment(self *TranscriptSegment, buf *vpack.Buffer) {
	vpack.Int(&self.StartMs, buf)
	vpack.Int(&self.EndMs, buf)
	vpack.String(&self.Text, buf)
	vpack.Float64(&self.Confidence, buf)
}

func PackVideoTranscript(self *VideoTranscript, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.ImageId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Model, buf)
	vpack.Int(&self.Version, buf)
	vpack.String(&self.Language, buf)
	vpack.IntEnum(&self.Status, buf)
	vpack.Slice(&self.Segments, PackTranscriptSegment, buf)
	vpack.Int(&self.EditedBy, buf)
	vpack.Time(&self.EditedAt, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var VideoTranscriptBkt = vbolt.Bucket(&cfg.Info, "video_transcripts", vpack.FInt, PackVideoTranscript)

func GetVideoTranscriptRecord(tx *vbolt.Tx, imageId int) (t VideoTranscript, ok bool) {
	ok = vbolt.Read(tx, VideoTranscriptBkt, imageId, &t)
	return
}

// transcriptSettled is a transcript a rerun must leave alone: edited by
// someone, or removed on purpose.
func transcriptSettled(t VideoTranscript) bool {
	return t.EditedBy != 0 || t.Status == TranscriptRemoved
}

func transcriptOutdated(t VideoTranscript, found bool) bool {
	if !found {
		return true
	}
	return !transcriptSettled(t) && (t.Model != speechModelName() || t.Version < transcriptVersion)
}

func videoNeedsTranscript(tx *vbolt.Tx, image Image) bool {
	if !image.IsVideo() || image.Status != 0 || image.PlaybackKey == "" {
		return false
	}
	t, found := GetVideoTranscriptRecord(tx, image.Id)
	return transcriptOutdated(t, found)
}

func writeVideoTranscriptTx(tx *vbolt.Tx, t *VideoTranscript) {
	vbolt.Write(tx, VideoTranscriptBkt, t.ImageId, t)
	ReindexPhotoDates(tx, t.ImageId)
}

func deleteVideoTranscriptTx(tx *vbolt.Tx, imageId int) {
	vbolt.Delete(tx, VideoTranscriptBkt, imageId)
}

// transcriptWords are the words search indexes for a video, without filler,
// which queries drop anyway.
func transcriptWords(tx *vbolt.Tx, imageId int) []string {
	t, ok := GetVideoTranscriptRecord(tx, imageId)
	if !ok || t.Status != TranscriptDone {
		return nil
	}
	var words []string
	for _, s := range t.Segments {
		for _, word := range searchFieldWords(s.Text) {
			if !searchFillerWords[word] {
				words = append(words, word)
			}
		}
	}
	return words
}

// transcriptMatch finds the first segment holding every term, or failing
// that the first holding any.
func transcriptMatch(t VideoTranscript, terms []string) (TranscriptSegment, bool) {
	if t.Status != TranscriptDone || len(terms) == 0 {
		return TranscriptSegment{}, false
	}
	for _, s := range t.Segments {
		if termsMatch(terms, searchFieldWords(s.Text)) {
			return s, true
		}
	}
	for _, s := range t.Segments {
		words := searchFieldWords(s.Text)
		for _, term := range terms {
			if termsMatch([]string{term}, words) {
				return s, true
			}
		}
	}
	return TranscriptSegment{}, false
}

type whisperOutput struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []struct {
		Offsets struct {
			From int `json:"from"`
			To   int `json:"to"`
		} `json:"offsets"`
		Text   string `json:"text"`
		Tokens []struct {
			Text string  `json:"text"`
			P    float64 `json:"p"`
		} `json:"tokens"`
	} `json:"transcription"`
}

func parseWhisperOutput(data []byte) (language string, segments []TranscriptSegment, err error) {
	var out whisperOutput
	if err = json.Unmarshal(data, &out); err != nil {
		return
	}
	language = out.Result.Language
	for _, s := range out.Transcription {
		var sum float64
		var n int
		for _, tok := range s.Tokens {
			if strings.HasPrefix(tok.Text, "[_") {
				continue
			}
			sum += tok.P
			n++
		}
		seg := TranscriptSegment{StartMs: s.Offsets.From, EndMs: s.Offsets.To, Text: strings.TrimSpace(s.Text)}
		if n > 0 {
			seg.Confidence = sum / float64(n)
		}
		segments = append(segments, seg)
	}
	return
}

// Phrases Whisper is known to invent over silence and music, learned from
// video subtitles. Kept only when it was confident.
var stockPhrases = []string{
	"thank you for watching", "thanks for watching", "please subscribe",
	"subscribe to my channel", "like and subscribe", "see you in the next video",
	"thank you", "thank you very much", "you", "bye",
}

const (
	stockPhraseMinConfidence = 0.75
	maxRepeats               = 2
)

func isAnnotation(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	if (strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]")) ||
		(strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")")) ||
		(strings.HasPrefix(text, "*") && strings.HasSuffix(text, "*")) {
		return true
	}
	return strings.Trim(text, " .,!?-♪♫…") == ""
}

func isStockPhrase(text string) bool {
	folded := strings.Join(searchFieldWords(text), " ")
	for _, p := range stockPhrases {
		if folded == p {
			return true
		}
	}
	return false
}

// cleanTranscript drops what Whisper invents rather than hears: sound
// annotations, stock outro lines it wasn't sure of, and runs of the same
// line.
func cleanTranscript(segments []TranscriptSegment) []TranscriptSegment {
	var out []TranscriptSegment
	repeats := 0
	for _, s := range segments {
		if isAnnotation(s.Text) {
			continue
		}
		if isStockPhrase(s.Text) && s.Confidence < stockPhraseMinConfidence {
			continue
		}
		if n := len(out); n > 0 && strings.EqualFold(out[n-1].Text, s.Text) {
			repeats++
			if repeats >= maxRepeats {
				continue
			}
		} else {
			repeats = 0
		}
		out = append(out, s)
	}
	return out
}

type GetVideoTranscriptRequest struct {
	Id int `json:"id"`
}

type GetVideoTranscriptResponse struct {
	Available  bool                `json:"available"`
	Status     TranscriptStatus    `json:"status"`
	Pending    bool                `json:"pending"`
	Automatic  bool                `json:"automatic"`
	Language   string              `json:"language"`
	Segments   []TranscriptSegment `json:"segments"`
	CanEdit    bool                `json:"canEdit"`
	CaptionUrl string              `json:"captionUrl"`
}

var ErrTranscriptNotFound = errors.New("Transcript not found or access denied")

func transcriptResponse(tx *vbolt.Tx, user User, image Image) (resp GetVideoTranscriptResponse) {
	resp.Segments = []TranscriptSegment{}
	resp.Available = speechAvailable()
	resp.CanEdit = CanAccessFamily(tx, user, image.FamilyId, AccessContribute)
	t, found := GetVideoTranscriptRecord(tx, image.Id)
	if !found {
		resp.Pending = image.Status == 0 && resp.Available
		resp.Status = TranscriptNoSpeech
		return
	}
	resp.Status = t.Status
	resp.Automatic = t.EditedBy == 0
	resp.Language = t.Language
	if t.Status == TranscriptDone {
		resp.Segments = t.Segments
		resp.CaptionUrl = fmt.Sprintf("/api/photo/%d/captions", image.Id)
	}
	return
}

func GetVideoTranscript(ctx *vbeam.Context, req GetVideoTranscriptRequest) (resp GetVideoTranscriptResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	image := GetImageById(ctx.Tx, req.Id)
	if image.Id == 0 || !image.IsVideo() || !CanAccessPhoto(ctx.Tx, user, image, AccessView) {
		err = ErrTranscriptNotFound
		return
	}
	resp = transcriptResponse(ctx.Tx, user, image)
	return
}

type UpdateVideoTranscriptRequest struct {
	Id    int      `json:"id"`
	Texts []string `json:"texts"`
}

const maxSegmentChars = 2000

func UpdateVideoTranscript(ctx *vbeam.Context, req UpdateVideoTranscriptRequest) (resp GetVideoTranscriptResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	image := GetImageById(ctx.Tx, req.Id)
	if image.Id == 0 || !image.IsVideo() || !CanAccessFamily(ctx.Tx, user, image.FamilyId, AccessContribute) {
		err = ErrTranscriptNotFound
		return
	}
	t, found := GetVideoTranscriptRecord(ctx.Tx, image.Id)
	if !found || t.Status != TranscriptDone || len(req.Texts) != len(t.Segments) {
		err = errors.New("The transcript changed; reload and try again.")
		return
	}
	var kept []TranscriptSegment
	for i, text := range req.Texts {
		text = strings.TrimSpace(text)
		if len(text) > maxSegmentChars {
			err = fmt.Errorf("A line can be at most %d characters.", maxSegmentChars)
			return
		}
		if text == "" {
			continue
		}
		s := t.Segments[i]
		if text != s.Text {
			s.Text, s.Confidence = text, 1
		}
		kept = append(kept, s)
	}
	t.Segments = kept
	if len(kept) == 0 {
		t.Status = TranscriptRemoved
	}
	t.EditedBy, t.EditedAt = user.Id, time.Now()
	writeVideoTranscriptTx(ctx.Tx, &t)
	resp = transcriptResponse(ctx.Tx, user, image)
	vbolt.TxCommit(ctx.Tx)
	return
}

func RemoveVideoTranscript(ctx *vbeam.Context, req GetVideoTranscriptRequest) (resp GetVideoTranscriptResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	image := GetImageById(ctx.Tx, req.Id)
	if image.Id == 0 || !image.IsVideo() || !CanAccessFamily(ctx.Tx, user, image.FamilyId, AccessContribute) {
		err = ErrTranscriptNotFound
		return
	}
	t, _ := GetVideoTranscriptRecord(ctx.Tx, image.Id)
	t.ImageId, t.FamilyId = image.Id, image.FamilyId
	t.Status, t.Segments = TranscriptRemoved, nil
	t.EditedBy, t.EditedAt = user.Id, time.Now()
	writeVideoTranscriptTx(ctx.Tx, &t)
	resp = transcriptResponse(ctx.Tx, user, image)
	vbolt.TxCommit(ctx.Tx)
	return
}

func formatVTTTime(ms int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

func transcriptVTT(t VideoTranscript) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n")
	for i, s := range t.Segments {
		text := strings.ReplaceAll(strings.ReplaceAll(s.Text, "-->", "->"), "\n", " ")
		fmt.Fprintf(&b, "\n%d\n%s --> %s\n%s\n", i+1, formatVTTTime(s.StartMs), formatVTTTime(s.EndMs), text)
	}
	return b.String()
}

func serveVideoCaptions(w http.ResponseWriter, image Image) {
	var t VideoTranscript
	var found bool
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) { t, found = GetVideoTranscriptRecord(tx, image.Id) })
	if !image.IsVideo() || !found || t.Status != TranscriptDone {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Write([]byte(transcriptVTT(t)))
}

func RegisterVideoTranscriptMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetVideoTranscript)
	vbeam.RegisterProc(app, UpdateVideoTranscript)
	vbeam.RegisterProc(app, RemoveVideoTranscript)
}
