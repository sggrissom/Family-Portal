package backend

import (
	"family/cfg"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestGetSystemHealthReportsEveryProblemClass(t *testing.T) {
	now := time.Now().UTC()
	recent := now.Add(-2 * time.Hour).Format(time.RFC3339)
	stale := now.Add(-72 * time.Hour).Format(time.RFC3339)

	withLogDir(t, map[string]string{
		"healthtest.log": `2026/08/22 10:00:00 {"timestamp":"` + recent + `","level":"ERROR","category":"SYSTEM","message":"Unexpected procedure error","data":{"requestId":"deadbeef0001"}}
2026/08/22 10:00:01 {"timestamp":"` + recent + `","level":"INFO","category":"API","message":"GET /rpc/GetPeople 500","httpStatus":500,"duration":900,"httpMethod":"GET","httpPath":"/rpc/GetPeople"}
2026/08/22 10:00:02 {"timestamp":"` + recent + `","level":"INFO","category":"API","message":"GET /missing 404","httpStatus":404,"duration":50,"httpMethod":"GET","httpPath":"/missing"}
2026/08/22 10:00:03 {"timestamp":"` + stale + `","level":"ERROR","category":"SYSTEM","message":"Three days ago, outside the window"}
`,
	})

	db := logTestDB(t, "test_system_health.db")
	token := adminContext(t, db)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		images := []Image{
			{Id: 1, FamilyId: 1, Status: 2, CreatedAt: now.Add(-2 * time.Hour)},
			{Id: 2, FamilyId: 1, Status: 1, CreatedAt: now.Add(-3 * time.Hour)},
			{Id: 3, FamilyId: 1, Status: 1, CreatedAt: now},
			{Id: 4, FamilyId: 1, Status: 0, AnalysisStatus: 3, CreatedAt: now},
		}
		for _, image := range images {
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
		}
		vbolt.TxCommit(tx)
	})

	var resp SystemHealthResponse
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		resp, err = GetSystemHealth(&vbeam.Context{Tx: tx, Token: token}, Empty{})
		if err != nil {
			t.Fatalf("GetSystemHealth() error = %v", err)
		}
	})

	if resp.Healthy {
		t.Error("Healthy = true with failed photos, errors and a 5xx present")
	}

	t.Run("logs", func(t *testing.T) {
		if resp.Logs.Errors != 1 {
			t.Errorf("Errors = %d, want 1 — the three-day-old one is outside the window", resp.Logs.Errors)
		}
		if resp.Logs.Requests5xx != 1 || resp.Logs.Requests4xx != 1 {
			t.Errorf("4xx/5xx = %d/%d, want 1/1", resp.Logs.Requests4xx, resp.Logs.Requests5xx)
		}
		if resp.Logs.WindowHours != 24 {
			t.Errorf("WindowHours = %d, want 24", resp.Logs.WindowHours)
		}
		if len(resp.Logs.RecentErrors) != 1 {
			t.Fatalf("RecentErrors has %d entries, want 1", len(resp.Logs.RecentErrors))
		}
		data, ok := resp.Logs.RecentErrors[0].Data.(map[string]interface{})
		if !ok || data["requestId"] != "deadbeef0001" {
			t.Errorf("RecentErrors[0].Data = %#v, want the requestId", resp.Logs.RecentErrors[0].Data)
		}
	})

	t.Run("photos", func(t *testing.T) {
		if resp.Photos.Failed != 1 {
			t.Errorf("Failed = %d, want 1", resp.Photos.Failed)
		}
		if resp.Photos.Stuck != 1 {
			t.Errorf("Stuck = %d, want 1 — the photo created just now is in progress, not stranded", resp.Photos.Stuck)
		}
		if resp.Photos.AnalysisFailed != 1 {
			t.Errorf("AnalysisFailed = %d, want 1", resp.Photos.AnalysisFailed)
		}
	})

	t.Run("non-admin is refused", func(t *testing.T) {
		regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			if _, err := GetSystemHealth(&vbeam.Context{Tx: tx, Token: regular}, Empty{}); err != ErrAdminRequired {
				t.Errorf("Expected ErrAdminRequired, got %v", err)
			}
		})
	})
}

func TestGetSystemHealthIsQuietWhenNothingIsWrong(t *testing.T) {
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	withLogDir(t, map[string]string{
		"quiettest.log": `2026/08/22 10:00:00 {"timestamp":"` + recent + `","level":"INFO","category":"API","message":"GET /rpc/GetPeople 200","httpStatus":200,"duration":900,"httpMethod":"GET","httpPath":"/rpc/GetPeople"}
`,
	})

	db := logTestDB(t, "test_system_health_quiet.db")
	token := adminContext(t, db)

	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil
	InitializePhotoWorker(5, db)
	t.Cleanup(func() {
		globalPhotoWorker.Stop()
		globalPhotoWorker = nil
	})

	var resp SystemHealthResponse
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		resp, err = GetSystemHealth(&vbeam.Context{Tx: tx, Token: token}, Empty{})
		if err != nil {
			t.Fatalf("GetSystemHealth() error = %v", err)
		}
	})

	if resp.Logs.Errors != 0 || resp.Logs.Requests5xx != 0 {
		t.Errorf("Logs reported problems on a clean log: %+v", resp.Logs)
	}
	if resp.Logs.Unavailable {
		t.Error("Unavailable = true with a log file written within the window")
	}
	if resp.Photos.Failed != 0 || resp.Photos.Stuck != 0 || resp.Photos.WorkerStopped {
		t.Errorf("Photos reported problems with no photos and a running worker: %+v", resp.Photos)
	}
}

func TestGetSystemHealthFlagsMissingLogs(t *testing.T) {
	if entries, err := os.ReadDir(cfg.LogDir); err == nil && len(entries) > 0 {
		t.Skip("cfg.LogDir already holds log files")
	}

	db := logTestDB(t, "test_system_health_nologs.db")
	token := adminContext(t, db)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		resp, err := GetSystemHealth(&vbeam.Context{Tx: tx, Token: token}, Empty{})
		if err != nil {
			t.Fatalf("GetSystemHealth() error = %v", err)
		}
		if !resp.Logs.Unavailable {
			t.Error("Unavailable = false with no log files to read")
		}
		if resp.Healthy {
			t.Error("Healthy = true while the logs cannot be read")
		}
	})
}
