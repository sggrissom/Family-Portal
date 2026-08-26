package backend

import (
	"family/cfg"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

const photoConsistencyListLimit = 50

type MissingOriginal struct {
	ImageId   int       `json:"imageId"`
	FamilyId  int       `json:"familyId"`
	Status    int       `json:"status"`
	FilePath  string    `json:"filePath"`
	CreatedAt time.Time `json:"createdAt"`
}

type OrphanOriginal struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	ModTime   time.Time `json:"modTime"`
}

type PhotoConsistencyReport struct {
	CheckedAt     time.Time         `json:"checkedAt"`
	DurationMs    int64             `json:"durationMs"`
	TotalImages   int               `json:"totalImages"`
	PresentCount  int               `json:"presentCount"`
	MissingCount  int               `json:"missingCount"`
	OrphanCount   int               `json:"orphanCount"`
	OrphanBytes   int64             `json:"orphanBytes"`
	Missing       []MissingOriginal `json:"missing"`
	Orphans       []OrphanOriginal  `json:"orphans"`
	ListLimit     int               `json:"listLimit"`
	OrphanScanErr string            `json:"orphanScanErr"`
}

func originalPathIn(staticDir, filePath string) string {
	base := filepath.Join(staticDir, filePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "_original" + ext
}

func ScanPhotoConsistency(tx *vbolt.Tx, staticDir string, listLimit int) PhotoConsistencyReport {
	start := time.Now()
	report := PhotoConsistencyReport{
		CheckedAt: start,
		Missing:   []MissingOriginal{},
		Orphans:   []OrphanOriginal{},
		ListLimit: listLimit,
	}

	referenced := make(map[string]bool)
	vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
		report.TotalImages++
		if staticDir == "" {
			return true
		}
		path := originalPathIn(staticDir, image.FilePath)
		referenced[filepath.Base(path)] = true
		if _, err := os.Stat(path); err != nil {
			report.MissingCount++
			if listLimit <= 0 || len(report.Missing) < listLimit {
				report.Missing = append(report.Missing, MissingOriginal{
					ImageId:   image.Id,
					FamilyId:  image.FamilyId,
					Status:    image.Status,
					FilePath:  image.FilePath,
					CreatedAt: image.CreatedAt,
				})
			}
		}
		return true
	})
	report.PresentCount = report.TotalImages - report.MissingCount

	if staticDir == "" {
		report.DurationMs = time.Since(start).Milliseconds()
		return report
	}

	entries, err := os.ReadDir(filepath.Join(staticDir, "photos"))
	if err != nil {
		report.OrphanScanErr = err.Error()
		report.DurationMs = time.Since(start).Milliseconds()
		return report
	}

	var orphans []OrphanOriginal
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.Contains(name, "_original.") || referenced[name] {
			continue
		}
		orphan := OrphanOriginal{Name: name}
		if info, infoErr := entry.Info(); infoErr == nil {
			orphan.SizeBytes = info.Size()
			orphan.ModTime = info.ModTime()
		}
		report.OrphanCount++
		report.OrphanBytes += orphan.SizeBytes
		orphans = append(orphans, orphan)
	}

	sort.Slice(orphans, func(i, j int) bool { return orphans[i].SizeBytes > orphans[j].SizeBytes })
	if listLimit > 0 && len(orphans) > listLimit {
		orphans = orphans[:listLimit]
	}
	report.Orphans = append(report.Orphans, orphans...)

	report.DurationMs = time.Since(start).Milliseconds()
	return report
}

type CheckPhotoConsistencyRequest struct{}

func CheckPhotoConsistency(ctx *vbeam.Context, req CheckPhotoConsistencyRequest) (resp PhotoConsistencyReport, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	resp = ScanPhotoConsistency(ctx.Tx, cfg.StaticDir, photoConsistencyListLimit)

	if resp.MissingCount > 0 {
		LogWarn(LogCategoryAdmin, "Photo consistency check found image rows with no original on disk", map[string]interface{}{
			"missing": resp.MissingCount,
			"total":   resp.TotalImages,
		})
	} else {
		LogInfo(LogCategoryAdmin, "Photo consistency check found every original present", map[string]interface{}{
			"total":   resp.TotalImages,
			"orphans": resp.OrphanCount,
		})
	}
	return
}
