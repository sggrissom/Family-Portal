package backend

import (
	"slices"
	"testing"
	"time"

	"family/vision"

	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) addMilestoneOn(t *testing.T, person Person, text, category string, date time.Time) Milestone {
	t.Helper()
	var m Milestone
	day := date.Format("2006-01-02")
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		m, err = AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId: person.Id, Description: text, Category: category, InputType: "date", MilestoneDate: &day,
		}, fx.familyId)
		if err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})
	return m
}

func useFakeVision(t *testing.T, pinned map[string]int) *visionClient {
	t.Helper()
	client := startFakePhraseVision(t, pinned)
	previous := globalVisionClient
	globalVisionClient = client
	categoryVectors.Lock()
	categoryVectors.byCategory = nil
	categoryVectors.Unlock()
	t.Cleanup(func() {
		globalVisionClient = previous
		categoryVectors.Lock()
		categoryVectors.byCategory = nil
		categoryVectors.Unlock()
	})
	return client
}

func nudged(i, j int, w float32) []float32 {
	v := axis(i)
	v[j] = w
	return vision.Normalize(v)
}

func TestMilestoneMatchesPairMutualBestAcrossSiblings(t *testing.T) {
	fx := setupResultsFixture(t)
	client := useFakeVision(t, map[string]int{
		"First steps":            0,
		"took a few steps today": 0,
		"First word":             1,
		"Lost first tooth":       2,
	})
	aliceSteps := fx.addMilestoneOn(t, fx.alice, "First steps", "development", fx.alice.Birthday.AddDate(0, 11, 0))
	fx.addMilestoneOn(t, fx.alice, "First word", "first", fx.alice.Birthday.AddDate(1, 0, 0))
	bobSteps := fx.addMilestoneOn(t, fx.bob, "took a few steps today", "development", fx.bob.Birthday.AddDate(1, 1, 3))
	fx.addMilestoneOn(t, fx.carol, "Lost first tooth", "health", fx.carol.Birthday.AddDate(6, 0, 0))

	for _, id := range milestonesNeedingEmbedding(fx.db) {
		if err := embedMilestone(fx.db, client, id); err != nil {
			t.Fatal(err)
		}
	}
	if left := milestonesNeedingEmbedding(fx.db); len(left) != 0 {
		t.Fatalf("still unembedded: %v", left)
	}

	resp, err := callAs(t, fx, GetMilestoneMatches, GetMilestoneMatchesRequest{MilestoneId: aliceSteps.Id})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AgeMonths != 11 {
		t.Errorf("alice's age = %d months, want 11", resp.AgeMonths)
	}
	if len(resp.Matches) != 1 || resp.Matches[0].Milestone.Id != bobSteps.Id || resp.Matches[0].AgeMonths != 13 {
		t.Fatalf("matches = %+v, want bob's steps at 13 months", resp.Matches)
	}
}

func TestSuggestMilestoneCategoryUsesExamplesThenFamilyHistory(t *testing.T) {
	fx := setupResultsFixture(t)
	client := useFakeVision(t, map[string]int{
		"had a fever":         5,
		"fever and a cough":   5,
		"croup again":         6,
		"croup, third winter": 6,
	})

	resp, err := callAs(t, fx, SuggestMilestoneCategory, SuggestMilestoneCategoryRequest{Description: "fever and a cough"})
	if err != nil || resp.Category != "health" {
		t.Fatalf("category = %q, %v; want health from the examples", resp.Category, err)
	}

	earlier := fx.addMilestoneOn(t, fx.alice, "croup again", "behavior", fx.alice.Birthday.AddDate(2, 0, 0))
	if err := embedMilestone(fx.db, client, earlier.Id); err != nil {
		t.Fatal(err)
	}
	resp, _ = callAs(t, fx, SuggestMilestoneCategory, SuggestMilestoneCategoryRequest{Description: "croup, third winter", PersonId: fx.bob.Id})
	if resp.Category != "behavior" {
		t.Errorf("category = %q, want the family's own filing of a near-identical milestone", resp.Category)
	}

	globalVisionClient = nil
	resp, _ = callAs(t, fx, SuggestMilestoneCategory, SuggestMilestoneCategoryRequest{Description: "fever and a cough"})
	if resp.Category != "" {
		t.Errorf("without the daemon the suggestion should be empty, got %q", resp.Category)
	}
}

func TestSearchMilestonesFindsParaphrases(t *testing.T) {
	fx := setupResultsFixture(t)
	client := useFakeVision(t, map[string]int{"First steps": 0, "walking": 0})
	steps := fx.addMilestoneOn(t, fx.alice, "First steps", "development", fx.alice.Birthday.AddDate(1, 0, 0))
	walks := fx.addMilestoneOn(t, fx.bob, "Walks to school alone", "achievement", fx.bob.Birthday.AddDate(8, 0, 0))
	fx.addMilestoneOn(t, fx.carol, "Lost first tooth", "health", fx.carol.Birthday.AddDate(6, 0, 0))
	for _, id := range milestonesNeedingEmbedding(fx.db) {
		embedMilestone(fx.db, client, id)
	}

	resp, err := callAs(t, fx, SearchMilestones, SearchMilestonesRequest{Query: "walking"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, m := range resp.Milestones {
		ids = append(ids, m.Id)
	}
	if !slices.Contains(ids, steps.Id) {
		t.Errorf("results %v are missing the paraphrase %d", ids, steps.Id)
	}
	if slices.Contains(ids, walks.Id) {
		t.Errorf("results %v include an unrelated milestone that merely shares no words", ids)
	}
}

func TestSuggestMilestonePhotosRanksNearbyPhotosOfThePerson(t *testing.T) {
	fx := setupResultsFixture(t)
	useFakeVision(t, map[string]int{"a photo of first bike ride": 0})
	day := fx.alice.Birthday.AddDate(5, 0, 0)

	bike := fx.addPhotoAt(t, "bike.jpg", day.Add(3*24*time.Hour))
	cake := fx.addPhotoAt(t, "cake.jpg", day)
	old := fx.addPhotoAt(t, "old.jpg", day.AddDate(0, -3, 0))
	notAlice := fx.addPhotoAt(t, "bob.jpg", day)
	for _, p := range []Image{bike, cake, old} {
		fx.tagPerson(t, p, fx.alice)
	}
	fx.tagPerson(t, notAlice, fx.bob)
	fx.setEmbedding(t, bike, nudged(0, 1, 0.2))
	fx.setEmbedding(t, cake, axis(1))

	date := day.Format("2006-01-02")
	resp, err := callAs(t, fx, SuggestMilestonePhotos, SuggestMilestonePhotosRequest{
		PersonId: fx.alice.Id, Description: "first bike ride", InputType: "date", MilestoneDate: &date,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Ranked || !slices.Equal(resp.PhotoIds, []int{bike.Id, cake.Id}) {
		t.Errorf("suggested %v (ranked %v), want [bike cake] = [%d %d]", resp.PhotoIds, resp.Ranked, bike.Id, cake.Id)
	}

	resp, _ = callAs(t, fx, SuggestMilestonePhotos, SuggestMilestonePhotosRequest{
		PersonId: fx.alice.Id, Description: "first bike ride", InputType: "date", MilestoneDate: &date,
		ExcludeIds: []int{bike.Id},
	})
	if !slices.Equal(resp.PhotoIds, []int{cake.Id}) {
		t.Errorf("with bike attached: %v", resp.PhotoIds)
	}
}
