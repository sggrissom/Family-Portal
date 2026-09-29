package backend

import (
	"encoding/json"
	"hash/fnv"
	"math/rand"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"family/vision"

	"go.hasen.dev/vbolt"
)

const phraseDims = 32

func axis(i int) []float32 {
	v := make([]float32, phraseDims)
	v[i] = 1
	return v
}

// phraseVector gives each phrase a fixed pseudo-random direction, except the
// ones pinned to an axis.
func phraseVector(text string, pinned map[string]int) []float32 {
	if i, ok := pinned[text]; ok {
		return axis(i)
	}
	h := fnv.New64a()
	h.Write([]byte(text))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	v := make([]float32, phraseDims)
	for i := 4; i < phraseDims; i++ {
		v[i] = float32(r.NormFloat64())
	}
	return vision.Normalize(v)
}

func startFakePhraseVision(t *testing.T, pinned map[string]int) *visionClient {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "vision.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /embed/text", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Texts []string `json:"texts"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		out := make([][]float32, len(req.Texts))
		for i, text := range req.Texts {
			out[i] = phraseVector(text, pinned)
		}
		json.NewEncoder(w).Encode(map[string]any{"embeddings": out})
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return newVisionClient(socket)
}

func (fx resultsFixture) suggestionsFor(t *testing.T, photo Image) (out []TagSuggestion) {
	t.Helper()
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { out = photoSuggestions(tx, photo.Id) })
	return
}

func TestScoreLabelsNeedsAClearWinner(t *testing.T) {
	image := vision.Normalize([]float32{1, 0.2, 0, 0})
	targets := [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}
	background := [][]float32{{0, 0, 1, 0}}
	scores := scoreLabels(image, targets, background)
	if scores[0].prob < 0.99 || scores[1].prob > 0.01 {
		t.Errorf("scores = %+v", scores)
	}
	tie := scoreLabels(vision.Normalize([]float32{1, 1, 0, 0}), targets, background)
	if tie[0].prob > suggestMinProb || tie[1].prob > suggestMinProb {
		t.Errorf("an even split should suggest neither: %+v", tie)
	}
}

func TestTagSuggestionsAcceptRejectAndRemember(t *testing.T) {
	fx := setupResultsFixture(t)
	client := startFakePhraseVision(t, map[string]int{
		"a photo at the beach with sand and ocean": 0,
		"our dog biscuit":                          1,
		"our dog biscuit at home":                  1,
	})
	previous := globalVisionClient
	globalVisionClient = client
	t.Cleanup(func() { globalVisionClient = previous })

	beach := fx.addPhotoAt(t, "beach.jpg", featuresEpoch)
	plain := fx.addPhotoAt(t, "plain.jpg", featuresEpoch.Add(1))
	fx.setEmbedding(t, beach, axis(0))
	fx.setEmbedding(t, plain, axis(3))

	for _, p := range []Image{beach, plain} {
		if err := suggestTagsForPhoto(fx.db, client, p.Id); err != nil {
			t.Fatal(err)
		}
	}
	got := fx.suggestionsFor(t, beach)
	if len(got) != 1 || got[0].Label != "Beach" || got[0].TagId != 0 {
		t.Fatalf("beach suggestions = %+v", got)
	}
	if s := fx.suggestionsFor(t, plain); len(s) != 0 {
		t.Errorf("plain photo got suggestions: %+v", s)
	}

	review, err := callAs(t, fx, GetTagSuggestions, Empty{})
	if err != nil || review.Total != 1 || review.Groups[0].Label != "Beach" {
		t.Fatalf("review = %+v, %v", review, err)
	}
	photo, _ := callAs(t, fx, GetPhoto, GetPhotoRequest{Id: beach.Id})
	if len(photo.Suggestions) != 1 || photo.Suggestions[0].Label != "Beach" {
		t.Errorf("photo suggestions = %+v", photo.Suggestions)
	}

	if _, err := callAs(t, fx, AcceptTagSuggestions, SuggestionIdsRequest{Ids: []int{got[0].Id}}); err != nil {
		t.Fatal(err)
	}
	var tagIds []int
	var tags []Tag
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		tagIds = GetPhotoTagIds(tx, beach.Id)
		tags = getTagsByFamily(tx, fx.familyId)
	})
	if len(tags) != 1 || tags[0].Name != "Beach" || len(tagIds) != 1 || tagIds[0] != tags[0].Id {
		t.Fatalf("after accept: tags %+v, photo tags %v", tags, tagIds)
	}

	custom, err := callAs(t, fx, CreateTag, CreateTagRequest{Name: "Biscuit", Color: "#aa5500", AutoPhrase: "our dog biscuit"})
	if err != nil {
		t.Fatal(err)
	}
	dog := fx.addPhotoAt(t, "dog.jpg", featuresEpoch.Add(2))
	fx.setEmbedding(t, dog, axis(1))
	if err := suggestTagsForPhoto(fx.db, client, dog.Id); err != nil {
		t.Fatal(err)
	}
	dogSuggestions := fx.suggestionsFor(t, dog)
	if len(dogSuggestions) != 1 || dogSuggestions[0].TagId != custom.Tag.Id {
		t.Fatalf("custom phrase suggestions = %+v", dogSuggestions)
	}
	if _, err := callAs(t, fx, RejectTagSuggestions, SuggestionIdsRequest{Ids: []int{dogSuggestions[0].Id}}); err != nil {
		t.Fatal(err)
	}

	edited := "our dog biscuit at home"
	if _, err := callAs(t, fx, UpdateTag, UpdateTagRequest{Id: custom.Tag.Id, Name: "Biscuit", Color: "#aa5500", AutoPhrase: &edited}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !suggestionsOutdated(tx, dog.Id, fx.familyId) {
			t.Error("a phrase change should mark the family's photos for rescoring")
		}
	})
	if err := suggestTagsForPhoto(fx.db, client, dog.Id); err != nil {
		t.Fatal(err)
	}
	if s := fx.suggestionsFor(t, dog); len(s) != 1 || s[0].Status != SuggestionRejected {
		t.Errorf("a rejected suggestion came back: %+v", s)
	}

	if _, err := callAs(t, fx, DeletePhoto, DeletePhotoRequest{Id: beach.Id}); err != nil {
		t.Fatal(err)
	}
	if s := fx.suggestionsFor(t, beach); len(s) != 0 {
		t.Errorf("deleted photo kept suggestions: %+v", s)
	}
}

func TestUpdateTagWithoutPhraseKeepsIt(t *testing.T) {
	fx := setupResultsFixture(t)
	created, err := callAs(t, fx, CreateTag, CreateTagRequest{Name: "Farm", Color: "#00aa00", AutoPhrase: "kids on a farm"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := callAs(t, fx, UpdateTag, UpdateTagRequest{Id: created.Tag.Id, Name: "Farm visits", Color: "#00aa00"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Tag.AutoPhrase != "kids on a farm" {
		t.Errorf("phrase = %q, want it kept", updated.Tag.AutoPhrase)
	}
}
