package backend

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func writePhotoFile(t *testing.T, staticDir, name string, size int) {
	t.Helper()
	path := filepath.Join(staticDir, "photos", name)
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func TestScanPhotoConsistency(t *testing.T) {
	db := logTestDB(t, "test_photo_consistency.db")

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "photos"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		images := []Image{
			{Id: 1, FamilyId: 1, Status: 0, FilePath: "photos/present.jpg"},
			{Id: 2, FamilyId: 1, Status: 2, FilePath: "photos/gone.jpg"},
			{Id: 3, FamilyId: 2, Status: 0, FilePath: "photos/also-gone.png"},
		}
		for _, image := range images {
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
		}
		vbolt.TxCommit(tx)
	})

	writePhotoFile(t, staticDir, "present_original.jpg", 10)
	writePhotoFile(t, staticDir, "present.jpg", 5)
	writePhotoFile(t, staticDir, "present_thumb.webp", 5)
	writePhotoFile(t, staticDir, "stray_original.jpg", 300)
	writePhotoFile(t, staticDir, "smaller-stray_original.png", 100)

	var report PhotoConsistencyReport
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		report = ScanPhotoConsistency(tx, staticDir, 0)
	})

	if report.TotalImages != 3 {
		t.Errorf("TotalImages = %d, want 3", report.TotalImages)
	}
	if report.PresentCount != 1 {
		t.Errorf("PresentCount = %d, want 1", report.PresentCount)
	}
	if report.MissingCount != 2 || len(report.Missing) != 2 {
		t.Fatalf("MissingCount = %d, len(Missing) = %d, want 2 and 2", report.MissingCount, len(report.Missing))
	}
	if report.Missing[0].ImageId != 2 || report.Missing[0].FilePath != "photos/gone.jpg" {
		t.Errorf("Missing[0] = %+v, want image 2", report.Missing[0])
	}
	if report.Missing[1].Status != 0 || report.Missing[1].FamilyId != 2 {
		t.Errorf("Missing[1] = %+v, want family 2 status 0", report.Missing[1])
	}

	if report.OrphanCount != 2 || len(report.Orphans) != 2 {
		t.Fatalf("OrphanCount = %d, len(Orphans) = %d, want 2 and 2", report.OrphanCount, len(report.Orphans))
	}
	if report.Orphans[0].Name != "stray_original.jpg" {
		t.Errorf("Orphans[0].Name = %q, want the largest orphan first", report.Orphans[0].Name)
	}
	if report.OrphanBytes != 400 {
		t.Errorf("OrphanBytes = %d, want 400", report.OrphanBytes)
	}
}

func TestScanPhotoConsistencyLimits(t *testing.T) {
	db := logTestDB(t, "test_photo_consistency_limits.db")

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "photos"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for id := 1; id <= 5; id++ {
			image := Image{Id: id, FamilyId: 1, FilePath: "photos/missing" + string(rune('a'+id-1)) + ".jpg"}
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
		}
		vbolt.TxCommit(tx)
	})
	for i := 0; i < 5; i++ {
		writePhotoFile(t, staticDir, "orphan"+string(rune('a'+i))+"_original.jpg", 10)
	}

	var report PhotoConsistencyReport
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		report = ScanPhotoConsistency(tx, staticDir, 2)
	})

	if report.MissingCount != 5 || len(report.Missing) != 2 {
		t.Errorf("MissingCount = %d, len(Missing) = %d, want 5 and 2", report.MissingCount, len(report.Missing))
	}
	if report.OrphanCount != 5 || len(report.Orphans) != 2 {
		t.Errorf("OrphanCount = %d, len(Orphans) = %d, want 5 and 2", report.OrphanCount, len(report.Orphans))
	}
	if report.ListLimit != 2 {
		t.Errorf("ListLimit = %d, want 2", report.ListLimit)
	}
}

func TestScanPhotoConsistencyMissingPhotoDir(t *testing.T) {
	db := logTestDB(t, "test_photo_consistency_nodir.db")

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		image := Image{Id: 1, FamilyId: 1, FilePath: "photos/one.jpg"}
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.TxCommit(tx)
	})

	var report PhotoConsistencyReport
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		report = ScanPhotoConsistency(tx, t.TempDir(), 0)
	})

	if report.OrphanScanErr == "" {
		t.Error("OrphanScanErr = \"\", want the unreadable photos directory reported")
	}
	if report.MissingCount != 1 {
		t.Errorf("MissingCount = %d, want 1 — rows are still checked when the directory scan fails", report.MissingCount)
	}
	if report.Orphans == nil {
		t.Error("Orphans = nil, want an empty list so the client can render it")
	}
}

func TestCheckPhotoConsistencyRequiresAdmin(t *testing.T) {
	db := logTestDB(t, "test_photo_consistency_auth.db")
	token := adminContext(t, db)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, err := CheckPhotoConsistency(&vbeam.Context{Tx: tx, Token: token}, CheckPhotoConsistencyRequest{}); err != nil {
			t.Errorf("CheckPhotoConsistency() error = %v", err)
		}
	})

	regular, _ := generateAuthJwt(User{Id: 2, Email: "regular@example.com"}, httptest.NewRecorder())
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if _, err := CheckPhotoConsistency(&vbeam.Context{Tx: tx, Token: regular}, CheckPhotoConsistencyRequest{}); err != ErrAdminRequired {
			t.Errorf("Expected ErrAdminRequired, got %v", err)
		}
	})
}
