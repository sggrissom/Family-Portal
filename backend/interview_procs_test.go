package backend

import (
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) startInterview(t *testing.T, person Person, date string, keys ...string) GetInterviewResponse {
	t.Helper()
	started, err := callAs(t, fx, StartInterview, StartInterviewRequest{PersonId: person.Id, Date: date, DefaultKeys: keys})
	if err != nil {
		t.Fatalf("StartInterview: %v", err)
	}
	got, err := callAs(t, fx, GetInterview, GetInterviewRequest{Id: started.Interview.Id})
	if err != nil {
		t.Fatalf("GetInterview: %v", err)
	}
	return got
}

func (fx resultsFixture) answer(t *testing.T, interviewId, questionId int, text string, skipped bool) SaveInterviewAnswerResponse {
	t.Helper()
	current, _ := callAs(t, fx, GetInterview, GetInterviewRequest{Id: interviewId})
	resp, err := callAs(t, fx, SaveInterviewAnswer, SaveInterviewAnswerRequest{
		Version: current.Interview.Version, InterviewId: interviewId, QuestionId: questionId, Answer: text, Skipped: skipped,
	})
	if err != nil {
		t.Fatalf("SaveInterviewAnswer: %v", err)
	}
	return resp
}

func TestAgeYearsAt(t *testing.T) {
	birthday := time.Date(2018, 5, 10, 0, 0, 0, 0, time.UTC)
	for date, want := range map[string]int{
		"2018-05-10": 0, "2019-05-09": 0, "2019-05-10": 1, "2025-04-30": 6, "2025-05-10": 7, "2017-01-01": -1,
	} {
		d, _ := time.Parse("2006-01-02", date)
		if got := ageYearsAt(birthday, d); got != want {
			t.Errorf("age on %s = %d, want %d", date, got, want)
		}
	}
}

func TestStartInterviewSeedsTheListAndRendersPlaceholders(t *testing.T) {
	fx := setupResultsFixture(t)

	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food", "best-age")
	if got.Interview.Status != InterviewInProgress || got.Interview.Source != InterviewConducted {
		t.Errorf("interview = %+v, want a conducted interview in progress", got.Interview)
	}
	if !got.Interview.Date.Equal(time.Date(2021, 3, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %v, want UTC midnight of the day", got.Interview.Date)
	}
	if len(got.Questions) != 2 || got.Questions[1].Text != "What's the best thing about being 7?" {
		t.Errorf("questions = %+v, want food then the rendered age question", got.Questions)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if n := len(familyQuestions(tx, fx.familyId)); n != len(defaultInterviewQuestions) {
			t.Errorf("starting an interview left %d rows, want the defaults copied", n)
		}
	})

	if _, err := callAs(t, fx, StartInterview, StartInterviewRequest{PersonId: fx.alice.Id, Date: "2021-03-05", DefaultKeys: []string{"fav-food"}}); err != ErrInterviewInProgress {
		t.Errorf("second start: err = %v, want ErrInterviewInProgress", err)
	}
	setup, _ := callAs(t, fx, GetInterviewSetup, GetInterviewSetupRequest{PersonId: fx.alice.Id})
	if setup.InProgress.Id != got.Interview.Id {
		t.Errorf("setup in progress = %d, want %d", setup.InProgress.Id, got.Interview.Id)
	}
}

