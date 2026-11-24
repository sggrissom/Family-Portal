package backend

import (
	"encoding/json"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestRecordsRememberWhoAddedThem(t *testing.T) {
	fx := setupResultsFixture(t)

	added, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{
		PersonId: fx.alice.Id, Description: "First steps", Category: "first", RecordTime: onDay("2015-01-10"),
	})
	if err != nil {
		t.Fatal(err)
	}
	date := "2015-01-10"
	checkup, err := callAs(t, fx, AddCheckup, AddCheckupRequest{
		PersonId: fx.alice.Id, InputType: "date", MeasurementDate: &date,
		Height: &CheckupValue{Value: 30, Unit: "in"}, Weight: &CheckupValue{Value: 22, Unit: "lbs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	interview := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	answer := fx.answer(t, interview.Interview.Id, interview.Questions[0].Id, "pizza", false)

	for _, id := range []int{added.Milestone.Id, answer.Answer.Id} {
		got, err := callAs(t, fx, GetMilestone, GetMilestoneRequest{Id: id})
		if err != nil {
			t.Fatal(err)
		}
		if got.Milestone.CreatedBy != fx.owner.Id || got.Milestone.RecordedBy != "Owner" {
			t.Errorf("moment %d recorded by %d %q, want the owner", id, got.Milestone.CreatedBy, got.Milestone.RecordedBy)
		}
	}
	for _, record := range checkup.GrowthData {
		got, err := callAs(t, fx, GetGrowthData, GetGrowthDataRequest{Id: record.Id})
		if err != nil {
			t.Fatal(err)
		}
		if got.GrowthData.CreatedBy != fx.owner.Id || got.GrowthData.RecordedBy != "Owner" {
			t.Errorf("growth %d recorded by %d %q, want the owner", record.Id, got.GrowthData.CreatedBy, got.GrowthData.RecordedBy)
		}
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		outsider := AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider@example.com"}, nil)
		if name := recorderName(tx, outsider, fx.familyId, fx.owner.Id); name != "" {
			t.Errorf("a non-member sees recorder %q, want nothing", name)
		}
		if name := recorderName(tx, fx.owner, fx.familyId, 0); name != "" {
			t.Errorf("an unattributed record names %q", name)
		}
	})
}

func TestRecorderSurvivesExportAndImportByName(t *testing.T) {
	fx := setupResultsFixture(t)
	if _, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{
		PersonId: fx.alice.Id, Description: "First steps", Category: "first", RecordTime: onDay("2015-01-10"),
	}); err != nil {
		t.Fatal(err)
	}
	date := "2015-01-10"
	if _, err := callAs(t, fx, AddCheckup, AddCheckupRequest{
		PersonId: fx.alice.Id, InputType: "date", MeasurementDate: &date,
		Height: &CheckupValue{Value: 30, Unit: "in"},
	}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if _, err := AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId: fx.alice.Id, Description: "Unattributed", Category: "first", RecordTime: onDay("2015-02-01"),
		}, fx.familyId); err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})

	exported, err := callAs(t, fx, ExportData, ExportDataRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var data ExportDataStructure
	if err := json.Unmarshal([]byte(exported.JsonData), &data); err != nil {
		t.Fatal(err)
	}
	byText := map[string]string{}
	for _, m := range data.Milestones {
		byText[m.Description] = m.RecordedBy
	}
	if byText["First steps"] != "Owner" || byText["Unattributed"] != "" {
		t.Errorf("exported recorders = %v", byText)
	}
	if len(data.Heights) != 1 || data.Heights[0].RecordedBy != "Owner" {
		t.Errorf("exported heights = %+v", data.Heights)
	}

	var other, namesake User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		other = AddUserTx(tx, CreateAccountRequest{Name: "Other", Email: "other-recorder@example.com"}, nil)
		namesake = AddUserTx(tx, CreateAccountRequest{Name: "owner", Email: "namesake@example.com"}, nil)
		EnsureMembershipTx(tx, namesake.Id, other.FamilyId, AccessContribute)
		vbolt.TxCommit(tx)
	})
	var resp ImportDataResponse
	fx.asUser(t, other, func(ctx *vbeam.Context) {
		resp, err = ImportData(ctx, ImportDataRequest{JsonData: exported.JsonData, ImportMilestones: true})
	})
	if err != nil {
		t.Fatal(err)
	}
	aliceThere := resp.PersonIdMapping[fx.alice.Id]
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		for _, m := range GetPersonMilestonesTx(tx, aliceThere) {
			want := namesake.Id
			if m.Description == "Unattributed" {
				want = 0
			}
			if m.CreatedBy != want {
				t.Errorf("imported %q recorded by %d, want %d", m.Description, m.CreatedBy, want)
			}
		}
		growth := GetPersonGrowthDataTx(tx, aliceThere)
		if len(growth) != 1 || growth[0].CreatedBy != namesake.Id {
			t.Errorf("imported growth = %+v, want it attributed to the namesake", growth)
		}
	})
}
