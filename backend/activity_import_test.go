package backend

import (
	"testing"

	"go.hasen.dev/vbolt"
)

func exportFamilyA(t *testing.T, fx activityFixture) ([]ExportActivity, map[int]int) {
	t.Helper()

	var activities []ExportActivity
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		bundle, err := buildExportData(tx, fx.famA)
		if err != nil {
			t.Fatalf("buildExportData: %v", err)
		}
		activities = bundle.Activities
	})
	if len(activities) == 0 {
		t.Fatal("fixture exported no activities")
	}

	return activities, map[int]int{
		fx.alice.Id: fx.alice.Id,
		fx.bob.Id:   fx.bob.Id,
	}
}

func TestActivityImportRestoresTheTreeIntoAnotherFamily(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, personIdMapping := exportFamilyA(t, fx)

	var counts ActivityImportCounts
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		counts, _ = importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})

	if counts.Activities != 1 || counts.Seasons != 1 || counts.Events != 1 {
		t.Errorf("top levels came back as %+v", counts)
	}
	if counts.Entries != 2 || counts.Appearances != 2 || counts.Results != 2 {
		t.Errorf("nested levels came back as %+v", counts)
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		imported := GetFamilyActivities(tx, fx.famB)
		if len(imported) != 1 {
			t.Fatalf("family B has %d activities after import", len(imported))
		}
		if imported[0].Name != "Dance" || imported[0].FamilyId != fx.famB {
			t.Errorf("imported activity is wrong: %+v", imported[0])
		}

		seasons := GetActivitySeasons(tx, imported[0].Id)
		if len(seasons) != 1 {
			t.Fatalf("imported activity has %d seasons", len(seasons))
		}

		events := GetSeasonEvents(tx, seasons[0].Id)
		if len(events) != 1 || events[0].Host != "Nuvo" {
			t.Fatalf("imported season's events are wrong: %+v", events)
		}

		appearances := GetEventAppearances(tx, events[0].Id)
		if len(appearances) != 2 {
			t.Fatalf("imported event has %d appearances", len(appearances))
		}

		for _, appearance := range appearances {
			if appearance.FamilyId != fx.famB {
				t.Errorf("appearance %d landed in family %d", appearance.Id, appearance.FamilyId)
			}
			for _, result := range GetAppearanceResults(tx, appearance.Id) {
				if result.FamilyId != fx.famB {
					t.Errorf("result %d landed in family %d", result.Id, result.FamilyId)
				}
			}
		}
	})
}

func TestActivityImportIsIdempotent(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, personIdMapping := exportFamilyA(t, fx)

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})

	var second ActivityImportCounts
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		second, _ = importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})

	if second.Activities != 0 || second.Seasons != 0 || second.Events != 0 ||
		second.Entries != 0 || second.Appearances != 0 || second.Results != 0 {
		t.Errorf("second import created records: %+v", second)
	}
	if second.Reused == 0 {
		t.Error("second import reported no reuse, so it matched nothing")
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilyActivities(tx, fx.famB)); got != 1 {
			t.Errorf("family B has %d activities after importing twice", got)
		}
		if got := len(GetFamilyAppearances(tx, fx.famB)); got != 2 {
			t.Errorf("family B has %d appearances after importing twice", got)
		}
		if got := len(GetFamilyResults(tx, fx.famB)); got != 2 {
			t.Errorf("family B has %d results after importing twice", got)
		}
	})
}

func TestActivityImportDropsUnmappedRosterMembers(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, _ := exportFamilyA(t, fx)
	partial := map[int]int{fx.alice.Id: fx.alice.Id}

	var warnings []string
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		_, warnings = importActivities(tx, activities, fx.famB, partial, nil)
		vbolt.TxCommit(tx)
	})

	if len(warnings) == 0 {
		t.Error("dropping a roster member went unreported")
	}

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		activityId := GetFamilyActivities(tx, fx.famB)[0].Id
		seasonId := GetActivitySeasons(tx, activityId)[0].Id
		for _, entry := range GetSeasonEntries(tx, seasonId) {
			for _, personId := range GetEntryPersonIds(tx, entry.Id) {
				if personId == fx.bob.Id {
					t.Errorf("%q kept bob on its roster despite him not being imported", entry.Name)
				}
			}
			if entry.Name == "On My Own" && len(GetEntryPersonIds(tx, entry.Id)) != 0 {
				t.Error("the solo's roster should be empty")
			}
		}
	})
}

