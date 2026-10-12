package backend

import (
	"slices"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) seedQuestions(t *testing.T) InterviewQuestionsResponse {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ensureFamilyQuestions(tx, fx.familyId)
		vbolt.TxCommit(tx)
	})
	resp, err := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (fx resultsFixture) recordInterview(t *testing.T, person Person, day time.Time, answers map[int]string) Interview {
	t.Helper()
	var interview Interview
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		interview = Interview{
			Id: vbolt.NextIntId(tx, InterviewBkt), FamilyId: person.FamilyId, PersonId: person.Id,
			Date: dayStart(day), Source: InterviewHistorical, Status: InterviewComplete,
		}
		for questionId, text := range answers {
			interview.QuestionIds = append(interview.QuestionIds, questionId)
			q := GetInterviewQuestionById(tx, questionId)
			writeMilestone(tx, Milestone{
				Id: vbolt.NextIntId(tx, MilestoneBkt), PersonId: person.Id, FamilyId: person.FamilyId,
				Description: text, Category: CategoryInterview, Context: q.Text, MilestoneDate: interview.Date,
				InterviewId: interview.Id, QuestionId: questionId,
			})
		}
		writeInterview(tx, interview)
		vbolt.TxCommit(tx)
	})
	return interview
}

func questionInputs(questions []InterviewQuestion) []InterviewQuestionInput {
	inputs := make([]InterviewQuestionInput, 0, len(questions))
	for _, q := range questions {
		inputs = append(inputs, InterviewQuestionInput{
			Id: q.Id, Text: q.Text, Section: q.Section, MinAgeYears: q.MinAgeYears,
			MaxAgeYears: q.MaxAgeYears, DefaultKey: q.DefaultKey, Archived: q.Archived,
		})
	}
	return inputs
}

func findQuestion(questions []InterviewQuestion, key string) InterviewQuestion {
	for _, q := range questions {
		if q.DefaultKey == key {
			return q
		}
	}
	return InterviewQuestion{}
}

func TestDefaultsAreOfferedWithoutWriting(t *testing.T) {
	fx := setupResultsFixture(t)

	resp, err := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Questions) != len(defaultInterviewQuestions) {
		t.Fatalf("got %d questions, want the %d defaults", len(resp.Questions), len(defaultInterviewQuestions))
	}
	for _, q := range resp.Questions {
		if q.Id != 0 || q.DefaultKey == "" {
			t.Fatalf("unseeded list returned %+v, want unsaved default rows", q)
		}
	}
	if len(resp.Suggested) != 0 {
		t.Errorf("an unseeded list suggested %d questions, want none", len(resp.Suggested))
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if rows := familyQuestions(tx, fx.familyId); len(rows) != 0 {
			t.Errorf("reading wrote %d rows", len(rows))
		}
	})
}

func TestEnsureFamilyQuestionsCopiesOnce(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.seedQuestions(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ensureFamilyQuestions(tx, fx.familyId)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		rows := familyQuestions(tx, fx.familyId)
		if len(rows) != len(defaultInterviewQuestions) {
			t.Fatalf("got %d rows after two copies, want %d", len(rows), len(defaultInterviewQuestions))
		}
		for i, q := range rows {
			if q.DefaultKey != defaultInterviewQuestions[i].Key {
				t.Fatalf("row %d is %q, want %q in default order", i, q.DefaultKey, defaultInterviewQuestions[i].Key)
			}
		}
	})
}

