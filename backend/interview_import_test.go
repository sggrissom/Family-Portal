package backend

import (
	"strings"
	"testing"
	"time"
)

type parsedSummary struct {
	header string
	year   int
	age    int
	date   string
	rows   []string
}

func summarize(parsed []ParsedInterview) []parsedSummary {
	var out []parsedSummary
	for _, p := range parsed {
		s := parsedSummary{header: p.Header, year: p.When.Year, age: p.When.Age}
		if !p.When.Date.IsZero() {
			s.date = p.When.Date.Format("2006-01-02")
		}
		for _, r := range p.Rows {
			s.rows = append(s.rows, r.Question+" => "+r.Answer)
		}
		out = append(out, s)
	}
	return out
}

func TestParseInterviewHeader(t *testing.T) {
	for in, want := range map[string]InterviewWhen{
		"2019":                     {Year: 2019, Age: -1},
		"Age 5":                    {Age: 5},
		"age 5":                    {Age: 5},
		"5 years old":              {Age: 5},
		"5yo":                      {Age: 5},
		"2021-05-03":               {Date: time.Date(2021, 5, 3, 0, 0, 0, 0, time.UTC), Age: -1},
		"5/3/2021":                 {Date: time.Date(2021, 5, 3, 0, 0, 0, 0, time.UTC), Age: -1},
		"May 3, 2021":              {Date: time.Date(2021, 5, 3, 0, 0, 0, 0, time.UTC), Age: -1},
		"May 2021":                 {Year: 2021, Age: -1},
		"## 2020 Interview":        {Year: 2020, Age: -1},
		"Birthday interview 2018:": {Year: 2018, Age: -1},
		"Clara - age 6 (2022)":     {Year: 2022, Age: 6},
		"6th birthday questions":   {Age: -1},
	} {
		got, ok := parseInterviewHeader(in)
		wantOk := want.known()
		if ok != wantOk || got.Year != want.Year || got.Age != want.Age || !got.Date.Equal(want.Date) {
			t.Errorf("parseInterviewHeader(%q) = %+v, %v; want %+v, %v", in, got, ok, want, wantOk)
		}
	}
	for _, notHeader := range []string{"5", "pizza", "Favorite color: blue", "What's your favorite number?", "Lily"} {
		if _, ok := parseInterviewHeader(notHeader); ok {
			t.Errorf("%q parsed as a header", notHeader)
		}
	}
}

