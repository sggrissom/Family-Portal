package backend

import (
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestResolveEntryDate(t *testing.T) {
	birthday := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		inputType string
		date      *string
		years     *int
		months    *int
		birthday  time.Time
		want      time.Time
		wantErr   string
	}{
		{name: "date", inputType: "date", date: stringPtr("2023-06-15"), want: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)},
		{name: "missing date", inputType: "date", wantErr: "Milestone date is required"},
		{name: "empty date", inputType: "date", date: stringPtr(""), wantErr: "Milestone date is required"},
		{name: "invalid date", inputType: "date", date: stringPtr("2023-13-45"), wantErr: "parsing time"},
		{name: "age years and months", inputType: "age", years: intPtr(2), months: intPtr(6), want: time.Date(2022, 12, 15, 0, 0, 0, 0, time.UTC)},
		{name: "age months default to zero", inputType: "age", years: intPtr(3), want: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)},
		{name: "age zero is the birthday", inputType: "age", years: intPtr(0), months: intPtr(0), want: birthday},
		{name: "age month overflow follows AddDate", inputType: "age", years: intPtr(0), months: intPtr(1), birthday: time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC), want: time.Date(2021, 3, 3, 0, 0, 0, 0, time.UTC)},
		{name: "missing years", inputType: "age", months: intPtr(3), wantErr: "Age years must be non-negative"},
		{name: "negative years", inputType: "age", years: intPtr(-1), months: intPtr(0), wantErr: "Age years must be non-negative"},
		{name: "negative months", inputType: "age", years: intPtr(1), months: intPtr(-1), wantErr: "Age months must be between 0 and 11"},
		{name: "months above 11", inputType: "age", years: intPtr(1), months: intPtr(12), wantErr: "Age months must be between 0 and 11"},
		{name: "unknown input type", inputType: "later", wantErr: "Input type must be"},
		{name: "empty input type", inputType: "", wantErr: "Input type must be"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			from := birthday
			if !test.birthday.IsZero() {
				from = test.birthday
			}
			got, err := resolveEntryDate("Milestone", test.inputType, test.date, test.years, test.months, from)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("err = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || !got.Equal(test.want) {
				t.Fatalf("got %v, %v; want %v", got, err, test.want)
			}
		})
	}

	before := time.Now()
	got, err := resolveEntryDate("Measurement", "today", nil, nil, nil, birthday)
	if err != nil || got.Before(before) || got.After(time.Now()) {
		t.Fatalf("today = %v, %v", got, err)
	}
	if _, err := resolveEntryDate("Measurement", "date", nil, nil, nil, birthday); err == nil || !strings.HasPrefix(err.Error(), "Measurement date is required") {
		t.Fatalf("measurement missing-date error = %v", err)
	}
}

func TestEntryProcsResolveDatesOnAddAndUpdate(t *testing.T) {
	fx := setupResultsFixture(t)
	aliceAt := func(years, months int) time.Time { return fx.alice.Birthday.AddDate(years, months, 0) }

	if _, err := callAs(t, fx, AddGrowthData, AddGrowthDataRequest{MeasurementType: "height", Value: 40, Unit: "in", InputType: "today"}); err == nil || err.Error() != "Person ID is required" {
		t.Fatalf("growth add without person: %v", err)
	}
	if _, err := callAs(t, fx, UpdateGrowthData, UpdateGrowthDataRequest{MeasurementType: "height", Value: 40, Unit: "in", InputType: "today"}); err == nil || err.Error() != "Growth data ID is required" {
		t.Fatalf("growth update without id: %v", err)
	}
	if _, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{Description: "Waved", Category: "first", InputType: "today"}); err == nil || err.Error() != "Person ID is required" {
		t.Fatalf("milestone add without person: %v", err)
	}
	if _, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{Description: "Waved", Category: "first", InputType: "today"}); err == nil || err.Error() != "Milestone ID is required" {
		t.Fatalf("milestone update without id: %v", err)
	}

	growth, err := callAs(t, fx, AddGrowthData, AddGrowthDataRequest{PersonId: fx.alice.Id, MeasurementType: "height", Value: 40, Unit: "in", InputType: "age", AgeYears: intPtr(2)})
	if err != nil || !growth.GrowthData.MeasurementDate.Equal(aliceAt(2, 0)) {
		t.Fatalf("growth add by age: %+v, %v", growth, err)
	}
	updated, err := callAs(t, fx, UpdateGrowthData, UpdateGrowthDataRequest{Id: growth.GrowthData.Id, MeasurementType: "weight", Value: 30, Unit: "lbs", InputType: "age", AgeYears: intPtr(3), AgeMonths: intPtr(4)})
	if err != nil || !updated.GrowthData.MeasurementDate.Equal(aliceAt(3, 4)) || updated.GrowthData.MeasurementType != Weight {
		t.Fatalf("growth update by age: %+v, %v", updated, err)
	}
	if _, err := callAs(t, fx, UpdateGrowthData, UpdateGrowthDataRequest{Id: growth.GrowthData.Id, MeasurementType: "weight", Value: 30, Unit: "lbs", InputType: "age", AgeYears: intPtr(3), AgeMonths: intPtr(12)}); err == nil {
		t.Fatal("growth update accepted month 12")
	}

	milestone, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{PersonId: fx.alice.Id, Description: "Waved", Category: "first", InputType: "age", AgeYears: intPtr(1)})
	if err != nil || !milestone.Milestone.MilestoneDate.Equal(aliceAt(1, 0)) {
		t.Fatalf("milestone add by age: %+v, %v", milestone, err)
	}
	day := "2016-01-02"
	edited, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{Id: milestone.Milestone.Id, Description: "Waved bye", Category: "first", InputType: "date", MilestoneDate: &day})
	if err != nil || edited.Milestone.MilestoneDate.Format("2006-01-02") != day {
		t.Fatalf("milestone update by date: %+v, %v", edited, err)
	}
	if _, err := callAs(t, fx, UpdateMilestone, UpdateMilestoneRequest{Id: milestone.Milestone.Id, Description: "Waved bye", Category: "first", InputType: "date"}); err == nil || err.Error() != "Milestone date is required when input type is 'date'" {
		t.Fatalf("milestone update without date: %v", err)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetGrowthDataById(tx, growth.GrowthData.Id); !got.MeasurementDate.Equal(aliceAt(3, 4)) {
			t.Fatalf("rejected growth update changed the record: %+v", got)
		}
		if got := GetMilestoneById(tx, milestone.Milestone.Id); got.Description != "Waved bye" {
			t.Fatalf("rejected milestone update changed the record: %+v", got)
		}
	})
}
