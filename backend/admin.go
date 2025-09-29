package backend

import (
	"bufio"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAdminMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListAllUsers)
	vbeam.RegisterProc(app, GetPhotoStats)
	vbeam.RegisterProc(app, ReprocessAllPhotos)
	vbeam.RegisterProc(app, GetPhotoProcessingStats)
	vbeam.RegisterProc(app, GetLogFiles)
	vbeam.RegisterProc(app, GetLogContent)
	vbeam.RegisterProc(app, GetLogStats)
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
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Log admin action
	LogInfo(LogCategoryAdmin, "Admin accessed user list", map[string]interface{}{
		"adminUserId": user.Id,
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
}

type ReprocessAllPhotosRequest struct{}

type ReprocessAllPhotosResponse struct {
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
	TotalTime string   `json:"totalTime"`
}

// Get photo statistics for admin dashboard
func GetPhotoStats(ctx *vbeam.Context, req GetPhotoStatsRequest) (resp GetPhotoStatsResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Count all photos
	var allPhotos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		allPhotos = append(allPhotos, image)
		return true
	})

	resp.TotalPhotos = len(allPhotos)

	// Count processed photos (those with modern format variants)
	processedCount := 0
	for _, photo := range allPhotos {
		if isPhotoProcessed(photo) {
			processedCount++
		}
	}

	resp.ProcessedPhotos = processedCount
	resp.PendingPhotos = resp.TotalPhotos - resp.ProcessedPhotos

	return
}

// Reprocess all photos with modern formats and responsive sizes
func ReprocessAllPhotos(ctx *vbeam.Context, req ReprocessAllPhotosRequest) (resp ReprocessAllPhotosResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	vbeam.UseWriteTx(ctx)
	startTime := time.Now()

	// Get all photos that need reprocessing
	var photosToProcess []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if !isPhotoProcessed(image) {
			photosToProcess = append(photosToProcess, image)
		}
		return true
	})

	var errors []string
	processed := 0
	failed := 0

	for _, photo := range photosToProcess {
		// Update status to processing
		photo.Status = 1 // Processing
		vbolt.Write(ctx.Tx, ImagesBkt, photo.Id, &photo)

		// Attempt to reprocess
		if reprocessErr := reprocessSinglePhoto(photo); reprocessErr != nil {
			failed++
			errors = append(errors, fmt.Sprintf("Photo %d: %v", photo.Id, reprocessErr))
			// Mark as failed/needs reprocessing
			photo.Status = 2
		} else {
			processed++
			// Mark as successfully processed
			photo.Status = 0
		}

		// Update final status
		vbolt.Write(ctx.Tx, ImagesBkt, photo.Id, &photo)
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Processed = processed
	resp.Failed = failed
	resp.Errors = errors
	resp.TotalTime = time.Since(startTime).String()

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

// Helper function to reprocess a single photo
func reprocessSinglePhoto(photo Image) error {
	// Read the original file
	originalPath := getOriginalPhotoPath(photo)
	originalData, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	// Reprocess with modern formats and sizes
	processedImages, _, _, err := ProcessAndSaveMultipleSizes(originalData, photo.MimeType)
	if err != nil {
		return fmt.Errorf("failed to process image: %w", err)
	}

	// Save all variants to disk
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	// Clean up old variants first (except original)
	cleanupOldVariants(baseFilename)

	// Save new variants
	for key, data := range processedImages {
		parts := strings.Split(key, "_")
		if len(parts) != 2 {
			continue
		}
		sizeName, format := parts[0], parts[1]

		var ext string
		switch format {
		case "webp":
			ext = ".webp"
		case "avif":
			ext = ".avif"
		case "png":
			ext = ".png"
		default:
			ext = ".jpg"
		}

		var fileName string
		if sizeName == "large" {
			fileName = baseFilename + ext
		} else {
			fileName = baseFilename + "_" + sizeName + ext
		}

		if err := os.WriteFile(fileName, data, 0644); err != nil {
			return fmt.Errorf("failed to save variant %s: %w", fileName, err)
		}
	}

	return nil
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
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Get processing statistics from photo worker
	resp = GetProcessingStats()
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
	// Check admin authentication
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Get log files from logs directory
	logDir := "logs"
	files, err := os.ReadDir(logDir)
	if err != nil {
		// If logs directory doesn't exist, return empty list
		if os.IsNotExist(err) {
			resp.Files = []LogFileInfo{}
			err = nil
			return
		}
		return
	}

	today := time.Now().Format("2006-01-02")
	var logFiles []LogFileInfo

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".log") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		// Check if this is today's log file
		// Could be family_portal-YYYY-MM-DD.log or family_portal.log (current day)
		isToday := strings.Contains(file.Name(), today) ||
			(file.Name() == "family_portal.log" && info.ModTime().Format("2006-01-02") == today)

		logFile := LogFileInfo{
			Name:       file.Name(),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			IsToday:    isToday,
			SizeString: formatFileSize(info.Size()),
		}

		logFiles = append(logFiles, logFile)
	}

	// Sort by modification time (newest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime.After(logFiles[j].ModTime)
	})

	resp.Files = logFiles
	return
}

