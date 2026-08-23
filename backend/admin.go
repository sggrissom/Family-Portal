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

// AdminUserId is the account that owns the admin panel. One operator, so
// membership is a fixed id rather than a role — but it is named here rather
// than being a bare 1 in sixteen conditions.
const AdminUserId = 1

// requireAdminAccess is the admin gate for every admin proc. Callers return
// its error unchanged; it is a declared public error so the text reaches the
// browser instead of a reference code.
func requireAdminAccess(ctx *vbeam.Context) error {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		return ErrAuthFailure
	}
	if user.Id != AdminUserId {
		return ErrAdminRequired
	}
	return nil
}

func RegisterAdminMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListAllUsers)
	vbeam.RegisterProc(app, GetPhotoStats)
	vbeam.RegisterProc(app, ReprocessAllPhotos)
	vbeam.RegisterProc(app, GetPhotoProcessingStats)
	vbeam.RegisterProc(app, GetAnalysisStats)
	vbeam.RegisterProc(app, ReanalyzeAllPhotos)
	vbeam.RegisterProc(app, GetLogFiles)
	vbeam.RegisterProc(app, GetLogContent)
	vbeam.RegisterProc(app, GetLogStats)
	vbeam.RegisterProc(app, GetPushStatus)
	vbeam.RegisterProc(app, ListPushDevices)
	vbeam.RegisterProc(app, SendTestPushNotification)
	RegisterAnalyticsMethods(app)
}

