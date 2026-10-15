package backend

import (
	"slices"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func noVision(t *testing.T) {
	previous := globalVisionClient
	globalVisionClient = nil
	t.Cleanup(func() { globalVisionClient = previous })
}

func momentIds(ms []Milestone) []int {
	ids := []int{}
	for _, m := range ms {
		ids = append(ids, m.Id)
	}
	return ids
}

func personIds(people []Person) []int {
	ids := []int{}
	for _, p := range people {
		ids = append(ids, p.Id)
	}
	return ids
}

func search(t *testing.T, fx resultsFixture, req SearchRequest) SearchResponse {
	t.Helper()
	resp, err := callAs(t, fx, Search, req)
	if err != nil {
		t.Fatalf("Search(%+v) error = %v", req, err)
	}
	return resp
}

func TestSearchEmptyQueryReturnsEmptyGroups(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	fx.addMilestoneOn(t, fx.alice, "Walked to the park", "development", day("2016-05-01"))

	for _, q := range []string{"", "   ", "the"} {
		resp := search(t, fx, SearchRequest{Query: q})
		if len(resp.People)+len(resp.Moments)+len(resp.Photos)+len(resp.Tags) != 0 {
			t.Errorf("%q returned results: %+v", q, resp)
		}
		if resp.Moments == nil || resp.Photos == nil || resp.Understood.PersonIds == nil {
			t.Errorf("%q left a group nil", q)
		}
	}
}

func TestSearchMatchesWordPrefixesAcrossGroups(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	walked := fx.addMilestoneOn(t, fx.alice, "Walked to the café", "development", day("2016-05-01"))
	fx.addMilestoneOn(t, fx.bob, "Sidewalk chalk", "artwork", day("2016-06-01"))
	tag := fx.addTag(t, fx.familyId, "Walking trips")
	photo := fx.addPhotoAt(t, "w.jpg", featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		photo.Title = "Walkabout"
		vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		ReindexPhotoDates(tx, photo.Id)
		place := FamilyPlace{Id: vbolt.NextIntId(tx, FamilyPlaceBkt), FamilyId: fx.familyId, Name: "Walker Park", RadiusMeters: 100}
		vbolt.Write(tx, FamilyPlaceBkt, place.Id, &place)
		vbolt.SetTargetSingleTerm(tx, FamilyPlaceByFamilyIndex, place.Id, fx.familyId)
		book := Book{Id: vbolt.NextIntId(tx, BookBkt), FamilyId: fx.familyId, PersonIds: []int{fx.alice.Id}, Title: "Walks with Alice"}
		writeBookTx(tx, book)
		vbolt.TxCommit(tx)
	})

	resp := search(t, fx, SearchRequest{Query: "walk"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{walked.Id}) || resp.MomentCount != 1 {
		t.Errorf("moments = %v (%d), want only %d: prefix of a word, not inside one", got, resp.MomentCount, walked.Id)
	}
	if len(resp.Photos) != 1 || resp.Photos[0].Image.Id != photo.Id {
		t.Errorf("photos = %+v", resp.Photos)
	}
	if len(resp.Tags) != 1 || resp.Tags[0].Id != tag.Id {
		t.Errorf("tags = %+v", resp.Tags)
	}
	if len(resp.Places) != 1 || resp.Places[0].Name != "Walker Park" {
		t.Errorf("places = %+v", resp.Places)
	}
	if len(resp.Books) != 1 || resp.Books[0].Title != "Walks with Alice" {
		t.Errorf("books = %+v", resp.Books)
	}
	if resp.SearchMode != "text" {
		t.Errorf("mode = %q with no daemon", resp.SearchMode)
	}

	if got := momentIds(search(t, fx, SearchRequest{Query: "CAFE"}).Moments); !slices.Equal(got, []int{walked.Id}) {
		t.Errorf("accent-folded search = %v", got)
	}
	if got := momentIds(search(t, fx, SearchRequest{Query: "walk park"}).Moments); len(got) != 0 {
		t.Errorf("every term is required, got %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "ali"})
	if got := personIds(resp.People); !slices.Equal(got, []int{fx.alice.Id}) {
		t.Errorf("people for a name prefix = %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "nuvo"})
	if len(resp.Activities) != 1 || resp.Activities[0].Kind != ActivityHitEvent || resp.Activities[0].Id != fx.event.Id {
		t.Errorf("activities = %+v", resp.Activities)
	}
	resp = search(t, fx, SearchRequest{Query: "dance"})
	if len(resp.Activities) == 0 || resp.Activities[0].Kind != ActivityHitActivity {
		t.Errorf("activities = %+v", resp.Activities)
	}
}

func TestSearchRanksDescriptionMatchesFirst(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	inContext := fx.addMilestoneOn(t, fx.alice, "Laughed all day", "behavior", day("2020-01-01"))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		inContext.Context = "at the zoo"
		writeMilestone(tx, inContext)
		vbolt.TxCommit(tx)
	})
	inDescription := fx.addMilestoneOn(t, fx.alice, "Zoo trip", "first", day("2019-01-01"))
	tagged := fx.addMilestoneOn(t, fx.alice, "Saw lions", "first", day("2021-01-01"))
	zooTag := fx.addTag(t, fx.familyId, "Zoo animals")
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if err := setMilestoneTags(tx, tagged, []int{zooTag.Id}); err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})

	resp := search(t, fx, SearchRequest{Query: "zoo trip"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{inDescription.Id}) {
		t.Errorf("zoo trip = %v", got)
	}
	resp = search(t, fx, SearchRequest{Query: "zo"})
	want := []int{inDescription.Id, tagged.Id, inContext.Id}
	if got := momentIds(resp.Moments); !slices.Equal(got, want) {
		t.Errorf("zo = %v, want description match then the rest newest first %v", got, want)
	}
}

func TestSearchCombinesPersonAgeAndText(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	atTwo := fx.addMilestoneOn(t, fx.alice, "Park picnic", "first", day("2016-05-01"))
	atFive := fx.addMilestoneOn(t, fx.alice, "Park picnic again", "first", day("2019-05-01"))
	fx.addMilestoneOn(t, fx.bob, "Park picnic", "first", day("2016-05-02"))
	quote := fx.addMilestoneOn(t, fx.alice, "I'm a big girl", "quote", day("2016-07-01"))

	resp := search(t, fx, SearchRequest{Query: "alice at 2 park"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{atTwo.Id}) {
		t.Errorf("alice at 2 park = %v, want [%d]", got, atTwo.Id)
	}
	u := resp.Understood
	if !slices.Equal(u.PersonIds, []int{fx.alice.Id}) || u.AgeLabel != "age 2" || u.Text != "park" {
		t.Errorf("understood = %+v", u)
	}
	if got := personIds(resp.People); !slices.Equal(got, []int{fx.alice.Id}) {
		t.Errorf("people = %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "alice at 2"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{quote.Id, atTwo.Id}) {
		t.Errorf("filters alone list newest first, got %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "alice quotes"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{quote.Id}) {
		t.Errorf("alice quotes = %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "alice park 2019"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{atFive.Id}) {
		t.Errorf("alice park 2019 = %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "alice park may"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{atFive.Id, atTwo.Id}) {
		t.Errorf("a month without a year matches every year, got %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "alice bob park"})
	if got := momentIds(resp.Moments); len(got) != 3 || slices.Contains(got, quote.Id) {
		t.Errorf("two people match either one's moments, got %v", got)
	}

	resp = search(t, fx, SearchRequest{Query: "alice at 2 park", Ignore: []string{SearchIgnoreAge}})
	if resp.Understood.AgeFromMonth != nil || resp.Understood.Text != "2 park" {
		t.Errorf("ignored age understood = %+v", resp.Understood)
	}

	resp = search(t, fx, SearchRequest{Query: "park", Limit: 1})
	if len(resp.Moments) != 1 || resp.MomentCount != 3 {
		t.Errorf("limit 1 = %d moments of %d", len(resp.Moments), resp.MomentCount)
	}
}

func TestSearchFiltersPhotosByPersonAgeAndTag(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	young := fx.addPhotoAt(t, "young.jpg", day("2016-06-01"))
	old := fx.addPhotoAt(t, "old.jpg", day("2020-06-01"))
	untagged := fx.addPhotoAt(t, "nobody.jpg", day("2016-06-02"))
	fx.tagPerson(t, young, fx.alice)
	fx.tagPerson(t, old, fx.alice)
	beach := fx.addTag(t, fx.familyId, "Beach")
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		addTagToPhoto(tx, old.Id, beach.Id, fx.familyId)
		vbolt.TxCommit(tx)
	})

	resp := search(t, fx, SearchRequest{Query: "alice at 2"})
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{young.Id}) {
		t.Errorf("alice at 2 photos = %v, want [%d]", got, young.Id)
	}
	resp = search(t, fx, SearchRequest{Query: "alice"})
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{old.Id, young.Id}) || resp.MorePhotos {
		t.Errorf("alice photos = %v", got)
	}
	resp = search(t, fx, SearchRequest{Query: "beach"})
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{old.Id}) {
		t.Errorf("beach photos = %v", got)
	}
	if !slices.Equal(resp.Understood.TagIds, []int{beach.Id}) || len(resp.Tags) != 1 {
		t.Errorf("beach tag understood = %+v, tags group %+v", resp.Understood, resp.Tags)
	}
	resp = search(t, fx, SearchRequest{Query: "june 2016"})
	if got := photoIds(resp.Photos); !slices.Equal(got, []int{untagged.Id, young.Id}) {
		t.Errorf("june 2016 photos = %v", got)
	}
}

func TestSearchUsesMeaningWhenTheDaemonIsUp(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalVisionClient
	globalVisionClient = startFakeTextVision(t, map[string][]float32{"giggle": {1, 0, 0}})
	t.Cleanup(func() { globalVisionClient = previous })

	laughed := fx.addMilestoneOn(t, fx.alice, "Laughed at the dog", "behavior", day("2020-01-01"))
	cried := fx.addMilestoneOn(t, fx.alice, "Cried at the vet", "behavior", day("2020-02-01"))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		for _, e := range []struct {
			m Milestone
			v []float32
		}{{laughed, []float32{1, 0, 0}}, {cried, []float32{0, 1, 0}}} {
			row := MilestoneEmbedding{
				MilestoneId: e.m.Id, PersonId: e.m.PersonId, FamilyId: fx.familyId,
				Model: sentenceModelId, Version: milestoneEmbedVersion, Text: e.m.Description, Vector: e.v,
			}
			vbolt.Write(tx, MilestoneEmbeddingBkt, row.MilestoneId, &row)
		}
		vbolt.TxCommit(tx)
	})

	resp := search(t, fx, SearchRequest{Query: "giggle"})
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{laughed.Id}) || resp.SearchMode != "semantic" {
		t.Errorf("giggle = %v (%s), want [%d]", got, resp.SearchMode, laughed.Id)
	}
}

func TestSearchLeavesOutPhotosOfAPersonSharedWithoutThem(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()
	noVision(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ReindexPhotoDates(tx, fx.alicePhoto.Id)
		vbolt.TxCommit(tx)
	})

	run := func() (resp SearchResponse) {
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			var err error
			resp, err = Search(&vbeam.Context{Tx: tx, Token: testAuthToken(t, fx.userB)}, SearchRequest{Query: "alice"})
			if err != nil {
				t.Fatal(err)
			}
		})
		return
	}

	resp := run()
	if got := personIds(resp.People); !slices.Equal(got, []int{fx.alice.Id}) {
		t.Fatalf("shared person not found: %v", got)
	}
	if len(resp.Photos) != 1 || len(resp.Moments) != 1 {
		t.Errorf("with every scope: %d photos, %d moments", len(resp.Photos), len(resp.Moments))
	}

	setLinkScopes(t, fx, fx.linkAB, LinkScopes{People: true, Milestones: true})
	resp = run()
	if len(resp.Photos) != 0 {
		t.Errorf("photos leaked without the photo scope: %+v", resp.Photos)
	}
	if len(resp.Moments) != 1 {
		t.Errorf("moments = %d", len(resp.Moments))
	}
	if len(resp.Tags) != 0 || len(resp.Places) != 0 || len(resp.Books) != 0 || len(resp.Activities) != 0 {
		t.Errorf("other groups leaked: %+v", resp)
	}
}

func TestSearchTreatsATagNameAsText(t *testing.T) {
	fx := setupResultsFixture(t)
	noVision(t)
	school := fx.addTag(t, fx.familyId, "School")
	mentions := fx.addMilestoneOn(t, fx.alice, "First day of school", "first", day("2020-09-01"))
	tagged := fx.addMilestoneOn(t, fx.alice, "Met her teacher", "first", day("2020-08-20"))
	fx.addMilestoneOn(t, fx.alice, "Lost a tooth", "health", day("2020-10-01"))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if err := setMilestoneTags(tx, tagged, []int{school.Id}); err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})

	resp := search(t, fx, SearchRequest{Query: "school"})
	if !slices.Equal(resp.Understood.TagIds, []int{school.Id}) || resp.Understood.Text != "school" {
		t.Errorf("understood = %+v", resp.Understood)
	}
	if got := momentIds(resp.Moments); !slices.Equal(got, []int{mentions.Id, tagged.Id}) {
		t.Errorf("school = %v, want the moment that says it, then the tagged one", got)
	}
	if len(resp.Tags) != 1 || resp.Tags[0].Id != school.Id {
		t.Errorf("tags = %+v", resp.Tags)
	}
}
