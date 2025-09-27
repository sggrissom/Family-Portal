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
	Processed    int      `json:"processed"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors"`
	TotalTime    string   `json:"totalTime"`
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
	Filename string `json:"filename"`
	Level    string `json:"level,omitempty"`    // Filter by log level
	Category string `json:"category,omitempty"` // Filter by category
	Limit    int    `json:"limit,omitempty"`    // Limit number of entries (default 1000)
	Offset   int    `json:"offset,omitempty"`   // Skip entries (for pagination)
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
}

// convertToPublicLogEntry converts internal logEntry to public API format
func convertToPublicLogEntry(entry logEntry) PublicLogEntry {
	return PublicLogEntry{
		Timestamp: entry.Timestamp.Format(time.RFC3339),
		Level:     string(entry.Level),
		Category:  string(entry.Category),
		Message:   entry.Message,
		Data:      entry.Data,
		UserID:    entry.UserID,
		IP:        entry.IP,
		UserAgent: entry.UserAgent,
	}
}

type LogStats struct {
	TotalFiles int                     `json:"totalFiles"`
	TotalSize  int64                   `json:"totalSize"`
	ByLevel    map[string]int          `json:"byLevel"`
	ByCategory map[string]int          `json:"byCategory"`
	Recent     []PublicLogEntry        `json:"recent"`     // Last 10 entries
	Errors     []PublicLogEntry        `json:"errors"`     // Recent errors
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

	var publicEntries []PublicLogEntry
	scanner := bufio.NewScanner(file)
	totalLines := 0
	skipped := 0

	for scanner.Scan() {
		totalLines++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip lines for pagination
		if skipped < req.Offset {
			skipped++
			continue
		}

		// Stop if we've reached our limit
		if len(publicEntries) >= req.Limit {
			break
		}

		// Try to parse as JSON log entry
		var entry logEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// If not JSON, parse as plain text log
			cleanLine := stripAnsiCodes(line)
			timestamp, message := parseLogTimestamp(cleanLine)
			category := categorizeLogMessage(message)

			entry = logEntry{
				Timestamp: timestamp,
				Level:     logLevelInfo,
				Category:  category,
				Message:   message,
			}
		}

		// Apply filters
		if req.Level != "" && string(entry.Level) != req.Level {
			continue
		}
		if req.Category != "" && string(entry.Category) != req.Category {
			continue
		}

		publicEntries = append(publicEntries, convertToPublicLogEntry(entry))
	}

	if err := scanner.Err(); err != nil {
		return resp, err
	}

	resp.Entries = publicEntries
	resp.TotalLines = totalLines
	resp.HasMore = totalLines > (req.Offset + len(publicEntries))

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
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// If not JSON, parse as plain text log
			cleanLine := stripAnsiCodes(line)
			timestamp, message := parseLogTimestamp(cleanLine)
			category := categorizeLogMessage(message)

			entry = logEntry{
				Timestamp: timestamp,
				Level:     logLevelInfo,
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

