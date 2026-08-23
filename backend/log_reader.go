package backend

import (
	"bufio"
	"encoding/json"
	"family/cfg"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Reading the log files back is its own concern, and it used to live in the
// middle of admin.go: about six hundred lines of ANSI stripping, three fallback
// parse strategies, a timing-line regex and stack-trace continuation, with the
// line-accumulation loop written out twice — once in GetLogContent and once in
// readRecentLogEntries — differing only in whether the result was paginated or
// ring-buffered.
//
// The two copies had also drifted: only GetLogContent's tried parseTimingLogLine,
// so every duration in the file was invisible to GetLogStats, and the latency
// percentiles it presented were computed over a set that was always empty.
//
// There is one scanner now, scanLogFile, and both callers pass it a visitor.

// scanLogFile parses every entry in one log file and hands each to visit, in
// file order. Returning false from visit stops the scan.
//
// A log entry is one line plus any following lines that do not start with a
// timestamp — that is how a stack trace reaches the file — so an entry can only
// be emitted once the line after it has been read.
func scanLogFile(path string, visit func(logEntry) bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var pending logEntry
	var hasPending bool
	var stackLines []string
	stopped := false

	emit := func() bool {
		if !hasPending {
			return true
		}
		if len(stackLines) > 0 {
			pending.StackTrace = strings.Join(stackLines, "\n")
		}
		hasPending = false
		return visit(pending)
	}

	scanner := bufio.NewScanner(file)
	// Stack frames and JSON payloads run well past bufio's 64KB default.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if isNewLogLine(trimmed) {
			if !emit() {
				stopped = true
				break
			}
			pending = parseLogLine(trimmed)
			stackLines = stackLines[:0]
			hasPending = true
		} else if hasPending {
			stackLines = append(stackLines, line)
		}
	}

	if !stopped {
		if err := scanner.Err(); err != nil {
			return err
		}
		emit()
	}

	return nil
}

// parseLogLine turns one log line into an entry, trying the three shapes the
// file actually contains, in decreasing order of how much they tell us: a JSON
// payload written by this package's logger, one of vbeam's HTTP timing lines,
// or a plain `log.Printf` whose level and category have to be guessed from its
// text.
func parseLogLine(trimmed string) logEntry {
	if jsonPart, hasJSON := extractJSONFromLogLine(trimmed); hasJSON {
		var entry logEntry
		if err := json.Unmarshal([]byte(jsonPart), &entry); err == nil {
			return entry
		}
	}

	if timingEntry, isTiming := parseTimingLogLine(trimmed); isTiming {
		return *timingEntry
	}

	cleanLine := stripAnsiCodes(trimmed)
	timestamp, message := parseLogTimestamp(cleanLine)
	return logEntry{
		Timestamp: timestamp,
		Level:     detectLogLevel(message),
		Category:  categorizeLogMessage(message),
		Message:   message,
	}
}

// listLogFiles returns the .log files in cfg.LogDir with their stat info,
// newest first. A missing directory is not an error: a build that has never
// written a log has nothing to show, which is different from a failure.
func listLogFiles() ([]LogFileInfo, error) {
	files, err := os.ReadDir(cfg.LogDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogFileInfo{}, nil
		}
		return nil, err
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

		// Rotated files carry the date in the name; the live file is
		// LogFileName and counts as today when it was written to today.
		isToday := strings.Contains(file.Name(), today) ||
			(file.Name() == LogFileName && info.ModTime().Format("2006-01-02") == today)

		logFiles = append(logFiles, LogFileInfo{
			Name:       file.Name(),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			IsToday:    isToday,
			SizeString: formatFileSize(info.Size()),
		})
	}

	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime.After(logFiles[j].ModTime)
	})
	return logFiles, nil
}

var errInvalidLogFilename = fmt.Errorf("Invalid filename")

// logFilePath resolves a caller-supplied log file name against cfg.LogDir,
// refusing anything that is not a plain name in that directory.
func logFilePath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", errInvalidLogFilename
	}
	return filepath.Join(cfg.LogDir, name), nil
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

var newLogLineRegex = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

