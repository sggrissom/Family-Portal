package backend

import (
	"errors"
	"os"
	"strings"
	"testing"

	"family/cfg"

	"go.hasen.dev/vbolt"
)

func quotaTestDB(t *testing.T) *vbolt.DB {
	t.Helper()

	tempFile, err := os.CreateTemp("", "quota_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	path := tempFile.Name()
	tempFile.Close()

	db := vbolt.Open(path)
	vbolt.InitBuckets(db, &cfg.Info)

	t.Cleanup(func() {
		db.Close()
		os.Remove(path)
	})
	return db
}

func writeSizedImage(t *testing.T, db *vbolt.DB, familyId, size int) {
	t.Helper()
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		image := Image{
			Id:       vbolt.NextIntId(tx, ImagesBkt),
			FamilyId: familyId,
			FileSize: size,
		}
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
		vbolt.TxCommit(tx)
	})
}

func TestFamilyStorageUsageSumsOnlyThatFamily(t *testing.T) {
	db := quotaTestDB(t)

	writeSizedImage(t, db, 1, 1000)
	writeSizedImage(t, db, 1, 2500)
	writeSizedImage(t, db, 2, 9000)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if used := FamilyStorageUsage(tx, 1); used != 3500 {
			t.Errorf("family 1: expected 3500, got %d", used)
		}
		if used := FamilyStorageUsage(tx, 2); used != 9000 {
			t.Errorf("family 2: expected 9000, got %d", used)
		}
		if used := FamilyStorageUsage(tx, 3); used != 0 {
			t.Errorf("empty family: expected 0, got %d", used)
		}
	})
}

func TestCheckFamilyStorageQuotaBoundary(t *testing.T) {
	db := quotaTestDB(t)
	writeSizedImage(t, db, 1, 900)

	tests := []struct {
		name     string
		incoming int64
		quota    int64
		rejected bool
	}{
		{name: "well under", incoming: 50, quota: 1000, rejected: false},
		{name: "exactly at the quota is allowed", incoming: 100, quota: 1000, rejected: false},
		{name: "one byte over is refused", incoming: 101, quota: 1000, rejected: true},
		{name: "quota of zero disables the check", incoming: 1 << 40, quota: 0, rejected: false},
		{name: "negative quota disables the check", incoming: 1 << 40, quota: -1, rejected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				err := CheckFamilyStorageQuota(tx, 1, tc.incoming, tc.quota)
				if tc.rejected && err == nil {
					t.Fatal("expected the write to be refused")
				}
				if !tc.rejected && err != nil {
					t.Fatalf("expected the write to be allowed, got %v", err)
				}
				if tc.rejected && err.Code != ErrCodeTooLarge {
					t.Errorf("expected %s, got %s", ErrCodeTooLarge, err.Code)
				}
			})
		})
	}
}

func TestCheckFamilyStorageQuotaAlreadyOver(t *testing.T) {
	db := quotaTestDB(t)
	writeSizedImage(t, db, 1, 5000)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		err := CheckFamilyStorageQuota(tx, 1, 1, 1000)
		if err == nil {
			t.Fatal("expected a family over quota to be refused")
		}
		if want := "remaining 0"; !strings.Contains(err.Details, want) {
			t.Errorf("expected details to clamp remaining to zero, got %q", err.Details)
		}
	})
}

func TestCheckDiskHeadroom(t *testing.T) {
	original := freeDiskBytes
	t.Cleanup(func() { freeDiskBytes = original })

	tests := []struct {
		name     string
		free     int64
		incoming int64
		floor    int64
		rejected bool
	}{
		{name: "plenty of room", free: 100 << 30, incoming: 1 << 20, floor: 1 << 30, rejected: false},
		{name: "write would land exactly on the floor", free: (1 << 30) + 100, incoming: 100, floor: 1 << 30, rejected: false},
		{name: "write would breach the floor", free: (1 << 30) + 99, incoming: 100, floor: 1 << 30, rejected: true},
		{name: "already below the floor", free: 1 << 20, incoming: 1, floor: 1 << 30, rejected: true},
		{name: "floor of zero disables the check", free: 0, incoming: 1 << 30, floor: 0, rejected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			freeDiskBytes = func(string) (int64, error) { return tc.free, nil }

			err := CheckDiskHeadroom("/nonexistent", tc.incoming, tc.floor)
			if tc.rejected && err == nil {
				t.Fatal("expected the write to be refused")
			}
			if !tc.rejected && err != nil {
				t.Fatalf("expected the write to be allowed, got %v", err)
			}
			if tc.rejected && err.Code != ErrCodeUnavailable {
				t.Errorf("expected %s, got %s", ErrCodeUnavailable, err.Code)
			}
		})
	}
}

func TestCheckDiskHeadroomAllowsWhenUnmeasurable(t *testing.T) {
	original := freeDiskBytes
	t.Cleanup(func() { freeDiskBytes = original })

	freeDiskBytes = func(string) (int64, error) { return 0, errors.New("statfs failed") }

	if err := CheckDiskHeadroom("/nonexistent", 1<<20, 1<<30); err != nil {
		t.Fatalf("expected an unmeasurable disk to allow the write, got %v", err)
	}
}

func TestFreeDiskBytesReadsRealFilesystem(t *testing.T) {
	free, err := freeDiskBytes(t.TempDir())
	if err != nil {
		t.Fatalf("Statfs on a temp dir failed: %v", err)
	}
	if free <= 0 {
		t.Errorf("expected a positive byte count, got %d", free)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{in: 512, want: "0 KB"},
		{in: 2048, want: "2 KB"},
		{in: 5 << 20, want: "5 MB"},
		{in: 10 << 30, want: "10.0 GB"},
	}
	for _, tc := range tests {
		if got := formatBytes(tc.in); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