func TestStartInterviewWithOneOffs(t *testing.T) {
	fx := setupResultsFixture(t)
	resp, err := callAs(t, fx, StartInterview, StartInterviewRequest{
		PersonId: fx.bob.Id, Date: "2020-01-01", DefaultKeys: []string{"fav-color"}, OneOffQuestions: []string{"Who is your favorite dinosaur?"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Interview.QuestionIds) != 2 {
		t.Fatalf("questions = %v", resp.Interview.QuestionIds)
	}
	list, _ := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	for _, q := range list.Questions {
		if q.Text == "Who is your favorite dinosaur?" {
			t.Error("a one-off question landed on the family list")
		}
	}
	if _, err := callAs(t, fx, StartInterview, StartInterviewRequest{PersonId: fx.carol.Id}); err == nil {
		t.Error("started an interview with no questions")
	}
}

func TestAnswersAreMomentsThatSaveSkipAndClear(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food", "fav-snack")
	id := got.Interview.Id
	food, snack := got.Questions[0].Id, got.Questions[1].Id

	saved := fx.answer(t, id, food, "  pizza ", false)
	if saved.Answer.Id == 0 || saved.Answer.Description != "pizza" || saved.Answer.Category != CategoryInterview ||
		saved.Answer.Context != "What's your favorite food?" || saved.Answer.QuestionId != food ||
		!saved.Answer.MilestoneDate.Equal(got.Interview.Date) {
		t.Fatalf("answer = %+v", saved.Answer)
	}
	again := fx.answer(t, id, food, "pizza and tacos", false)
	if again.Answer.Id != saved.Answer.Id {
		t.Errorf("re-saving created moment %d, want %d updated", again.Answer.Id, saved.Answer.Id)
	}

	skipped := fx.answer(t, id, snack, "", true)
	if !slices.Contains(skipped.Interview.SkippedIds, snack) || skipped.Answer.Id != 0 {
		t.Errorf("skip = %+v", skipped)
	}
	unskipped := fx.answer(t, id, snack, "goldfish", false)
	if slices.Contains(unskipped.Interview.SkippedIds, snack) {
		t.Error("answering a skipped question left it skipped")
	}

	cleared := fx.answer(t, id, food, "  ", false)
	if cleared.Answer.Id != 0 {
		t.Error("clearing returned an answer")
	}
	if m, _ := callAs(t, fx, GetMilestone, GetMilestoneRequest{Id: saved.Answer.Id}); m.Milestone.Id != 0 {
		t.Error("clearing an answer left its moment")
	}

	read, _ := callAs(t, fx, GetInterview, GetInterviewRequest{Id: id})
	if len(read.Answers) != 1 || read.Answers[0].Description != "goldfish" {
		t.Errorf("answers = %+v, want only goldfish", read.Answers)
	}

	if _, err := callAs(t, fx, SaveInterviewAnswer, SaveInterviewAnswerRequest{
		InterviewId: id, QuestionId: 99999, Answer: "x",
	}); err == nil {
		t.Error("answered a question that isn't in the interview")
	}
}

func TestPreviousAnswersComeFromEarlierInterviewsOfTheSamePerson(t *testing.T) {
	fx := setupResultsFixture(t)
	first := fx.startInterview(t, fx.alice, "2019-03-05", "fav-food")
	food := first.Questions[0].Id
	fx.answer(t, first.Interview.Id, food, "noodles", false)
	fx.finish(t, first.Interview)

	bobs := fx.startInterview(t, fx.bob, "2019-03-05", "fav-food")
	fx.answer(t, bobs.Interview.Id, food, "rice", false)
	fx.finish(t, bobs.Interview)

	second := fx.startInterview(t, fx.alice, "2020-03-05", "fav-food")
	fx.answer(t, second.Interview.Id, food, "pizza", false)
	fx.finish(t, second.Interview)

	third := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	prev := third.Previous[food]
	if len(prev) != 2 || prev[0].Text != "pizza" || prev[1].Text != "noodles" || prev[0].AgeYears != 6 {
		t.Errorf("previous = %+v, want pizza at 6 then noodles", prev)
	}
}

func (fx resultsFixture) finish(t *testing.T, interview Interview) Interview {
	t.Helper()
	current, _ := callAs(t, fx, GetInterview, GetInterviewRequest{Id: interview.Id})
	resp, err := callAs(t, fx, UpdateInterview, UpdateInterviewRequest{
		Id: interview.Id, Version: current.Interview.Version, Date: interview.Date.Format("2006-01-02"), Status: InterviewComplete,
	})
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	return resp.Interview
}

func TestAddInterviewQuestionKeepsOrOneOff(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-color", "fav-food")
	color := got.Questions[0]

	kept, err := callAs(t, fx, AddInterviewQuestion, AddInterviewQuestionRequest{
		Version: got.Interview.Version, InterviewId: got.Interview.Id, Text: "What's your favorite shape?", AfterQuestionId: color.Id, Keep: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Interview.QuestionIds[1] != kept.Question.Id {
		t.Errorf("question order = %v, want the new one second", kept.Interview.QuestionIds)
	}
	got.Interview = kept.Interview
	once, err := callAs(t, fx, AddInterviewQuestion, AddInterviewQuestionRequest{
		Version: got.Interview.Version, InterviewId: got.Interview.Id, Text: "Why?", AfterQuestionId: kept.Question.Id,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !once.Question.OneOff {
		t.Error("a just-this-once question was saved to the list")
	}

	list, _ := callAs(t, fx, GetInterviewQuestions, GetInterviewQuestionsRequest{})
	if list.Questions[1].Id != kept.Question.Id || list.Questions[1].Section != color.Section {
		t.Errorf("list[1] = %+v, want the kept question right after color in its section", list.Questions[1])
	}
	for _, q := range list.Questions {
		if q.Id == once.Question.Id {
			t.Error("one-off on the family list")
		}
	}
}

func TestUpdateInterviewMovesAnswerDatesAndChecksVersion(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	answer := fx.answer(t, got.Interview.Id, got.Questions[0].Id, "pizza", false).Answer
	photo := fx.addPhoto(t, fx.familyId, "portrait.jpg")

	updated, err := callAs(t, fx, UpdateInterview, UpdateInterviewRequest{
		Id: got.Interview.Id, Version: got.Interview.Version + 1, Date: "2021-03-02",
		Title: "7th birthday", Notes: "Very silly", PhotoId: photo.Id, Status: InterviewComplete,
	})
	if err != nil {
		t.Fatal(err)
	}
	i := updated.Interview
	if i.Status != InterviewComplete || i.CompletedAt.IsZero() || i.PhotoId != photo.Id || i.Title != "7th birthday" {
		t.Errorf("updated = %+v", i)
	}
	moved, _ := callAs(t, fx, GetMilestone, GetMilestoneRequest{Id: answer.Id})
	if !moved.Milestone.MilestoneDate.Equal(time.Date(2021, 3, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("answer date = %v, want it moved with the interview", moved.Milestone.MilestoneDate)
	}

	if _, err := callAs(t, fx, UpdateInterview, UpdateInterviewRequest{
		Id: i.Id, Version: got.Interview.Version, Date: "2021-03-02", Status: InterviewComplete,
	}); err != ErrInterviewChanged {
		t.Errorf("stale version: err = %v", err)
	}
}

func TestDeleteInterviewRemovesItsAnswers(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	answer := fx.answer(t, got.Interview.Id, got.Questions[0].Id, "pizza", false).Answer

	if _, err := callAs(t, fx, DeleteInterview, DeleteInterviewRequest{Id: got.Interview.Id, Version: 2}); err != nil {
		t.Fatal(err)
	}
	if m, _ := callAs(t, fx, GetMilestone, GetMilestoneRequest{Id: answer.Id}); m.Milestone.Id != 0 {
		t.Error("answer survived its interview")
	}
	if _, err := callAs(t, fx, GetInterview, GetInterviewRequest{Id: got.Interview.Id}); err == nil {
		t.Error("deleted interview still readable")
	}
	fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
}

func TestInterviewAccessThroughLinksIsReadOnly(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()
	call := func(user User, fn func(ctx *vbeam.Context)) {
		token := testAuthToken(t, user)
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) { fn(&vbeam.Context{Tx: tx, Token: token}) })
	}

	var interview Interview
	call(fx.userA, func(ctx *vbeam.Context) {
		resp, err := StartInterview(ctx, StartInterviewRequest{PersonId: fx.alice.Id, Date: "2021-03-05", DefaultKeys: []string{"fav-food"}})
		if err != nil {
			t.Fatal(err)
		}
		interview = resp.Interview
	})
	call(fx.userA, func(ctx *vbeam.Context) {
		if _, err := SaveInterviewAnswer(ctx, SaveInterviewAnswerRequest{
			Version: interview.Version, InterviewId: interview.Id, QuestionId: interview.QuestionIds[0], Answer: "pizza",
		}); err != nil {
			t.Fatal(err)
		}
	})

	call(fx.userB, func(ctx *vbeam.Context) {
		resp, err := GetInterview(ctx, GetInterviewRequest{Id: interview.Id})
		if err != nil || len(resp.Answers) != 1 {
			t.Errorf("linked grandparent read: %v, %d answers", err, len(resp.Answers))
		}
	})
	call(fx.userB, func(ctx *vbeam.Context) {
		if _, err := SaveInterviewAnswer(ctx, SaveInterviewAnswerRequest{
			InterviewId: interview.Id, QuestionId: interview.QuestionIds[0], Answer: "broccoli",
		}); err == nil {
			t.Error("linked grandparent changed an answer")
		}
	})
	call(fx.userB, func(ctx *vbeam.Context) {
		if _, err := StartInterview(ctx, StartInterviewRequest{PersonId: fx.alice.Id, Date: "2021-03-05", DefaultKeys: []string{"fav-food"}}); err == nil {
			t.Error("linked grandparent started an interview")
		}
	})
	call(fx.userB, func(ctx *vbeam.Context) {
		if _, err := DeleteInterview(ctx, DeleteInterviewRequest{Id: interview.Id}); err == nil {
			t.Error("linked grandparent deleted an interview")
		}
	})
	call(fx.userC, func(ctx *vbeam.Context) {
		if _, err := GetInterview(ctx, GetInterviewRequest{Id: interview.Id}); err == nil {
			t.Error("an unlinked family read the interview")
		}
	})
}
