package backend

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbolt"
)

// PushNotificationJob represents a push notification to be sent
type PushNotificationJob struct {
	MessageId        int
	FamilyId         int
	SenderId         int
	SenderName       string
	Content          string
	RecipientUserIds []int
	// IsTest marks an admin-issued verification push. It carries a distinct
	// payload type so the companion app does not try to open a chat message
	// that was never written.
	IsTest bool
}

// APNsPayload represents the Apple Push Notification payload
type APNsPayload struct {
	Aps  APNsAps        `json:"aps"`
	Data APNsCustomData `json:"data"`
}

type APNsAps struct {
	Alert    APNsAlert `json:"alert"`
	Sound    string    `json:"sound"`
	Badge    int       `json:"badge"`
	Category string    `json:"category"`
}

type APNsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type APNsCustomData struct {
	Type       string `json:"type"`
	MessageId  int    `json:"message_id"`
	SenderId   int    `json:"sender_id"`
	SenderName string `json:"sender_name"`
}

// APNsConfig holds the configuration for APNs
type APNsConfig struct {
	TeamId   string
	KeyId    string
	BundleId string
	KeyPath  string
	Key      *ecdsa.PrivateKey
}

// maxRecentPushAttempts bounds the in-memory delivery history kept for the admin
// page. It exists to answer "did the push I just triggered reach APNs?", not to
// be an audit log, so a short window is enough and nothing is persisted.
const maxRecentPushAttempts = 50

// PushAttempt records the outcome of one delivery to one device.
type PushAttempt struct {
	Time      time.Time `json:"time"`
	UserId    int       `json:"userId"`
	TokenId   int       `json:"tokenId"`
	TokenHint string    `json:"tokenHint"`
	// Kind is "chat" or "test".
	Kind    string `json:"kind"`
	Success bool   `json:"success"`
	// StatusCode is the APNs HTTP status, or 0 if the request never completed.
	StatusCode int `json:"statusCode"`
	// Reason is the APNs failure reason (e.g. "BadDeviceToken") or a transport error.
	Reason string `json:"reason"`
	// ApnsId is Apple's identifier for the notification, which is what Apple asks
	// for when a delivery is disputed.
	ApnsId string `json:"apnsId"`
}

// PushWorkerStats is a live snapshot of push activity. Every field is in-memory:
// a restart resets the counters and empties the history.
type PushWorkerStats struct {
	Enabled        bool          `json:"enabled"`
	IsRunning      bool          `json:"isRunning"`
	QueueLength    int           `json:"queueLength"`
	Sent           int           `json:"sent"`
	Failed         int           `json:"failed"`
	Deactivated    int           `json:"deactivated"`
	LastSentAt     time.Time     `json:"lastSentAt"`
	LastError      string        `json:"lastError"`
	LastErrorAt    time.Time     `json:"lastErrorAt"`
	RecentAttempts []PushAttempt `json:"recentAttempts"`
}

// PushWorker manages background push notification sending
type PushWorker struct {
	jobQueue    chan PushNotificationJob
	stopChannel chan bool
	isRunning   bool
	db          *vbolt.DB
	apnsConfig  *APNsConfig
	httpClient  *http.Client
	tokenMu     sync.RWMutex
	jwtToken    string
	tokenExpiry time.Time

	// statsMu guards everything below. Sends happen on the worker goroutine
	// while the admin page reads from request goroutines.
	statsMu     sync.Mutex
	sent        int
	failed      int
	deactivated int
	lastSentAt  time.Time
	lastError   string
	lastErrorAt time.Time
	recent      []PushAttempt
}

var globalPushWorker *PushWorker