func TestFirstSaveKeepsWhatTheEditorShowed(t *testing.T) {
	fx := setupResultsFixture(t)
	resp, _ := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})

	inputs := questionInputs(resp.Questions)
	inputs = slices.DeleteFunc(inputs, func(in InterviewQuestionInput) bool { return in.DefaultKey == "fav-season" })
	inputs[0].Text = "What color do you like best?"
	inputs = append(inputs, InterviewQuestionInput{Text: "Who is your favorite superhero?", Section: "Favorites"})

	saved, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{Revision: resp.Revision, Questions: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != resp.Revision+1 {
		t.Errorf("revision = %d, want %d", saved.Revision, resp.Revision+1)
	}
	if len(saved.Questions) != len(inputs) {
		t.Fatalf("saved %d questions, want %d", len(saved.Questions), len(inputs))
	}
	if saved.Questions[0].Text != "What color do you like best?" || saved.Questions[0].DefaultKey != "fav-color" {
		t.Errorf("first question = %+v, want the reworded color question", saved.Questions[0])
	}
	if last := saved.Questions[len(saved.Questions)-1]; last.Text != "Who is your favorite superhero?" || last.Id == 0 {
		t.Errorf("last question = %+v, want the added one, saved", last)
	}
	if findQuestion(saved.Questions, "fav-season").Id != 0 {
		t.Error("the removed default came back")
	}
	if len(saved.Suggested) != 0 {
		t.Errorf("suggested %v, want nothing: a removed default is never offered again", saved.Suggested)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ensureFamilyQuestions(tx, fx.familyId)
		vbolt.TxCommit(tx)
	})
	again, _ := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	if len(again.Questions) != len(inputs) {
		t.Errorf("ensureFamilyQuestions after the editor's save changed the list to %d", len(again.Questions))
	}
}

func TestSaveReordersRewordsAndArchives(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)

	inputs := questionInputs(resp.Questions)
	inputs[0], inputs[1] = inputs[1], inputs[0]
	inputs[2].Text = "  What's   your favorite   snack right now? "
	inputs[2].Section = "Snacks"
	inputs[3].Archived = true
	inputs[4].MinAgeYears, inputs[4].MaxAgeYears = 3, 10

	saved, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{Revision: resp.Revision, Questions: inputs})
	if err != nil {
		t.Fatal(err)
	}
	q := saved.Questions
	if q[0].DefaultKey != "fav-food" || q[1].DefaultKey != "fav-color" {
		t.Errorf("order = %s, %s; want food then color", q[0].DefaultKey, q[1].DefaultKey)
	}
	if q[2].Text != "What's your favorite snack right now?" || q[2].Section != "Snacks" || q[2].Id != resp.Questions[2].Id {
		t.Errorf("reworded question = %+v, want the same row with tidy text in a new section", q[2])
	}
	if !q[3].Archived {
		t.Error("archived question is not archived")
	}
	if q[4].MinAgeYears != 3 || q[4].MaxAgeYears != 10 {
		t.Errorf("ages = %d–%d, want 3–10", q[4].MinAgeYears, q[4].MaxAgeYears)
	}
}

func TestSaveRejectsAStaleRevision(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)

	if _, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{
		Revision: resp.Revision, Questions: questionInputs(resp.Questions),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{
		Revision: resp.Revision, Questions: questionInputs(resp.Questions),
	})
	if err != ErrInterviewQuestionsChanged {
		t.Errorf("second save with the old revision: err = %v, want ErrInterviewQuestionsChanged", err)
	}
}

func TestSaveValidatesQuestions(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)

	for name, mutate := range map[string]func([]InterviewQuestionInput) []InterviewQuestionInput{
		"blank text": func(in []InterviewQuestionInput) []InterviewQuestionInput { in[0].Text = "  "; return in },
		"long text": func(in []InterviewQuestionInput) []InterviewQuestionInput {
			in[0].Text = strings.Repeat("a", 301)
			return in
		},
		"ages swapped": func(in []InterviewQuestionInput) []InterviewQuestionInput {
			in[0].MinAgeYears, in[0].MaxAgeYears = 9, 4
			return in
		},
		"duplicate":  func(in []InterviewQuestionInput) []InterviewQuestionInput { return append(in, in[0]) },
		"foreign id": func(in []InterviewQuestionInput) []InterviewQuestionInput { in[0].Id = 99999; return in },
	} {
		_, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{
			Revision: resp.Revision, Questions: mutate(questionInputs(resp.Questions)),
		})
		if err == nil {
			t.Errorf("%s: saved, want an error", name)
		}
	}
}

