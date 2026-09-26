package backend

import (
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func (fx resultsFixture) linkOwnerTo(t *testing.T, personId int) {
	t.Helper()
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		user := GetUser(tx, fx.owner.Id)
		user.PersonId = personId
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.TxCommit(tx)
	})
}

func entryRoster(tx *vbolt.Tx, entryId int) map[int]int {
	roster := map[int]int{}
	for _, member := range GetEntryMembers(tx, entryId) {
		roster[member.PersonId]++
	}
	return roster
}

func TestMergePeopleMovesActivities(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)
	fx.setResults(t, appearance.Id, []ResultInput{
		{Kind: ResultKindAward, Label: "Alice award", PersonId: intPtr(fx.alice.Id)},
		{Kind: ResultKindAward, Label: "Bob award", PersonId: intPtr(fx.bob.Id)},
	})
	fx.linkOwnerTo(t, fx.alice.Id)

	var resp MergePeopleResponse
	fx.as(t, func(ctx *vbeam.Context) {
		var err error
		resp, err = MergePeople(ctx, MergePeopleRequest{SourcePersonId: fx.alice.Id, TargetPersonId: fx.bob.Id})
		if err != nil {
			t.Fatalf("MergePeople() error = %v", err)
		}
	})

	if resp.MergedEntries != 1 || resp.MergedResults != 1 {
		t.Errorf("merged entries = %d, results = %d, want 1 and 1", resp.MergedEntries, resp.MergedResults)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if roster := entryRoster(tx, fx.entry.Id); len(roster) != 1 || roster[fx.bob.Id] != 1 {
			t.Errorf("shared entry roster = %v, want only Bob once", roster)
		}
		if roster := entryRoster(tx, fx.otherEntry.Id); len(roster) != 1 || roster[fx.bob.Id] != 1 {
			t.Errorf("Alice's other entry roster = %v, want Bob", roster)
		}
		if members := GetPersonEntryMembers(tx, fx.alice.Id); len(members) != 0 {
			t.Errorf("Alice still has %d roster rows", len(members))
		}
		if results := GetPersonResults(tx, fx.alice.Id); len(results) != 0 {
			t.Errorf("Alice still has %d results", len(results))
		}
		if results := GetPersonResults(tx, fx.bob.Id); len(results) != 2 {
			t.Errorf("Bob has %d results, want 2", len(results))
		}
		if user := GetUser(tx, fx.owner.Id); user.PersonId != fx.bob.Id {
			t.Errorf("owner PersonId = %d, want Bob %d", user.PersonId, fx.bob.Id)
		}
	})
}

func TestDeletePersonRemovesEverythingThatNamesThem(t *testing.T) {
	fx := setupResultsFixture(t)
	appearance := fx.newAppearance(t)
	fx.setResults(t, appearance.Id, []ResultInput{
		{Kind: ResultKindAward, Label: "Alice award", PersonId: intPtr(fx.alice.Id)},
		{Kind: ResultKindAward, Label: "Bob award", PersonId: intPtr(fx.bob.Id)},
	})
	fx.linkOwnerTo(t, fx.alice.Id)

	var milestoneId, growthId int
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		milestone, err := AddMilestoneTx(tx, AddMilestoneRequest{
			PersonId: fx.alice.Id, Description: "First steps", Category: "development",
			InputType: "age", AgeYears: intPtr(1), AgeMonths: intPtr(0),
		}, fx.familyId)
		if err != nil {
			t.Fatalf("AddMilestoneTx() error = %v", err)
		}
		milestoneId = milestone.Id
		growth, err := AddGrowthDataTx(tx, AddGrowthDataRequest{
			PersonId: fx.alice.Id, MeasurementType: "height", Value: 80, Unit: "cm",
			InputType: "date", MeasurementDate: stringPtr("2015-03-02"),
		}, fx.familyId)
		if err != nil {
			t.Fatalf("AddGrowthDataTx() error = %v", err)
		}
		growthId = growth.Id
		if _, err := AddRelationTx(tx, Relation{FromId: fx.carol.Id, ToId: fx.alice.Id, Kind: RelationParent}); err != nil {
			t.Fatalf("AddRelationTx() error = %v", err)
		}
		vbolt.TxCommit(tx)
	})

	fx.as(t, func(ctx *vbeam.Context) {
		summary, err := GetPersonDeletionSummary(ctx, PersonDeletionRequest{PersonId: fx.alice.Id})
		if err != nil {
			t.Fatalf("GetPersonDeletionSummary() error = %v", err)
		}
		want := PersonDeletionSummary{
			PersonId: fx.alice.Id, Name: "Alice", Milestones: 1, GrowthRecords: 1,
			ActivityRoles: 2, Results: 1, Relations: 1,
		}
		if summary != want {
			t.Errorf("summary = %+v, want %+v", summary, want)
		}
	})

	fx.as(t, func(ctx *vbeam.Context) {
		resp, err := DeletePerson(ctx, PersonDeletionRequest{PersonId: fx.alice.Id})
		if err != nil {
			t.Fatalf("DeletePerson() error = %v", err)
		}
		if !resp.Success || resp.Deleted.Milestones != 1 {
			t.Errorf("DeletePerson() = %+v", resp)
		}
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if person := GetPersonById(tx, fx.alice.Id); person.Id != 0 {
			t.Errorf("Alice still exists")
		}
		var milestone Milestone
		vbolt.Read(tx, MilestoneBkt, milestoneId, &milestone)
		if milestone.Id != 0 {
			t.Errorf("milestone survived")
		}
		var growth GrowthData
		vbolt.Read(tx, GrowthDataBkt, growthId, &growth)
		if growth.Id != 0 {
			t.Errorf("growth record survived")
		}
		if roster := entryRoster(tx, fx.entry.Id); len(roster) != 1 || roster[fx.bob.Id] != 1 {
			t.Errorf("entry roster = %v, want only Bob", roster)
		}
		if members := GetPersonEntryMembers(tx, fx.alice.Id); len(members) != 0 {
			t.Errorf("Alice still has %d roster rows", len(members))
		}
		results := GetAppearanceResults(tx, appearance.Id)
		if len(results) != 2 {
			t.Fatalf("appearance has %d results, want 2", len(results))
		}
		for _, result := range results {
			if result.PersonId != nil && *result.PersonId == fx.alice.Id {
				t.Errorf("result %q still names Alice", result.Label)
			}
		}
		if rels := GetPersonRelationsTx(tx, fx.carol.Id); len(rels) != 0 {
			t.Errorf("Carol still has %d relations", len(rels))
		}
		if people := GetPersonFamilies(tx, fx.alice.Id); len(people) != 0 {
			t.Errorf("Alice still on %d rosters", len(people))
		}
		if user := GetUser(tx, fx.owner.Id); user.PersonId != 0 {
			t.Errorf("owner PersonId = %d, want 0", user.PersonId)
		}
		if bob := GetPersonById(tx, fx.bob.Id); bob.Id == 0 {
			t.Errorf("Bob was deleted too")
		}
	})
}
