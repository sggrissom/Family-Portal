package backend

import (
	"family/cfg"
	"fmt"
	"syscall"

	"go.hasen.dev/vbolt"
)

// The quota bounds what one account can cost; the disk floor bounds the machine,
// which per-family quotas cannot do on their own. The database shares a
// filesystem with the photos, so a full disk stops BoltDB committing.

// Counts originals only, which is what the Image row records. Derived variants
// roughly triple what lands on disk.
func FamilyStorageUsage(tx *vbolt.Tx, familyId int) int64 {
	var total int64
	for _, image := range GetFamilyImages(tx, familyId) {
		total += int64(image.FileSize)
	}
	return total
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/float64(1<<20))
	default:
		return fmt.Sprintf("%d KB", b/(1<<10))
	}
}

// The limit is a parameter so tests can hit the boundary without writing
// gigabytes; callers pass cfg.FamilyStorageQuotaBytes.
func CheckFamilyStorageQuota(tx *vbolt.Tx, familyId int, incoming int64, quota int64) *AppError {
	if quota <= 0 {
		return nil
	}

	used := FamilyStorageUsage(tx, familyId)
	if used+incoming <= quota {
		return nil
	}

	remaining := quota - used
	if remaining < 0 {
		remaining = 0
	}

	return NewAppError(
		ErrCodeTooLarge,
		fmt.Sprintf(
			"This family has used %s of its %s photo storage. Delete some photos to make room, or contact %s.",
			formatBytes(used), formatBytes(quota), cfg.SupportEmail,
		),
		fmt.Sprintf("family %d: used %d, incoming %d, quota %d, remaining %d",
			familyId, used, incoming, quota, remaining),
	)
}

// Bavail rather than Bfree: the difference is blocks only root may use.
var freeDiskBytes = func(dir string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// A failure to measure allows the write: refusing every upload because Statfs
// broke trades a rare problem for a total one.
func CheckDiskHeadroom(dir string, incoming int64, floor int64) *AppError {
	if floor <= 0 {
		return nil
	}

	free, err := freeDiskBytes(dir)
	if err != nil {
		LogErrorSimple(LogCategorySystem, "Could not measure free disk space; allowing the write", map[string]interface{}{
			"dir":   dir,
			"error": err.Error(),
		})
		return nil
	}

	if free-incoming >= floor {
		return nil
	}

	LogErrorSimple(LogCategorySystem, "Refusing an upload to protect disk headroom", map[string]interface{}{
		"dir":       dir,
		"freeBytes": free,
		"incoming":  incoming,
		"floor":     floor,
	})

	return NewAppError(
		ErrCodeUnavailable,
		"The server is low on storage and cannot accept uploads right now. Please try again later.",
		fmt.Sprintf("free %d, incoming %d, floor %d", free, incoming, floor),
	)
}

// Family quota first: that is the answer the uploader can act on.
func CheckStorageAccepts(tx *vbolt.Tx, familyId int, incoming int64) *AppError {
	if err := CheckFamilyStorageQuota(tx, familyId, incoming, cfg.FamilyStorageQuotaBytes); err != nil {
		return err
	}
	return CheckDiskHeadroom(cfg.StaticDir, incoming, cfg.MinFreeDiskBytes)
}