// Helper functions for parsing non-JSON log lines
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripAnsiCodes removes ANSI escape sequences from a string
func stripAnsiCodes(input string) string {
	return ansiEscapeRegex.ReplaceAllString(input, "")
}

// extractJSONFromLogLine attempts to extract JSON from a timestamp-prefixed log line
func extractJSONFromLogLine(line string) (string, bool) {
	// Check if line contains JSON (starts with timestamp, then has JSON)
	if idx := strings.Index(line, "{"); idx != -1 {
		jsonPart := line[idx:]
		// Verify this looks like JSON by checking if it ends with }
		if strings.HasSuffix(strings.TrimSpace(jsonPart), "}") {
			return jsonPart, true
		}
	}
	return line, false
}

// parseLogTimestamp attempts to parse a timestamp from a plain text log line
func parseLogTimestamp(line string) (time.Time, string) {
	// Try to match the Go log format: 2025/09/26 15:53:22
	timestampRegex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+(.*)`)
	matches := timestampRegex.FindStringSubmatch(line)

	if len(matches) == 3 {
		if timestamp, err := time.Parse("2006/01/02 15:04:05", matches[1]); err == nil {
			return timestamp, matches[2] // Return timestamp and remaining message
		}
	}

	// If no timestamp found, return current time and original line
	return time.Now(), line
}

// detectLogLevel attempts to detect log level from plain text log message
func detectLogLevel(message string) logLevel {
	upperMessage := strings.ToUpper(message)

	// Check for error indicators
	errorKeywords := []string{"ERROR", "FATAL", "PANIC", "FAILED", "FAILURE", "EXCEPTION", "CRITICAL"}
	for _, keyword := range errorKeywords {
		if strings.Contains(upperMessage, keyword) {
			return logLevelError
		}
	}

	// Check for warning indicators
	warnKeywords := []string{"WARN", "WARNING", "DEPRECATED"}
	for _, keyword := range warnKeywords {
		if strings.Contains(upperMessage, keyword) {
			return logLevelWarn
		}
	}

	// Check for debug indicators
	debugKeywords := []string{"DEBUG", "TRACE", "VERBOSE"}
	for _, keyword := range debugKeywords {
		if strings.Contains(upperMessage, keyword) {
			return logLevelDebug
		}
	}

	// Default to info
	return logLevelInfo
}

// categorizeLogMessage attempts to categorize a plain text log message
func categorizeLogMessage(message string) logCategory {
	message = strings.ToUpper(message)

	if strings.Contains(message, "PHOTO") || strings.Contains(message, "IMAGE") {
		return logCategoryPhoto
	}
	if strings.Contains(message, "AUTH") || strings.Contains(message, "LOGIN") {
		return logCategoryAuth
	}
	if strings.Contains(message, "ADMIN") {
		return logCategoryAdmin
	}
	if strings.Contains(message, "API") || strings.Contains(message, "RPC") || strings.Contains(message, "GET") || strings.Contains(message, "POST") {
		return logCategoryAPI
	}
	if strings.Contains(message, "WORKER") || strings.Contains(message, "PROCESSING") || strings.Contains(message, "QUEUE") {
		return logCategoryWorker
	}

	return logCategorySystem
}

// parseTimingLogLine attempts to parse HTTP timing log entries
// Format: "2025/09/27 17:31:28 200 POST /rpc/SendMessage ⎯⎯⎯ 12759µs [12602µs]"
func parseTimingLogLine(line string) (*logEntry, bool) {
	// Clean ANSI codes first
	cleanLine := stripAnsiCodes(line)

	// Pattern to match timing logs: timestamp, status, method, path, duration, optional handler duration
	timingRegex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+(\d+)\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s⎯]+).*?(\d+)µs(?:\s+\[(\d+)µs\])?`)
	matches := timingRegex.FindStringSubmatch(cleanLine)

	if len(matches) < 6 {
		return nil, false
	}

	// Parse timestamp
	timestamp, err := time.Parse("2006/01/02 15:04:05", matches[1])
	if err != nil {
		return nil, false
	}

	// Parse status code
	status, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, false
	}

	// Parse duration
	duration, err := strconv.Atoi(matches[5])
	if err != nil {
		return nil, false
	}

	// Parse handler duration if present
	var handlerDuration *int
	if len(matches) > 6 && matches[6] != "" {
		if hd, err := strconv.Atoi(matches[6]); err == nil {
			handlerDuration = &hd
		}
	}

	// Build log entry
	entry := &logEntry{
		Timestamp:       timestamp,
		Level:           logLevelInfo,   // HTTP timing logs are info level
		Category:        logCategoryAPI, // HTTP requests are API category
		Message:         fmt.Sprintf("%s %s %s", matches[3], matches[4], matches[2]),
		Duration:        &duration,
		HandlerDuration: handlerDuration,
		HTTPMethod:      matches[3],
		HTTPPath:        matches[4],
		HTTPStatus:      &status,
	}

	return entry, true
}

