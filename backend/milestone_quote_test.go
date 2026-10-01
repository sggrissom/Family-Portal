package backend

import (
	"encoding/json"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestQuoteKeepsTheWordsAndContext(t *testing.T) {
	fx := setupResultsFixture(t)
	day := "2018-05-01"

	added, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{
		PersonId: fx.alice.Id, Description: "  “The moon is following our car!” ", Category: "quote",
		Context: "On the drive home from Grandma's", InputType: "date", MilestoneDate: &day,
	})
	if err != nil {
		t.Fatal(err)
	}
	quote := added.Milestone
	if quote.Description != "The moon is following our car!" {
		t.Errorf("description = %q, want the words without the typed quotation marks", quote.Description)
	}
	if quote.Context != "On the drive home from Grandma's" {
		t.Errorf("context = %q", quote.Context)
	}

	got, _ := callAs(t, fx, GetMilestone, GetMilestoneRequest{Id: quote.Id})
	if got.Milestone.Context != quote.Context {
		t.Errorf("stored context = %q, want %q", got.Milestone.Context, quote.Context)
	}

	found, _ := callAs(t, fx, SearchMilestones, SearchMilestonesRequest{Query: "drive"})
	if len(found.Milestones) != 1 || found.Milestones[0].Id != quote.Id {
		t.Errorf("searching the context found %v, want the quote", found.Milestones)
	}

	updated, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{
		Id: quote.Id, Description: "Said the moon follows the car", Category: "behavior",
		Context: "kept by mistake", InputType: "date", MilestoneDate: &day,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Milestone.Context != "" {
		t.Errorf("context = %q after leaving the quote category, want empty", updated.Milestone.Context)
	}
}

func TestUnquoteOnlyStripsMarksAroundTheWholeQuote(t *testing.T) {
	for in, want := range map[string]string{
		`"No, YOU'RE silly"`:     `No, YOU'RE silly`,
		`‘Why?’`:                 `Why?`,
		`He said "no" twice`:     `He said "no" twice`,
		`"Mine," then "yours"`:   `"Mine," then "yours"`,
		`'Twas the night before`: `'Twas the night before`,
		`""`:                     `""`,
		`I want the "big" spoon`: `I want the "big" spoon`,
		`«Encore»`:               `Encore`,
	} {
		if got := unquote(in); got != want {
			t.Errorf("unquote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSuggestsQuoteForQuotedTextWithoutTheDaemon(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalVisionClient
	globalVisionClient = nil
	t.Cleanup(func() { globalVisionClient = previous })

	for text, want := range map[string]string{
		`"I'm not tired, my eyes are`:  "quote",
		`“I'm not tired, my eyes are”`: "quote",
		`Said I'm not tired`:           "",
	} {
		resp, err := callAs(t, fx, SuggestMilestoneCategory, SuggestMilestoneCategoryRequest{Description: text})
		if err != nil || resp.Category != want {
			t.Errorf("category for %q = %q, %v; want %q", text, resp.Category, err, want)
		}
	}
}

func TestQuoteContextSurvivesExportAndImport(t *testing.T) {
	fx := setupResultsFixture(t)
	quote := fx.addMilestoneOn(t, fx.alice, "Can we have pancakes for dinner", "quote", fx.alice.Birthday.AddDate(4, 0, 0))
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		quote.Context = "Every single night"
		writeMilestone(tx, quote)
		vbolt.TxCommit(tx)
	})

	exported, err := callAs(t, fx, ExportData, ExportDataRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var data ExportDataStructure
	if err := json.Unmarshal([]byte(exported.JsonData), &data); err != nil {
		t.Fatal(err)
	}
	var row ExportMilestone
	for _, m := range data.Milestones {
		if m.Id == quote.Id {
			row = m
		}
	}
	if row.Context != "Every single night" || row.Category != "quote" {
		t.Fatalf("exported %+v, want the quote with its context", row)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		imported, _, errs, ids := importMilestones(tx, []ExportMilestone{row}, map[int]int{row.PersonId: fx.bob.Id}, fx.familyId, nil)
		if imported != 1 || len(errs) > 0 {
			t.Fatalf("imported %d, errors %v", imported, errs)
		}
		if got := GetMilestoneById(tx, ids[0]); got.Context != "Every single night" || got.Category != "quote" {
			t.Errorf("imported %+v, want the quote with its context", got)
		}
	})
}