type AdminUserInfo struct {
	Id         int       `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Creation   time.Time `json:"creation"`
	LastLogin  time.Time `json:"lastLogin"`
	FamilyId   int       `json:"familyId"`
	FamilyName string    `json:"familyName"`
	IsAdmin    bool      `json:"isAdmin"`
}

type ListAllUsersResponse struct {
	Users []AdminUserInfo `json:"users"`
}

// Admin-only procedure to list all registered users
func ListAllUsers(ctx *vbeam.Context, req Empty) (resp ListAllUsersResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	// Log admin action
	LogInfo(LogCategoryAdmin, "Admin accessed user list", map[string]interface{}{
		"adminUserId": AdminUserId,
	})

	// Get all users using IterateAll
	var users []User
	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		users = append(users, user)
		return true // Continue iteration
	})

	// Convert to AdminUserInfo with family names
	resp.Users = make([]AdminUserInfo, 0, len(users))
	for _, u := range users {
		familyName := ""
		if u.FamilyId != 0 {
			family := GetFamily(ctx.Tx, u.FamilyId)
			familyName = family.Name
		}

		adminUser := AdminUserInfo{
			Id:         u.Id,
			Name:       u.Name,
			Email:      u.Email,
			Creation:   u.Creation,
			LastLogin:  u.LastLogin,
			FamilyId:   u.FamilyId,
			FamilyName: familyName,
			IsAdmin:    u.Id == 1, // Admin check
		}
		resp.Users = append(resp.Users, adminUser)
	}

	return
}

// Photo Management Types and Procedures

type GetPhotoStatsRequest struct{}

type GetPhotoStatsResponse struct {
	TotalPhotos     int `json:"totalPhotos"`
	ProcessedPhotos int `json:"processedPhotos"`
	PendingPhotos   int `json:"pendingPhotos"`
	// Face analysis
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

// Get photo statistics for admin dashboard
func GetPhotoStats(ctx *vbeam.Context, req GetPhotoStatsRequest) (resp GetPhotoStatsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	// Count all photos
	var allPhotos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		allPhotos = append(allPhotos, image)
		return true
	})

	resp.TotalPhotos = len(allPhotos)

	// Count processed photos (those with modern format variants) and analysis status counts
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

	// Count auto-tagged PhotoPerson records
	vbolt.IterateAll(ctx.Tx, PhotoPersonBkt, func(key int, pp PhotoPerson) bool {
		if pp.AutoTagged {
			resp.AutoTaggedCount++
		}
		return true
	})

	// Count persons with a stored face descriptor
	vbolt.IterateAll(ctx.Tx, PeopleBkt, func(key int, p Person) bool {
		if len(p.FaceDescriptor) == 128 {
			resp.PersonsWithFace++
		}
		return true
	})

	return
}

// ReprocessAllPhotos queues every unprocessed photo for the photo worker.
//
// This used to decode and re-encode every photo inline, at seven sizes in two
// formats each, inside an open write transaction. bolt allows one writer, so
// for the length of that loop every upload, chat message and milestone save in
// the application blocked — and the two-minute RPC write timeout severed the
// response long before a real backlog finished, leaving rows stuck in
// Processing with nothing that would ever move them.
//
// It queues now, the same shape ReanalyzeAllPhotos already used, and returns a
// count immediately. Progress is the worker queue depth, which the diagnostics
// strip and this page already poll.
func ReprocessAllPhotos(ctx *vbeam.Context, req ReprocessAllPhotosRequest) (resp ReprocessAllPhotosResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	// Queueing into a worker that is not running would report a confident
	// count of photos nothing will ever pick up.
	if !GetProcessingStats().IsRunning {
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

	// Feed the queue from a goroutine with blocking sends. A backlog is
	// routinely larger than the queue, and the caller should not wait on the
	// worker to drain it — that is the timeout this rewrite exists to avoid.
	// The jobs carry no image bytes, so the pending slice is cheap to hold.
	if len(toQueue) > 0 {
		go func(jobs []PhotoProcessingJob) {
			for _, job := range jobs {
				if queueErr := QueuePhotoProcessingBlocking(job); queueErr != nil {
					LogErrorSimple(LogCategoryAdmin, "Failed to queue photo for reprocessing", map[string]interface{}{
						"photoId": job.ImageId,
						"error":   queueErr.Error(),
					})
					return
				}
			}
			LogInfo(LogCategoryAdmin, "Reprocess backlog fully queued", map[string]interface{}{
				"count": len(jobs),
			})
		}(toQueue)
	}

	resp.Queued = len(toQueue)
	return
}

// Helper function to check if a photo has been processed with modern formats
func isPhotoProcessed(photo Image) bool {
	// Check if modern format files exist
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	// Check for at least one AVIF or WebP variant
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
				return true // Found at least one modern format variant
			}
		}
	}

	return false
}

// Helper function to get the original photo path
func getOriginalPhotoPath(photo Image) string {
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(basePath, ext)
	return base + "_original" + ext
}

// Helper function to clean up old format variants
func cleanupOldVariants(baseFilename string) {
	// Remove old JPEG variants (except original)
	oldVariants := []string{
		baseFilename + ".jpg",
		baseFilename + "_thumb.jpg",
		baseFilename + "_medium.jpg",
		baseFilename + "_small.jpg",
		baseFilename + "_xlarge.jpg",
		baseFilename + "_xxlarge.jpg",
	}

	for _, variant := range oldVariants {
		os.Remove(variant) // Ignore errors - files may not exist
	}
}

// GetPhotoProcessingStats returns statistics about photo processing queue
func GetPhotoProcessingStats(ctx *vbeam.Context, req Empty) (resp ProcessingStats, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	// Get processing statistics from photo worker
	resp = GetProcessingStats()
	return
}

// GetAnalysisStats returns live stats from the face analysis worker
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

// ReanalyzeAllPhotos queues all pending/failed photos for face analysis
func ReanalyzeAllPhotos(ctx *vbeam.Context, req ReanalyzeAllPhotosRequest) (resp ReanalyzeAllPhotosResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	// Face analysis is optional and often absent — a local build has no worker
	// at all, and a release build skips it when the daemon socket is not
	// reachable. Queueing into nothing used to report a confident count of
	// photos that would never be looked at, so say so instead.
	if !GetAnalysisWorkerStats().IsRunning {
		err = ErrFaceAnalysisUnavailable
		return
	}

	// Collect images that need (re)analysis
	var toQueue []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if image.AnalysisStatus == 0 || image.AnalysisStatus == 3 {
			toQueue = append(toQueue, image)
		} else {
			resp.Skipped++
		}
		return true
	})

	// For failed photos reset status to pending so stats stay consistent
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

// Log-related types and structures

type LogFileInfo struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	IsToday    bool      `json:"isToday"`
	SizeString string    `json:"sizeString"`
}

type GetLogFilesResponse struct {
	Files []LogFileInfo `json:"files"`
}

type GetLogContentRequest struct {
	Filename    string `json:"filename"`
	Level       string `json:"level,omitempty"`       // Filter by log level
	Category    string `json:"category,omitempty"`    // Filter by category
	Limit       int    `json:"limit,omitempty"`       // Limit number of entries (default 1000)
	Offset      int    `json:"offset,omitempty"`      // Skip entries (for pagination)
	MinDuration *int   `json:"minDuration,omitempty"` // Minimum duration in microseconds
	SortBy      string `json:"sortBy,omitempty"`      // Sort by: "time" or "duration"
	SortDesc    *bool  `json:"sortDesc,omitempty"`    // Sort descending (default: false)
}

type GetLogContentResponse struct {
	Entries    []PublicLogEntry `json:"entries"`
	TotalLines int              `json:"totalLines"`
	HasMore    bool             `json:"hasMore"`
}

// Public log entry for API responses
type PublicLogEntry struct {
	Timestamp string      `json:"timestamp"`
	Level     string      `json:"level"`
	Category  string      `json:"category"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	UserID    *int        `json:"userId,omitempty"`
	IP        string      `json:"ip,omitempty"`
	UserAgent string      `json:"userAgent,omitempty"`
	// HTTP timing fields for performance analysis
	Duration        *int   `json:"duration,omitempty"`        // Total duration in microseconds
	HandlerDuration *int   `json:"handlerDuration,omitempty"` // Handler duration in microseconds
	HTTPMethod      string `json:"httpMethod,omitempty"`      // HTTP method (GET, POST, etc.)
	HTTPPath        string `json:"httpPath,omitempty"`        // HTTP path
	HTTPStatus      *int   `json:"httpStatus,omitempty"`      // HTTP status code
	StackTrace      string `json:"stackTrace,omitempty"`
}

