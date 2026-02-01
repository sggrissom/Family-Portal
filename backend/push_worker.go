package backend

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
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
}

// APNsPayload represents the Apple Push Notification payload
type APNsPayload struct {
	Aps  APNsAps         `json:"aps"`
	Data APNsCustomData  `json:"data"`
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
		log.Printf("Cannot queue push notification: worker not initialized")
		return fmt.Errorf("push worker not initialized")
	}

	select {
	case globalPushWorker.jobQueue <- job:
		log.Printf("Push notification queued for message %d (queue length: %d)", job.MessageId, len(globalPushWorker.jobQueue))
		return nil
	default:
		log.Printf("Cannot queue push notification for message %d: queue is full", job.MessageId)
		return fmt.Errorf("push notification queue is full")
	}
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
	log.Printf("[PUSH_NOTIFICATION] Processing notification for message %d to %d recipients",
		job.MessageId, len(job.RecipientUserIds))

	// Get device tokens for all recipients
	var allTokens []PushDeviceToken
	vbolt.WithReadTx(pw.db, func(tx *vbolt.Tx) {
		for _, userId := range job.RecipientUserIds {
			tokens := GetActiveDeviceTokensForUser(tx, userId)
			allTokens = append(allTokens, tokens...)
		}
	})

	if len(allTokens) == 0 {
		log.Printf("[PUSH_NOTIFICATION] No active device tokens for message %d recipients", job.MessageId)
		return
	}

	log.Printf("[PUSH_NOTIFICATION] Found %d device tokens for message %d", len(allTokens), job.MessageId)

	// Send notification to each device
	for _, token := range allTokens {
		if token.Platform == "ios" {
			err := pw.sendAPNsNotification(token, job)
			if err != nil {
				log.Printf("[PUSH_NOTIFICATION] Failed to send to device %d: %v", token.Id, err)
			}
		}
		// Android support can be added here in the future
	}
}

// sendAPNsNotification sends a push notification via APNs
func (pw *PushWorker) sendAPNsNotification(token PushDeviceToken, job PushNotificationJob) error {
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

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
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
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Get JWT token for APNs
	jwtToken, err := pw.getAPNsJWT()
	if err != nil {
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
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response
	if resp.StatusCode == http.StatusOK {
		log.Printf("[PUSH_NOTIFICATION] Successfully sent to device %d", token.Id)
		return nil
	}

	// Read error response
	bodyBytes, _ := io.ReadAll(resp.Body)
	var errorResp struct {
		Reason string `json:"reason"`
	}
	json.Unmarshal(bodyBytes, &errorResp)

	// Handle specific errors that require token deactivation
	if errorResp.Reason == "BadDeviceToken" || errorResp.Reason == "Unregistered" {
		log.Printf("[PUSH_NOTIFICATION] Deactivating invalid token %d: %s", token.Id, errorResp.Reason)
		DeactivatePushDeviceTokenById(pw.db, token.Id)
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
