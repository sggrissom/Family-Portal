package backend

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestTagRescoreReplacesPendingAndPreservesReviewedDecisions(t *testing.T) {
	fx := setupResultsFixture(t)
	client := startFakePhraseVision(t, map[string]int{"our beach": 0, "our snow": 1})
	created, err := callAs(t, fx, CreateTag, CreateTagRequest{Name: "Custom", Color: "#ffffff", AutoPhrase: "our beach"})
	if err != nil {
		t.Fatal(err)
	}
	var photos []Image
	for _, name := range []string{"pending", "accepted", "rejected"} {
		p := fx.addPhotoAt(t, "rescore-"+name+".jpg", featuresEpoch)
		fx.setEmbedding(t, p, axis(0))
		if err := suggestTagsForPhoto(fx.db, client, p.Id); err != nil {
			t.Fatal(err)
		}
		if len(fx.suggestionsFor(t, p)) != 1 {
			t.Fatal("expected initial suggestion")
		}
		photos = append(photos, p)
	}
	accepted := fx.suggestionsFor(t, photos[1])[0]
	rejected := fx.suggestionsFor(t, photos[2])[0]
	if _, err := callAs(t, fx, AcceptTagSuggestions, SuggestionIdsRequest{Ids: []int{accepted.Id}}); err != nil {
		t.Fatal(err)
	}
	if _, err := callAs(t, fx, RejectTagSuggestions, SuggestionIdsRequest{Ids: []int{rejected.Id}}); err != nil {
		t.Fatal(err)
	}
	phrase := "our snow"
	if _, err := callAs(t, fx, UpdateTag, UpdateTagRequest{Id: created.Tag.Id, Name: "Custom", Color: "#ffffff", AutoPhrase: &phrase}); err != nil {
		t.Fatal(err)
	}
	for _, p := range photos {
		if err := suggestTagsForPhoto(fx.db, client, p.Id); err != nil {
			t.Fatal(err)
		}
	}
	if got := fx.suggestionsFor(t, photos[0]); len(got) != 0 {
		t.Fatalf("obsolete pending suggestions retained: %+v", got)
	}
	for i, status := range []int{SuggestionAccepted, SuggestionRejected} {
		got := fx.suggestionsFor(t, photos[i+1])
		if len(got) != 1 || got[0].Status != status {
			t.Fatalf("reviewed decision lost: %+v", got)
		}
	}
	// Switching back must not resurrect the family's explicit rejection.
	phrase = "our beach"
	if _, err := callAs(t, fx, UpdateTag, UpdateTagRequest{Id: created.Tag.Id, Name: "Custom", Color: "#ffffff", AutoPhrase: &phrase}); err != nil {
		t.Fatal(err)
	}
	if err := suggestTagsForPhoto(fx.db, client, photos[2].Id); err != nil {
		t.Fatal(err)
	}
	if got := fx.suggestionsFor(t, photos[2]); len(got) != 1 || got[0].Status != SuggestionRejected {
		t.Fatalf("rejection resurfaced: %+v", got)
	}
}

func TestTagRescoreDiscardsResultsFromAnOldGeneration(t *testing.T) {
	fx := setupResultsFixture(t)
	photo := fx.addPhotoAt(t, "generation-change.jpg", featuresEpoch)
	fx.setEmbedding(t, photo, axis(0))
	phraseVectors.Lock()
	oldCache := phraseVectors.byText
	phraseVectors.byText = map[string][]float32{}
	phraseVectors.Unlock()
	t.Cleanup(func() { phraseVectors.Lock(); phraseVectors.byText = oldCache; phraseVectors.Unlock() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Texts []string `json:"texts"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) { bumpFamilySuggestionsTx(tx, fx.familyId); vbolt.TxCommit(tx) })
		vectors := make([][]float32, len(req.Texts))
		for i := range vectors {
			vectors[i] = axis(0)
		}
		json.NewEncoder(w).Encode(map[string]any{"embeddings": vectors})
	}))
	defer server.Close()
	client := &visionClient{http: &http.Client{Transport: rewriteTransport{server.URL, http.DefaultTransport}}}
	if err := suggestTagsForPhoto(fx.db, client, photo.Id); !errors.Is(err, errRetryLater) {
		t.Fatalf("stale result wasn't requeued: %v", err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if !suggestionsOutdated(tx, photo.Id, fx.familyId) {
			t.Fatal("old inference marked new generation complete")
		}
		if len(photoSuggestions(tx, photo.Id)) != 0 {
			t.Fatal("stale suggestions saved")
		}
	})
}
