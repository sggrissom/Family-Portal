package backend

import (
	"slices"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) addTag(t *testing.T, familyId int, name string) Tag {
	t.Helper()
	var tag Tag
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		tag = Tag{Id: vbolt.NextIntId(tx, TagBkt), FamilyId: familyId, Name: name, CreatedAt: time.Now()}
		vbolt.Write(tx, TagBkt, tag.Id, &tag)
		vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, tag.Id, familyId)
		vbolt.TxCommit(tx)
	})
	return tag
}

func (fx resultsFixture) outsiderPerson(t *testing.T) Person {
	t.Helper()
	var person Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		outsider := AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: "outsider-checkup@example.com"}, nil)
		var err error
		person, err = AddPersonTx(tx, AddPersonRequest{Name: "Stranger", Birthdate: "2015-01-01"}, outsider.FamilyId)
		if err != nil {
			t.Fatal(err)
		}
		vbolt.TxCommit(tx)
	})
	return person
}

func TestAddCheckupSavesAllOrNothing(t *testing.T) {
	fx := setupResultsFixture(t)
	count := func() int {
		var n int
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { n = len(GetPersonGrowthDataTx(tx, fx.alice.Id)) })
		return n
	}

	resp, err := callAs(t, fx, AddCheckup, AddCheckupRequest{PersonId: fx.alice.Id, InputType: "today", Height: &CheckupValue{40, "in"}})
	if err != nil || len(resp.GrowthData) != 1 || resp.GrowthData[0].MeasurementType != Height {
		t.Fatalf("height only: %+v, %v", resp, err)
	}
	resp, err = callAs(t, fx, AddCheckup, AddCheckupRequest{PersonId: fx.alice.Id, InputType: "today", Weight: &CheckupValue{30, "kg"}})
	if err != nil || len(resp.GrowthData) != 1 || resp.GrowthData[0].MeasurementType != Weight || resp.GrowthData[0].Unit != "kg" {
		t.Fatalf("weight only: %+v, %v", resp, err)
	}
	resp, err = callAs(t, fx, AddCheckup, AddCheckupRequest{PersonId: fx.alice.Id, InputType: "today", Height: &CheckupValue{41, "in"}, Weight: &CheckupValue{31, "lbs"}})
	if err != nil || len(resp.GrowthData) != 2 {
		t.Fatalf("both: %+v, %v", resp, err)
	}
	if !resp.GrowthData[0].MeasurementDate.Equal(resp.GrowthData[1].MeasurementDate) {
		t.Fatalf("one checkup got two dates: %+v", resp.GrowthData)
	}
	if count() != 4 {
		t.Fatalf("stored %d records, want 4", count())
	}

	for name, req := range map[string]AddCheckupRequest{
		"empty":                {PersonId: fx.alice.Id, InputType: "today"},
		"invalid second":       {PersonId: fx.alice.Id, InputType: "today", Height: &CheckupValue{42, "in"}, Weight: &CheckupValue{31, "stone"}},
		"invalid date":         {PersonId: fx.alice.Id, InputType: "date", Height: &CheckupValue{42, "in"}, Weight: &CheckupValue{31, "lbs"}},
		"another family's kid": {PersonId: fx.outsiderPerson(t).Id, InputType: "today", Height: &CheckupValue{42, "in"}},
	} {
		if _, err := callAs(t, fx, AddCheckup, req); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if count() != 4 {
		t.Fatalf("a rejected checkup saved something: %d records", count())
	}

	single, err := callAs(t, fx, AddGrowthData, AddGrowthDataRequest{PersonId: fx.alice.Id, MeasurementType: "height", Value: 43, Unit: "cm", InputType: "today"})
	if err != nil || single.GrowthData.Unit != "cm" || count() != 5 {
		t.Fatalf("single-measurement API: %+v, %v", single, err)
	}
}