// GetLogContent returns filtered log content from a specific file
func GetLogContent(ctx *vbeam.Context, req GetLogContentRequest) (resp GetLogContentResponse, err error) {
	// Check admin authentication
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Validate filename (security check)
	if strings.Contains(req.Filename, "..") || strings.Contains(req.Filename, "/") {
		err = errors.New("Invalid filename")
		return
	}

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 1000
	}

	// Read log file
	logPath := filepath.Join("logs", req.Filename)
	file, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer file.Close()

	// First pass: parse all entries and apply filters to get total count
	var allFilteredEntries []logEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON log entry
		var entry logEntry
		jsonPart, hasJSON := extractJSONFromLogLine(line)

		if hasJSON {
			// Try parsing the extracted JSON
			if err := json.Unmarshal([]byte(jsonPart), &entry); err == nil {
				// JSON parsing succeeded, use the parsed entry
			} else {
				// JSON extraction found brackets but parsing failed, treat as plain text
				hasJSON = false
			}
		}

		if !hasJSON {
			// Try to parse as timing log first
			if timingEntry, isTiming := parseTimingLogLine(line); isTiming {
				entry = *timingEntry
			} else {
				// If not timing log, parse as plain text log
				cleanLine := stripAnsiCodes(line)
				timestamp, message := parseLogTimestamp(cleanLine)
				category := categorizeLogMessage(message)
				level := detectLogLevel(message)

				entry = logEntry{
					Timestamp: timestamp,
					Level:     level,
					Category:  category,
					Message:   message,
				}
			}
		}

		// Apply filters
		if req.Level != "" && string(entry.Level) != req.Level {
			continue
		}
		if req.Category != "" && string(entry.Category) != req.Category {
			continue
		}
		if req.MinDuration != nil && (entry.Duration == nil || *entry.Duration < *req.MinDuration) {
			continue
		}

		allFilteredEntries = append(allFilteredEntries, entry)
	}

	if err := scanner.Err(); err != nil {
		return resp, err
	}

	// Sort entries if requested
	if req.SortBy == "duration" {
		sort.Slice(allFilteredEntries, func(i, j int) bool {
			// Handle nil durations - put entries without duration at the end
			if allFilteredEntries[i].Duration == nil && allFilteredEntries[j].Duration == nil {
				return false // Equal, keep original order
			}
			if allFilteredEntries[i].Duration == nil {
				return false // i goes after j
			}
			if allFilteredEntries[j].Duration == nil {
				return true // i goes before j
			}

			// Both have durations, compare them
			if req.SortDesc != nil && *req.SortDesc {
				return *allFilteredEntries[i].Duration > *allFilteredEntries[j].Duration // Descending
			}
			return *allFilteredEntries[i].Duration < *allFilteredEntries[j].Duration // Ascending
		})
	} else if req.SortBy == "time" && req.SortDesc != nil && *req.SortDesc {
		// Sort by time descending (most recent first)
		sort.Slice(allFilteredEntries, func(i, j int) bool {
			return allFilteredEntries[i].Timestamp.After(allFilteredEntries[j].Timestamp)
		})
	}
	// Default is chronological order (ascending by time), no sorting needed

	// Calculate pagination based on filtered results
	totalFilteredLines := len(allFilteredEntries)
	startIdx := req.Offset
	endIdx := req.Offset + req.Limit

	// Ensure we don't go beyond available entries
	if startIdx > totalFilteredLines {
		startIdx = totalFilteredLines
	}
	if endIdx > totalFilteredLines {
		endIdx = totalFilteredLines
	}

	// Extract the requested page
	var publicEntries []PublicLogEntry
	if startIdx < totalFilteredLines {
		for i := startIdx; i < endIdx; i++ {
			publicEntries = append(publicEntries, convertToPublicLogEntry(allFilteredEntries[i]))
		}
	}

	resp.Entries = publicEntries
	resp.TotalLines = totalFilteredLines
	resp.HasMore = endIdx < totalFilteredLines

	return
}

