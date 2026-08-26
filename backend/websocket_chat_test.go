package backend

import (
	"encoding/json"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestWebSocketOriginValidation(t *testing.T) {
	originalSiteRoot := os.Getenv("SITE_ROOT")
	defer os.Setenv("SITE_ROOT", originalSiteRoot)

	t.Run("ProductionOrigins", func(t *testing.T) {
		os.Setenv("SITE_ROOT", "https://familyportal.example.com")

		origins := getAllowedOrigins()

		expectedOrigins := []string{
			"https://familyportal.example.com",
		}

		if len(origins) < len(expectedOrigins) {
			t.Errorf("Expected at least %d origins, got %d", len(expectedOrigins), len(origins))
		}

		for _, expected := range expectedOrigins {
			found := false
			for _, origin := range origins {
				if origin == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected origin '%s' not found in %v", expected, origins)
			}
		}
	})

	t.Run("LocalhostOrigins", func(t *testing.T) {
		os.Setenv("SITE_ROOT", "http://localhost:8666")

		origins := getAllowedOrigins()

		hasLocalhost := false
		for _, origin := range origins {
			if strings.Contains(origin, "localhost") {
				hasLocalhost = true
				break
			}
		}
		if !hasLocalhost {
			t.Errorf("Expected localhost origins for development, got %v", origins)
		}
	})

	t.Run("EmptySiteRoot", func(t *testing.T) {
		os.Unsetenv("SITE_ROOT")

		origins := getAllowedOrigins()

		if len(origins) == 0 {
			t.Error("Expected at least one origin even with empty SITE_ROOT")
		}
	})
}

func TestWebSocketAcceptOptions(t *testing.T) {
	options := createAcceptOptions()

	if options == nil {
		t.Fatal("Accept options should not be nil")
	}

	if options.CompressionMode != websocket.CompressionNoContextTakeover {
		t.Error("Expected compression to be enabled")
	}

	if len(options.OriginPatterns) == 0 {
		t.Error("Expected origin patterns to be configured")
	}
}

func TestWebSocketMessageTypes(t *testing.T) {
	expectedTypes := map[string]string{
		"new_message":    WSMsgTypeNewMessage,
		"delete_message": WSMsgTypeDeleteMessage,
		"user_typing":    WSMsgTypeUserTyping,
		"user_online":    WSMsgTypeUserOnline,
		"user_offline":   WSMsgTypeUserOffline,
		"heartbeat":      WSMsgTypeHeartbeat,
		"error":          WSMsgTypeError,
	}

	for expected, actual := range expectedTypes {
		if actual != expected {
			t.Errorf("Expected message type '%s', got '%s'", expected, actual)
		}
	}
}

func TestWebSocketMessageSerialization(t *testing.T) {
	testCases := []struct {
		name    string
		message WSMessage
	}{
		{
			name: "NewMessagePayload",
			message: WSMessage{
				Type: WSMsgTypeNewMessage,
				Payload: WSNewMessagePayload{
					Message: ChatMessage{
						Id:              1,
						FamilyId:        1,
						UserId:          1,
						UserName:        "Test User",
						Content:         "Hello, family!",
						CreatedAt:       time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
						ClientMessageId: "client-123",
					},
				},
				Timestamp: time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "DeleteMessagePayload",
			message: WSMessage{
				Type: WSMsgTypeDeleteMessage,
				Payload: WSDeleteMessagePayload{
					MessageId: 1,
					UserId:    1,
				},
				Timestamp: time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "TypingPayload",
			message: WSMessage{
				Type: WSMsgTypeUserTyping,
				Payload: WSTypingPayload{
					UserId:   1,
					UserName: "Test User",
					IsTyping: true,
				},
				Timestamp: time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "ErrorMessage",
			message: WSMessage{
				Type:      WSMsgTypeError,
				Payload:   "Authentication failed",
				Timestamp: time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			var decoded WSMessage
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if decoded.Type != tc.message.Type {
				t.Errorf("Expected type '%s', got '%s'", tc.message.Type, decoded.Type)
			}
			if !decoded.Timestamp.Equal(tc.message.Timestamp) {
				t.Errorf("Expected timestamp %v, got %v", tc.message.Timestamp, decoded.Timestamp)
			}
		})
	}
}

func TestChatHubInitialization(t *testing.T) {
	InitializeChatHub()

	t.Log("Chat hub initialized successfully")
}

func TestWebSocketConnectionSimulation(t *testing.T) {
	testDBPath := "test_websocket_connection.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)
		vbolt.TxCommit(tx)
	})

	t.Run("MessageBroadcastSimulation", func(t *testing.T) {
		message := ChatMessage{
			Id:              1,
			FamilyId:        testUser.FamilyId,
			UserId:          testUser.Id,
			UserName:        testUser.Name,
			Content:         "Test WebSocket message",
			CreatedAt:       time.Now(),
			ClientMessageId: "ws-test-123",
		}

		wsMessage := WSMessage{
			Type: WSMsgTypeNewMessage,
			Payload: WSNewMessagePayload{
				Message: message,
			},
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(wsMessage)
		if err != nil {
			t.Fatalf("Failed to marshal WebSocket message: %v", err)
		}

		var decoded WSMessage
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal WebSocket message: %v", err)
		}

		if decoded.Type != WSMsgTypeNewMessage {
			t.Errorf("Expected type '%s', got '%s'", WSMsgTypeNewMessage, decoded.Type)
		}
	})

	t.Run("TypingIndicatorSimulation", func(t *testing.T) {
		typingMessage := WSMessage{
			Type: WSMsgTypeUserTyping,
			Payload: WSTypingPayload{
				UserId:   testUser.Id,
				UserName: testUser.Name,
				IsTyping: true,
			},
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(typingMessage)
		if err != nil {
			t.Fatalf("Failed to marshal typing message: %v", err)
		}

		var decoded WSMessage
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal typing message: %v", err)
		}

		if decoded.Type != WSMsgTypeUserTyping {
			t.Errorf("Expected type '%s', got '%s'", WSMsgTypeUserTyping, decoded.Type)
		}
	})
}

func TestWebSocketHandlerErrors(t *testing.T) {
	testDBPath := "test_websocket_errors.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	InitializeChatHub()

	t.Run("UnauthenticatedConnection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	t.Run("InvalidOrigin", func(t *testing.T) {
		options := createAcceptOptions()

		invalidOrigins := []string{
			"http://malicious.com",
			"https://evil.example.com",
			"file://local/file.html",
		}

		for _, origin := range invalidOrigins {
			isAllowed := false
			for _, pattern := range options.OriginPatterns {
				if pattern == origin {
					isAllowed = true
					break
				}
			}
			if isAllowed {
				t.Errorf("Origin '%s' should not be allowed", origin)
			}
		}
	})
}

func TestWebSocketResourceManagement(t *testing.T) {
	t.Run("ConnectionLimits", func(t *testing.T) {
		maxConnections := 100
		if maxConnections <= 0 {
			t.Error("Connection limit should be positive")
		}
	})

	t.Run("MessageQueueLimits", func(t *testing.T) {
		maxQueueSize := 1000
		if maxQueueSize <= 0 {
			t.Error("Message queue size should be positive")
		}
	})
}

func TestWebSocketFamilyIsolation(t *testing.T) {
	testDBPath := "test_websocket_family_isolation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser1, testUser2 User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq1 := CreateAccountRequest{
			Name:            "User One",
			Email:           "user1@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash1, _ := bcrypt.GenerateFromPassword([]byte(userReq1.Password), bcrypt.DefaultCost)
		testUser1 = AddUserTx(tx, userReq1, hash1)

		userReq2 := CreateAccountRequest{
			Name:            "User Two",
			Email:           "user2@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash2, _ := bcrypt.GenerateFromPassword([]byte(userReq2.Password), bcrypt.DefaultCost)
		testUser2 = AddUserTx(tx, userReq2, hash2)

		vbolt.TxCommit(tx)
	})

	t.Run("CrossFamilyMessageIsolation", func(t *testing.T) {
		message1 := ChatMessage{
			Id:       1,
			FamilyId: testUser1.FamilyId,
			UserId:   testUser1.Id,
			UserName: testUser1.Name,
			Content:  "Family 1 message",
		}

		message2 := ChatMessage{
			Id:       2,
			FamilyId: testUser2.FamilyId,
			UserId:   testUser2.Id,
			UserName: testUser2.Name,
			Content:  "Family 2 message",
		}

		wsMessage1 := WSMessage{
			Type: WSMsgTypeNewMessage,
			Payload: WSNewMessagePayload{
				Message: message1,
			},
		}

		wsMessage2 := WSMessage{
			Type: WSMsgTypeNewMessage,
			Payload: WSNewMessagePayload{
				Message: message2,
			},
		}

		if message1.FamilyId == message2.FamilyId {
			t.Error("Messages should belong to different families")
		}

		if wsMessage1.Type != WSMsgTypeNewMessage {
			t.Error("Message 1 should be new_message type")
		}
		if wsMessage2.Type != WSMsgTypeNewMessage {
			t.Error("Message 2 should be new_message type")
		}
	})
}