func TestParseInterviewText(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		csv  bool
		want []parsedSummary
	}{
		{
			name: "question and answer lines with mixed separators",
			text: "Favorite color: purple\n  - Favorite food - mac and cheese\nWhat's your favorite snack? Goldfish\n\n3. Best friend: Lily: from school\n",
			want: []parsedSummary{{age: -1, rows: []string{
				"Favorite color => purple",
				"Favorite food => mac and cheese",
				"What's your favorite snack? => Goldfish",
				"Best friend => Lily: from school",
			}}},
		},
		{
			name: "several years in one note, split by headers",
			text: "2019\nFav color: red\nFav food: pasta\n\nAge 5\nFav color: blue\nFav food: pizza\n",
			want: []parsedSummary{
				{header: "2019", year: 2019, age: -1, rows: []string{"Fav color => red", "Fav food => pasta"}},
				{header: "Age 5", age: 5, rows: []string{"Fav color => blue", "Fav food => pizza"}},
			},
		},
		{
			name: "age written as a field",
			text: "Age: 4\nFavorite animal: tiger\n",
			want: []parsedSummary{{age: 4, rows: []string{"Favorite animal => tiger"}}},
		},
		{
			name: "alternating question and answer lines",
			text: "What's your favorite color?\nGreen\nWho is your best friend?\nSam\nWhat do you want to be when you grow up?\nAn astronaut\n",
			want: []parsedSummary{{age: -1, rows: []string{
				"What's your favorite color? => Green",
				"Who is your best friend? => Sam",
				"What do you want to be when you grow up? => An astronaut",
			}}},
		},
		{
			name: "a sheet with one column per year",
			text: "Question\t2019\t2020\tAge 6\nFavorite color\tred\tblue\tgreen\nFavorite food\tpasta\t\tpizza\n",
			want: []parsedSummary{
				{header: "2019", year: 2019, age: -1, rows: []string{"Favorite color => red", "Favorite food => pasta"}},
				{header: "2020", year: 2020, age: -1, rows: []string{"Favorite color => blue", "Favorite food => "}},
				{header: "Age 6", age: 6, rows: []string{"Favorite color => green", "Favorite food => pizza"}},
			},
		},
		{
			name: "a sheet with one row per year",
			text: "Year\tFavorite color\tFavorite food\n2019\tred\tpasta\n2020\tblue\tpizza\n",
			want: []parsedSummary{
				{header: "2019", year: 2019, age: -1, rows: []string{"Favorite color => red", "Favorite food => pasta"}},
				{header: "2020", year: 2020, age: -1, rows: []string{"Favorite color => blue", "Favorite food => pizza"}},
			},
		},
		{
			name: "a csv file with quoted commas",
			csv:  true,
			text: "Question,2021-05-03,2022-05-04\nFavorite food,\"pizza, tacos\",sushi\n",
			want: []parsedSummary{
				{header: "2021-05-03", date: "2021-05-03", age: -1, rows: []string{"Favorite food => pizza, tacos"}},
				{header: "2022-05-04", date: "2022-05-04", age: -1, rows: []string{"Favorite food => sushi"}},
			},
		},
		{
			name: "a sheet without a header row",
			text: "Favorite color\tred\nFavorite food\tpasta\n",
			want: []parsedSummary{{age: -1, rows: []string{"Favorite color => red", "Favorite food => pasta"}}},
		},
	} {
		got := summarize(parseInterviewText(tc.text, tc.csv))
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %d interviews %+v, want %d", tc.name, len(got), got, len(tc.want))
			continue
		}
		for i := range got {
			g, w := got[i], tc.want[i]
			if g.header != w.header || g.year != w.year || g.age != w.age || g.date != w.date ||
				strings.Join(g.rows, "|") != strings.Join(w.rows, "|") {
				t.Errorf("%s: interview %d = %+v, want %+v", tc.name, i, g, w)
			}
		}
	}
}

func TestMatchQuestions(t *testing.T) {
	var candidates []InterviewQuestion
	for _, d := range defaultInterviewQuestions {
		candidates = append(candidates, d.row(1))
	}
	raw := []string{
		"Fav food",
		"Favourite colour",
		"What's your favorite snack",
		"Best friend",
		"Who's your BFF?",
		"Fav TV show",
		"When I grow up I want to be",
		"Favorite dinosaur",
	}
	matches := matchQuestions(raw, candidates, nil)
	want := []questionMatch{
		{"key:fav-food", matchExact},
		{"key:fav-color", matchExact},
		{"key:fav-snack", matchExact},
		{"key:best-friend", matchExact},
		{"", matchNone},
		{"key:fav-show", matchExact},
		{"key:grow-up", matchClose},
		{"", matchNone},
	}
	for i := range raw {
		if matches[i] != want[i] {
			t.Errorf("%q matched %+v, want %+v", raw[i], matches[i], want[i])
		}
	}

	embedded := matchQuestions([]string{"Who's your BFF?"}, candidates, func(texts []string) ([][]float32, error) {
		vectors := make([][]float32, len(texts))
		for i, text := range texts {
			if i == 0 || text == "Who is your best friend?" {
				vectors[i] = []float32{1, 0}
			} else {
				vectors[i] = []float32{0, 1}
			}
		}
		return vectors, nil
	})
	if embedded[0] != (questionMatch{"key:best-friend", matchGuess}) {
		t.Errorf("with sentence similarity BFF matched %+v", embedded[0])
	}
}

