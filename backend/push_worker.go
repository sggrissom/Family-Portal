package backend

import (
	"bytes"
	"context"
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

// Push event types. A job names one of these; everything else about the
// notification — what the app opens, how the alert reads — follows from it.
const (
	PushEventChatMessage = "chat_message"
	PushEventTest        = "test"
)

// pushPayloadVersion is the schema version of the payload's `data` object. The
// companion app reads it before anything else and ignores a payload it does not
// understand, so an app build older than the server never misroutes a tap.
// Bump it when a field changes meaning or disappears; adding a field does not
// need a bump, because an older build simply will not look at it.
const pushPayloadVersion = 1

// maxAlertBodyLength bounds the visible text. APNs accepts far more, but a lock
// screen shows a couple of lines and the rest is only weight on the wire.
const maxAlertBodyLength = 100

// PushNotificationJob represents a push notification to be sent
type PushNotificationJob struct {
	// Event is one of the PushEvent constants above. QueuePushNotification
	// refuses a job naming anything else, so a producer finds out at the queue
	// rather than by the notification silently not arriving.
	Event string
	// RecordId identifies what the event is about — a chat message id for
	// PushEventChatMessage. Zero when there is no record to open, as with a
	// test push.
	RecordId         int
	FamilyId         int
	SenderId         int
	SenderName       string
	Content          string
	RecipientUserIds []int
}

// pushEventSpec is everything the payload builder needs to know about one event
// type. Adding an event means adding a row here rather than another branch in
// the builder.
type pushEventSpec struct {
	// Category is the APNs category, which selects the notification's actions
	// on the device.
	Category string
	// Destination is the site-relative path the app should open when the
	// notification is tapped. It matches the web route for the same content, so
	// the same string works as a universal link.
	Destination string
	// Title is the alert title when message previews are on.
	Title string
	// QuietTitle and QuietBody are the wording used when previews are off. They
	// must name nothing about the family: no member names, no content.
	QuietTitle string
	QuietBody  string
}

var pushEventSpecs = map[string]pushEventSpec{
	PushEventChatMessage: {
		Category:    "chat_message",
		Destination: "/chat",
		Title:       "New message",
		QuietTitle:  "Family Portal",
		QuietBody:   "New message",
	},
	PushEventTest: {
		Category:    "test",
		Destination: "/settings",
		Title:       "Test notification",
		QuietTitle:  "Test notification",
	},
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

// APNsCustomData is the routing half of the payload. iOS never displays it, so
// it can carry what the app needs to open the right screen — but for that same
// reason it must not be the only copy of anything the user should see, and it
// deliberately does not carry message content: text that belongs on the lock
// screen goes in the alert, subject to the recipient's preferences.
type APNsCustomData struct {
	// Version is pushPayloadVersion at the time of sending.
	Version int    `json:"v"`
	Type    string `json:"type"`
	// RecordId is the id of the record named by Type.
	RecordId int `json:"record_id"`
	// Destination is the in-app path to open on tap.
	Destination string `json:"destination"`
	FamilyId    int    `json:"family_id"`
	SenderId    int    `json:"sender_id"`
	SenderName  string `json:"sender_name"`
	// MessageId repeats RecordId for chat events only. It is what the payload
	// carried before it was versioned, kept so an app build that shipped
	// against the old shape keeps working.
	MessageId int `json:"message_id"`
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
	// Kind is the job's event type, so a row in the admin history says what the
	// notification was for.
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
	Enabled     bool `json:"enabled"`
	IsRunning   bool `json:"isRunning"`
	QueueLength int  `json:"queueLength"`
	Sent        int  `json:"sent"`
	Failed      int  `json:"failed"`
	Deactivated int  `json:"deactivated"`
	// Suppressed counts recipients skipped because they turned this kind of
	// notification off. It is the difference between "nothing was sent" and
	// "nothing was sent because nobody wanted it".
	Suppressed     int           `json:"suppressed"`
	LastSentAt     time.Time     `json:"lastSentAt"`
	LastError      string        `json:"lastError"`
	LastErrorAt    time.Time     `json:"lastErrorAt"`
	RecentAttempts []PushAttempt `json:"recentAttempts"`
}

// PushWorker manages background push notification sending
type PushWorker struct {
	workerLifecycle
	jobQueue    chan PushNotificationJob
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
	suppressed  int
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
		jobQueue:   make(chan PushNotificationJob, queueSize),
		db:         db,
		apnsConfig: config,
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
	// An event the payload builder does not know about would reach a phone as a
	// notification the app cannot route, so it is refused where the producer
	// can still see the error.
	if _, known := pushEventSpecs[job.Event]; !known {
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Cannot queue: unknown event type", map[string]interface{}{
			"event": job.Event,
		})
		return fmt.Errorf("unknown push event %q", job.Event)
	}

	if globalPushWorker == nil {
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Cannot queue: worker not initialized")
		return fmt.Errorf("push worker not initialized")
	}

	select {
	case globalPushWorker.jobQueue <- job:
		LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Queued notification", map[string]interface{}{
			"event":       job.Event,
			"recordId":    job.RecordId,
			"recipients":  len(job.RecipientUserIds),
			"queueLength": len(globalPushWorker.jobQueue),
		})
		return nil
	default:
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Cannot queue: queue is full", map[string]interface{}{
			"event":    job.Event,
			"recordId": job.RecordId,
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

