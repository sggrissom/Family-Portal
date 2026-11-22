package backend

import (
	"encoding/json"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func updateAlice(t *testing.T, fx resultsFixture, birthdate string, details BirthDetails) (GetPersonResponse, error) {
	t.Helper()
	return callAs(t, fx, UpdatePerson, UpdatePersonRequest{
		Id: fx.alice.Id, Name: fx.alice.Name, Gender: int(fx.alice.Gender), Birthdate: birthdate, BirthDetails: details,
	})
}

func birthGrowth(t *testing.T, fx resultsFixture, personId int) map[MeasurementType][]GrowthData {
	t.Helper()
	out := map[MeasurementType][]GrowthData{}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		person := GetPersonById(tx, personId)
		for _, kind := range []MeasurementType{Height, Weight} {
			out[kind] = birthRecords(tx, person, kind)
		}
	})
	return out
}

func TestBirthDetailsSeedTheGrowthChartAtBirth(t *testing.T) {
	fx := setupResultsFixture(t)

	resp, err := updateAlice(t, fx, "2014-03-02", BirthDetails{
		BirthTime: stringPtr("04:12"), BirthPlace: stringPtr("  St. Mary's  "),
		BirthWeight: &CheckupValue{Value: 7.25, Unit: "lbs"}, BirthLength: &CheckupValue{Value: 50, Unit: "cm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Person.BirthTime != "04:12" || resp.Person.BirthPlace != "St. Mary's" {
		t.Errorf("person = %q %q", resp.Person.BirthTime, resp.Person.BirthPlace)
	}
	if len(resp.GrowthData) != 2 {
		t.Fatalf("growth = %+v, want a length and a weight", resp.GrowthData)
	}
	birthday := time.Date(2014, 3, 2, 0, 0, 0, 0, time.UTC)
	for _, g := range resp.GrowthData {
		if !g.MeasurementDate.Equal(birthday) || g.CreatedBy != fx.owner.Id {
			t.Errorf("birth record %+v, want on the birthday by the owner", g)
		}
	}

	if _, err := updateAlice(t, fx, "2014-03-02", BirthDetails{BirthWeight: &CheckupValue{Value: 3.3, Unit: "kg"}}); err != nil {
		t.Fatal(err)
	}
	got := birthGrowth(t, fx, fx.alice.Id)
	if len(got[Weight]) != 1 || got[Weight][0].Value != 3.3 || got[Weight][0].Unit != "kg" {
		t.Errorf("weight = %+v, want the one record updated", got[Weight])
	}
	if len(got[Height]) != 1 || got[Height][0].Value != 50 {
		t.Errorf("length = %+v, want it left alone", got[Height])
	}

	// An editor that predates birth details sends none of them.
	resp, err = updateAlice(t, fx, "2014-03-03", BirthDetails{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Person.BirthTime != "04:12" || resp.Person.BirthPlace != "St. Mary's" {
		t.Errorf("an older editor cleared the time or place: %+v", resp.Person)
	}
	got = birthGrowth(t, fx, fx.alice.Id)
	if len(got[Weight]) != 1 || len(got[Height]) != 1 {
		t.Errorf("after correcting the birthday, birth records = %+v; want both moved", got)
	}

	resp, err = updateAlice(t, fx, "2014-03-03", BirthDetails{
		BirthTime: stringPtr(""), BirthPlace: stringPtr(""), BirthWeight: &CheckupValue{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Person.BirthTime != "" || resp.Person.BirthPlace != "" {
		t.Errorf("time and place not cleared: %+v", resp.Person)
	}
	got = birthGrowth(t, fx, fx.alice.Id)
	if len(got[Weight]) != 0 || len(got[Height]) != 1 {
		t.Errorf("after clearing the weight, birth records = %+v", got)
	}
}

func TestBirthDetailsValidation(t *testing.T) {
	fx := setupResultsFixture(t)
	for name, details := range map[string]BirthDetails{
		"time":   {BirthTime: stringPtr("4pm")},
		"unit":   {BirthWeight: &CheckupValue{Value: 7, Unit: "stone"}},
		"length": {BirthLength: &CheckupValue{Value: -2, Unit: "in"}},
	} {
		if _, err := updateAlice(t, fx, "2014-03-02", details); err == nil {
			t.Errorf("%s: saved, want an error", name)
		}
	}
}

func TestAddPersonWithBirthDetails(t *testing.T) {
	fx := setupResultsFixture(t)
	today := time.Now().UTC().Format("2006-01-02")
	details := BirthDetails{
		BirthTime: stringPtr("23:59"), BirthPlace: stringPtr("Home"),
		BirthWeight: &CheckupValue{Value: 8, Unit: "lbs"}, BirthLength: &CheckupValue{Value: 20, Unit: "in"},
	}
	born, err := callAs(t, fx, AddPerson, AddPersonRequest{Name: "Dot", Gender: 1, Birthdate: today, BirthDetails: details})
	if err != nil {
		t.Fatal(err)
	}
	if born.Person.BirthPlace != "Home" {
		t.Errorf("person = %+v", born.Person)
	}
	if got := birthGrowth(t, fx, born.Person.Id); len(got[Weight]) != 1 || len(got[Height]) != 1 || got[Weight][0].CreatedBy != fx.owner.Id {
		t.Errorf("birth records = %+v", got)
	}

	due := time.Now().UTC().AddDate(0, 3, 0).Format("2006-01-02")
	expected, err := callAs(t, fx, AddPerson, AddPersonRequest{Name: "Bump", Gender: 2, Birthdate: due, IsPregnancy: true, BirthDetails: details})
	if err != nil {
		t.Fatal(err)
	}
	if expected.Person.BirthPlace != "" || expected.Person.BirthTime != "" {
		t.Errorf("an unborn baby got birth details: %+v", expected.Person)
	}
	if got := birthGrowth(t, fx, expected.Person.Id); len(got[Weight])+len(got[Height]) != 0 {
		t.Errorf("an unborn baby got birth measurements: %+v", got)
	}
}

func TestBirthDetailsSurviveExportAndImport(t *testing.T) {
	fx := setupResultsFixture(t)
	if _, err := updateAlice(t, fx, "2014-03-02", BirthDetails{
		BirthTime: stringPtr("04:12"), BirthPlace: stringPtr("St. Mary's"), BirthWeight: &CheckupValue{Value: 7.25, Unit: "lbs"},
	}); err != nil {
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

	var other User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		other = AddUserTx(tx, CreateAccountRequest{Name: "Other", Email: "other-birth@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	var resp ImportDataResponse
	fx.asUser(t, other, func(ctx *vbeam.Context) {
		resp, err = ImportData(ctx, ImportDataRequest{JsonData: exported.JsonData, MergeStrategy: "merge_people"})
	})
	if err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		alice := GetPersonById(tx, resp.PersonIdMapping[fx.alice.Id])
		if alice.BirthTime != "04:12" || alice.BirthPlace != "St. Mary's" {
			t.Errorf("imported person = %+v", alice)
		}
		if weights := birthRecords(tx, alice, Weight); len(weights) != 1 {
			t.Errorf("imported birth weights = %+v", weights)
		}
	})
}
