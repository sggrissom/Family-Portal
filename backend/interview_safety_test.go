package backend

import (
	"encoding/json"
	"go.hasen.dev/vbolt"
	"os"
	"testing"
)

func TestInterviewStaleWritesAndFrozenPrompts(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food", "fav-snack")
	first := got.Questions[0]
	saved := fx.answer(t, got.Interview.Id, first.Id, "pizza", false)
	if saved.Interview.Version != got.Interview.Version+1 {
		t.Fatal("answer did not advance version")
	}
	_, err := callAs(t, fx, SaveInterviewAnswer, SaveInterviewAnswerRequest{InterviewId: got.Interview.Id, QuestionId: first.Id, Answer: "stale", Version: got.Interview.Version})
	if err != ErrInterviewChanged {
		t.Fatalf("stale save: %v", err)
	}
	list, _ := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	inputs := questionInputs(list.Questions)
	for j := range inputs {
		if inputs[j].Id == first.Id {
			inputs[j].Text = "Different food question?"
			inputs[j].Section = "Different section"
		}
	}
	if _, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{Revision: list.Revision, Questions: inputs}); err != nil {
		t.Fatal(err)
	}
	fx.answer(t, got.Interview.Id, first.Id, "", false)
	again := fx.answer(t, got.Interview.Id, first.Id, "tacos", false)
	read, _ := callAs(t, fx, GetInterview, GetInterviewRequest{Id: got.Interview.Id})
	if read.Questions[0].Text != first.Text || read.Questions[0].Section != first.Section || again.Answer.Context != first.Text {
		t.Fatal("clearing/reanswering or rewording changed historical prompt")
	}
	_, err = callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{Id: again.Answer.Id, Category: CategoryInterview, Description: "stale generic edit", InputType: "today", InterviewVersion: saved.Interview.Version})
	if err != ErrInterviewChanged {
		t.Fatalf("stale generic edit: %v", err)
	}
}

func TestInterviewHistoricalReplayAndRawPrompt(t *testing.T) {
	fx := setupResultsFixture(t)
	req := ImportInterviewsRequest{PersonId: fx.alice.Id, RequestId: "import-once", Interviews: []HistoricalInterview{{InputType: "date", Date: "2020-03-02", Answers: []ImportAnswer{{Ref: "key:fav-food", Answer: "pizza", RawQuestion: "Fav food on the old form"}}}}}
	first, err := callAs(t, fx, ImportInterviews, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := callAs(t, fx, ImportInterviews, req)
	if err != nil || first.Interviews[0].Id != again.Interviews[0].Id {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	read, _ := callAs(t, fx, GetPersonInterviews, GetPersonInterviewsRequest{PersonId: fx.alice.Id})
	if len(read.Interviews) != 1 || read.Answers[0].Context != "Fav food on the old form" {
		t.Fatal("replay duplicated interview or rewrote raw wording")
	}
	req.Interviews[0].Answers[0].Answer = "tacos"
	if _, err := callAs(t, fx, ImportInterviews, req); err == nil {
		t.Fatal("reused request id for different payload")
	}
	if _, err := callAs(t, fx, DeleteInterview, DeleteInterviewRequest{Id: first.Interviews[0].Id, Version: first.Interviews[0].Version}); err != nil {
		t.Fatal(err)
	}
	req.Interviews[0].Answers[0].Answer = "pizza"
	if _, err := callAs(t, fx, ImportInterviews, req); err == nil {
		t.Fatal("replay recreated deleted interview")
	}
}

func TestInterviewBundlePreservesSameDaySittings(t *testing.T) {
	fx := setupResultsFixture(t)
	req := ImportInterviewsRequest{PersonId: fx.alice.Id, Interviews: []HistoricalInterview{
		{InputType: "date", Date: "2020-03-02", Answers: []ImportAnswer{{Ref: "key:fav-food", Answer: "pizza"}}},
		{InputType: "date", Date: "2020-03-02", Answers: []ImportAnswer{{Ref: "key:fav-food", Answer: "tacos"}}},
	}}
	if _, err := callAs(t, fx, ImportInterviews, req); err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		qs, is := buildInterviewExport(tx, fx.familyId)
		var ms []ExportMilestone
		for _, i := range is {
			for _, a := range interviewAnswers(tx, i.Id) {
				ms = append(ms, ExportMilestone{Id: a.Id, InterviewId: i.Id, QuestionId: a.QuestionId, Description: a.Description, Context: a.Context})
			}
		}
		user := AddUserTx(tx, CreateAccountRequest{Name: "Restore", Email: "same-day@example.com"}, nil)
		restorePerson, personErr := AddPersonTx(tx, AddPersonRequest{Name: "Alice", Birthdate: "2014-03-02"}, user.FamilyId)
		if personErr != nil {
			t.Fatal(personErr)
		}
		user.PersonId = restorePerson.Id
		list := familyQuestionList(tx, fx.familyId)
		imp := interviewImport{familyId: user.FamilyId, personIdMapping: map[int]int{fx.alice.Id: user.PersonId}, list: &list}
		// A family member person exists after signup and belongs to the restoring family.
		if user.PersonId == 0 {
			t.Fatal("missing restore person")
		}
		first := imp.run(tx, qs, is, ms)
		second := imp.run(tx, qs, is, ms)
		if first.imported != 2 || second.imported != 0 || len(personInterviews(tx, user.PersonId)) != 2 {
			t.Fatalf("same-day restore first=%+v second=%+v", first, second)
		}
		texts := map[string]bool{}
		for _, i := range personInterviews(tx, user.PersonId) {
			for _, a := range interviewAnswers(tx, i.Id) {
				texts[a.Description] = true
			}
		}
		if !texts["pizza"] || !texts["tacos"] {
			t.Fatal("lost a conflicting same-day answer")
		}
		vbolt.TxCommit(tx)
	})
}