func TestDeleteOnlyUnaskedQuestions(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)
	food := findQuestion(resp.Questions, "fav-food")
	season := findQuestion(resp.Questions, "fav-season")
	fx.recordInterview(t, fx.alice, time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC), map[int]string{food.Id: "pizza"})

	resp, _ = callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	if u := resp.Usage[food.Id]; u.Interviews != 1 || u.Answers != 1 {
		t.Errorf("usage of food = %+v, want one interview and one answer", u)
	}

	without := func(id int) []InterviewQuestionInput {
		return slices.DeleteFunc(questionInputs(resp.Questions), func(in InterviewQuestionInput) bool { return in.Id == id })
	}
	if _, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{
		Revision: resp.Revision, Questions: without(food.Id), DeleteIds: []int{food.Id},
	}); err == nil {
		t.Fatal("deleted a question with an answer")
	}

	saved, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{
		Revision: resp.Revision, Questions: without(season.Id), DeleteIds: []int{season.Id},
	})
	if err != nil {
		t.Fatal(err)
	}
	if findQuestion(saved.Questions, "fav-season").Id != 0 {
		t.Error("the unasked question is still on the list")
	}
	if findQuestion(saved.Suggested, "fav-season").DefaultKey != "" {
		t.Error("a deleted built-in was offered again")
	}
}

func TestNewDefaultsAreSuggestedNotAdded(t *testing.T) {
	fx := setupResultsFixture(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		ensureFamilyQuestions(tx, fx.familyId)
		for _, q := range familyQuestions(tx, fx.familyId) {
			if q.DefaultKey == "fav-word" || q.DefaultKey == "travel" {
				deleteInterviewQuestion(tx, q.Id)
			}
		}
		vbolt.TxCommit(tx)
	})

	resp, _ := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	if len(resp.Suggested) != 2 {
		t.Fatalf("suggested %d, want the 2 built-ins the family doesn't have", len(resp.Suggested))
	}
	if len(resp.Questions) != len(defaultInterviewQuestions)-2 {
		t.Errorf("reading added suggestions to the list")
	}

	inputs := append(questionInputs(resp.Questions), questionInputs(resp.Suggested[:1])...)
	saved, err := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{Revision: resp.Revision, Questions: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Suggested) != 1 || len(saved.Questions) != len(defaultInterviewQuestions)-1 {
		t.Errorf("after adopting one: %d suggested, %d questions", len(saved.Suggested), len(saved.Questions))
	}
}

func TestMergeMovesAnswersAndArchivesTheSource(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)
	food := findQuestion(resp.Questions, "fav-food")

	inputs := append(questionInputs(resp.Questions), InterviewQuestionInput{Text: "Fav. food", Section: "Imported"})
	saved, _ := callAs(t, fx, SaveInterviewQuestions, SaveInterviewQuestionsRequest{Revision: resp.Revision, Questions: inputs})
	dup := saved.Questions[len(saved.Questions)-1]

	old := fx.recordInterview(t, fx.alice, time.Date(2021, 3, 2, 0, 0, 0, 0, time.UTC), map[int]string{dup.Id: "noodles"})
	fx.recordInterview(t, fx.alice, time.Date(2022, 3, 2, 0, 0, 0, 0, time.UTC), map[int]string{food.Id: "pizza"})

	merged, err := callAs(t, fx, MergeInterviewQuestions, MergeInterviewQuestionsRequest{
		FromId: dup.Id, IntoId: food.Id, Revision: saved.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Usage[food.Id].Answers != 2 || merged.Usage[dup.Id].Answers != 0 {
		t.Errorf("usage after merge: food %+v, dup %+v", merged.Usage[food.Id], merged.Usage[dup.Id])
	}
	for _, q := range merged.Questions {
		if q.Id == dup.Id && !q.Archived {
			t.Error("the merged question was not archived")
		}
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		answers := interviewAnswers(tx, old.Id)
		if len(answers) != 1 || answers[0].QuestionId != food.Id || answers[0].Context != "Fav. food" {
			t.Errorf("old answer = %+v, want it on the food question with its original wording", answers)
		}
		if got := GetInterviewById(tx, old.Id).QuestionIds; !slices.Equal(got, []int{food.Id}) {
			t.Errorf("old interview questions = %v, want [%d]", got, food.Id)
		}
	})
}

func TestMergeRefusesTwoAnswersInOneInterview(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)
	food := findQuestion(resp.Questions, "fav-food")
	snack := findQuestion(resp.Questions, "fav-snack")
	fx.recordInterview(t, fx.alice, time.Date(2022, 3, 2, 0, 0, 0, 0, time.UTC), map[int]string{food.Id: "pizza", snack.Id: "goldfish"})

	if _, err := callAs(t, fx, MergeInterviewQuestions, MergeInterviewQuestionsRequest{
		FromId: snack.Id, IntoId: food.Id, Revision: resp.Revision,
	}); err == nil {
		t.Error("merged two questions answered in the same interview")
	}
}