// GetLogStats returns summary statistics about logs
func GetLogStats(ctx *vbeam.Context, req Empty) (resp GetLogStatsResponse, err error) {
	// Check admin authentication
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
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

	// Get log files
	logDir := "logs"
	files, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			resp.Stats = stats
			err = nil
			return
		}
		return
	}

	var totalSize int64
	var allRecentEntries []logEntry

	// Process each log file
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".log") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		stats.TotalFiles++
		totalSize += info.Size()

		// Read recent entries from the file (last few lines)
		entries := readRecentLogEntries(filepath.Join(logDir, file.Name()), 50)
		allRecentEntries = append(allRecentEntries, entries...)

		// Count levels and categories
		for _, entry := range entries {
			stats.ByLevel[string(entry.Level)]++
			stats.ByCategory[string(entry.Category)]++

			// Collect errors
			if entry.Level == logLevelError && len(stats.Errors) < 10 {
				stats.Errors = append(stats.Errors, convertToPublicLogEntry(entry))
			}
		}
	}

	stats.TotalSize = totalSize

	// Calculate performance statistics
	calculatePerformanceStats(&stats.PerformanceStats, allRecentEntries)

	// Sort all recent entries by timestamp and take the most recent
	sort.Slice(allRecentEntries, func(i, j int) bool {
		return allRecentEntries[i].Timestamp.After(allRecentEntries[j].Timestamp)
	})

	// Convert to public format
	var publicRecentEntries []PublicLogEntry
	for _, entry := range allRecentEntries {
		publicRecentEntries = append(publicRecentEntries, convertToPublicLogEntry(entry))
	}

	if len(publicRecentEntries) > 10 {
		stats.Recent = publicRecentEntries[:10]
	} else {
		stats.Recent = publicRecentEntries
	}

	resp.Stats = stats
	return
}