func TestInterviewPartialDatesAndMeaningfulMatching(t *testing.T) {
	fx := setupResultsFixture(t)
	preview, err := callAs(t, fx, PreviewInterviewImport, PreviewInterviewImportRequest{PersonId: fx.alice.Id, Text: "Question\t2019\tAge 4\tMay 2020\nFavorite food\tpizza\ttacos\tnoodles"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range preview.Proposals {
		if p.Date != "" {
			t.Fatalf("partial header %q became %s", p.Header, p.Date)
		}
	}
	if normalizeQuestion("Fav food") != normalizeQuestion("What's your favorite food?") {
		t.Fatal("fav abbreviation failed")
	}
	if normalizeQuestion("favorite food") == normalizeQuestion("food") || normalizeQuestion("food you dislike") == normalizeQuestion("favorite food") {
		t.Fatal("question matching lost meaning")
	}
	_, err = callAs(t, fx, SaveHistoricalInterview, SaveHistoricalInterviewRequest{PersonId: fx.alice.Id, Interview: HistoricalInterview{InputType: "age", AgeYears: 4, Answers: []ImportAnswer{{Ref: "key:fav-food", Answer: "pizza"}}}})
	if err == nil {
		t.Fatal("age-only entry manufactured an exact date")
	}
}

func TestInterviewSearchCountsCards(t *testing.T) {
	hits := []Milestone{{Id: 1, InterviewId: 9}, {Id: 2, InterviewId: 9}, {Id: 3}, {Id: 4, InterviewId: 10}, {Id: 5, InterviewId: 9}}
	got := limitMilestoneCards(hits, 2)
	if len(got) != 4 || got[2].Id != 3 || got[3].Id != 5 {
		t.Fatalf("card budget: %+v", got)
	}
}

func TestInterviewNormalizationFixtures(t *testing.T) {
	bytes, err := os.ReadFile("testdata/interview-normalization.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Input, Expected string }
	if err := json.Unmarshal(bytes, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if got := normalizeInterviewAnswer(c.Input); got != c.Expected {
			t.Errorf("normalize %q = %q", c.Input, got)
		}
	}
}

func TestInterviewReplayDoesNotDuplicateOneOffs(t *testing.T) {
	fx := setupResultsFixture(t)
	_, err := callAs(t, fx, ImportInterviews, ImportInterviewsRequest{PersonId: fx.alice.Id, Interviews: []HistoricalInterview{{InputType: "date", Date: "2020-03-02", Answers: []ImportAnswer{{NewText: "What made you laugh today?", Answer: "the cat"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		qs, interviews := buildInterviewExport(tx, fx.familyId)
		before := len(familyQuestions(tx, fx.familyId))
		imp := interviewImport{familyId: fx.familyId, personIdMapping: map[int]int{fx.alice.Id: fx.alice.Id}}
		result := imp.run(tx, qs, interviews, nil)
		if result.imported != 0 || result.skipped != 1 || len(familyQuestions(tx, fx.familyId)) != before {
			t.Fatalf("replay changed questions: %+v", result)
		}
	})
}

func TestInterviewSnapshotBackfillAndEmptyListRestore(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2020-03-02", "fav-food", "fav-snack")
	saved := fx.answer(t, got.Interview.Id, got.Questions[0].Id, "pizza", false)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		legacy := GetInterviewById(tx, got.Interview.Id)
		legacy.Questions = nil
		legacy.OriginKey = ""
		writeInterview(tx, legacy)
		BackfillInterviewSnapshots(tx)
		migrated := GetInterviewById(tx, legacy.Id)
		if len(migrated.Questions) != 2 || migrated.Questions[0].Text != saved.Answer.Context || migrated.OriginKey == "" || migrated.Version != legacy.Version {
			t.Fatalf("backfill = %+v", migrated)
		}
		BackfillInterviewSnapshots(tx)
		if GetInterviewById(tx, legacy.Id).OriginKey != migrated.OriginKey {
			t.Fatal("backfill changed origin")
		}
		user := AddUserTx(tx, CreateAccountRequest{Name: "Empty", Email: "empty-restore@example.com"}, nil)
		empty := InterviewQuestionList{Seeded: true, DismissedKeys: []string{"fav-food"}, Revision: 7}
		imp := interviewImport{familyId: user.FamilyId, list: &empty}
		imp.run(tx, nil, nil, nil)
		ensureFamilyQuestions(tx, user.FamilyId)
		list := familyQuestionList(tx, user.FamilyId)
		if len(familyQuestions(tx, user.FamilyId)) != 0 || !list.Seeded || len(list.DismissedKeys) != 1 {
			t.Fatalf("empty list reseeded: %+v", list)
		}
	})
}
