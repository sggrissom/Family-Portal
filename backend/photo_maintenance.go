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

func RegisterPhotoMaintenanceMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetPhotoStats)
	vbeam.RegisterProc(app, ReprocessAllPhotos)
	vbeam.RegisterProc(app, GetPhotoProcessingStats)
	vbeam.RegisterProc(app, GetAnalysisStats)
	vbeam.RegisterProc(app, ReanalyzeAllPhotos)
	vbeam.RegisterProc(app, CheckPhotoConsistency)
}

type GetPhotoStatsRequest struct{}

type GetPhotoStatsResponse struct {
	TotalPhotos       int `json:"totalPhotos"`
	ProcessedPhotos   int `json:"processedPhotos"`
	PendingPhotos     int `json:"pendingPhotos"`
	AnalysisPending   int `json:"analysisPending"`
	AnalysisAnalyzing int `json:"analysisAnalyzing"`
	AnalysisDone      int `json:"analysisDone"`
	AnalysisFailed    int `json:"analysisFailed"`
	AutoTaggedCount   int `json:"autoTaggedCount"`
	PersonsWithFace   int `json:"personsWithFace"`
}

type ReprocessAllPhotosRequest struct{}

type ReprocessAllPhotosResponse struct {
	Queued int `json:"queued"`
}

func GetPhotoStats(ctx *vbeam.Context, req GetPhotoStatsRequest) (resp GetPhotoStatsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	var allPhotos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		allPhotos = append(allPhotos, image)
		return true
	})

	resp.TotalPhotos = len(allPhotos)

	processedCount := 0
	for _, photo := range allPhotos {
		if isPhotoProcessed(photo) {
			processedCount++
		}
		switch photo.AnalysisStatus {
		case 0:
			resp.AnalysisPending++
		case 1:
			resp.AnalysisAnalyzing++
		case 2:
			resp.AnalysisDone++
		case 3:
			resp.AnalysisFailed++
		}
	}

	resp.ProcessedPhotos = processedCount
	resp.PendingPhotos = resp.TotalPhotos - resp.ProcessedPhotos

	vbolt.IterateAll(ctx.Tx, PhotoPersonBkt, func(key int, pp PhotoPerson) bool {
		if pp.AutoTagged {
			resp.AutoTaggedCount++
		}
		return true
	})

	vbolt.IterateAll(ctx.Tx, PeopleBkt, func(key int, p Person) bool {
		if len(p.FaceDescriptor) == 128 {
			resp.PersonsWithFace++
		}
		return true
	})

	return
}

func ReprocessAllPhotos(ctx *vbeam.Context, req ReprocessAllPhotosRequest) (resp ReprocessAllPhotosResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	pw := activePhotoWorker()
	if pw == nil {
		err = ErrPhotoWorkerUnavailable
		return
	}

	var toQueue []PhotoProcessingJob
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if !isPhotoProcessed(image) {
			toQueue = append(toQueue, PhotoProcessingJob{
				ImageId:   image.Id,
				FamilyId:  image.FamilyId,
				FilePath:  image.FilePath,
				MimeType:  image.MimeType,
				Reprocess: true,
			})
		}
		return true
	})

	queueBacklog(pw, toQueue,
		"Failed to queue photo for reprocessing",
		"Reprocess backlog fully queued")

	resp.Queued = len(toQueue)
	return
}

func isPhotoProcessed(photo Image) bool {
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	modernFormats := []string{".avif", ".webp"}
	sizes := []string{"", "_small", "_thumb", "_medium", "_large", "_xlarge", "_xxlarge"}

	for _, format := range modernFormats {
		for _, size := range sizes {
			var fileName string
			if size == "" || size == "_large" {
				fileName = baseFilename + format
			} else {
				fileName = baseFilename + size + format
			}

			if _, err := os.Stat(fileName); err == nil {
				return true
			}
		}
	}

	return false
}

func getOriginalPhotoPath(photo Image) string {
	return originalPathIn(cfg.StaticDir, photo.FilePath)
}

func cleanupOldVariants(baseFilename string) {
	oldVariants := []string{
		baseFilename + ".jpg",
		baseFilename + "_thumb.jpg",
		baseFilename + "_medium.jpg",
		baseFilename + "_small.jpg",
		baseFilename + "_xlarge.jpg",
		baseFilename + "_xxlarge.jpg",
	}

	for _, variant := range oldVariants {
		os.Remove(variant)
	}
}

func GetPhotoProcessingStats(ctx *vbeam.Context, req Empty) (resp ProcessingStats, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	resp = GetProcessingStats()
	return
}

func GetAnalysisStats(ctx *vbeam.Context, req Empty) (resp AnalysisWorkerStats, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	resp = GetAnalysisWorkerStats()
	return
}

type ReanalyzeAllPhotosRequest struct{}

type ReanalyzeAllPhotosResponse struct {
	Queued  int `json:"queued"`
	Skipped int `json:"skipped"`
}

func ReanalyzeAllPhotos(ctx *vbeam.Context, req ReanalyzeAllPhotosRequest) (resp ReanalyzeAllPhotosResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	if !GetAnalysisWorkerStats().IsRunning {
		err = ErrFaceAnalysisUnavailable
		return
	}

	var toQueue []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if image.AnalysisStatus == 0 || image.AnalysisStatus == 3 {
			toQueue = append(toQueue, image)
		} else {
			resp.Skipped++
		}
		return true
	})

	if len(toQueue) > 0 {
		vbeam.UseWriteTx(ctx)
		for _, image := range toQueue {
			if image.AnalysisStatus == 3 {
				image.AnalysisStatus = 0
				vbolt.Write(ctx.Tx, ImagesBkt, image.Id, &image)
			}
		}
		vbolt.TxCommit(ctx.Tx)
	}

	for _, image := range toQueue {
		QueuePhotoAnalysis(PhotoAnalysisJob{ImageId: image.Id, FamilyId: image.FamilyId})
		resp.Queued++
	}

	return
}

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