// recordSuppression counts a recipient who has this kind of notification
// turned off.
func (pw *PushWorker) recordSuppression() {
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()
	pw.suppressed++
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
	quit, done, ok := pw.start()
	if !ok {
		return
	}

	go pw.processJobs(quit, done)
	LogInfo(LogCategoryWorker, "Push notification worker started")
}

// Stop signals the worker to exit, abandoning anything still queued.
func (pw *PushWorker) Stop() {
	pw.stopImmediately()
	LogInfo(LogCategoryWorker, "Push notification worker stopped")
}

// StopAndDrain stops the worker and delivers the notifications already queued,
// giving up when ctx expires. Each one is a single HTTPS call to APNs, so the
// backlog usually clears in well under the budget — and a notification about a
// chat message that already exists is the sort of thing whose absence is only
// ever noticed by the person waiting for it.
func (pw *PushWorker) StopAndDrain(ctx context.Context) bool {
	return pw.stopAndWait(ctx, true)
}

// processJobs is the main worker loop
func (pw *PushWorker) processJobs(quit <-chan struct{}, done chan struct{}) {
	defer close(done)
	for {
		select {
		case job := <-pw.jobQueue:
			pw.processPushJob(job)
		case <-quit:
			drained := drainQueue(pw.drainContext(), pw.jobQueue, pw.processPushJob)
			LogInfo(LogCategoryWorker, "Push worker received stop signal", map[string]interface{}{
				"drained":   drained,
				"abandoned": len(pw.jobQueue),
			})
			return
		}
	}
}

// pushDelivery pairs one device with the preferences of the account that
// registered it. The preferences decide how much of the notification is
// readable without unlocking the phone, so they have to travel with the token
// rather than be looked up again at send time.
type pushDelivery struct {
	Token PushDeviceToken
	Prefs NotificationPreferences
}

// processPushJob processes a single push notification job
func (pw *PushWorker) processPushJob(job PushNotificationJob) {
	LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Processing notification", map[string]interface{}{
		"event":      job.Event,
		"recordId":   job.RecordId,
		"recipients": len(job.RecipientUserIds),
	})

	// Resolve preferences and device tokens together, in one read transaction.
	var deliveries []pushDelivery
	suppressed := 0
	vbolt.WithReadTx(pw.db, func(tx *vbolt.Tx) {
		for _, userId := range job.RecipientUserIds {
			prefs := loadNotificationPreferences(tx, userId)
			if !prefs.allowsEvent(job.Event) {
				suppressed++
				continue
			}
			for _, token := range GetActiveDeviceTokensForUser(tx, userId) {
				deliveries = append(deliveries, pushDelivery{Token: token, Prefs: prefs})
			}
		}
	})

	for i := 0; i < suppressed; i++ {
		pw.recordSuppression()
	}

	if len(deliveries) == 0 {
		LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Nothing to deliver to", map[string]interface{}{
			"event":      job.Event,
			"recordId":   job.RecordId,
			"recipients": len(job.RecipientUserIds),
			"suppressed": suppressed,
		})
		return
	}

	LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Found device tokens", map[string]interface{}{
		"event":      job.Event,
		"recordId":   job.RecordId,
		"tokens":     len(deliveries),
		"suppressed": suppressed,
	})

	// Send notification to each device
	for _, delivery := range deliveries {
		if delivery.Token.Platform == "ios" {
			err := pw.sendAPNsNotification(delivery.Token, job, delivery.Prefs)
			if err != nil {
				LogWarn(LogCategoryWorker, "[PUSH_NOTIFICATION] Failed to send to device", map[string]interface{}{
					"event":    job.Event,
					"recordId": job.RecordId,
					"tokenId":  delivery.Token.Id,
					"userId":   delivery.Token.UserId,
					"error":    err.Error(),
				})
			}
		}
		// Android support can be added here in the future
	}
}

