package backend

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vpack"
)

type recordTimeFixture struct {
	Name     string     `json:"name"`
	Version  *int       `json:"version"`
	Record   RecordTime `json:"record"`
	Birthday string     `json:"birthday"`
	Expect   struct {
		Start    string `json:"start"`
		End      string `json:"end"`
		Label    string `json:"label"`
		ExactDay bool   `json:"exactDay"`
	} `json:"expect"`
}

func loadRecordTimeFixtures(t *testing.T) (valid, invalid []recordTimeFixture) {
	t.Helper()
	data, err := os.ReadFile("../frontend/lib/recordTimeFixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Valid   []recordTimeFixture `json:"valid"`
		Invalid []recordTimeFixture `json:"invalid"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	prepare := func(list []recordTimeFixture) {
		for i := range list {
			list[i].Record.Version = RecordTimeVersion
			if list[i].Version != nil {
				list[i].Record.Version = *list[i].Version
			}
			list[i].Record.Source = SourceFamily
		}
	}
	prepare(fixtures.Valid)
	prepare(fixtures.Invalid)
	return fixtures.Valid, fixtures.Invalid
}

func fixtureAnchor(t *testing.T, birthday string) RecordAnchor {
	if birthday == "" {
		return RecordAnchor{}
	}
	born, err := time.Parse(dateOnlyLayout, birthday)
	if err != nil {
		t.Fatal(err)
	}
	return RecordAnchor{Birthday: born, BirthdayKnown: true}
}

func TestRecordTimeFixtures(t *testing.T) {
	valid, invalid := loadRecordTimeFixtures(t)
	for _, f := range valid {
		t.Run(f.Name, func(t *testing.T) {
			rt := f.Record
			if err := validateRecordTime(rt, true); err != nil {
				t.Fatalf("validate: %v", err)
			}
			resolveRecordTime(&rt, fixtureAnchor(t, f.Birthday))
			if rt.Start != f.Expect.Start || rt.End != f.Expect.End {
				t.Errorf("range = [%q, %q), want [%q, %q)", rt.Start, rt.End, f.Expect.Start, f.Expect.End)
			}
			if rt.Resolved != (f.Expect.Start != "") {
				t.Errorf("resolved = %v", rt.Resolved)
			}
			if got := rt.Label(); got != f.Expect.Label {
				t.Errorf("label = %q, want %q", got, f.Expect.Label)
			}
			if rt.ExactDay() != f.Expect.ExactDay {
				t.Errorf("exactDay = %v, want %v", rt.ExactDay(), f.Expect.ExactDay)
			}
		})
	}
	for _, f := range invalid {
		t.Run("invalid "+f.Name, func(t *testing.T) {
			if err := validateRecordTime(f.Record, true); err == nil {
				t.Error("expected a validation error")
			}
		})
	}
}

func TestRecordTimeMediaCannotDescribeAPeriod(t *testing.T) {
	rt := RecordTime{Version: RecordTimeVersion, Meaning: MeaningPeriod, Basis: BasisCalendar,
		Precision: PrecisionYear, From: "1970", Source: SourceFamily}
	if err := validateRecordTime(rt, false); err == nil {
		t.Error("expected media period assertions to be rejected")
	}
}

func TestRecordTimeOverlapIsHalfOpen(t *testing.T) {
	rt := RecordTime{Version: RecordTimeVersion, Meaning: MeaningPeriod, Basis: BasisAge,
		Precision: PrecisionYear, AnchorPersonId: 1, AgeYears: 4, Source: SourceFamily}
	resolveRecordTime(&rt, fixtureAnchor(t, "2020-06-15"))

	cases := []struct {
		start, end string
		want       bool
	}{
		{"2024-06-15", "2024-06-16", true},
		{"2025-06-14", "2025-06-15", true},
		{"2025-06-15", "2025-06-16", false},
		{"2024-06-14", "2024-06-15", false},
		{"2024-01-01", "2026-01-01", true},
	}
	for _, c := range cases {
		if got := rt.Overlaps(c.start, c.end); got != c.want {
			t.Errorf("Overlaps(%s, %s) = %v, want %v", c.start, c.end, got, c.want)
		}
	}

	unresolved := rt
	resolveRecordTime(&unresolved, RecordAnchor{})
	if unresolved.Overlaps("0001-01-01", "9999-01-01") {
		t.Error("an unresolved assertion must not match any dated query")
	}
}

func TestRecordTimeBirthdayCorrectionKeepsTheAge(t *testing.T) {
	rt := RecordTime{Version: RecordTimeVersion, Meaning: MeaningPeriod, Basis: BasisAge,
		Precision: PrecisionYear, AnchorPersonId: 1, AgeYears: 4, Source: SourceFamily}
	resolveRecordTime(&rt, fixtureAnchor(t, "2020-06-15"))
	resolveRecordTime(&rt, fixtureAnchor(t, "2020-07-01"))
	if rt.Start != "2024-07-01" || rt.End != "2025-07-01" || rt.Label() != "At age 4" {
		t.Errorf("got [%s, %s) %q", rt.Start, rt.End, rt.Label())
	}
}

func TestRecordTimeFuture(t *testing.T) {
	today := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	current := RecordTime{Version: RecordTimeVersion, Meaning: MeaningPeriod, Basis: BasisAge,
		Precision: PrecisionYear, AnchorPersonId: 1, AgeYears: 6, Source: SourceFamily}
	resolveRecordTime(&current, fixtureAnchor(t, "2020-06-15"))
	if err := checkRecordTimeNotFuture(current, today); err != nil {
		t.Errorf("a current age may end in the future: %v", err)
	}

	next := current
	next.AgeYears = 7
	resolveRecordTime(&next, fixtureAnchor(t, "2020-06-15"))
	if err := checkRecordTimeNotFuture(next, today); err == nil {
		t.Error("an age that hasn't started yet must be rejected")
	}

	tomorrow := exactRecordTime(today.AddDate(0, 0, 1), SourceFamily)
	if err := checkRecordTimeNotFuture(tomorrow, today); err == nil {
		t.Error("a future day must be rejected")
	}
}

func TestRecordTimeAnchorIgnoresPregnancyDueDate(t *testing.T) {
	anchor := anchorOf(Person{Id: 1, Birthday: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), IsPregnancy: true})
	if anchor.BirthdayKnown {
		t.Error("a due date is not a birthday")
	}
}

func TestRecordTimePackRoundTrip(t *testing.T) {
	rt := RecordTime{Version: RecordTimeVersion, Meaning: MeaningOccurrence, Basis: BasisAge,
		Precision: PrecisionRange, Estimated: true, Around: "8", AnchorPersonId: 7, AgeYears: 6, AgeToYears: 9,
		Source: SourceFamily, Note: "on the back", ChangedBy: 3, ChangedAt: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)}
	resolveRecordTime(&rt, fixtureAnchor(t, "1962-03-01"))

	var out RecordTime
	vpack.FromBytesInto(vpack.ToBytes(&rt, PackRecordTime), &out, PackRecordTime)
	if out != rt {
		t.Errorf("round trip changed the assertion:\n got %+v\nwant %+v", out, rt)
	}
}