func TestInterviewQuestionAccess(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()
	call := func(user User, fn func(ctx *vbeam.Context)) {
		token := testAuthToken(t, user)
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) { fn(&vbeam.Context{Tx: tx, Token: token}) })
	}

	var own InterviewQuestionsResponse
	call(fx.userA, func(ctx *vbeam.Context) {
		var err error
		if own, err = SaveInterviewQuestions(ctx, SaveInterviewQuestionsRequest{
			Questions: []InterviewQuestionInput{{Text: "Favorite dinosaur?", Section: "Favorites"}},
		}); err != nil {
			t.Fatal(err)
		}
	})

	call(fx.userB, func(ctx *vbeam.Context) {
		if _, err := GetInterviewQuestions(ctx, GetInterviewQuestionsRequest{FamilyId: fx.famA}); err == nil {
			t.Error("a linked family read another family's question list")
		}
	})
	call(fx.userB, func(ctx *vbeam.Context) {
		if _, err := SaveInterviewQuestions(ctx, SaveInterviewQuestionsRequest{
			FamilyId: fx.famA, Revision: own.Revision, Questions: questionInputs(own.Questions),
		}); err == nil {
			t.Error("a linked family saved another family's question list")
		}
	})
	call(fx.userB, func(ctx *vbeam.Context) {
		if _, err := MergeInterviewQuestions(ctx, MergeInterviewQuestionsRequest{
			FromId: own.Questions[0].Id, IntoId: own.Questions[0].Id + 1, Revision: own.Revision,
		}); err == nil {
			t.Error("a linked family merged another family's questions")
		}
	})
	call(fx.userB, func(ctx *vbeam.Context) {
		resp, err := GetInterviewQuestions(ctx, GetInterviewQuestionsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if resp.FamilyId != fx.famB || len(resp.Questions) != len(defaultInterviewQuestions) {
			t.Errorf("own list for B = family %d with %d questions", resp.FamilyId, len(resp.Questions))
		}
	})
}

func TestNormalMomentFormsCannotTouchInterviewAnswers(t *testing.T) {
	fx := setupResultsFixture(t)
	resp := fx.seedQuestions(t)
	food := findQuestion(resp.Questions, "fav-food")
	interview := fx.recordInterview(t, fx.alice, time.Date(2022, 3, 2, 0, 0, 0, 0, time.UTC), map[int]string{food.Id: "pizza"})
	day := "2020-01-01"

	if _, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{
		PersonId: fx.alice.Id, Description: "pizza", Category: CategoryInterview, InputType: "today",
	}); err != ErrInterviewCategory {
		t.Errorf("AddMilestone(interview) err = %v, want ErrInterviewCategory", err)
	}

	var answer Milestone
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { answer = interviewAnswers(tx, interview.Id)[0] })

	if _, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{
		Id: answer.Id, Description: "pizza", Category: "other", InputType: "date", MilestoneDate: &day,
	}); err != ErrInterviewCategory {
		t.Errorf("switching an answer out of interview: err = %v", err)
	}

	plain := fx.addMilestoneOn(t, fx.alice, "Lost a tooth", "first", time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))
	if _, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{
		Id: plain.Id, Description: "Lost a tooth", Category: CategoryInterview, InputType: "date", MilestoneDate: &day,
	}); err != ErrInterviewCategory {
		t.Errorf("switching a moment into interview: err = %v", err)
	}

	updated, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{
		Id: answer.Id, InterviewVersion: 0, Description: " pepperoni pizza ", Category: CategoryInterview, InputType: "date", MilestoneDate: &day,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := updated.Milestone
	if m.Description != "pepperoni pizza" || !m.MilestoneDate.Equal(answer.MilestoneDate) ||
		m.Context != answer.Context || m.InterviewId != interview.Id || m.QuestionId != food.Id {
		t.Errorf("updated answer = %+v, want new text with the date, question and interview kept", m)
	}
}
