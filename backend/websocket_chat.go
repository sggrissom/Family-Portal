package backend

import (
	"context"
	"encoding/json"
	"family/cfg"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"go.hasen.dev/vbeam"
)

// getAllowedOrigins returns allowed WebSocket origins based on environment
func getAllowedOrigins() []string {
	// Get site root from environment or use default
	siteRoot := os.Getenv("SITE_ROOT")
	if siteRoot == "" {
		siteRoot = cfg.SiteURL
	}

	allowedOrigins := []string{}

	// Add the configured site URL
	if siteRoot != "" {
		allowedOrigins = append(allowedOrigins, siteRoot)
	}

	// For development, allow localhost with any port
	if strings.Contains(siteRoot, "localhost") || strings.Contains(cfg.SiteURL, "localhost") {
		allowedOrigins = append(allowedOrigins,
			"http://localhost:*",
			"http://127.0.0.1:*",
			"http://family.localhost:*",
		)
	}

	// For production, ensure we have the production domain
	if strings.Contains(cfg.SiteURL, "grissom.zone") {
		allowedOrigins = append(allowedOrigins, "https://grissom.zone")
	}

	LogInfo(LogCategorySystem, "WebSocket allowed origins configured", map[string]interface{}{
		"origins":  allowedOrigins,
		"siteRoot": siteRoot,
	})

	return allowedOrigins
}

// WebSocket accept options with CORS support
var acceptOptions = &websocket.AcceptOptions{
	// Enable compression
	CompressionMode: websocket.CompressionNoContextTakeover,
	// CORS origin check - dynamically set based on environment
	OriginPatterns: getAllowedOrigins(),
}

// WebSocket message types
const (
	WSMsgTypeNewMessage    = "new_message"
	WSMsgTypeDeleteMessage = "delete_message"
	WSMsgTypeUserTyping    = "user_typing"
	WSMsgTypeUserOnline    = "user_online"
	WSMsgTypeUserOffline   = "user_offline"
	WSMsgTypeHeartbeat     = "heartbeat"
	WSMsgTypeError         = "error"
)

// WebSocket message structure
type WSMessage struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Message payload for new chat messages
type WSNewMessagePayload struct {
	Message ChatMessage `json:"message"`
}

// Message payload for deleted messages
type WSDeleteMessagePayload struct {
	MessageId int `json:"messageId"`
	UserId    int `json:"userId"`
}

// Message payload for typing indicator
type WSTypingPayload struct {
	UserId   int    `json:"userId"`
	UserName string `json:"userName"`
	IsTyping bool   `json:"isTyping"`
}

// Message payload for user online status
type WSUserStatusPayload struct {
	UserId   int    `json:"userId"`
	UserName string `json:"userName"`
	IsOnline bool   `json:"isOnline"`
}

