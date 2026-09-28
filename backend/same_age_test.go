package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestSameAgeStep(t *testing.T) {
	for age, want := range map[int]int{0: 1, 23: 1, 24: 3, 71: 3, 72: 6, 200: 6} {
		if got := sameAgeStep(age); got != want {
			t.Errorf("sameAgeStep(%d) = %d, want %d", age, got, want)
		}
	}
	if got, want := sameAgeRecordWindow(40), monthsDuration(1.5); got != want {
		t.Errorf("sameAgeRecordWindow(40) = %v, want %v", got, want)
	}
	if got, want := sameAgeGrowthWindow(10), monthsDuration(1); got != want {
		t.Errorf("sameAgeGrowthWindow(10) = %v, want %v", got, want)
	}
}

func TestSortSameAgeRows(t *testing.T) {
	older := SameAgeRow{Person: Person{Id: 1, Birthday: day("2015-01-01")}, PhotoIds: []int{7}}
	younger := SameAgeRow{Person: Person{Id: 2, Birthday: day("2018-01-01")}, PhotoIds: []int{8}}
	empty := SameAgeRow{Person: Person{Id: 3, Birthday: day("2019-01-01")}}
	from := SameAgeRow{Person: Person{Id: 4, Birthday: day("2021-01-01")}}

	rows := []SameAgeRow{empty, older, from, younger}
	sortSameAgeRows(rows, 4)

	var ids []int
	for _, row := range rows {
		ids = append(ids, row.Person.Id)
	}
	want := []int{4, 2, 1, 3}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
}

func months(n int) *int { return &n }

func TestGetSameAge(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	call := func(user User, req GetSameAgeRequest) (GetSameAgeResponse, error) {
		t.Helper()
		token, err := generateJwtTokenString(user)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		var resp GetSameAgeResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = GetSameAge(&vbeam.Context{Tx: tx, Token: token}, req)
		})
		return resp, err
	}

	// The fixture child, born 2020-06-15, was 43 months old on 2024-01-15:
	// measured 2024-01-05 and a milestone on 2024-02-10.
	resp, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(43), Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge() error = %v", err)
	}
	if resp.FromPersonId != fx.person.Id {
		t.Errorf("FromPersonId = %d, want the fixture child %d", resp.FromPersonId, fx.person.Id)
	}
	if len(resp.Rows) == 0 || resp.Rows[0].Person.Id != fx.person.Id {
		t.Fatalf("Rows = %+v, want the fixture child first", resp.Rows)
	}
	row := resp.Rows[0]
	if row.Height == nil || row.Height.Id != fx.growth.Id {
		t.Errorf("Height = %+v, want the 2024-01-05 measurement", row.Height)
	}
	if len(row.Milestones) != 1 || row.Milestones[0].Id != fx.milestone.Id {
		t.Errorf("Milestones = %+v, want the 2024-02-10 milestone", row.Milestones)
	}
	if !row.Date.Equal(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Date = %v, want 2024-01-15", row.Date)
	}

	defaulted, err := call(fx.owner, GetSameAgeRequest{Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge(default) error = %v", err)
	}
	if defaulted.AgeMonths != 75 || defaulted.FromPersonId != fx.person.Id {
		t.Errorf("default = %d months from %d, want 75 months from the fixture child",
			defaulted.AgeMonths, defaulted.FromPersonId)
	}

	future, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(120), Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge(120) error = %v", err)
	}
	for _, r := range future.Rows {
		if r.Person.Id == fx.person.Id {
			t.Error("a child who has not reached the age should not get a row")
		}
	}

	outsider, err := call(fx.outsider, GetSameAgeRequest{AgeMonths: months(43), Today: "2026-09-27"})
	if err != nil {
		t.Fatalf("GetSameAge(outsider) error = %v", err)
	}
	for _, r := range outsider.Rows {
		if r.Person.Id == fx.person.Id {
			t.Error("an outsider got a row for another family's child")
		}
	}

	newborn, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(0), Today: "2026-09-27"})
	if err != nil || newborn.AgeMonths != 0 {
		t.Errorf("an explicit age of zero = %d months (%v), want 0", newborn.AgeMonths, err)
	}

	if _, err := call(fx.owner, GetSameAgeRequest{AgeMonths: months(-1)}); err != ErrInvalidAge {
		t.Errorf("negative age error = %v, want ErrInvalidAge", err)
	}
}