// calculatePerformanceStats computes performance metrics from log entries
func calculatePerformanceStats(perfStats *PerformanceStats, entries []logEntry) {
	var durations []int
	endpointData := make(map[string][]endpointRequest)

	// Collect timing data from entries
	for _, entry := range entries {
		if entry.Duration != nil && *entry.Duration > 0 {
			durations = append(durations, *entry.Duration)
			perfStats.TotalRequests++

			// Group by endpoint if we have HTTP method and path
			if entry.HTTPMethod != "" && entry.HTTPPath != "" {
				key := entry.HTTPMethod + " " + entry.HTTPPath
				endpointData[key] = append(endpointData[key], endpointRequest{
					Duration:   *entry.Duration,
					StatusCode: entry.HTTPStatus,
				})
			}
		}
	}

	if len(durations) == 0 {
		return
	}

	// Sort durations for percentile calculations
	sort.Ints(durations)

	// Calculate overall statistics
	perfStats.AverageResponse = calculateAverage(durations)
	perfStats.MedianResponse = calculatePercentile(durations, 50)
	perfStats.P90Response = calculatePercentile(durations, 90)
	perfStats.P95Response = calculatePercentile(durations, 95)
	perfStats.P99Response = calculatePercentile(durations, 99)

	// Calculate endpoint statistics
	var slowestEndpoints []EndpointStats
	for endpoint, requests := range endpointData {
		stats := calculateEndpointStats(endpoint, requests)
		perfStats.EndpointStats[endpoint] = stats
		slowestEndpoints = append(slowestEndpoints, stats)
	}

	// Sort endpoints by average response time and take top 10
	sort.Slice(slowestEndpoints, func(i, j int) bool {
		return slowestEndpoints[i].AverageResponse > slowestEndpoints[j].AverageResponse
	})

	if len(slowestEndpoints) > 10 {
		perfStats.SlowestEndpoints = slowestEndpoints[:10]
	} else {
		perfStats.SlowestEndpoints = slowestEndpoints
	}
}

// Helper struct for endpoint request data
type endpointRequest struct {
	Duration   int
	StatusCode *int
}

// calculateEndpointStats computes statistics for a specific endpoint
func calculateEndpointStats(endpoint string, requests []endpointRequest) EndpointStats {
	parts := strings.SplitN(endpoint, " ", 2)
	method := parts[0]
	path := ""
	if len(parts) > 1 {
		path = parts[1]
	}

	durations := make([]int, len(requests))
	errors := 0

	for i, req := range requests {
		durations[i] = req.Duration
		if req.StatusCode != nil && *req.StatusCode >= 400 {
			errors++
		}
	}

	sort.Ints(durations)

	return EndpointStats{
		Path:            path,
		Method:          method,
		Count:           len(requests),
		AverageResponse: calculateAverage(durations),
		MinResponse:     durations[0],
		MaxResponse:     durations[len(durations)-1],
		ErrorRate:       float64(errors) / float64(len(requests)) * 100,
	}
}

// calculateAverage computes the average of a slice of integers
func calculateAverage(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

// calculatePercentile computes the nth percentile of a sorted slice
func calculatePercentile(sortedValues []int, percentile int) int {
	if len(sortedValues) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sortedValues[0]
	}
	if percentile >= 100 {
		return sortedValues[len(sortedValues)-1]
	}

	index := float64(percentile) / 100.0 * float64(len(sortedValues)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sortedValues) {
		return sortedValues[lower]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return int(float64(sortedValues[lower])*(1-weight) + float64(sortedValues[upper])*weight)
}

// Helper function to format file sizes
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper function to read recent log entries from a file
func readRecentLogEntries(filepath string, maxEntries int) []logEntry {
	file, err := os.Open(filepath)
	if err != nil {
		return []logEntry{}
	}
	defer file.Close()

	var entries []logEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry logEntry
		jsonPart, hasJSON := extractJSONFromLogLine(line)

		if hasJSON {
			// Try parsing the extracted JSON
			if err := json.Unmarshal([]byte(jsonPart), &entry); err == nil {
				// JSON parsing succeeded, use the parsed entry
			} else {
				// JSON extraction found brackets but parsing failed, treat as plain text
				hasJSON = false
			}
		}

		if !hasJSON {
			// If no JSON found or JSON parsing failed, parse as plain text log
			cleanLine := stripAnsiCodes(line)
			timestamp, message := parseLogTimestamp(cleanLine)
			category := categorizeLogMessage(message)
			level := detectLogLevel(message)

			entry = logEntry{
				Timestamp: timestamp,
				Level:     level,
				Category:  category,
				Message:   message,
			}
		}

		entries = append(entries, entry)

		// Keep only the most recent entries (simple sliding window)
		if len(entries) > maxEntries {
			entries = entries[1:]
		}
	}

	return entries
}