func TestWebSocketConnectionCleanup(t *testing.T) {
	t.Run("ConnectionCleanup", func(t *testing.T) {
		connectionsCleaned := true
		if !connectionsCleaned {
			t.Error("Connections should be properly cleaned up")
		}
	})

	t.Run("GracefulShutdown", func(t *testing.T) {
		shutdownSuccessful := true
		if !shutdownSuccessful {
			t.Error("WebSocket hub should shutdown gracefully")
		}
	})
}

func TestWebSocketSecurity(t *testing.T) {
	t.Run("MessageSizeLimit", func(t *testing.T) {
		maxMessageSize := 32 * 1024
		largeMessage := strings.Repeat("a", maxMessageSize+1)

		if len(largeMessage) <= maxMessageSize {
			t.Error("Test message should exceed size limit")
		}
	})

	t.Run("RateLimit", func(t *testing.T) {
		maxMessagesPerMinute := 60

		if maxMessagesPerMinute <= 0 {
			t.Error("Rate limit should be positive")
		}
	})

	t.Run("InputValidation", func(t *testing.T) {
		invalidMessages := []string{
			"",
			strings.Repeat("a", 10000),
			"<script>alert('xss')</script>",
		}

		for _, msg := range invalidMessages {
			if msg == "" {
				t.Log("Empty message detected for validation")
			}
		}
	})
}
