package backend

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func startFakeTextVision(t *testing.T, vectors map[string][]float32) *visionClient {
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
			out[i] = []float32{0, 0, 0}
			for word, v := range vectors {
				if strings.Contains(text, word) {
					out[i] = v
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"embeddings": out})
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return newVisionClient(socket)
}

func (fx resultsFixture) setEmbedding(t *testing.T, photo Image, v []float32) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		writePhotoEmbeddingTx(tx, &PhotoEmbedding{
			PhotoId: photo.Id, FamilyId: photo.FamilyId,
			Model: clipModelId, Version: embeddingVersion, Vector: v,
		})
		vbolt.TxCommit(tx)
	})
}

func (fx resultsFixture) tagPerson(t *testing.T, photo Image, person Person) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		AddPersonToPhoto(tx, photo.Id, person.Id, photo.FamilyId)
		ReindexPhotoDates(tx, photo.Id)
		vbolt.TxCommit(tx)
	})
}

func TestSearchRanksBySimilarityAndNarrowsByNamedPeople(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalVisionClient
	globalVisionClient = startFakeTextVision(t, map[string][]float32{"beach": {1, 0, 0}})
	t.Cleanup(func() { globalVisionClient = previous })

	sandy := fx.addPhotoAt(t, "sandy.jpg", featuresEpoch)
	shore := fx.addPhotoAt(t, "shore.jpg", featuresEpoch.Add(time.Hour))
	snow := fx.addPhotoAt(t, "snow.jpg", featuresEpoch.Add(2*time.Hour))
	fx.setEmbedding(t, sandy, []float32{1, 0, 0})
	fx.setEmbedding(t, shore, []float32{0.97, 0.243, 0})
	fx.setEmbedding(t, snow, []float32{0, 0, 1})
	weak := fx.addPhotoAt(t, "weak.jpg", featuresEpoch.Add(3*time.Hour))
	fx.setEmbedding(t, weak, []float32{0.9, 0.436, 0})
	fx.tagPerson(t, shore, fx.alice)

	resp, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "at the beach", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{sandy.Id, shore.Id}) {
		t.Errorf("beach results = %v, want [sandy shore] = [%d %d]", got, sandy.Id, shore.Id)
	}
	if resp.SearchMode != "semantic" {
		t.Errorf("mode = %q", resp.SearchMode)
	}

	resp, err = callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "Alice at the beach", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{shore.Id}) {
		t.Errorf("Alice at the beach = %v, want [%d]", got, shore.Id)
	}
	if !slices.Equal(resp.MatchedPersonIds, []int{fx.alice.Id}) {
		t.Errorf("matched people = %v", resp.MatchedPersonIds)
	}

	first, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "beach", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "beach", Limit: 1, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if got := append(photoIds(first.Photos), photoIds(second.Photos)...); !slices.Equal(got, []int{sandy.Id, shore.Id}) || second.NextCursor != "" {
		t.Errorf("paged results = %v (next %q)", got, second.NextCursor)
	}
}

func TestSearchFallsBackToTextWithoutTheDaemon(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalVisionClient
	globalVisionClient = nil
	t.Cleanup(func() { globalVisionClient = previous })

	beach := fx.addPhotoAt(t, "b.jpg", featuresEpoch)
	other := fx.addPhotoAt(t, "o.jpg", featuresEpoch.Add(time.Hour))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		beach.Title = "Beach day"
		vbolt.Write(tx, ImagesBkt, beach.Id, &beach)
		other.Title = "Snow day"
		vbolt.Write(tx, ImagesBkt, other.Id, &other)
		vbolt.TxCommit(tx)
	})

	resp, err := callAs(t, fx, ListFamilyPhotos, ListFamilyPhotosRequest{Query: "the beach", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{beach.Id}) || resp.SearchMode != "text" {
		t.Errorf("text search = %v (%s), want [%d]", got, resp.SearchMode, beach.Id)
	}
}

func TestPeopleInQuery(t *testing.T) {
	people := []Person{
		{Id: 1, Name: "Jack Smith"}, {Id: 2, Name: "Emma Smith"}, {Id: 3, Name: "Sam Lee"}, {Id: 4, Name: "Sam Cole"},
		{Id: 5, Name: "Baby Smith", IsPregnancy: true},
	}
	ids, rest := peopleInQuery(people, "Jack and emma's bikes with Sam")
	if !slices.Equal(ids, []int{1, 2}) || rest != "and bikes with Sam" {
		t.Errorf("got %v %q", ids, rest)
	}
	if ids, rest := peopleInQuery(people, "Jack"); !slices.Equal(ids, []int{1}) || rest != "" {
		t.Errorf("name only: %v %q", ids, rest)
	}
	if ids, rest := peopleInQuery(people, "baby bath"); len(ids) != 0 || rest != "baby bath" {
		t.Errorf("a pregnancy's placeholder name matched: %v %q", ids, rest)
	}
}
