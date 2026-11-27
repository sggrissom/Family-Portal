package backend

import (
	"bufio"
	"encoding/json"
	"family/cfg"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
)

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

func logFilePath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", errInvalidLogFilename
	}
	return filepath.Join(cfg.LogDir, name), nil
}

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsiCodes(input string) string {
	return ansiEscapeRegex.ReplaceAllString(input, "")
}

func extractJSONFromLogLine(line string) (string, bool) {
	if idx := strings.Index(line, "{"); idx != -1 {
		jsonPart := line[idx:]
		if strings.HasSuffix(strings.TrimSpace(jsonPart), "}") {
			return jsonPart, true
		}
	}
	return line, false
}

func parseLogTimestamp(line string) (time.Time, string) {
	timestampRegex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+(.*)`)
	matches := timestampRegex.FindStringSubmatch(line)

	if len(matches) == 3 {
		if timestamp, err := time.ParseInLocation("2006/01/02 15:04:05", matches[1], time.Local); err == nil {
			return timestamp, matches[2]
		}
	}

	return time.Now(), line
}

var newLogLineRegex = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

func isNewLogLine(line string) bool {
	return newLogLineRegex.MatchString(line)
}

func detectLogLevel(message string) logLevel {
	upperMessage := strings.ToUpper(message)

	errorKeywords := []string{"ERROR", "FATAL", "PANIC", "FAILED", "FAILURE", "EXCEPTION", "CRITICAL"}
	for _, keyword := range errorKeywords {
		if strings.Contains(upperMessage, keyword) {
			return logLevelError
		}
	}

	warnKeywords := []string{"WARN", "WARNING", "DEPRECATED"}
	for _, keyword := range warnKeywords {
		if strings.Contains(upperMessage, keyword) {
			return logLevelWarn
		}
	}

	debugKeywords := []string{"DEBUG", "TRACE", "VERBOSE"}
	for _, keyword := range debugKeywords {
		if strings.Contains(upperMessage, keyword) {
			return logLevelDebug
		}
	}

	return logLevelInfo
}

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

// vbeam includes the client address in production timing lines, but omits it
// in some local/test formats. Parse both forms so a path containing a word
// such as "error" cannot fall through to the plain-text severity detector and
// turn an ordinary 404 into an application error.
var timingLogRegex = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+(?:(\S+)\s+)?(\d+)\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s⎯]+).*?(\d+)µs(?:\s+\[(\d+)µs\])?`)

func parseTimingLogLine(line string) (*logEntry, bool) {
	cleanLine := stripAnsiCodes(line)
	matches := timingLogRegex.FindStringSubmatch(cleanLine)

	if len(matches) < 7 {
		return nil, false
	}

	timestamp, err := time.ParseInLocation("2006/01/02 15:04:05", matches[1], time.Local)
	if err != nil {
		return nil, false
	}

	status, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, false
	}

	duration, err := strconv.Atoi(matches[6])
	if err != nil {
		return nil, false
	}

	var handlerDuration *int
	if len(matches) > 7 && matches[7] != "" {
		if hd, err := strconv.Atoi(matches[7]); err == nil {
			handlerDuration = &hd
		}
	}

	entry := &logEntry{
		Timestamp:       timestamp,
		Level:           logLevelInfo,
		Category:        logCategoryAPI,
		IP:              matches[2],
		Message:         fmt.Sprintf("%s %s %s", matches[4], matches[5], matches[3]),
		Duration:        &duration,
		HandlerDuration: handlerDuration,
		HTTPMethod:      matches[4],
		HTTPPath:        matches[5],
		HTTPStatus:      &status,
	}

	return entry, true
}

var errEmptyReference = fmt.Errorf("Enter a reference code to look up")