func isNewLogLine(line string) bool {
	return newLogLineRegex.MatchString(line)
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

// timingLogRegex matches vbeam's HTTP timing lines: timestamp, status, method,
// path, total duration, optional handler duration. Compiled once — it is
// applied to every line of every log file scanned.
var timingLogRegex = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+(\d+)\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s⎯]+).*?(\d+)µs(?:\s+\[(\d+)µs\])?`)

// parseTimingLogLine attempts to parse HTTP timing log entries
// Format: "2025/09/27 17:31:28 200 POST /rpc/SendMessage ⎯⎯⎯ 12759µs [12602µs]"
func parseTimingLogLine(line string) (*logEntry, bool) {
	cleanLine := stripAnsiCodes(line)
	matches := timingLogRegex.FindStringSubmatch(cleanLine)

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

// statsRecentEntries and statsRecentErrors bound what GetLogStats keeps while
// scanning. Reading a whole day of traffic is fine; holding all of it is not.
const (
	statsRecentEntries = 10
	statsRecentErrors  = 10
)

// entryRing keeps the last n entries seen, in order, without growing.
type entryRing struct {
	buf   []logEntry
	next  int
	full  bool
	limit int
}

func newEntryRing(limit int) *entryRing {
	return &entryRing{buf: make([]logEntry, limit), limit: limit}
}

func (r *entryRing) add(entry logEntry) {
	r.buf[r.next] = entry
	r.next = (r.next + 1) % r.limit
	if r.next == 0 {
		r.full = true
	}
}

// newestFirst returns the retained entries most recent first.
func (r *entryRing) newestFirst() []logEntry {
	count := r.next
	if r.full {
		count = r.limit
	}
	out := make([]logEntry, 0, count)
	for i := 0; i < count; i++ {
		idx := (r.next - 1 - i + r.limit) % r.limit
		out = append(out, r.buf[idx])
	}
	return out
}

// perfAccumulator builds the request-latency summary while the files are being
// scanned. Per-endpoint figures are accumulated rather than collected, so the
// only thing that grows with traffic is the duration slice the percentiles
// genuinely need — the previous shape held an endpointRequest struct for every
// request in the corpus.
type perfAccumulator struct {
	durations []int
	endpoints map[string]*endpointAccumulator
}

type endpointAccumulator struct {
	count  int
	sum    int
	min    int
	max    int
	errors int
}

func newPerfAccumulator() *perfAccumulator {
	return &perfAccumulator{endpoints: make(map[string]*endpointAccumulator)}
}

func (p *perfAccumulator) add(entry logEntry) {
	if entry.Duration == nil || *entry.Duration <= 0 {
		return
	}
	duration := *entry.Duration
	p.durations = append(p.durations, duration)

	if entry.HTTPMethod == "" || entry.HTTPPath == "" {
		return
	}
	key := entry.HTTPMethod + " " + entry.HTTPPath
	acc := p.endpoints[key]
	if acc == nil {
		acc = &endpointAccumulator{min: duration, max: duration}
		p.endpoints[key] = acc
	}
	acc.count++
	acc.sum += duration
	if duration < acc.min {
		acc.min = duration
	}
	if duration > acc.max {
		acc.max = duration
	}
	if entry.HTTPStatus != nil && *entry.HTTPStatus >= 400 {
		acc.errors++
	}
}

func (p *perfAccumulator) result() PerformanceStats {
	stats := PerformanceStats{
		TotalRequests:    len(p.durations),
		EndpointStats:    make(map[string]EndpointStats, len(p.endpoints)),
		SlowestEndpoints: []EndpointStats{},
	}
	if len(p.durations) == 0 {
		return stats
	}

	sort.Ints(p.durations)
	stats.AverageResponse = calculateAverage(p.durations)
	stats.MedianResponse = calculatePercentile(p.durations, 50)
	stats.P90Response = calculatePercentile(p.durations, 90)
	stats.P95Response = calculatePercentile(p.durations, 95)
	stats.P99Response = calculatePercentile(p.durations, 99)

	var slowest []EndpointStats
	for endpoint, acc := range p.endpoints {
		method, path, _ := strings.Cut(endpoint, " ")
		entry := EndpointStats{
			Path:            path,
			Method:          method,
			Count:           acc.count,
			AverageResponse: acc.sum / acc.count,
			MinResponse:     acc.min,
			MaxResponse:     acc.max,
			ErrorRate:       float64(acc.errors) / float64(acc.count) * 100,
		}
		stats.EndpointStats[endpoint] = entry
		slowest = append(slowest, entry)
	}

	sort.Slice(slowest, func(i, j int) bool {
		return slowest[i].AverageResponse > slowest[j].AverageResponse
	})
	if len(slowest) > 10 {
		slowest = slowest[:10]
	}
	stats.SlowestEndpoints = slowest
	return stats
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

// formatFileSize renders a byte count for display.
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
