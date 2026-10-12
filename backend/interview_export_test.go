package backend

import (
	"encoding/json"
	"strings"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) asUser(t *testing.T, user User, fn func(ctx *vbeam.Context)) {
	t.Helper()
	token := testAuthToken(t, user)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) { fn(&vbeam.Context{Tx: tx, Token: token}) })
}

func TestInterviewsSurviveExportAndImport(t *testing.T) {
	fx := setupResultsFixture(t)
	first := fx.startInterview(t, fx.alice, "2020-03-05", "fav-food", "best-age")
	fx.answer(t, first.Interview.Id, first.Questions[0].Id, "pizza", false)
	fx.answer(t, first.Interview.Id, first.Questions[1].Id, "staying up late", false)
	fx.finish(t, first.Interview)
	fx.addMilestoneOn(t, fx.alice, "Lost a tooth", "first", first.Interview.Date)
	if _, err := callAs(t, fx, ImportInterviews, ImportInterviewsRequest{PersonId: fx.alice.Id, Interviews: []HistoricalInterview{{
		InputType: "date", Date: "2019-03-02",
		Answers: []ImportAnswer{{Ref: "key:fav-food", Answer: "noodles"}, {NewText: "Favorite dinosaur", Keep: true, Answer: "T. rex"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	exported, err := callAs(t, fx, ExportData, ExportDataRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var data ExportDataStructure
	if err := json.Unmarshal([]byte(exported.JsonData), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Interviews) != 2 || data.TotalInterviews != 2 || len(data.InterviewQuestions) == 0 {
		t.Fatalf("exported %d interviews and %d questions", len(data.Interviews), len(data.InterviewQuestions))
	}
	answers := 0
	for _, m := range data.Milestones {
		if m.InterviewId > 0 {
			answers++
			if m.QuestionId == 0 {
				t.Errorf("exported answer %+v has no question", m)
			}
		}
	}
	if answers != 4 {
		t.Errorf("exported %d answers, want 4", answers)
	}

	var other User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		other = AddUserTx(tx, CreateAccountRequest{Name: "Other", Email: "other-import@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	importOnce := func() ImportDataResponse {
		var resp ImportDataResponse
		fx.asUser(t, other, func(ctx *vbeam.Context) {
			resp, err = ImportData(ctx, ImportDataRequest{JsonData: exported.JsonData, ImportMilestones: true, MergeStrategy: "merge_people"})
		})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	resp := importOnce()
	if resp.ImportedInterviews != 2 || resp.ImportedMilestones != 1 {
		t.Fatalf("import = %d interviews, %d moments; want 2 and 1", resp.ImportedInterviews, resp.ImportedMilestones)
	}
	aliceThere := resp.PersonIdMapping[fx.alice.Id]
	var bundle GetPersonInterviewsResponse
	fx.asUser(t, other, func(ctx *vbeam.Context) {
		bundle, err = GetPersonInterviews(ctx, GetPersonInterviewsRequest{PersonId: aliceThere})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Interviews) != 2 || len(bundle.Answers) != 4 {
		t.Fatalf("imported bundle = %d interviews, %d answers", len(bundle.Interviews), len(bundle.Answers))
	}
	texts := map[string]string{}
	for _, a := range bundle.Answers {
		texts[a.Context] = a.Description
		if a.FamilyId != other.FamilyId || a.Category != CategoryInterview || a.QuestionId == 0 {
			t.Errorf("imported answer %+v", a)
		}
	}
	if texts["What's the best thing about being 6?"] != "staying up late" || texts["Favorite dinosaur"] != "T. rex" {
		t.Errorf("imported answers = %v", texts)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		food := 0
		for _, q := range familyQuestions(tx, other.FamilyId) {
			if q.DefaultKey == "fav-food" {
				food++
			}
			if q.Text == "Favorite dinosaur" && q.OneOff {
				t.Error("a kept question came back as a one-off")
			}
		}
		if food != 1 {
			t.Errorf("fav-food rows = %d, want the built-in reused once", food)
		}
	})

	again := importOnce()
	if again.ImportedInterviews != 0 || again.SkippedInterviews != 2 {
		t.Errorf("re-import = %d imported, %d skipped; want nothing new", again.ImportedInterviews, again.SkippedInterviews)
	}

	old := strings.Replace(exported.JsonData, `"interviews"`, `"ignored"`, 1)
	var legacy map[string]any
	_ = json.Unmarshal([]byte(old), &legacy)
	delete(legacy, "interviewQuestions")
	delete(legacy, "ignored")
	var plain []any
	for _, m := range legacy["milestones"].([]any) {
		if m.(map[string]any)["interviewId"] == nil {
			plain = append(plain, m)
		}
	}
	legacy["milestones"] = plain
	encoded, _ := json.Marshal(legacy)
	var third User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		third = AddUserTx(tx, CreateAccountRequest{Name: "Third", Email: "third-import@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	fx.asUser(t, third, func(ctx *vbeam.Context) {
		resp, err = ImportData(ctx, ImportDataRequest{JsonData: string(encoded), ImportMilestones: true})
	})
	if err != nil || resp.ImportedMilestones != 1 || resp.ImportedInterviews != 0 {
		t.Errorf("an export from before interviews: %+v, %v", resp, err)
	}
}

func TestBooksCarryInterviews(t *testing.T) {
	fx := setupResultsFixture(t)
	first := fx.startInterview(t, fx.alice, "2020-03-05", "fav-food", "fav-color")
	fx.answer(t, first.Interview.Id, first.Questions[0].Id, "pizza", false)
	fx.answer(t, first.Interview.Id, first.Questions[1].Id, "blue", false)
	fx.finish(t, first.Interview)
	second := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food", "fav-color")
	fx.answer(t, second.Interview.Id, second.Questions[0].Id, "Pizza!", false)
	fx.answer(t, second.Interview.Id, second.Questions[1].Id, "green", false)
	fx.finish(t, second.Interview)
	open := fx.startInterview(t, fx.alice, "2021-06-01", "fav-food")
	fx.answer(t, open.Interview.Id, open.Questions[0].Id, "tacos", false)

	sources, err := callAs(t, fx, GetBookSources, GetBookSourcesRequest{
		PersonIds: []int{fx.alice.Id}, Preset: BookPresetYear, StartDate: "2021-01-01", EndDate: "2022-01-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources.Sources.Interviews) != 1 {
		t.Fatalf("sources hold %d interviews, want only the finished one in range", len(sources.Sources.Interviews))
	}
	bi := sources.Sources.Interviews[0]
	if bi.Interview.Id != second.Interview.Id || len(bi.Answers) != 2 {
		t.Errorf("book interview = %+v", bi)
	}
	if len(bi.Changes) != 1 || bi.Changes[0].Before != "blue" || bi.Changes[0].After != "green" {
		t.Errorf("changes = %+v, want only the color (pizza → Pizza! is the same answer)", bi.Changes)
	}
	for _, m := range sources.Sources.Milestones {
		if m.InterviewId > 0 {
			t.Errorf("answer %d appears in the plain moment stream", m.Id)
		}
	}

	created, err := callAs(t, fx, CreateBook, CreateBookRequest{
		PersonIds: []int{fx.alice.Id}, Preset: BookPresetYear, StartDate: "2021-01-01", EndDate: "2022-01-01",
		Content: BookContent{Categories: []string{"interviews"}, Items: []BookItem{{Kind: BookItemInterview, SourceId: second.Interview.Id}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Book.Items) != 1 || created.Book.Items[0].Kind != BookItemInterview || created.Book.Categories[0] != "interviews" {
		t.Errorf("book = %+v", created.Book)
	}
	if _, err := callAs(t, fx, CreateBook, CreateBookRequest{
		PersonIds: []int{fx.bob.Id}, Preset: BookPresetYear, StartDate: "2021-01-01", EndDate: "2022-01-01",
		Content: BookContent{Items: []BookItem{{Kind: BookItemInterview, SourceId: second.Interview.Id}}},
	}); err == nil {
		t.Error("a book about Bob took Alice's interview")
	}

	exported, _ := callAs(t, fx, ExportData, ExportDataRequest{})
	var data ExportDataStructure
	_ = json.Unmarshal([]byte(exported.JsonData), &data)
	if len(data.Books) != 1 || data.Books[0].Items[0].Kind != "interview" {
		t.Fatalf("exported book items = %+v", data.Books)
	}
	var other User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		other = AddUserTx(tx, CreateAccountRequest{Name: "Other", Email: "other-book@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	var resp ImportDataResponse
	fx.asUser(t, other, func(ctx *vbeam.Context) {
		resp, err = ImportData(ctx, ImportDataRequest{JsonData: exported.JsonData, ImportMilestones: true})
	})
	if err != nil || resp.ImportedBooks != 1 {
		t.Fatalf("book import: %+v, %v", resp, err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		books := GetFamilyBooks(tx, other.FamilyId)
		item := books[0].Items[0]
		interview := GetInterviewById(tx, item.SourceId)
		if item.Kind != BookItemInterview || interview.FamilyId != other.FamilyId {
			t.Errorf("imported book item %+v points at %+v", item, interview)
		}
	})
}

func TestCompareAnswersAcrossSiblings(t *testing.T) {
	fx := setupResultsFixture(t)
	for _, tc := range []struct {
		person Person
		date   string
		food   string
	}{
		{fx.alice, "2020-03-05", "pizza"},
		{fx.alice, "2021-03-05", "tacos"},
		{fx.bob, "2020-03-05", "sushi"},
	} {
		got := fx.startInterview(t, tc.person, tc.date, "fav-food")
		fx.answer(t, got.Interview.Id, got.Questions[0].Id, tc.food, false)
		fx.finish(t, got.Interview)
	}
	var food InterviewQuestion
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, q := range familyQuestions(tx, fx.familyId) {
			if q.DefaultKey == "fav-food" {
				food = q
			}
		}
	})

	resp, err := callAs(t, fx, CompareInterviewAnswers, CompareInterviewAnswersRequest{QuestionId: food.Id})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Question.Id != food.Id || len(resp.Questions) != 1 || len(resp.Rows) != 2 {
		t.Fatalf("compare = %+v", resp)
	}
	for _, row := range resp.Rows {
		if row.Person.Id == fx.alice.Id && (len(row.Answers) != 2 || row.Answers[0].Text != "pizza" || row.Answers[0].AgeYears != 6) {
			t.Errorf("alice's answers = %+v", row.Answers)
		}
	}
	only, _ := callAs(t, fx, CompareInterviewAnswers, CompareInterviewAnswersRequest{QuestionId: food.Id, PersonIds: []int{fx.bob.Id}})
	if len(only.Rows) != 1 || only.Rows[0].Person.Id != fx.bob.Id {
		t.Errorf("filtered compare = %+v", only.Rows)
	}
	first, _ := callAs(t, fx, CompareInterviewAnswers, CompareInterviewAnswersRequest{})
	if first.Question.Id != food.Id {
		t.Errorf("default question = %+v", first.Question)
	}

	var outsider User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		outsider = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider-compare@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	fx.asUser(t, outsider, func(ctx *vbeam.Context) {
		if _, err := CompareInterviewAnswers(ctx, CompareInterviewAnswersRequest{QuestionId: food.Id}); err == nil {
			t.Error("an outsider compared another family's answers")
		}
	})
}