// InitializePushWorker starts the background push notification worker
func InitializePushWorker(queueSize int, db *vbolt.DB) {
	if globalPushWorker != nil {
		LogInfo(LogCategoryWorker, "Push worker already initialized, skipping")
		return
	}

	// Load APNs configuration from environment
	config, err := loadAPNsConfig()
	if err != nil {
		LogWarn(LogCategoryWorker, "Push notifications disabled: APNs configuration not available", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	LogInfo(LogCategoryWorker, "Initializing push notification worker", map[string]interface{}{
		"queueSize": queueSize,
		"teamId":    config.TeamId,
		"keyId":     config.KeyId,
		"bundleId":  config.BundleId,
	})

	globalPushWorker = &PushWorker{
		jobQueue:    make(chan PushNotificationJob, queueSize),
		stopChannel: make(chan bool),
		isRunning:   false,
		db:          db,
		apnsConfig:  config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	globalPushWorker.Start()
	LogInfo(LogCategoryWorker, "Push notification worker started")
}

// loadAPNsConfig loads APNs configuration from environment variables
func loadAPNsConfig() (*APNsConfig, error) {
	teamId := os.Getenv("APNS_TEAM_ID")
	keyId := os.Getenv("APNS_KEY_ID")
	bundleId := os.Getenv("APNS_BUNDLE_ID")
	keyPath := os.Getenv("APNS_KEY_PATH")

	if teamId == "" || keyId == "" || bundleId == "" || keyPath == "" {
		return nil, fmt.Errorf("missing APNs configuration: APNS_TEAM_ID, APNS_KEY_ID, APNS_BUNDLE_ID, and APNS_KEY_PATH are required")
	}

	// Load the private key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read APNs key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from APNs key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse APNs private key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("APNs key is not an ECDSA private key")
	}

	return &APNsConfig{
		TeamId:   teamId,
		KeyId:    keyId,
		BundleId: bundleId,
		KeyPath:  keyPath,
		Key:      ecdsaKey,
	}, nil
}

// QueuePushNotification adds a notification job to the processing queue
func QueuePushNotification(job PushNotificationJob) error {
	if globalPushWorker == nil {
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Cannot queue: worker not initialized")
		return fmt.Errorf("push worker not initialized")
	}

	select {
	case globalPushWorker.jobQueue <- job:
		LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Queued notification", map[string]interface{}{
			"messageId":   job.MessageId,
			"isTest":      job.IsTest,
			"recipients":  len(job.RecipientUserIds),
			"queueLength": len(globalPushWorker.jobQueue),
		})
		return nil
	default:
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Cannot queue: queue is full", map[string]interface{}{
			"messageId": job.MessageId,
		})
		return fmt.Errorf("push notification queue is full")
	}
}

// recordAttempt files one delivery outcome into the counters and the bounded
// history the admin page reads.
func (pw *PushWorker) recordAttempt(attempt PushAttempt) {
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()

	if attempt.Success {
		pw.sent++
		pw.lastSentAt = attempt.Time
	} else {
		pw.failed++
		pw.lastError = attempt.Reason
		pw.lastErrorAt = attempt.Time
	}

	pw.recent = append(pw.recent, attempt)
	if len(pw.recent) > maxRecentPushAttempts {
		pw.recent = pw.recent[len(pw.recent)-maxRecentPushAttempts:]
	}
}

// recordDeactivation counts a token APNs told us to stop using.
func (pw *PushWorker) recordDeactivation() {
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()
	pw.deactivated++
}

// maskPushToken renders a device token as a prefix and suffix. It is enough to
// match a row against what the companion app logs at registration without
// putting a usable token on an admin screen.
func maskPushToken(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:8] + "…" + token[len(token)-4:]
}

// Start begins the background worker goroutine
func (pw *PushWorker) Start() {
	if pw.isRunning {
		return
	}

	pw.isRunning = true
	go pw.processJobs()
	LogInfo(LogCategoryWorker, "Push notification worker started")
}

// Stop gracefully shuts down the worker
func (pw *PushWorker) Stop() {
	if !pw.isRunning {
		return
	}

	pw.stopChannel <- true
	pw.isRunning = false
	LogInfo(LogCategoryWorker, "Push notification worker stopped")
}

// processJobs is the main worker loop
func (pw *PushWorker) processJobs() {
	for {
		select {
		case job := <-pw.jobQueue:
			pw.processPushJob(job)
		case <-pw.stopChannel:
			LogInfo(LogCategoryWorker, "Push worker received stop signal")
			return
		}
	}
}

// processPushJob processes a single push notification job
func (pw *PushWorker) processPushJob(job PushNotificationJob) {
	LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Processing notification", map[string]interface{}{
		"messageId":  job.MessageId,
		"isTest":     job.IsTest,
		"recipients": len(job.RecipientUserIds),
	})

	// Get device tokens for all recipients
	var allTokens []PushDeviceToken
	vbolt.WithReadTx(pw.db, func(tx *vbolt.Tx) {
		for _, userId := range job.RecipientUserIds {
			tokens := GetActiveDeviceTokensForUser(tx, userId)
			allTokens = append(allTokens, tokens...)
		}
	})

	if len(allTokens) == 0 {
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] No active device tokens for recipients", map[string]interface{}{
			"messageId":  job.MessageId,
			"recipients": len(job.RecipientUserIds),
		})
		return
	}

	LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Found device tokens", map[string]interface{}{
		"messageId": job.MessageId,
		"tokens":    len(allTokens),
	})

	// Send notification to each device
	for _, token := range allTokens {
		if token.Platform == "ios" {
			err := pw.sendAPNsNotification(token, job)
			if err != nil {
				LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Failed to send to device", map[string]interface{}{
					"messageId": job.MessageId,
					"tokenId":   token.Id,
					"userId":    token.UserId,
					"error":     err.Error(),
				})
			}
		}
		// Android support can be added here in the future
	}
}