func resolveLogTargets(filename string) (paths []string, names []string, err error) {
	if filename != "" {
		path, pathErr := logFilePath(filename)
		if pathErr != nil {
			return nil, nil, pathErr
		}
		return []string{path}, []string{filename}, nil
	}

	files, listErr := listLogFiles()
	if listErr != nil {
		return nil, nil, ProcError(listErr)
	}
	for _, file := range files {
		paths = append(paths, filepath.Join(cfg.LogDir, file.Name))
		names = append(names, file.Name)
	}
	return paths, names, nil
}

type logFilter struct {
	level       string
	category    string
	search      string
	since       time.Time
	minDuration *int
	minStatus   int
	maxStatus   int
}

func newLogFilter(req GetLogContentRequest) (logFilter, error) {
	filter := logFilter{
		level:       req.Level,
		category:    req.Category,
		search:      strings.ToLower(strings.TrimSpace(req.Search)),
		minDuration: req.MinDuration,
	}
	if req.SinceHours > 0 {
		filter.since = time.Now().Add(-time.Duration(req.SinceHours) * time.Hour)
	}

	switch status := strings.ToLower(strings.TrimSpace(req.HTTPStatus)); status {
	case "":
	case "4xx/5xx":
		filter.minStatus, filter.maxStatus = 400, 599
	case "4xx":
		filter.minStatus, filter.maxStatus = 400, 499
	case "5xx":
		filter.minStatus, filter.maxStatus = 500, 599
	default:
		code, err := strconv.Atoi(status)
		if err != nil || len(status) != 3 || code < 100 || code > 599 {
			return logFilter{}, NewAppError(ErrCodeBadRequest, "Invalid HTTP status filter")
		}
		filter.minStatus, filter.maxStatus = code, code
	}
	return filter, nil
}

func (f logFilter) keep(entry logEntry) bool {
	if f.level != "" && string(entry.Level) != f.level {
		return false
	}
	if f.category != "" && string(entry.Category) != f.category {
		return false
	}
	if f.minStatus != 0 && (entry.HTTPStatus == nil || *entry.HTTPStatus < f.minStatus || *entry.HTTPStatus > f.maxStatus) {
		return false
	}
	if !f.since.IsZero() && entry.Timestamp.Before(f.since) {
		return false
	}
	if f.minDuration != nil && (entry.Duration == nil || *entry.Duration < *f.minDuration) {
		return false
	}
	if f.search != "" && !entryMatchesText(entry, f.search) {
		return false
	}
	return true
}

func entryMatchesText(entry logEntry, needle string) bool {
	if strings.Contains(strings.ToLower(entry.Message), needle) {
		return true
	}
	if entry.StackTrace != "" && strings.Contains(strings.ToLower(entry.StackTrace), needle) {
		return true
	}
	if entry.HTTPPath != "" && strings.Contains(strings.ToLower(entry.HTTPPath), needle) {
		return true
	}
	if entry.Data != nil {
		if encoded, err := json.Marshal(entry.Data); err == nil {
			return strings.Contains(strings.ToLower(string(encoded)), needle)
		}
	}
	return false
}

func entryHasReference(entry logEntry, reference string) bool {
	if data, ok := entry.Data.(map[string]interface{}); ok {
		if id, ok := data["requestId"].(string); ok && id == reference {
			return true
		}
	}
	return entryMatchesText(entry, strings.ToLower(reference))
}

const (
	statsRecentEntries = 10
	statsRecentErrors  = 10
)

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

type perfAccumulator struct {
	durations         []int
	endpoints         map[string]*endpointAccumulator
	appDurations      []int
	uploadDurations   []int
	downloadDurations []int
	cacheDurations    []int
	appEndpoints      map[string]*endpointAccumulator
}

type endpointAccumulator struct {
	count  int
	sum    int
	min    int
	max    int
	errors int
}

