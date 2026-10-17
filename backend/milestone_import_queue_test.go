package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestMilestoneImportsQueueCommittedRecords(t *testing.T) {
	for _, mode := range []string{"json", "zip", "dry-run", "preview"} {
		t.Run(mode, func(t *testing.T) {
			fx := setupResultsFixture(t)
			previous := globalMilestoneWorker
			globalMilestoneWorker = newBacklogWorker("import test", func(int) error { return nil })
			t.Cleanup(func() { globalMilestoneWorker = previous })
			data := ImportDataStructure{
				People:     []ImportPerson{{Id: 100, FamilyId: 100, Name: "Imported child", Birthday: fx.alice.Birthday}},
				Milestones: []ExportMilestone{{Id: 100, PersonId: 100, Description: "First steps", Category: "development", MilestoneDate: featuresEpoch}},
			}
			encoded, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			var response ImportDataResponse
			if mode == "zip" {
				var archive bytes.Buffer
				zw := zip.NewWriter(&archive)
				entry, err := zw.Create("data.json")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = entry.Write(encoded); err != nil {
					t.Fatal(err)
				}
				if err = zw.Close(); err != nil {
					t.Fatal(err)
				}
				var body bytes.Buffer
				form := multipart.NewWriter(&body)
				part, err := form.CreateFormFile("file", "family.zip")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = part.Write(archive.Bytes()); err != nil {
					t.Fatal(err)
				}
				if err = form.Close(); err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/api/import-bundle", &body)
				req.Header.Set("Content-Type", form.FormDataContentType())
				req = req.WithContext(context.WithValue(req.Context(), UserContextKey, fx.owner))
				recorder := httptest.NewRecorder()
				importBundleHandler(recorder, req)
				if recorder.Code != http.StatusOK {
					t.Fatalf("import failed: %d %s", recorder.Code, recorder.Body.String())
				}
				if err = json.NewDecoder(recorder.Body).Decode(&response); err != nil {
					t.Fatal(err)
				}
			} else {
				response, err = callAs(t, fx, ImportData, ImportDataRequest{JsonData: string(encoded), ImportMilestones: true, DryRun: mode == "dry-run", PreviewOnly: mode == "preview"})
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "dry-run" || mode == "preview" {
				if globalMilestoneWorker.length() != 0 {
					t.Fatal("uncommitted import queued work")
				}
				return
			}
			id, ok := globalMilestoneWorker.pop()
			if !ok || response.ImportedMilestones != 1 {
				t.Fatalf("import not queued: %+v", response)
			}
			vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
				m := GetMilestoneById(tx, id)
				if m.Id == 0 || m.Description != "First steps" || m.FamilyId != fx.familyId {
					t.Fatalf("queued record not committed: %+v", m)
				}
			})
			assertMilestoneSearchable(t, fx, id, "steps")
			// Re-importing the same record should not queue duplicates.
			if _, err := callAs(t, fx, ImportData, ImportDataRequest{JsonData: string(encoded), ImportMilestones: true, MergeStrategy: "merge_people"}); err != nil {
				t.Fatal(err)
			}
			if globalMilestoneWorker.length() != 0 {
				t.Fatal("duplicate milestone import queued work")
			}
		})
	}
}

func assertMilestoneSearchable(t *testing.T, fx resultsFixture, id int, query string) {
	t.Helper()
	var outsider User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		outsider = AddUserTx(tx, CreateAccountRequest{Name: "Outsider", Email: fmt.Sprintf("outsider-%d@example.com", id)}, nil)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		found := false
		for _, m := range SearchVisibleMilestones(tx, query, fx.owner, 10) {
			found = found || m.Id == id
		}
		if !found {
			t.Fatalf("milestone %d not found by %q", id, query)
		}
		if leaked := SearchVisibleMilestones(tx, query, outsider, 10); len(leaked) != 0 {
			t.Fatalf("unrelated family found %+v", leaked)
		}
	})
}

func TestMilestonesWrittenOutsideTheSavePathAreSearchable(t *testing.T) {
	fx := setupResultsFixture(t)
	legacy := Milestone{PersonId: fx.alice.Id, FamilyId: fx.familyId, Description: "Rode a bicycle", Category: "achievement", MilestoneDate: featuresEpoch, CreatedAt: featuresEpoch}
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		legacy.Id = vbolt.NextIntId(tx, MilestoneBkt)
		vbolt.Write(tx, MilestoneBkt, legacy.Id, &legacy)
		vbolt.SetTargetSingleTerm(tx, MilestoneByPersonIndex, legacy.Id, legacy.PersonId)
		vbolt.SetTargetSingleTerm(tx, MilestoneByFamilyIndex, legacy.Id, legacy.FamilyId)
		vbolt.TxCommit(tx)
	})
	assertMilestoneSearchable(t, fx, legacy.Id, "bicycle")
}

func TestPersonMergeQueuesMilestonesForTheirNewOwner(t *testing.T) {
	fx := setupResultsFixture(t)
	previous := globalMilestoneWorker
	globalMilestoneWorker = newBacklogWorker("merge test", func(int) error { return nil })
	t.Cleanup(func() { globalMilestoneWorker = previous })
	m := fx.addMilestoneOn(t, fx.alice, "First steps", "development", featuresEpoch)
	if _, err := callAs(t, fx, MergePeople, MergePeopleRequest{SourcePersonId: fx.alice.Id, TargetPersonId: fx.bob.Id}); err != nil {
		t.Fatal(err)
	}
	id, ok := globalMilestoneWorker.pop()
	if !ok || id != m.Id {
		t.Fatal("merged milestone was not queued")
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := GetMilestoneById(tx, id); got.PersonId != fx.bob.Id {
			t.Fatalf("queued milestone has stale owner: %+v", got)
		}
		if got := SearchVisibleMilestones(tx, "bob first steps", fx.owner, 10); len(got) != 1 || got[0].Id != id {
			t.Fatalf("merged milestone not found under its new owner: %+v", got)
		}
	})
}

func TestDropMilestoneSearchIndexRemovesTheOldBucket(t *testing.T) {
	fx := setupResultsFixture(t)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		bucket, err := tx.CreateBucketIfNotExists([]byte(legacyMilestoneSearchBucket))
		if err != nil {
			t.Fatal(err)
		}
		bucket.Put([]byte("walk"), []byte{1})
		vbolt.TxCommit(tx)
	})
	for range 2 {
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			DropMilestoneSearchIndex(tx)
			vbolt.TxCommit(tx)
		})
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if tx.Bucket([]byte(legacyMilestoneSearchBucket)) != nil {
			t.Error("the old search index is still there")
		}
	})
}
