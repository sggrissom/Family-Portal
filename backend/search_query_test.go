package backend

import (
	"reflect"
	"testing"
	"time"
)

func TestParseSearchQuery(t *testing.T) {
	people := []Person{
		{Id: 1, Name: "Ada Grissom", Relationship: "daughter"},
		{Id: 2, Name: "Ben Grissom", Relationship: "son"},
		{Id: 3, Name: "Ada Smith", Relationship: "niece"},
		{Id: 4, Name: "Mary Jones", Relationship: "grandmother"},
		{Id: 5, Name: "Ruth Jones", Relationship: "grandmother"},
		{Id: 6, Name: "Joe Grissom", Relationship: "father"},
		{Id: 7, Name: "Zoë Martin"},
		{Id: 8, Name: "Baby Grissom", IsPregnancy: true},
		{Id: 9, Name: "May Lee"},
	}
	tags := []Tag{
		{Id: 10, Name: "Beach"},
		{Id: 11, Name: "First day of school"},
		{Id: 12, Name: "Disney World"},
		{Id: 13, Name: "Ben"},
	}
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	type want struct {
		people   []int
		groups   [][]int
		age      []int
		ageLabel string
		year     int
		month    int
		category string
		tags     []int
		text     string
	}
	cases := []struct {
		query  string
		ignore []string
		want   want
	}{
		{query: "", want: want{}},
		{query: "first steps", want: want{text: "first steps"}},
		{query: "walk", want: want{text: "walk"}},

		{query: "ben", want: want{people: []int{2}, groups: [][]int{{2}}}},
		{query: "Ben's first steps", want: want{people: []int{2}, groups: [][]int{{2}}, text: "first steps"}},
		{query: "ben grissom", want: want{people: []int{2}, groups: [][]int{{2}}}},
		{query: "ada", want: want{people: []int{1, 3}, groups: [][]int{{1, 3}}}},
		{query: "ada grissom", want: want{people: []int{1}, groups: [][]int{{1}}}},
		{query: "ada and ben", want: want{people: []int{1, 3, 2}, groups: [][]int{{1, 3}, {2}}}},
		{query: "zoe", want: want{people: []int{7}, groups: [][]int{{7}}}},
		{query: "Zoë's bike", want: want{people: []int{7}, groups: [][]int{{7}}, text: "bike"}},
		{query: "baby grissom", want: want{age: []int{0, 12}, ageLabel: "baby", text: "grissom"}},

		{query: "dad", want: want{people: []int{6}, groups: [][]int{{6}}}},
		{query: "daddy's birthday", want: want{people: []int{6}, groups: [][]int{{6}}, text: "birthday"}},
		{query: "daughter", want: want{people: []int{1}, groups: [][]int{{1}}}},
		{query: "grandma", want: want{text: "grandma"}},

		{query: "ben at 2", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2"}},
		{query: "ben 2", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2"}},
		{query: "ben age 2", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2"}},
		{query: "ben 2 years", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2"}},
		{query: "ben 2yo", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2"}},
		{query: "ben 2 years old", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2"}},
		{query: "ben when he was 3 years old", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{36, 48}, ageLabel: "age 3", text: "he"}},
		{query: "ben 18 months", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{18, 19}, ageLabel: "18 months"}},
		{query: "ben 18mo", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{18, 19}, ageLabel: "18 months"}},
		{query: "ben 18m", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{18, 19}, ageLabel: "18 months"}},
		{query: "1 month", want: want{age: []int{1, 2}, ageLabel: "1 month"}},
		{query: "newborn ben", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{0, 1}, ageLabel: "newborn"}},
		{query: "baby bath", want: want{age: []int{0, 12}, ageLabel: "baby", text: "bath"}},
		{query: "age 4 party", want: want{age: []int{48, 60}, ageLabel: "age 4", text: "party"}},
		{query: "3 years", want: want{age: []int{36, 48}, ageLabel: "age 3"}},

		{query: "2", want: want{text: "2"}},
		{query: "at 2", want: want{text: "2"}},
		{query: "25 pumpkins", want: want{text: "25 pumpkins"}},
		{query: "ben 25 pumpkins", want: want{people: []int{2}, groups: [][]int{{2}}, text: "25 pumpkins"}},
		{query: "ben 2 3", want: want{people: []int{2}, groups: [][]int{{2}}, age: []int{24, 36}, ageLabel: "age 2", text: "3"}},

		{query: "beach 2024", want: want{tags: []int{10}, year: 2024}},
		{query: "1899", want: want{text: "1899"}},
		{query: "ben 2024", want: want{people: []int{2}, groups: [][]int{{2}}, year: 2024}},
		{query: "june", want: want{month: 6}},
		{query: "june 2024", want: want{month: 6, year: 2024}},
		{query: "Christmas December 2023", want: want{month: 12, year: 2023, text: "Christmas"}},
		{query: "this year", want: want{year: 2026}},
		{query: "last year", want: want{year: 2025}},
		{query: "may", want: want{people: []int{9}, groups: [][]int{{9}}}},

		{query: "quote", want: want{category: "quote"}},
		{query: "ben quotes", want: want{people: []int{2}, groups: [][]int{{2}}, category: "quote"}},
		{query: "drawing of a dog", want: want{category: "artwork", text: "dog"}},
		{query: "interview answers", want: want{category: "interview", text: "answers"}},

		{query: "first day of school", want: want{tags: []int{11}}},
		{query: "disney world 2023", want: want{tags: []int{12}, year: 2023}},
		{query: "disney", want: want{text: "disney"}},
		{query: "photos of the beach", want: want{tags: []int{10}}},

		{query: "ben at 2", ignore: []string{"age"}, want: want{people: []int{2}, groups: [][]int{{2}}, text: "2"}},
		{query: "ben at 2", ignore: []string{"person"}, want: want{tags: []int{13}, text: "2"}},
		{query: "ben at 2", ignore: []string{"person", "tag"}, want: want{text: "ben 2"}},
		{query: "beach 2024", ignore: []string{"date"}, want: want{tags: []int{10}, text: "2024"}},
		{query: "beach 2024", ignore: []string{"tag"}, want: want{year: 2024, text: "beach"}},
		{query: "quotes", ignore: []string{"category"}, want: want{text: "quotes"}},
	}

	for _, c := range cases {
		got := parseSearchQuery(c.query, people, tags, now, c.ignore)
		w := c.want
		if w.people == nil {
			w.people = []int{}
		}
		if w.tags == nil {
			w.tags = []int{}
		}
		var age []int
		if got.AgeFromMonth != nil {
			age = []int{*got.AgeFromMonth, *got.AgeToMonth}
		}
		gotWant := want{
			people: got.PersonIds, groups: got.personGroups, age: age, ageLabel: got.AgeLabel,
			year: got.Year, month: got.Month, category: got.Category, tags: got.TagIds, text: got.Text,
		}
		if !reflect.DeepEqual(gotWant, w) {
			t.Errorf("%q ignore=%v:\n got  %+v\n want %+v", c.query, c.ignore, gotWant, w)
		}
	}
}

func TestFoldText(t *testing.T) {
	if got := foldText("Café ZOË Ñandú"); got != "cafe zoe nandu" {
		t.Errorf("foldText = %q", got)
	}
}