// Client represents a websocket connection
type Client struct {
	hub      *ChatHub
	conn     *websocket.Conn
	send     chan []byte
	userId   int
	familyId int
	userName string
	lastSeen time.Time
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// ChatHub maintains active connections and broadcasts messages
type ChatHub struct {
	// Map of familyId to map of clients
	families map[int]map[*Client]bool

	// Broadcast channel for sending messages to all clients in a family
	broadcast chan BroadcastMessage

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe access to families map
	mu sync.RWMutex
}

// BroadcastMessage contains a message and target family
type BroadcastMessage struct {
	FamilyId int
	Message  WSMessage
}

// Global chat hub instance
var globalChatHub *ChatHub

// InitializeChatHub creates and starts the global chat hub
func InitializeChatHub() *ChatHub {
	hub := &ChatHub{
		families:   make(map[int]map[*Client]bool),
		broadcast:  make(chan BroadcastMessage, 256),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
	}

	go hub.run()
	globalChatHub = hub

	// Start heartbeat checker
	go hub.heartbeatChecker()

	LogInfo(LogCategorySystem, "Chat hub initialized", map[string]interface{}{
		"broadcast_buffer": 256,
		"register_buffer":  256,
	})

	return hub
}

// GetChatHub returns the global chat hub instance
func GetChatHub() *ChatHub {
	return globalChatHub
}

// run handles hub operations
func (h *ChatHub) run() {
	defer func() {
		if r := recover(); r != nil {
			LogErrorSimple(LogCategorySystem, "Chat hub goroutine panic, restarting", map[string]interface{}{
				"panic": r,
			})
			// Restart the hub goroutine
			go h.run()
		}
	}()

	LogInfo(LogCategorySystem, "Chat hub goroutine started", map[string]interface{}{
		"time": time.Now(),
	})

	for {
		select {
		case client := <-h.register:
			LogInfo(LogCategoryAPI, "Processing register request", map[string]interface{}{
				"userId":   client.userId,
				"familyId": client.familyId,
			})
			h.registerClient(client)

		case client := <-h.unregister:
			LogInfo(LogCategoryAPI, "Processing unregister request", map[string]interface{}{
				"userId":   client.userId,
				"familyId": client.familyId,
			})
			h.unregisterClient(client)

		case message := <-h.broadcast:
			LogInfo(LogCategoryAPI, "Processing broadcast request", map[string]interface{}{
				"familyId":    message.FamilyId,
				"messageType": message.Message.Type,
			})
			h.broadcastToFamily(message.FamilyId, message.Message)
		}
	}
}

// registerClient adds a client to the hub
func (h *ChatHub) registerClient(client *Client) {
	h.mu.Lock()
	shouldBroadcastOnline := false

	if h.families[client.familyId] == nil {
		h.families[client.familyId] = make(map[*Client]bool)
	}

	// Check if user has any other connections before adding this one
	hasOtherConnections := false
	for otherClient := range h.families[client.familyId] {
		if otherClient.userId == client.userId {
			hasOtherConnections = true
			break
		}
	}

	// Add the new client
	h.families[client.familyId][client] = true
	familyClients := len(h.families[client.familyId])
	totalFamilies := len(h.families)

	// Set flag to broadcast online status outside the lock
	shouldBroadcastOnline = !hasOtherConnections

	h.mu.Unlock()

	LogInfo(LogCategoryAPI, "WebSocket client registered", map[string]interface{}{
		"userId":              client.userId,
		"familyId":            client.familyId,
		"userName":            client.userName,
		"familyClients":       familyClients,
		"totalFamilies":       totalFamilies,
		"hasOtherConnections": hasOtherConnections,
		"willBroadcastOnline": shouldBroadcastOnline,
	})

	// Notify family that user came online (only if this is their first connection)
	if shouldBroadcastOnline {
		h.broadcastToFamily(client.familyId, WSMessage{
			Type: WSMsgTypeUserOnline,
			Payload: WSUserStatusPayload{
				UserId:   client.userId,
				UserName: client.userName,
				IsOnline: true,
			},
			Timestamp: time.Now(),
		})
	}
}

// unregisterClient removes a client from the hub
func (h *ChatHub) unregisterClient(client *Client) {
	h.mu.Lock()
	shouldBroadcastOffline := false

	if clients, ok := h.families[client.familyId]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			// Clean up empty family groups
			if len(clients) == 0 {
				delete(h.families, client.familyId)
			}

			LogInfo(LogCategoryAPI, "WebSocket client unregistered", map[string]interface{}{
				"userId":   client.userId,
				"familyId": client.familyId,
				"userName": client.userName,
			})

			// Check if user has any other connections
			hasOtherConnections := false
			for otherClient := range clients {
				if otherClient.userId == client.userId {
					hasOtherConnections = true
					break
				}
			}

			// Set flag to broadcast offline status outside the lock
			shouldBroadcastOffline = !hasOtherConnections
		}
	}
	h.mu.Unlock()

	// Broadcast offline status outside mutex to avoid deadlock
	if shouldBroadcastOffline {
		h.broadcastToFamily(client.familyId, WSMessage{
			Type: WSMsgTypeUserOffline,
			Payload: WSUserStatusPayload{
				UserId:   client.userId,
				UserName: client.userName,
				IsOnline: false,
			},
			Timestamp: time.Now(),
		})
	}
}

