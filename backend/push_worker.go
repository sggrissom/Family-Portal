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

const (
	PushEventChatMessage = "chat_message"
	PushEventTest        = "test"
)

// Bump when a `data` field changes meaning or disappears; adding one does not,
// since older app builds simply ignore it.
const pushPayloadVersion = 1

const maxAlertBodyLength = 100

type PushNotificationJob struct {
	Event            string
	RecordId         int
	FamilyId         int
	SenderId         int
	SenderName       string
	Content          string
	RecipientUserIds []int
}

type pushEventSpec struct {
	Category    string
	Destination string
	Title       string
	QuietTitle  string
	QuietBody   string
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
	Version     int    `json:"v"`
	Type        string `json:"type"`
	RecordId    int    `json:"record_id"`
	Destination string `json:"destination"`
	FamilyId    int    `json:"family_id"`
	SenderId    int    `json:"sender_id"`
	SenderName  string `json:"sender_name"`
	MessageId   int    `json:"message_id"`
}

type APNsConfig struct {
	TeamId   string
	KeyId    string
	BundleId string
	KeyPath  string
	Key      *ecdsa.PrivateKey
}

const maxRecentPushAttempts = 50

type PushAttempt struct {
	Time       time.Time `json:"time"`
	UserId     int       `json:"userId"`
	TokenId    int       `json:"tokenId"`
	TokenHint  string    `json:"tokenHint"`
	Kind       string    `json:"kind"`
	Success    bool      `json:"success"`
	StatusCode int       `json:"statusCode"`
	Reason     string    `json:"reason"`
	ApnsId     string    `json:"apnsId"`
}

type PushWorkerStats struct {
	Enabled        bool          `json:"enabled"`
	IsRunning      bool          `json:"isRunning"`
	QueueLength    int           `json:"queueLength"`
	Sent           int           `json:"sent"`
	Failed         int           `json:"failed"`
	Deactivated    int           `json:"deactivated"`
	Suppressed     int           `json:"suppressed"`
	LastSentAt     time.Time     `json:"lastSentAt"`
	LastError      string        `json:"lastError"`
	LastErrorAt    time.Time     `json:"lastErrorAt"`
	RecentAttempts []PushAttempt `json:"recentAttempts"`
}

type PushWorker struct {
	workerLifecycle
	jobQueue    chan PushNotificationJob
	db          *vbolt.DB
	apnsConfig  *APNsConfig
	httpClient  *http.Client
	tokenMu     sync.RWMutex
	jwtToken    string
	tokenExpiry time.Time

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

func InitializePushWorker(queueSize int, db *vbolt.DB) {
	if globalPushWorker != nil {
		LogInfo(LogCategoryWorker, "Push worker already initialized, skipping")
		return
	}

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

func loadAPNsConfig() (*APNsConfig, error) {
	teamId := os.Getenv("APNS_TEAM_ID")
	keyId := os.Getenv("APNS_KEY_ID")
	bundleId := os.Getenv("APNS_BUNDLE_ID")
	keyPath := os.Getenv("APNS_KEY_PATH")

	if teamId == "" || keyId == "" || bundleId == "" || keyPath == "" {
		return nil, fmt.Errorf("missing APNs configuration: APNS_TEAM_ID, APNS_KEY_ID, APNS_BUNDLE_ID, and APNS_KEY_PATH are required")
	}

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

func QueuePushNotification(job PushNotificationJob) error {
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

func (pw *PushWorker) recordSuppression() {
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()
	pw.suppressed++
}

func (pw *PushWorker) recordDeactivation() {
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()
	pw.deactivated++
}

func maskPushToken(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:8] + "…" + token[len(token)-4:]
}

func (pw *PushWorker) Start() {
	quit, done, ok := pw.start()
	if !ok {
		return
	}

	go pw.processJobs(quit, done)
	LogInfo(LogCategoryWorker, "Push notification worker started")
}

func (pw *PushWorker) Stop() {
	pw.stopImmediately()
	LogInfo(LogCategoryWorker, "Push notification worker stopped")
}

func (pw *PushWorker) StopAndDrain(ctx context.Context) bool {
	return pw.stopAndWait(ctx, true)
}

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

type pushDelivery struct {
	Token PushDeviceToken
	Prefs NotificationPreferences
}

func (pw *PushWorker) processPushJob(job PushNotificationJob) {
	LogInfo(LogCategoryWorker, "[PUSH_NOTIFICATION] Processing notification", map[string]interface{}{
		"event":      job.Event,
		"recordId":   job.RecordId,
		"recipients": len(job.RecipientUserIds),
	})

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
	}
}

func truncateAlertBody(body string) string {
	if len(body) > maxAlertBodyLength {
		return body[:maxAlertBodyLength-3] + "..."
	}
	return body
}

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
		payload.Aps.Alert = APNsAlert{Title: spec.QuietTitle, Body: spec.QuietBody}
	}

	return payload
}

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

	var apnsHost string
	if token.Environment == "sandbox" {
		apnsHost = "api.sandbox.push.apple.com"
	} else {
		apnsHost = "api.push.apple.com"
	}

	url := fmt.Sprintf("https://%s/3/device/%s", apnsHost, token.Token)

	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		attempt.Reason = "request build failed: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to create request: %w", err)
	}

	jwtToken, err := pw.getAPNsJWT()
	if err != nil {
		attempt.Reason = "JWT signing failed: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to get JWT: %w", err)
	}

	req.Header.Set("authorization", "bearer "+jwtToken)
	req.Header.Set("apns-topic", token.BundleId)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("content-type", "application/json")

	resp, err := pw.httpClient.Do(req)
	if err != nil {
		attempt.Reason = "transport error: " + err.Error()
		pw.recordAttempt(attempt)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	attempt.StatusCode = resp.StatusCode
	attempt.ApnsId = resp.Header.Get("apns-id")

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

func (pw *PushWorker) getAPNsJWT() (string, error) {
	pw.tokenMu.RLock()
	if pw.jwtToken != "" && time.Now().Before(pw.tokenExpiry) {
		token := pw.jwtToken
		pw.tokenMu.RUnlock()
		return token, nil
	}
	pw.tokenMu.RUnlock()

	pw.tokenMu.Lock()
	defer pw.tokenMu.Unlock()

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
	pw.tokenExpiry = now.Add(50 * time.Minute)

	return signedToken, nil
}

func GetPushQueueLength() int {
	if globalPushWorker == nil {
		return 0
	}
	return len(globalPushWorker.jobQueue)
}

func GetPushWorkerStats() PushWorkerStats {
	if globalPushWorker == nil {
		return PushWorkerStats{RecentAttempts: []PushAttempt{}}
	}

	pw := globalPushWorker
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()

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

type APNsConfigInfo struct {
	Configured  bool   `json:"configured"`
	TeamId      string `json:"teamId"`
	KeyId       string `json:"keyId"`
	BundleId    string `json:"bundleId"`
	KeyPath     string `json:"keyPath"`
	Environment string `json:"environment"`
	KeyLoaded   bool   `json:"keyLoaded"`
	LoadError   string `json:"loadError"`
}

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

func StopPushWorker() {
	if globalPushWorker != nil {
		globalPushWorker.Stop()
	}
}

func stopPushWorkerAndDrain(ctx context.Context) bool {
	if globalPushWorker == nil {
		return true
	}
	return globalPushWorker.StopAndDrain(ctx)
}

func IsPushWorkerEnabled() bool {
	return globalPushWorker != nil
}
