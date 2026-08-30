package backend

import (
	"family/cfg"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

const AdminUserId = 1

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
	RegisterPhotoMaintenanceMethods(app)
	vbeam.RegisterProc(app, GetLogFiles)
	vbeam.RegisterProc(app, GetLogContent)
	vbeam.RegisterProc(app, LookupLogReference)
	vbeam.RegisterProc(app, GetLogStats)
	vbeam.RegisterProc(app, GetPushStatus)
	vbeam.RegisterProc(app, ListPushDevices)
	vbeam.RegisterProc(app, SendTestPushNotification)
	RegisterAdminSeedMethods(app)
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

func ListAllUsers(ctx *vbeam.Context, req Empty) (resp ListAllUsersResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	LogInfo(LogCategoryAdmin, "Admin accessed user list", map[string]interface{}{
		"adminUserId": AdminUserId,
	})

	var users []User
	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		users = append(users, user)
		return true
	})

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
			IsAdmin:    u.Id == 1,
		}
		resp.Users = append(resp.Users, adminUser)
	}

	return
}

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
	Level       string `json:"level,omitempty"`
	Category    string `json:"category,omitempty"`
	Search      string `json:"search,omitempty"`
	SinceHours  int    `json:"sinceHours,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
	MinDuration *int   `json:"minDuration,omitempty"`
	SortBy      string `json:"sortBy,omitempty"`
	SortDesc    *bool  `json:"sortDesc,omitempty"`
}

type GetLogContentResponse struct {
	Entries       []PublicLogEntry `json:"entries"`
	TotalLines    int              `json:"totalLines"`
	HasMore       bool             `json:"hasMore"`
	FilesSearched []string         `json:"filesSearched"`
}

type PublicLogEntry struct {
	Timestamp       string      `json:"timestamp"`
	Level           string      `json:"level"`
	Category        string      `json:"category"`
	Message         string      `json:"message"`
	Data            interface{} `json:"data,omitempty"`
	UserID          *int        `json:"userId,omitempty"`
	IP              string      `json:"ip,omitempty"`
	UserAgent       string      `json:"userAgent,omitempty"`
	Duration        *int        `json:"duration,omitempty"`
	HandlerDuration *int        `json:"handlerDuration,omitempty"`
	HTTPMethod      string      `json:"httpMethod,omitempty"`
	HTTPPath        string      `json:"httpPath,omitempty"`
	HTTPStatus      *int        `json:"httpStatus,omitempty"`
	StackTrace      string      `json:"stackTrace,omitempty"`
}

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
	TotalFiles       int              `json:"totalFiles"`
	TotalSize        int64            `json:"totalSize"`
	ByLevel          map[string]int   `json:"byLevel"`
	ByCategory       map[string]int   `json:"byCategory"`
	Recent           []PublicLogEntry `json:"recent"`
	Errors           []PublicLogEntry `json:"errors"`
	PerformanceStats PerformanceStats `json:"performanceStats"`
}

type PerformanceStats struct {
	TotalRequests    int                      `json:"totalRequests"`
	AverageResponse  int                      `json:"averageResponse"`
	MedianResponse   int                      `json:"medianResponse"`
	P90Response      int                      `json:"p90Response"`
	P95Response      int                      `json:"p95Response"`
	P99Response      int                      `json:"p99Response"`
	SlowestEndpoints []EndpointStats          `json:"slowestEndpoints"`
	EndpointStats    map[string]EndpointStats `json:"endpointStats"`
}

type EndpointStats struct {
	Path            string  `json:"path"`
	Method          string  `json:"method"`
	Count           int     `json:"count"`
	AverageResponse int     `json:"averageResponse"`
	MinResponse     int     `json:"minResponse"`
	MaxResponse     int     `json:"maxResponse"`
	ErrorRate       float64 `json:"errorRate"`
}

type GetLogStatsResponse struct {
	Stats LogStats `json:"stats"`
}

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

func GetLogContent(ctx *vbeam.Context, req GetLogContentRequest) (resp GetLogContentResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	paths, names, pathErr := resolveLogTargets(req.Filename)
	if pathErr != nil {
		err = pathErr
		return
	}

	if req.Limit <= 0 {
		req.Limit = 1000
	}

	filter := newLogFilter(req)

	var matched []logEntry
	for _, path := range paths {
		if scanErr := scanLogFile(path, func(entry logEntry) bool {
			if filter.keep(entry) {
				matched = append(matched, entry)
			}
			return true
		}); scanErr != nil {
			if req.Filename != "" {
				err = ProcError(scanErr)
				return
			}
			LogErrorSimple(LogCategoryAdmin, "Could not read a log file while searching", map[string]interface{}{
				"path":  path,
				"error": scanErr.Error(),
			})
		}
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
	resp.FilesSearched = names
	return
}

type LookupLogReferenceRequest struct {
	Reference string `json:"reference"`
	Context   int    `json:"context,omitempty"`
}

type LookupLogReferenceResponse struct {
	Found         bool             `json:"found"`
	File          string           `json:"file"`
	Entry         PublicLogEntry   `json:"entry"`
	Before        []PublicLogEntry `json:"before"`
	After         []PublicLogEntry `json:"after"`
	FilesSearched []string         `json:"filesSearched"`
}

func LookupLogReference(ctx *vbeam.Context, req LookupLogReferenceRequest) (resp LookupLogReferenceResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	reference := strings.TrimSpace(req.Reference)
	if idx := strings.Index(reference, ReferencePrefix); idx >= 0 {
		reference = strings.TrimSpace(reference[idx+len(ReferencePrefix):])
	}
	if reference == "" {
		err = errEmptyReference
		return
	}

	context := req.Context
	if context <= 0 {
		context = 10
	}
	if context > 100 {
		context = 100
	}

	paths, names, pathErr := resolveLogTargets("")
	if pathErr != nil {
		err = pathErr
		return
	}
	resp.FilesSearched = names

	for i, path := range paths {
		before := newEntryRing(context)
		var match *logEntry
		var after []logEntry

		scanErr := scanLogFile(path, func(entry logEntry) bool {
			if match != nil {
				after = append(after, entry)
				return len(after) < context
			}
			if entryHasReference(entry, reference) {
				found := entry
				match = &found
				return true
			}
			before.add(entry)
			return true
		})
		if scanErr != nil {
			LogErrorSimple(LogCategoryAdmin, "Could not read a log file while looking up a reference", map[string]interface{}{
				"path":  path,
				"error": scanErr.Error(),
			})
			continue
		}

		if match == nil {
			continue
		}

		resp.Found = true
		resp.File = names[i]
		resp.Entry = convertToPublicLogEntry(*match)
		preceding := before.newestFirst()
		for j := len(preceding) - 1; j >= 0; j-- {
			resp.Before = append(resp.Before, convertToPublicLogEntry(preceding[j]))
		}
		for _, entry := range after {
			resp.After = append(resp.After, convertToPublicLogEntry(entry))
		}
		return
	}

	return
}

func sortLogEntries(entries []logEntry, sortBy string, sortDesc *bool) {
	desc := true
	if sortDesc != nil {
		desc = *sortDesc
	}

	switch sortBy {
	case "duration":
		if sortDesc == nil {
			desc = true
		}
		sort.SliceStable(entries, func(i, j int) bool {
			a, b := entries[i].Duration, entries[j].Duration
			if a == nil || b == nil {
				return a != nil && b == nil
			}
			if desc {
				return *a > *b
			}
			return *a < *b
		})
	default:
		sort.SliceStable(entries, func(i, j int) bool {
			if desc {
				return entries[i].Timestamp.After(entries[j].Timestamp)
			}
			return entries[i].Timestamp.Before(entries[j].Timestamp)
		})
	}
}

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
			LogErrorSimple(LogCategoryAdmin, "Could not read a log file for stats", map[string]interface{}{
				"file":  file.Name,
				"error": scanErr.Error(),
			})
		}
	}

	stats.PerformanceStats = perf.result()

	for _, entry := range recent.newestFirst() {
		stats.Recent = append(stats.Recent, convertToPublicLogEntry(entry))
	}
	for _, entry := range recentErrors.newestFirst() {
		stats.Errors = append(stats.Errors, convertToPublicLogEntry(entry))
	}

	resp.Stats = stats
	return
}