// sendAPNsNotification sends a push notification via APNs
func (pw *PushWorker) sendAPNsNotification(token PushDeviceToken, job PushNotificationJob) error {
	kind := "chat"
	if job.IsTest {
		kind = "test"
	}

	attempt := PushAttempt{
		Time:      time.Now(),
		UserId:    token.UserId,
		TokenId:   token.Id,
		TokenHint: maskPushToken(token.Token),
		Kind:      kind,
	}

	// Truncate content for notification body
	body := job.Content
	if len(body) > 100 {
		body = body[:97] + "..."
	}

	// Build the payload
	payload := APNsPayload{
		Aps: APNsAps{
			Alert: APNsAlert{
				Title: "New message",
				Body:  fmt.Sprintf("%s: %s", job.SenderName, body),
			},
			Sound:    "default",
			Badge:    1,
			Category: "chat_message",
		},
		Data: APNsCustomData{
			Type:       "chat_message",
			MessageId:  job.MessageId,
			SenderId:   job.SenderId,
			SenderName: job.SenderName,
		},
	}

	// A test push must not look like a chat message: the app routes on
	// data.type, and there is no message for it to open.
	if job.IsTest {
		payload.Aps.Alert.Title = "Test notification"
		payload.Aps.Alert.Body = body
		payload.Aps.Category = "test"
		payload.Data.Type = "test"
		payload.Data.MessageId = 0
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		attempt.Reason = "payload marshal failed: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Determine APNs endpoint based on environment
	var apnsHost string
	if token.Environment == "sandbox" {
		apnsHost = "api.sandbox.push.apple.com"
	} else {
		apnsHost = "api.push.apple.com"
	}

	url := fmt.Sprintf("https://%s/3/device/%s", apnsHost, token.Token)

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		attempt.Reason = "request build failed: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Get JWT token for APNs
	jwtToken, err := pw.getAPNsJWT()
	if err != nil {
		attempt.Reason = "JWT signing failed: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to get JWT: %w", err)
	}

	// Set headers
	req.Header.Set("authorization", "bearer "+jwtToken)
	req.Header.Set("apns-topic", token.BundleId)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("content-type", "application/json")

	// Send request
	resp, err := pw.httpClient.Do(req)
	if err != nil {
		attempt.Reason = "transport error: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	attempt.StatusCode = resp.StatusCode
	// Apple's identifier for this notification, which is what Apple asks for
	// when a delivery has to be chased down.
	attempt.ApnsId = resp.Header.Get("apns-id")

	// Handle response
	if resp.StatusCode == http.StatusOK {
		attempt.Success = true
		pw.recordAttempt(attempt)
		LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Delivered to APNs", map[string]interface{}{
			"tokenId": token.Id,
			"userId":  token.UserId,
			"kind":    kind,
			"apnsId":  attempt.ApnsId,
		})
		return nil
	}

	// Read error response
	bodyBytes, _ := io.ReadAll(resp.Body)
	var errorResp struct {
		Reason string `json:"reason"`
	}
	json.Unmarshal(bodyBytes, &errorResp)
	attempt.Reason = errorResp.Reason
	if attempt.Reason == "" {
		attempt.Reason = fmt.Sprintf("HTTP %d with no reason", resp.StatusCode)
	}
	pw.recordAttempt(attempt)

	// Handle specific errors that require token deactivation
	if errorResp.Reason == "BadDeviceToken" || errorResp.Reason == "Unregistered" {
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Deactivating invalid token", map[string]interface{}{
			"tokenId": token.Id,
			"userId":  token.UserId,
			"reason":  errorResp.Reason,
		})
		if err := DeactivatePushDeviceTokenById(pw.db, token.Id); err != nil {
			return fmt.Errorf("APNs error: %d %s; failed to deactivate token: %w", resp.StatusCode, errorResp.Reason, err)
		}
		pw.recordDeactivation()
	}

	return fmt.Errorf("APNs error: %d %s", resp.StatusCode, errorResp.Reason)
}