func TestMilestoneSavesTagsInTheSameTransaction(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalMilestoneWorker
	globalMilestoneWorker = newBacklogWorker("atomic test", func(int) error { return nil })
	t.Cleanup(func() { globalMilestoneWorker = previous })

	first := fx.addTag(t, fx.familyId, "first")
	funny := fx.addTag(t, fx.familyId, "funny")
	foreign := fx.addTag(t, fx.familyId+999, "theirs")
	tagsOf := func(id int) (tags []int) {
		vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { tags = GetMilestoneTagIds(tx, id) })
		slices.Sort(tags)
		return
	}
	drain := func() {
		for globalMilestoneWorker.length() > 0 {
			globalMilestoneWorker.pop()
		}
	}

	plain, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{PersonId: fx.alice.Id, Description: "Clapped", Category: "first", InputType: "today"})
	if err != nil || len(tagsOf(plain.Milestone.Id)) != 0 {
		t.Fatalf("omitted tags on add: %+v, %v", plain, err)
	}
	tagged, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{PersonId: fx.alice.Id, Description: "Laughed", Category: "first", InputType: "today", TagIds: []int{first.Id, funny.Id, first.Id}})
	if err != nil || !slices.Equal(tagsOf(tagged.Milestone.Id), []int{first.Id, funny.Id}) || !slices.Equal(tagged.Milestone.TagIds, tagsOf(tagged.Milestone.Id)) {
		t.Fatalf("tags on add: %+v, %v", tagged, err)
	}
	drain()

	if _, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{PersonId: fx.alice.Id, Description: "Leaked", Category: "first", InputType: "today", TagIds: []int{first.Id, foreign.Id}}); err == nil {
		t.Fatal("add accepted another family's tag")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if found := SearchMilestonesTx(tx, "leaked", fx.familyId, 10); len(found) != 0 {
			t.Fatalf("rejected add left a milestone: %+v", found)
		}
	})

	update := UpdateMilestoneRequest{Id: tagged.Milestone.Id, Description: "Laughed loudly", Category: "first", InputType: "today"}
	if _, err := callAs(t, fx, UpdateMilestone, update); err != nil || !slices.Equal(tagsOf(tagged.Milestone.Id), []int{first.Id, funny.Id}) {
		t.Fatalf("omitted tags on update changed them: %v, %v", tagsOf(tagged.Milestone.Id), err)
	}
	update.TagIds = []int{funny.Id}
	if _, err := callAs(t, fx, UpdateMilestone, update); err != nil || !slices.Equal(tagsOf(tagged.Milestone.Id), []int{funny.Id}) {
		t.Fatalf("replacing tags on update: %v, %v", tagsOf(tagged.Milestone.Id), err)
	}
	drain()

	bad := update
	bad.Description = "Should not stick"
	bad.TagIds = []int{first.Id, foreign.Id}
	if _, err := callAs(t, fx, UpdateMilestone, bad); err == nil {
		t.Fatal("update accepted another family's tag")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetMilestoneById(tx, tagged.Milestone.Id); got.Description != "Laughed loudly" {
			t.Fatalf("rejected update changed the milestone: %+v", got)
		}
	})
	if !slices.Equal(tagsOf(tagged.Milestone.Id), []int{funny.Id}) || globalMilestoneWorker.length() != 0 {
		t.Fatalf("rejected update left partial changes: tags %v, queued %d", tagsOf(tagged.Milestone.Id), globalMilestoneWorker.length())
	}

	update.TagIds = []int{}
	if _, err := callAs(t, fx, UpdateMilestone, update); err != nil || len(tagsOf(tagged.Milestone.Id)) != 0 {
		t.Fatalf("empty tags on update should clear: %v, %v", tagsOf(tagged.Milestone.Id), err)
	}

	if _, err := callAs(t, fx, UpdateMilestoneTags, UpdateMilestoneTagsRequest{MilestoneId: plain.Milestone.Id, TagIds: []int{first.Id}}); err != nil || !slices.Equal(tagsOf(plain.Milestone.Id), []int{first.Id}) {
		t.Fatalf("UpdateMilestoneTags: %v, %v", tagsOf(plain.Milestone.Id), err)
	}
	if _, err := callAs(t, fx, UpdateMilestoneTags, UpdateMilestoneTagsRequest{MilestoneId: plain.Milestone.Id, TagIds: []int{foreign.Id}}); err == nil || !slices.Equal(tagsOf(plain.Milestone.Id), []int{first.Id}) {
		t.Fatalf("UpdateMilestoneTags accepted another family's tag: %v", tagsOf(plain.Milestone.Id))
	}
}
