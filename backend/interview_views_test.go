package backend

import (
	"testing"
	"time"
)

func TestInterviewDueAroundBirthdays(t *testing.T) {
	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	child := Person{Birthday: day("2019-06-15")}
	toddler := Person{Birthday: day("2024-06-15")}
	lastYear := []Interview{{Date: day("2024-06-20")}}
	recent := []Interview{{Date: day("2025-01-10")}}

	for name, tc := range map[string]struct {
		person     Person
		interviews []Interview
		today      string
		want       bool
	}{
		"two weeks before":      {child, nil, "2025-06-01", true},
		"three weeks before":    {child, nil, "2025-05-25", false},
		"a month after":         {child, lastYear, "2025-07-14", true},
		"too long after":        {child, nil, "2025-07-20", false},
		"interviewed recently":  {child, recent, "2025-06-10", false},
		"interviewed last year": {child, lastYear, "2025-06-10", true},
		"turning two":           {toddler, nil, "2026-06-10", false},
		"turning three":         {toddler, nil, "2027-06-10", true},
		"new year wrap":         {Person{Birthday: day("2019-01-05")}, nil, "2025-12-28", true},
		"no birthday":           {Person{}, nil, "2025-06-10", false},
	} {
		if got := interviewDue(tc.person, tc.interviews, day(tc.today)); got != tc.want {
			t.Errorf("%s: due = %v, want %v", name, got, tc.want)
		}
	}
}

func TestPersonInterviewsBundleEverythingTheViewsNeed(t *testing.T) {
	fx := setupResultsFixture(t)
	first := fx.startInterview(t, fx.alice, "2020-03-05", "fav-food", "fav-color")
	fx.answer(t, first.Interview.Id, first.Questions[0].Id, "noodles", false)
	fx.finish(t, first.Interview)
	second := fx.startInterview(t, fx.alice, "2019-03-05", "fav-food")
	fx.answer(t, second.Interview.Id, second.Questions[0].Id, "rice", false)

	resp, err := callAs(t, fx, GetPersonInterviews, GetPersonInterviewsRequest{PersonId: fx.alice.Id})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Interviews) != 2 || resp.Interviews[0].Id != second.Interview.Id {
		t.Errorf("interviews = %+v, want both, oldest first", resp.Interviews)
	}
	if len(resp.Answers) != 2 || len(resp.Questions) != 2 {
		t.Errorf("got %d answers and %d questions, want 2 and 2", len(resp.Answers), len(resp.Questions))
	}

	family, err := callAs(t, fx, GetFamilyInterviews, GetFamilyInterviewsRequest{Today: "2020-04-01"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range family.People {
		if row.Person.Id != fx.alice.Id {
			continue
		}
		if row.Count != 1 || row.Last.Id != first.Interview.Id || row.InProgress.Id != second.Interview.Id || row.Answered != 1 {
			t.Errorf("alice's row = %+v", row)
		}
		if !row.Child {
			t.Error("a six-year-old isn't listed as a child")
		}
	}

	outsider := fx.outsiderPerson(t)
	if _, err := callAs(t, fx, GetPersonInterviews, GetPersonInterviewsRequest{PersonId: outsider.Id}); err == nil {
		t.Error("read another family's interviews")
	}
}