func newPerfAccumulator() *perfAccumulator {
	return &perfAccumulator{
		endpoints:    make(map[string]*endpointAccumulator),
		appEndpoints: make(map[string]*endpointAccumulator),
	}
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
	status := 0
	if entry.HTTPStatus != nil {
		status = *entry.HTTPStatus
	}
	recordEndpoint(p.endpoints, entry.HTTPMethod+" "+entry.HTTPPath, duration, status)

	if status == http.StatusNotModified {
		p.cacheDurations = append(p.cacheDurations, duration)
		return
	}
	if status < 200 || status >= 300 || entry.HTTPMethod == http.MethodHead || entry.HTTPMethod == http.MethodOptions {
		return
	}

	// Timing logs contain the request URI, including query parameters.
	path, _, _ := strings.Cut(entry.HTTPPath, "?")
	switch {
	case path == "/api/upload-photo", path == vbeam.PREFIX_RPC+"ImportData",
		strings.HasPrefix(path, localMediaUploadPrefix):
		p.uploadDurations = append(p.uploadDurations, duration)
	case path == "/api/export-bundle", path == SnapshotPath, path == vbeam.PREFIX_RPC+"ExportData",
		strings.HasPrefix(path, "/api/photo/"), strings.HasPrefix(path, localMediaObjectPrefix):
		p.downloadDurations = append(p.downloadDurations, duration)
	case strings.HasPrefix(path, vbeam.PREFIX_RPC), strings.HasPrefix(path, "/api/"):
		p.appDurations = append(p.appDurations, duration)
		recordEndpoint(p.appEndpoints, entry.HTTPMethod+" "+path, duration, status)
	}
}

func recordEndpoint(endpoints map[string]*endpointAccumulator, key string, duration int, status int) {
	acc := endpoints[key]
	if acc == nil {
		acc = &endpointAccumulator{min: duration, max: duration}
		endpoints[key] = acc
	}
	acc.count++
	acc.sum += duration
	if duration < acc.min {
		acc.min = duration
	}
	if duration > acc.max {
		acc.max = duration
	}
	if status >= 400 {
		acc.errors++
	}
}

func (p *perfAccumulator) result() PerformanceStats {
	endpoints, slowest := summarizeEndpoints(p.endpoints)
	_, slowestApp := summarizeEndpoints(p.appEndpoints)
	stats := PerformanceStats{
		TotalRequests:    len(p.durations),
		EndpointStats:    endpoints,
		SlowestEndpoints: slowest,
		RequestLatency: RequestLatencyStats{
			App:                 summarizeResponseTimes(p.appDurations),
			Uploads:             summarizeResponseTimes(p.uploadDurations),
			Downloads:           summarizeResponseTimes(p.downloadDurations),
			CacheChecks:         summarizeResponseTimes(p.cacheDurations),
			SlowestAppEndpoints: slowestApp,
		},
	}

	sort.Ints(p.durations)
	stats.AverageResponse = calculateAverage(p.durations)
	stats.MedianResponse = calculatePercentile(p.durations, 50)
	stats.P90Response = calculatePercentile(p.durations, 90)
	stats.P95Response = calculatePercentile(p.durations, 95)
	stats.P99Response = calculatePercentile(p.durations, 99)
	return stats
}

func summarizeResponseTimes(durations []int) ResponseTimingStats {
	sort.Ints(durations)
	return ResponseTimingStats{
		Count:          len(durations),
		MedianResponse: calculatePercentile(durations, 50),
		P95Response:    calculatePercentile(durations, 95),
	}
}

func summarizeEndpoints(accumulators map[string]*endpointAccumulator) (map[string]EndpointStats, []EndpointStats) {
	endpoints := make(map[string]EndpointStats, len(accumulators))
	slowest := make([]EndpointStats, 0, len(accumulators))
	for endpoint, acc := range accumulators {
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
		endpoints[endpoint] = entry
		slowest = append(slowest, entry)
	}

	sort.Slice(slowest, func(i, j int) bool {
		return slowest[i].AverageResponse > slowest[j].AverageResponse
	})
	if len(slowest) > 10 {
		slowest = slowest[:10]
	}
	return endpoints, slowest
}

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

	weight := index - float64(lower)
	return int(float64(sortedValues[lower])*(1-weight) + float64(sortedValues[upper])*weight)
}

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