// broadcastToFamily sends a message to all clients in a family
func (h *ChatHub) broadcastToFamily(familyId int, message WSMessage) {
	h.mu.RLock()
	clients := h.families[familyId]
	if clients == nil {
		h.mu.RUnlock()
		LogInfo(LogCategoryAPI, "No clients found for family", map[string]interface{}{
			"familyId":    familyId,
			"messageType": message.Type,
		})
		return
	}

	// Create a slice of clients to avoid modification during iteration
	clientList := make([]*Client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	h.mu.RUnlock()

	messageBytes, err := json.Marshal(message)
	if err != nil {
		LogErrorSimple(LogCategoryAPI, "Failed to marshal WebSocket message", map[string]interface{}{
			"familyId":    familyId,
			"messageType": message.Type,
			"error":       err.Error(),
		})
		return
	}

	LogInfo(LogCategoryAPI, "Broadcasting message to family clients", map[string]interface{}{
		"familyId":    familyId,
		"messageType": message.Type,
		"clientCount": len(clientList),
	})

	// Track failed clients for cleanup
	var failedClients []*Client

	// Send to all clients in the family
	for _, client := range clientList {
		select {
		case client.send <- messageBytes:
			// Message sent successfully
		default:
			// Client's send channel is blocked, mark for cleanup
			failedClients = append(failedClients, client)
			LogWarn(LogCategoryAPI, "Client send channel blocked, marking for cleanup", map[string]interface{}{
				"familyId": familyId,
				"userId":   client.userId,
			})
		}
	}

	// Clean up failed clients outside the broadcast loop
	if len(failedClients) > 0 {
		h.cleanupFailedClients(familyId, failedClients)
	}

	LogInfo(LogCategoryAPI, "Message broadcast completed", map[string]interface{}{
		"familyId":      familyId,
		"messageType":   message.Type,
		"sentTo":        len(clientList) - len(failedClients),
		"failedClients": len(failedClients),
	})
}

// cleanupFailedClients removes clients that failed to receive messages
func (h *ChatHub) cleanupFailedClients(familyId int, failedClients []*Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := h.families[familyId]
	if clients == nil {
		return
	}

	for _, client := range failedClients {
		if _, exists := clients[client]; exists {
			close(client.send)
			delete(clients, client)
			LogInfo(LogCategoryAPI, "Cleaned up failed client", map[string]interface{}{
				"familyId": familyId,
				"userId":   client.userId,
			})
		}
	}

	// Clean up empty family groups
	if len(clients) == 0 {
		delete(h.families, familyId)
		LogInfo(LogCategoryAPI, "Removed empty family group", map[string]interface{}{
			"familyId": familyId,
		})
	}
}

// BroadcastNewMessage broadcasts a new chat message to family members
func (h *ChatHub) BroadcastNewMessage(familyId int, message ChatMessage) {
	if h == nil {
		LogWarn(LogCategoryAPI, "BroadcastNewMessage called with nil hub", map[string]interface{}{
			"familyId":  familyId,
			"messageId": message.Id,
		})
		return
	}

	LogInfo(LogCategoryAPI, "BroadcastNewMessage called", map[string]interface{}{
		"familyId":  familyId,
		"messageId": message.Id,
		"userId":    message.UserId,
		"content":   message.Content,
	})

	wsMessage := WSMessage{
		Type: WSMsgTypeNewMessage,
		Payload: WSNewMessagePayload{
			Message: message,
		},
		Timestamp: time.Now(),
	}

	select {
	case h.broadcast <- BroadcastMessage{FamilyId: familyId, Message: wsMessage}:
		LogInfo(LogCategoryAPI, "Message queued for broadcast", map[string]interface{}{
			"familyId":  familyId,
			"messageId": message.Id,
		})
	default:
		LogWarn(LogCategoryAPI, "WebSocket broadcast channel full", map[string]interface{}{
			"familyId":    familyId,
			"messageType": WSMsgTypeNewMessage,
			"messageId":   message.Id,
		})
	}
}

// BroadcastDeleteMessage broadcasts a message deletion to family members
func (h *ChatHub) BroadcastDeleteMessage(familyId int, messageId int, userId int) {
	if h == nil {
		return
	}

	wsMessage := WSMessage{
		Type: WSMsgTypeDeleteMessage,
		Payload: WSDeleteMessagePayload{
			MessageId: messageId,
			UserId:    userId,
		},
		Timestamp: time.Now(),
	}

	select {
	case h.broadcast <- BroadcastMessage{FamilyId: familyId, Message: wsMessage}:
	default:
		LogWarn(LogCategoryAPI, "WebSocket broadcast channel full", map[string]interface{}{
			"familyId":    familyId,
			"messageType": WSMsgTypeDeleteMessage,
		})
	}
}

// heartbeatChecker periodically checks for stale connections
func (h *ChatHub) heartbeatChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.RLock()
		var staleClients []*Client
		for _, clients := range h.families {
			for client := range clients {
				client.mu.RLock()
				if time.Since(client.lastSeen) > 60*time.Second {
					staleClients = append(staleClients, client)
				}
				client.mu.RUnlock()
			}
		}
		h.mu.RUnlock()

		// Disconnect stale clients
		for _, client := range staleClients {
			client.conn.Close(websocket.StatusGoingAway, "Connection stale")
		}
	}
}