// convertToPublicLogEntry converts internal logEntry to public API format
func convertToPublicLogEntry(entry logEntry) PublicLogEntry {
	return PublicLogEntry{
		Timestamp:       entry.Timestamp.Format(time.RFC3339),
		Level:           string(entry.Level),
		Category:        string(entry.Category),
		Message:         entry.Message,
		Data:            entry.Data,
		UserID:          entry.UserID,
		IP:              entry.IP,
		UserAgent:       entry.UserAgent,
		Duration:        entry.Duration,
		HandlerDuration: entry.HandlerDuration,
		HTTPMethod:      entry.HTTPMethod,
		HTTPPath:        entry.HTTPPath,
		HTTPStatus:      entry.HTTPStatus,
		StackTrace:      entry.StackTrace,
	}
}

type LogStats struct {
	TotalFiles int              `json:"totalFiles"`
	TotalSize  int64            `json:"totalSize"`
	ByLevel    map[string]int   `json:"byLevel"`
	ByCategory map[string]int   `json:"byCategory"`
	Recent     []PublicLogEntry `json:"recent"` // Last 10 entries
	Errors     []PublicLogEntry `json:"errors"` // Recent errors
	// Performance statistics
	PerformanceStats PerformanceStats `json:"performanceStats"`
}

type PerformanceStats struct {
	TotalRequests    int                      `json:"totalRequests"`
	AverageResponse  int                      `json:"averageResponse"`  // In microseconds
	MedianResponse   int                      `json:"medianResponse"`   // In microseconds
	P90Response      int                      `json:"p90Response"`      // In microseconds
	P95Response      int                      `json:"p95Response"`      // In microseconds
	P99Response      int                      `json:"p99Response"`      // In microseconds
	SlowestEndpoints []EndpointStats          `json:"slowestEndpoints"` // Top 10 slowest
	EndpointStats    map[string]EndpointStats `json:"endpointStats"`    // Stats by endpoint
}

type EndpointStats struct {
	Path            string  `json:"path"`
	Method          string  `json:"method"`
	Count           int     `json:"count"`
	AverageResponse int     `json:"averageResponse"` // In microseconds
	MinResponse     int     `json:"minResponse"`     // In microseconds
	MaxResponse     int     `json:"maxResponse"`     // In microseconds
	ErrorRate       float64 `json:"errorRate"`       // Percentage
}

type GetLogStatsResponse struct {
	Stats LogStats `json:"stats"`
}

// GetLogFiles returns list of available log files
func GetLogFiles(ctx *vbeam.Context, req Empty) (resp GetLogFilesResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	files, listErr := listLogFiles()
	if listErr != nil {
		err = ProcError(listErr)
		return
	}
	resp.Files = files
	return
}