func TestActivityImportClearsResultsNamingUnimportedPeople(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, _ := exportFamilyA(t, fx)
	partial := map[int]int{fx.bob.Id: fx.bob.Id}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		importActivities(tx, activities, fx.famB, partial, nil)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		results := GetFamilyResults(tx, fx.famB)
		if len(results) != 2 {
			t.Fatalf("expected both results to survive, got %d", len(results))
		}
		for _, result := range results {
			if result.PersonId != nil {
				t.Errorf("result %q still names person %d", result.Label, *result.PersonId)
			}
		}
		if got := len(GetPersonResults(tx, fx.alice.Id)); got != 1 {
			t.Errorf("alice has %d indexed results; the import added one", got)
		}
	})
}

func TestActivityImportPreservesOptionalResultFields(t *testing.T) {
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

	activities, personIdMapping := exportFamilyA(t, fx)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		var placements, adjudications int
		for _, result := range GetFamilyResults(tx, fx.famB) {
			switch result.Kind {
			case ResultKindPlacement:
				placements++
				if result.Rank == nil || *result.Rank != 2 {
					t.Errorf("imported rank is %v", result.Rank)
				}
				if result.OutOf == nil || *result.OutOf != 14 {
					t.Errorf("imported outOf is %v", result.OutOf)
				}
			case ResultKindAdjudication:
				adjudications++
				if result.Rank != nil || result.OutOf != nil || result.Score != nil {
					t.Errorf("adjudication picked up numeric fields: %+v", result)
				}
			}
		}
		if placements != 1 || adjudications != 1 {
			t.Errorf("expected 1 placement and 1 adjudication, got %d and %d", placements, adjudications)
		}
	})
}

func TestActivityImportSkipsUnknownResultKinds(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, personIdMapping := exportFamilyA(t, fx)
	activities[0].Seasons[0].Events[0].Appearances[0].Results[0].Kind = "vibes"

	var counts ActivityImportCounts
	var warnings []string
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		counts, warnings = importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})

	if counts.Skipped != 1 {
		t.Errorf("expected 1 skipped result, got %d", counts.Skipped)
	}
	if len(warnings) == 0 {
		t.Error("dropping a result went unreported")
	}
	if counts.Appearances != 2 {
		t.Errorf("the bad row took its performance with it: %+v", counts)
	}
}

func TestActivityImportPopulatesTheIndexes(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, personIdMapping := exportFamilyA(t, fx)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilySeasons(tx, fx.famB)); got != 1 {
			t.Errorf("SeasonByFamilyIndex has %d entries", got)
		}
		if got := len(GetFamilyEvents(tx, fx.famB)); got != 1 {
			t.Errorf("EventByFamilyIndex has %d entries", got)
		}
		if got := len(GetFamilyEntries(tx, fx.famB)); got != 2 {
			t.Errorf("EntryByFamilyIndex has %d entries", got)
		}
		if got := len(GetFamilyEntryMembers(tx, fx.famB)); got != 3 {
			t.Errorf("EntryMemberByFamilyIndex has %d entries", got)
		}
		if got := len(GetFamilyAppearances(tx, fx.famB)); got != 2 {
			t.Errorf("AppearanceByFamilyIndex has %d entries", got)
		}
		if got := len(GetFamilyResults(tx, fx.famB)); got != 2 {
			t.Errorf("ResultByFamilyIndex has %d entries", got)
		}

		activityId := GetFamilyActivities(tx, fx.famB)[0].Id
		seasonId := GetActivitySeasons(tx, activityId)[0].Id
		for _, entry := range GetSeasonEntries(tx, seasonId) {
			if len(GetEntryAppearances(tx, entry.Id)) != 1 {
				t.Errorf("%q is not reachable through AppearanceByEntryIndex", entry.Name)
			}
		}
	})
}

func TestActivityImportAttachesPhotosOnlyWithAMapping(t *testing.T) {
	fx, cleanup := setupActivityFixture(t)
	defer cleanup()

	activities, personIdMapping := exportFamilyA(t, fx)

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		importActivities(tx, activities, fx.famB, personIdMapping, nil)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilyEventPhotos(tx, fx.famB)); got != 0 {
			t.Errorf("event photo joins came back without a mapping: %d", got)
		}
		if got := len(GetFamilyAppearancePhotos(tx, fx.famB)); got != 0 {
			t.Errorf("appearance photo joins came back without a mapping: %d", got)
		}
	})

	photoIdMapping := map[int]int{
		fx.alicePhoto.Id:    fx.alicePhoto.Id,
		fx.untaggedPhoto.Id: fx.untaggedPhoto.Id,
	}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		importActivities(tx, activities, fx.famC, personIdMapping, photoIdMapping)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := len(GetFamilyEventPhotos(tx, fx.famC)); got != 1 {
			t.Errorf("expected 1 event photo join, got %d", got)
		}
		if got := len(GetFamilyAppearancePhotos(tx, fx.famC)); got != 1 {
			t.Errorf("expected 1 appearance photo join, got %d", got)
		}
	})
}
