package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
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
	})
}