// GetLogContent returns filtered log content from a specific file
func GetLogContent(ctx *vbeam.Context, req GetLogContentRequest) (resp GetLogContentResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	path, pathErr := logFilePath(req.Filename)
	if pathErr != nil {
		err = pathErr
		return
	}

	if req.Limit <= 0 {
		req.Limit = 1000
	}

	keep := func(entry logEntry) bool {
		if req.Level != "" && string(entry.Level) != req.Level {
			return false
		}
		if req.Category != "" && string(entry.Category) != req.Category {
			return false
		}
		if req.MinDuration != nil && (entry.Duration == nil || *entry.Duration < *req.MinDuration) {
			return false
		}
		return true
	}

	var matched []logEntry
	// A read failure names the path it tried, which is a directory layout
	// nobody outside the box needs; ProcError trades it for a reference code.
	if scanErr := scanLogFile(path, func(entry logEntry) bool {
		if keep(entry) {
			matched = append(matched, entry)
		}
		return true
	}); scanErr != nil {
		err = ProcError(scanErr)
		return
	}

	sortLogEntries(matched, req.SortBy, req.SortDesc)

	total := len(matched)
	startIdx := min(req.Offset, total)
	endIdx := min(req.Offset+req.Limit, total)

	var publicEntries []PublicLogEntry
	for i := startIdx; i < endIdx; i++ {
		publicEntries = append(publicEntries, convertToPublicLogEntry(matched[i]))
	}

	resp.Entries = publicEntries
	resp.TotalLines = total
	resp.HasMore = endIdx < total
	return
}

// sortLogEntries applies the request's ordering. The default — no sortBy — is
// the order the entries appear in the file, which is chronological.
func sortLogEntries(entries []logEntry, sortBy string, sortDesc *bool) {
	desc := sortDesc != nil && *sortDesc

	switch sortBy {
	case "duration":
		sort.SliceStable(entries, func(i, j int) bool {
			// Entries with no duration are not slow or fast; they sort last
			// either way rather than clustering at whichever end zero is.
			a, b := entries[i].Duration, entries[j].Duration
			if a == nil || b == nil {
				return a != nil && b == nil
			}
			if desc {
				return *a > *b
			}
			return *a < *b
		})
	case "time":
		sort.SliceStable(entries, func(i, j int) bool {
			if desc {
				return entries[i].Timestamp.After(entries[j].Timestamp)
			}
			return entries[i].Timestamp.Before(entries[j].Timestamp)
		})
	}
}

// GetLogStats returns summary statistics about logs.
//
// Every statistic here used to be derived from the last 50 lines of each file:
// the level and category histograms, the error list, the request count, and the
// p50/p90/p95/p99 latency percentiles. On a normal day that is well under a
// minute of traffic, presented as a performance summary. It reads the files
// through now — this is an admin page loaded by one person, and §2.1 keeps a
// day's traffic in one file.
func GetLogStats(ctx *vbeam.Context, req Empty) (resp GetLogStatsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	stats := LogStats{
		ByLevel:    make(map[string]int),
		ByCategory: make(map[string]int),
		Recent:     []PublicLogEntry{},
		Errors:     []PublicLogEntry{},
		PerformanceStats: PerformanceStats{
			EndpointStats:    make(map[string]EndpointStats),
			SlowestEndpoints: []EndpointStats{},
		},
	}

	files, listErr := listLogFiles()
	if listErr != nil {
		err = ProcError(listErr)
		return
	}

	// Recent entries and the error list are the tail of the whole corpus, so
	// they are collected in ring buffers rather than by materialising every
	// entry in every file and sorting it.
	recent := newEntryRing(statsRecentEntries)
	recentErrors := newEntryRing(statsRecentErrors)
	perf := newPerfAccumulator()

	for _, file := range files {
		stats.TotalFiles++
		stats.TotalSize += file.Size

		scanErr := scanLogFile(filepath.Join(cfg.LogDir, file.Name), func(entry logEntry) bool {
			stats.ByLevel[string(entry.Level)]++
			stats.ByCategory[string(entry.Category)]++
			if entry.Level == logLevelError {
				recentErrors.add(entry)
			}
			recent.add(entry)
			perf.add(entry)
			return true
		})
		if scanErr != nil {
			// One unreadable file should not blank the whole page.
			LogErrorSimple(LogCategoryAdmin, "Could not read a log file for stats", map[string]interface{}{
				"file":  file.Name,
				"error": scanErr.Error(),
			})
		}
	}

	stats.PerformanceStats = perf.result()

	// Newest first: when this list is short, it is the last thing that happened.
	for _, entry := range recent.newestFirst() {
		stats.Recent = append(stats.Recent, convertToPublicLogEntry(entry))
	}
	for _, entry := range recentErrors.newestFirst() {
		stats.Errors = append(stats.Errors, convertToPublicLogEntry(entry))
	}

	resp.Stats = stats
	return
}