func TestPreviewAndImportPastInterviews(t *testing.T) {
	fx := setupResultsFixture(t)
	text := "Question\t2019\tAge 7\nFav food\tpasta\tpizza\nFavorite dinosaur\tT. rex\tstegosaurus\nWho's your BFF?\tSam\t\n"

	preview, err := callAs(t, fx, PreviewInterviewImport, PreviewInterviewImportRequest{PersonId: fx.alice.Id, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Proposals) != 2 {
		t.Fatalf("proposals = %+v", preview.Proposals)
	}
	if p := preview.Proposals[0]; p.Date != "" || p.Rows[0].Ref != "key:fav-food" || p.Rows[1].Confidence != matchNone {
		t.Errorf("first proposal = %+v, want 2019 on the birthday with food matched", p)
	}
	if p := preview.Proposals[1]; p.Date != "" || p.AgeYears != 7 {
		t.Errorf("second proposal = %+v, want age 7 on the seventh birthday", p)
	}
	var interviews []HistoricalInterview
	for _, p := range preview.Proposals {
		in := HistoricalInterview{InputType: "date", Date: "2020-03-02"}
		for _, r := range p.Rows {
			a := ImportAnswer{Ref: r.Ref, Answer: r.Answer}
			if r.Ref == "" && strings.Contains(r.Raw, "dinosaur") {
				a.NewText, a.Keep = r.Raw, true
			} else if r.Ref == "" {
				a.NewText = r.Raw
			}
			in.Answers = append(in.Answers, a)
		}
		interviews = append(interviews, in)
	}
	imported, err := callAs(t, fx, ImportInterviews, ImportInterviewsRequest{PersonId: fx.alice.Id, Interviews: interviews})
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Interviews) != 2 || imported.Interviews[0].Source != InterviewHistorical || imported.Interviews[0].Status != InterviewComplete {
		t.Fatalf("imported = %+v", imported.Interviews)
	}
	if len(imported.Interviews[0].QuestionIds) != 3 || len(imported.Interviews[1].QuestionIds) != 2 {
		t.Errorf("question counts = %d and %d, want 3 and 2 (blank answers aren't asked)",
			len(imported.Interviews[0].QuestionIds), len(imported.Interviews[1].QuestionIds))
	}

	bundle, _ := callAs(t, fx, GetPersonInterviews, GetPersonInterviewsRequest{PersonId: fx.alice.Id})
	dinos := map[int]bool{}
	for _, q := range bundle.Questions {
		if q.Text == "Favorite dinosaur" {
			dinos[q.Id] = true
			if q.OneOff {
				t.Error("a question marked add-to-my-list was saved as a one-off")
			}
		}
		if q.Text == "Who's your BFF?" && !q.OneOff {
			t.Error("an unchecked new question landed on the list")
		}
	}
	if len(dinos) != 1 {
		t.Errorf("the dinosaur question was created %d times, want once across both interviews", len(dinos))
	}

	dup := HistoricalInterview{InputType: "date", Date: "2020-01-01", Answers: []ImportAnswer{
		{Ref: "key:fav-food", Answer: "a"}, {Ref: "key:fav-food", Answer: "b"},
	}}
	if _, err := callAs(t, fx, ImportInterviews, ImportInterviewsRequest{PersonId: fx.alice.Id, Interviews: []HistoricalInterview{dup}}); err == nil {
		t.Error("imported one question answered twice in an interview")
	}

	byAge, err := callAs(t, fx, SaveHistoricalInterview, SaveHistoricalInterviewRequest{PersonId: fx.bob.Id, Interview: HistoricalInterview{
		InputType: "date", Date: "2018-03-02", Notes: "From the baby book",
		Answers: []ImportAnswer{{Ref: "key:fav-color", Answer: "orange"}, {Ref: "key:fav-food", Answer: " "}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := byAge.Interview; got.Date.Format("2006-01-02") != "2018-03-02" || len(got.QuestionIds) != 1 || got.Notes != "From the baby book" {
		t.Errorf("typed-in interview = %+v", got)
	}

	outsider := fx.outsiderPerson(t)
	if _, err := callAs(t, fx, ImportInterviews, ImportInterviewsRequest{PersonId: outsider.Id, Interviews: interviews}); err == nil {
		t.Error("imported into another family")
	}
}
