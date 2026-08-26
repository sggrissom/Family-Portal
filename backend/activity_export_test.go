package backend

import (
	"encoding/json"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestExportCarriesTheWholeActivityTree(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		bundle, err = buildExportData(tx, fx.famA)
		if err != nil {
			t.Fatalf("buildExportData: %v", err)
		}
	})

	if len(bundle.Activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(bundle.Activities))
	}
	activity := bundle.Activities[0]
	if activity.Name != "Dance" || activity.Kind != ActivityKindDance {
		t.Errorf("activity came through wrong: %+v", activity)
	}

	if len(activity.Seasons) != 1 {
		t.Fatalf("expected 1 season, got %d", len(activity.Seasons))
	}
	season := activity.Seasons[0]
	if season.Name != "2025-26" {
		t.Errorf("season name came through as %q", season.Name)
	}
	if len(season.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(season.Entries))
	}
	if len(season.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(season.Events))
	}

	event := season.Events[0]
	if len(event.Appearances) != 2 {
		t.Fatalf("expected 2 appearances, got %d", len(event.Appearances))
	}

	var results []ExportResult
	for _, appearance := range event.Appearances {
		if appearance.EntryId == fx.groupEntry.Id {
			results = appearance.Results
			if appearance.EntryName != "Rise Up" {
				t.Errorf("appearance names its entry as %q", appearance.EntryName)
			}
		}
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results on the group appearance, got %d", len(results))
	}

	if results[0].SortOrder > results[1].SortOrder {
		t.Errorf("results came out unsorted: %d then %d", results[0].SortOrder, results[1].SortOrder)
	}

	if bundle.TotalActivities != 1 || bundle.TotalSeasons != 1 || bundle.TotalEvents != 1 {
		t.Errorf("top-level totals wrong: %+v", bundle)
	}
	if bundle.TotalEntries != 2 || bundle.TotalAppearances != 2 || bundle.TotalResults != 2 {
		t.Errorf("nested totals wrong: entries=%d appearances=%d results=%d",
			bundle.TotalEntries, bundle.TotalAppearances, bundle.TotalResults)
	}
}

func TestExportedResultsKeepTheirOptionalFieldsOptional(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		placement := Result{
			Id: vbolt.NextIntId(tx, ResultBkt), AppearanceId: fx.soloAppr.Id, FamilyId: fx.famA,
			Kind: ResultKindPlacement, Rank: intPtr(2), OutOf: intPtr(14),
			Category: "Mini Solo Jazz", CreatedAt: fx.soloAppr.CreatedAt,
		}
		writeResultTx(tx, &placement)
		vbolt.TxCommit(tx)
	})

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.famA)
	})

	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ExportDataStructure
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var placement, adjudication *ExportResult
	for i, appearance := range decoded.Activities[0].Seasons[0].Events[0].Appearances {
		for j, result := range appearance.Results {
			row := &decoded.Activities[0].Seasons[0].Events[0].Appearances[i].Results[j]
			switch result.Kind {
			case ResultKindPlacement:
				placement = row
			case ResultKindAdjudication:
				adjudication = row
			}
		}
	}

	if placement == nil {
		t.Fatal("placement result did not survive the round trip")
	}
	if placement.Rank == nil || *placement.Rank != 2 {
		t.Errorf("rank came back as %v", placement.Rank)
	}
	if placement.OutOf == nil || *placement.OutOf != 14 {
		t.Errorf("outOf came back as %v", placement.OutOf)
	}

	if adjudication == nil {
		t.Fatal("adjudication result did not survive the round trip")
	}
	if adjudication.Rank != nil || adjudication.OutOf != nil || adjudication.Score != nil {
		t.Errorf("adjudication picked up numeric fields it never had: %+v", *adjudication)
	}
}

func TestExportedActivitiesNamePeople(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.famA)
	})

	season := bundle.Activities[0].Seasons[0]
	var group *ExportEntry
	for i := range season.Entries {
		if season.Entries[i].Id == fx.groupEntry.Id {
			group = &season.Entries[i]
		}
	}
	if group == nil {
		t.Fatal("group entry missing from the bundle")
	}
	if len(group.PersonIds) != 2 || len(group.PersonNames) != 2 {
		t.Errorf("roster came through as ids=%v names=%v", group.PersonIds, group.PersonNames)
	}

	var award *ExportResult
	for _, appearance := range season.Events[0].Appearances {
		for i, result := range appearance.Results {
			if result.Kind == ResultKindAward {
				award = &appearance.Results[i]
			}
		}
	}
	if award == nil {
		t.Fatal("award result missing from the bundle")
	}
	if award.PersonId == nil || *award.PersonId != fx.alice.Id {
		t.Fatalf("award lost the person it narrows to: %v", award.PersonId)
	}
	if award.PersonName != fx.alice.Name {
		t.Errorf("award names %q, expected %q", award.PersonName, fx.alice.Name)
	}
}

func TestExportCarriesActivityPhotoJoins(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.famA)
	})

	event := bundle.Activities[0].Seasons[0].Events[0]
	if len(event.PhotoIds) != 1 || event.PhotoIds[0] != fx.untaggedPhoto.Id {
		t.Errorf("event photo ids came through as %v", event.PhotoIds)
	}

	var groupPhotos []int
	for _, appearance := range event.Appearances {
		if appearance.EntryId == fx.groupEntry.Id {
			groupPhotos = appearance.PhotoIds
		}
	}
	if len(groupPhotos) != 1 || groupPhotos[0] != fx.alicePhoto.Id {
		t.Errorf("appearance photo ids came through as %v", groupPhotos)
	}
}

func TestExportActivitiesStayWithTheirFamily(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.famB)
	})

	if len(bundle.Activities) != 0 {
		t.Errorf("family B's bundle carries %d activities from family A", len(bundle.Activities))
	}
	if bundle.TotalResults != 0 {
		t.Errorf("family B's bundle counts %d results", bundle.TotalResults)
	}
}

func TestEmptyActivitiesAreOmittedFromTheBundle(t *testing.T) {
	fx, cleanup := setupFamilyLinkFixture(t)
	defer cleanup()

	var bundle ExportDataStructure
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, _ = buildExportData(tx, fx.famA)
	})

	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"activities", "total_activities", "total_results"} {
		if _, present := generic[key]; present {
			t.Errorf("empty bundle carries %q", key)
		}
	}
}