// WebSocket connection handler
func HandleWebSocketChat(app *vbeam.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		LogInfo(LogCategoryAPI, "WebSocket connection attempt", map[string]interface{}{
			"path":   r.URL.Path,
			"origin": r.Header.Get("Origin"),
			"host":   r.Host,
		})

		// Authenticate before upgrade
		user, err := authenticateWebSocketRequest(r, app)
		if err != nil {
			LogWarnWithRequest(r, LogCategoryAPI, "WebSocket authentication failed", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		// Accept the WebSocket connection
		conn, err := websocket.Accept(w, r, acceptOptions)
		if err != nil {
			LogErrorWithRequest(r, LogCategoryAPI, "WebSocket accept failed", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})
			return
		}

		LogInfo(LogCategoryAPI, "WebSocket connection successful", map[string]interface{}{
			"userId":   user.Id,
			"userName": user.Name,
			"familyId": user.FamilyId,
		})

		// Create context for this connection
		ctx, cancel := context.WithCancel(context.Background())

		// Create client
		client := &Client{
			hub:      GetChatHub(),
			conn:     conn,
			send:     make(chan []byte, 256),
			userId:   user.Id,
			familyId: user.FamilyId,
			userName: user.Name,
			lastSeen: time.Now(),
			ctx:      ctx,
			cancel:   cancel,
		}

		// Register client with hub
		client.hub.register <- client

		// Start goroutines for reading and writing
		go client.writePump()
		go client.readPump()
	}
}

// authenticateWebSocketRequest validates the WebSocket connection request
func authenticateWebSocketRequest(r *http.Request, app *vbeam.Application) (User, error) {
	// Log request details for debugging
	cookies := ""
	for _, cookie := range r.Cookies() {
		if cookie.Name == "authToken" {
			cookies += "authToken=<present>; "
		} else {
			cookies += cookie.Name + "=" + cookie.Value + "; "
		}
	}

	LogInfo(LogCategoryAPI, "WebSocket authentication attempt", map[string]interface{}{
		"cookies":   cookies,
		"userAgent": r.UserAgent(),
		"origin":    r.Header.Get("Origin"),
		"host":      r.Host,
	})

	// Use existing authentication logic - relies on authToken cookie sent with request
	user, err := AuthenticateRequest(r)
	if err != nil {
		LogWarn(LogCategoryAPI, "WebSocket authentication failed", map[string]interface{}{
			"error": err.Error(),
		})
		return User{}, err
	}

	LogInfo(LogCategoryAPI, "WebSocket authentication successful", map[string]interface{}{
		"userId":   user.Id,
		"userName": user.Name,
		"familyId": user.FamilyId,
	})

	return user, nil
}