// getAPNsJWT returns a valid JWT for APNs authentication
func (pw *PushWorker) getAPNsJWT() (string, error) {
	pw.tokenMu.RLock()
	if pw.jwtToken != "" && time.Now().Before(pw.tokenExpiry) {
		token := pw.jwtToken
		pw.tokenMu.RUnlock()
		return token, nil
	}
	pw.tokenMu.RUnlock()

	// Generate new token
	pw.tokenMu.Lock()
	defer pw.tokenMu.Unlock()

	// Double-check after acquiring write lock
	if pw.jwtToken != "" && time.Now().Before(pw.tokenExpiry) {
		return pw.jwtToken, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": pw.apnsConfig.TeamId,
		"iat": now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = pw.apnsConfig.KeyId

	signedToken, err := token.SignedString(pw.apnsConfig.Key)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	pw.jwtToken = signedToken
	// APNs tokens are valid for up to 1 hour, refresh every 50 minutes
	pw.tokenExpiry = now.Add(50 * time.Minute)

	return signedToken, nil
}

// GetPushQueueLength returns the current number of jobs in the queue
func GetPushQueueLength() int {
	if globalPushWorker == nil {
		return 0
	}
	return len(globalPushWorker.jobQueue)
}

// GetPushWorkerStats returns a snapshot of push activity since process start.
// When push is unconfigured the worker was never created, which is itself the
// answer the admin page needs, so a zeroed snapshot is returned rather than an
// error.
func GetPushWorkerStats() PushWorkerStats {
	if globalPushWorker == nil {
		return PushWorkerStats{RecentAttempts: []PushAttempt{}}
	}

	pw := globalPushWorker
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()

	// Copy the history so callers cannot observe it being appended to, and
	// reverse it so the most recent attempt reads first.
	recent := make([]PushAttempt, 0, len(pw.recent))
	for i := len(pw.recent) - 1; i >= 0; i-- {
		recent = append(recent, pw.recent[i])
	}

	return PushWorkerStats{
		Enabled:        true,
		IsRunning:      pw.isRunning,
		QueueLength:    len(pw.jobQueue),
		Sent:           pw.sent,
		Failed:         pw.failed,
		Deactivated:    pw.deactivated,
		LastSentAt:     pw.lastSentAt,
		LastError:      pw.lastError,
		LastErrorAt:    pw.lastErrorAt,
		RecentAttempts: recent,
	}
}

// APNsConfigInfo is the non-secret half of the APNs configuration, safe to show
// on an admin screen. The signing key itself never leaves the process.
type APNsConfigInfo struct {
	Configured  bool   `json:"configured"`
	TeamId      string `json:"teamId"`
	KeyId       string `json:"keyId"`
	BundleId    string `json:"bundleId"`
	KeyPath     string `json:"keyPath"`
	Environment string `json:"environment"`
	KeyLoaded   bool   `json:"keyLoaded"`
	// LoadError explains why the worker did not start, which is the question an
	// unconfigured-looking push subsystem actually raises.
	LoadError string `json:"loadError"`
}

// GetAPNsConfigInfo reports what the process sees in its environment. It reads
// the environment directly rather than the worker so it can explain the case
// where the worker failed to start.
func GetAPNsConfigInfo() APNsConfigInfo {
	info := APNsConfigInfo{
		TeamId:      os.Getenv("APNS_TEAM_ID"),
		KeyId:       os.Getenv("APNS_KEY_ID"),
		BundleId:    os.Getenv("APNS_BUNDLE_ID"),
		KeyPath:     os.Getenv("APNS_KEY_PATH"),
		Environment: os.Getenv("APNS_ENVIRONMENT"),
	}

	config, err := loadAPNsConfig()
	if err != nil {
		info.LoadError = err.Error()
	} else {
		info.KeyLoaded = config.Key != nil
	}

	info.Configured = info.TeamId != "" && info.KeyId != "" &&
		info.BundleId != "" && info.KeyPath != "" && info.Environment != ""

	return info
}

// StopPushWorker gracefully shuts down the global push worker
func StopPushWorker() {
	if globalPushWorker != nil {
		globalPushWorker.Stop()
	}
}

// IsPushWorkerEnabled returns true if push notifications are configured
func IsPushWorkerEnabled() bool {
	return globalPushWorker != nil
}