// truncateAlertBody keeps the visible text to something a lock screen can show.
func truncateAlertBody(body string) string {
	if len(body) > maxAlertBodyLength {
		return body[:maxAlertBodyLength-3] + "..."
	}
	return body
}

// buildAPNsPayload renders one job for one recipient.
//
// The split between `aps` and `data` is the whole privacy story: `aps.alert` is
// what iOS puts on the lock screen, and it says nothing about the family unless
// the recipient asked for previews; `data` is never displayed, so it can carry
// the identifiers the app needs once somebody has actually unlocked the phone
// and opened it.
func buildAPNsPayload(job PushNotificationJob, prefs NotificationPreferences) APNsPayload {
	spec := pushEventSpecs[job.Event]

	payload := APNsPayload{
		Aps: APNsAps{
			Sound:    "default",
			Badge:    1,
			Category: spec.Category,
		},
		Data: APNsCustomData{
			Version:     pushPayloadVersion,
			Type:        job.Event,
			RecordId:    job.RecordId,
			Destination: spec.Destination,
			FamilyId:    job.FamilyId,
			SenderId:    job.SenderId,
			SenderName:  job.SenderName,
		},
	}

	switch job.Event {
	case PushEventTest:
		// An admin typed this text to check that a specific phone can receive
		// anything at all. It is not family content, so the preview preference
		// does not apply — and there is no record for the app to open, which is
		// why RecordId stays out of the compatibility field below.
		payload.Aps.Alert = APNsAlert{
			Title: spec.Title,
			Body:  truncateAlertBody(job.Content),
		}
	case PushEventChatMessage:
		payload.Data.MessageId = job.RecordId
		if prefs.ShowMessageText {
			payload.Aps.Alert = APNsAlert{
				Title: spec.Title,
				Body:  truncateAlertBody(fmt.Sprintf("%s: %s", job.SenderName, job.Content)),
			}
		} else {
			payload.Aps.Alert = APNsAlert{Title: spec.QuietTitle, Body: spec.QuietBody}
		}
	default:
		// Unreachable: QueuePushNotification refuses an event with no spec.
		// Falling back to the quiet wording keeps a mistake from putting
		// unreviewed text on a lock screen.
		payload.Aps.Alert = APNsAlert{Title: spec.QuietTitle, Body: spec.QuietBody}
	}

	return payload
}

// sendAPNsNotification sends a push notification via APNs
func (pw *PushWorker) sendAPNsNotification(token PushDeviceToken, job PushNotificationJob, prefs NotificationPreferences) error {
	attempt := PushAttempt{
		Time:      time.Now(),
		UserId:    token.UserId,
		TokenId:   token.Id,
		TokenHint: maskPushToken(token.Token),
		Kind:      job.Event,
	}

	payload := buildAPNsPayload(job, prefs)

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
			"kind":    job.Event,
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
		IsRunning:      pw.isRunning(),
		QueueLength:    len(pw.jobQueue),
		Sent:           pw.sent,
		Failed:         pw.failed,
		Deactivated:    pw.deactivated,
		Suppressed:     pw.suppressed,
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

// stopPushWorkerAndDrain is the shutdown path's entry point.
func stopPushWorkerAndDrain(ctx context.Context) bool {
	if globalPushWorker == nil {
		return true
	}
	return globalPushWorker.StopAndDrain(ctx)
}

// IsPushWorkerEnabled returns true if push notifications are configured
func IsPushWorkerEnabled() bool {
	return globalPushWorker != nil
}