// readPump handles incoming WebSocket messages
func (c *Client) readPump() {
	defer func() {
		c.cancel() // Cancel context to stop write pump
		c.hub.unregister <- c
		c.conn.CloseNow()
	}()

	// Create context with timeout for reads
	ctx := c.ctx

	for {
		// Set read timeout
		readCtx, cancel := context.WithTimeout(ctx, 60*time.Second)

		var wsMsg WSMessage
		err := wsjson.Read(readCtx, c.conn, &wsMsg)
		cancel() // Cancel timeout context

		if err != nil {
			// Check if it's a normal close or context cancellation
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway ||
				ctx.Err() != nil {
				// Normal close or context cancelled
				break
			}

			LogWarn(LogCategoryAPI, "WebSocket read error", map[string]interface{}{
				"userId": c.userId,
				"error":  err.Error(),
				"status": websocket.CloseStatus(err),
			})
			break
		}

		c.mu.Lock()
		c.lastSeen = time.Now()
		c.mu.Unlock()

		// Handle incoming message based on type
		c.handleIncomingMessage(wsMsg)
	}
}

// writePump handles outgoing WebSocket messages
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	ctx := c.ctx

	for {
		select {
		case message, ok := <-c.send:
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

			if !ok {
				// Channel closed
				cancel()
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			err := c.conn.Write(writeCtx, websocket.MessageText, message)
			cancel()

			if err != nil {
				return
			}

		case <-ticker.C:
			// Send ping
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.conn.Ping(pingCtx)
			cancel()

			if err != nil {
				return
			}

		case <-ctx.Done():
			// Context cancelled, exit
			return
		}
	}
}

// handleIncomingMessage processes incoming WebSocket messages
func (c *Client) handleIncomingMessage(wsMsg WSMessage) {
	switch wsMsg.Type {
	case WSMsgTypeUserTyping:
		// Parse incoming typing payload
		var incomingPayload WSTypingPayload
		if payloadBytes, err := json.Marshal(wsMsg.Payload); err == nil {
			if err := json.Unmarshal(payloadBytes, &incomingPayload); err == nil {
				// Broadcast typing indicator to other family members with real typing state
				c.hub.broadcastToFamily(c.familyId, WSMessage{
					Type: WSMsgTypeUserTyping,
					Payload: WSTypingPayload{
						UserId:   c.userId,
						UserName: c.userName,
						IsTyping: incomingPayload.IsTyping,
					},
					Timestamp: time.Now(),
				})
			} else {
				LogWarn(LogCategoryAPI, "Failed to parse typing payload", map[string]interface{}{
					"userId": c.userId,
					"error":  err.Error(),
				})
			}
		} else {
			LogWarn(LogCategoryAPI, "Failed to marshal typing payload", map[string]interface{}{
				"userId": c.userId,
				"error":  err.Error(),
			})
		}

	case WSMsgTypeHeartbeat:
		// Respond to heartbeat with JSON
		response := WSMessage{
			Type:      WSMsgTypeHeartbeat,
			Payload:   "pong",
			Timestamp: time.Now(),
		}

		// Use wsjson to write response directly
		writeCtx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
		err := wsjson.Write(writeCtx, c.conn, response)
		cancel()

		if err != nil {
			LogWarn(LogCategoryAPI, "Failed to send heartbeat response", map[string]interface{}{
				"userId": c.userId,
				"error":  err.Error(),
			})
		}

	default:
		LogWarn(LogCategoryAPI, "Unknown WebSocket message type", map[string]interface{}{
			"userId":      c.userId,
			"messageType": wsMsg.Type,
		})
	}
}

// GetFamilyConnectionCount returns the number of active connections for a family
func (h *ChatHub) GetFamilyConnectionCount(familyId int) int {
	if h == nil {
		return 0
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.families[familyId]; ok {
		return len(clients)
	}
	return 0
}

// GetOnlineUsers returns a list of unique users currently online in a family
func (h *ChatHub) GetOnlineUsers(familyId int) []int {
	if h == nil {
		return nil
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	userIds := make(map[int]bool)
	if clients, ok := h.families[familyId]; ok {
		for client := range clients {
			userIds[client.userId] = true
		}
	}

	result := make([]int, 0, len(userIds))
	for userId := range userIds {
		result = append(result, userId)
	}
	return result
}
