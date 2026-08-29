package backend

import (
	"context"
	"encoding/json"
	"family/cfg"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"go.hasen.dev/vbeam"
)

func getAllowedOrigins() []string {
	siteRoot := os.Getenv("SITE_ROOT")
	if siteRoot == "" {
		siteRoot = cfg.SiteURL
	}

	allowedOrigins := []string{}

	if siteRoot != "" {
		allowedOrigins = append(allowedOrigins, siteRoot)
	}

	if strings.Contains(siteRoot, "localhost") || strings.Contains(cfg.SiteURL, "localhost") {
		allowedOrigins = append(allowedOrigins,
			"http://localhost:*",
			"http://127.0.0.1:*",
			"http://family.localhost:*",
		)
	}

	LogInfo(LogCategorySystem, "WebSocket allowed origins configured", map[string]interface{}{
		"origins":  allowedOrigins,
		"siteRoot": siteRoot,
	})

	return allowedOrigins
}

func createAcceptOptions() *websocket.AcceptOptions {
	return &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionNoContextTakeover,
		OriginPatterns:  getAllowedOrigins(),
	}
}

const (
	WSMsgTypeNewMessage    = "new_message"
	WSMsgTypeDeleteMessage = "delete_message"
	WSMsgTypeUserTyping    = "user_typing"
	WSMsgTypeUserOnline    = "user_online"
	WSMsgTypeUserOffline   = "user_offline"
	WSMsgTypeHeartbeat     = "heartbeat"
	WSMsgTypeError         = "error"
)

type WSMessage struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

type WSNewMessagePayload struct {
	Message ChatMessage `json:"message"`
}

type WSDeleteMessagePayload struct {
	MessageId int `json:"messageId"`
	UserId    int `json:"userId"`
}

type WSTypingPayload struct {
	UserId   int    `json:"userId"`
	UserName string `json:"userName"`
	IsTyping bool   `json:"isTyping"`
}

type WSUserStatusPayload struct {
	UserId   int    `json:"userId"`
	UserName string `json:"userName"`
	IsOnline bool   `json:"isOnline"`
}

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

type ChatHub struct {
	families map[int]map[*Client]bool

	broadcast chan BroadcastMessage

	register chan *Client

	unregister chan *Client

	mu sync.RWMutex

	closing atomic.Bool
}

type BroadcastMessage struct {
	FamilyId int
	Message  WSMessage
}

var globalChatHub *ChatHub

func InitializeChatHub() *ChatHub {
	hub := &ChatHub{
		families:   make(map[int]map[*Client]bool),
		broadcast:  make(chan BroadcastMessage, 256),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
	}

	go hub.run()
	globalChatHub = hub

	go hub.heartbeatChecker()

	LogInfo(LogCategorySystem, "Chat hub initialized", map[string]interface{}{
		"broadcast_buffer": 256,
		"register_buffer":  256,
	})

	return hub
}

func GetChatHub() *ChatHub {
	return globalChatHub
}

func (h *ChatHub) Shutdown(ctx context.Context) bool {
	h.closing.Store(true)

	h.mu.RLock()
	var clients []*Client
	for _, familyClients := range h.families {
		for client := range familyClients {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()

	LogInfo(LogCategorySystem, "Closing chat connections for shutdown", map[string]interface{}{
		"clients": len(clients),
	})

	for _, client := range clients {
		go func(c *Client) {
			if err := c.conn.Close(websocket.StatusGoingAway, "server shutting down"); err != nil {
				c.cancel()
			}
		}(client)
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if h.connectionCount() == 0 {
			return true
		}
		select {
		case <-ctx.Done():
			LogWarn(LogCategorySystem, "Chat connections did not close before the deadline", map[string]interface{}{
				"remaining": h.connectionCount(),
			})
			return false
		case <-ticker.C:
		}
	}
}

func (h *ChatHub) connectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, clients := range h.families {
		total += len(clients)
	}
	return total
}

func ShutdownChatHub(ctx context.Context) bool {
	if globalChatHub == nil {
		return true
	}
	return globalChatHub.Shutdown(ctx)
}

func (h *ChatHub) run() {
	defer func() {
		if r := recover(); r != nil {
			LogErrorSimple(LogCategorySystem, "Chat hub goroutine panic, restarting", map[string]interface{}{
				"panic": r,
			})
			go h.run()
		}
	}()

	LogInfo(LogCategorySystem, "Chat hub goroutine started", map[string]interface{}{
		"time": time.Now(),
	})

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastToFamily(message.FamilyId, message.Message)
		}
	}
}

func (h *ChatHub) registerClient(client *Client) {
	h.mu.Lock()
	shouldBroadcastOnline := false

	if h.families[client.familyId] == nil {
		h.families[client.familyId] = make(map[*Client]bool)
	}

	hasOtherConnections := false
	for otherClient := range h.families[client.familyId] {
		if otherClient.userId == client.userId {
			hasOtherConnections = true
			break
		}
	}

	h.families[client.familyId][client] = true

	shouldBroadcastOnline = !hasOtherConnections

	h.mu.Unlock()

	LogInfo(LogCategoryAPI, "WebSocket client registered", map[string]interface{}{
		"userId":   client.userId,
		"familyId": client.familyId,
	})

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

func (h *ChatHub) unregisterClient(client *Client) {
	h.mu.Lock()
	shouldBroadcastOffline := false

	if clients, ok := h.families[client.familyId]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			if len(clients) == 0 {
				delete(h.families, client.familyId)
			}

			LogInfo(LogCategoryAPI, "WebSocket client unregistered", map[string]interface{}{
				"userId":   client.userId,
				"familyId": client.familyId,
			})

			hasOtherConnections := false
			for otherClient := range clients {
				if otherClient.userId == client.userId {
					hasOtherConnections = true
					break
				}
			}

			shouldBroadcastOffline = !hasOtherConnections
		}
	}
	h.mu.Unlock()

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

	var failedClients []*Client

	for _, client := range clientList {
		select {
		case client.send <- messageBytes:
		default:
			failedClients = append(failedClients, client)
			LogWarn(LogCategoryAPI, "Client send channel blocked, marking for cleanup", map[string]interface{}{
				"familyId": familyId,
				"userId":   client.userId,
			})
		}
	}

	if len(failedClients) > 0 {
		h.cleanupFailedClients(familyId, failedClients)
	}
}

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

	if len(clients) == 0 {
		delete(h.families, familyId)
		LogInfo(LogCategoryAPI, "Removed empty family group", map[string]interface{}{
			"familyId": familyId,
		})
	}
}

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
	default:
		LogWarn(LogCategoryAPI, "WebSocket broadcast channel full", map[string]interface{}{
			"familyId":    familyId,
			"messageType": WSMsgTypeNewMessage,
			"messageId":   message.Id,
		})
	}
}

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

		for _, client := range staleClients {
			client.conn.Close(websocket.StatusGoingAway, "Connection stale")
		}
	}
}

func HandleWebSocketChat(app *vbeam.Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hub := GetChatHub()
		if hub == nil || hub.closing.Load() {
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
			return
		}

		user, err := authenticateWebSocketRequest(r, app)
		if err != nil {
			LogWarnWithRequest(r, LogCategoryAPI, "WebSocket authentication failed", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, createAcceptOptions())
		if err != nil {
			LogErrorWithRequest(r, LogCategoryAPI, "WebSocket accept failed", map[string]interface{}{
				"error": err.Error(),
				"path":  r.URL.Path,
			})
			return
		}

		ctx, cancel := context.WithCancel(context.Background())

		client := &Client{
			hub:      hub,
			conn:     conn,
			send:     make(chan []byte, 256),
			userId:   user.Id,
			familyId: user.FamilyId,
			userName: user.Name,
			lastSeen: time.Now(),
			ctx:      ctx,
			cancel:   cancel,
		}

		client.hub.register <- client

		go client.writePump()
		go client.readPump()
	}
}

func authenticateWebSocketRequest(r *http.Request, app *vbeam.Application) (User, error) {
	user, err := AuthenticateRequest(r)
	if err != nil {
		LogWarn(LogCategoryAPI, "WebSocket authentication failed", map[string]interface{}{
			"error": err.Error(),
		})
		return User{}, err
	}

	return user, nil
}

func (c *Client) readPump() {
	defer func() {
		c.cancel()
		c.hub.unregister <- c
		c.conn.CloseNow()
	}()

	ctx := c.ctx
	c.conn.SetReadLimit(64 << 10)

	for {
		readCtx, cancel := context.WithTimeout(ctx, 60*time.Second)

		var wsMsg WSMessage
		err := wsjson.Read(readCtx, c.conn, &wsMsg)
		cancel()

		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway ||
				ctx.Err() != nil {
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

		c.handleIncomingMessage(wsMsg)
	}
}

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
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.conn.Ping(pingCtx)
			cancel()

			if err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) handleIncomingMessage(wsMsg WSMessage) {
	switch wsMsg.Type {
	case WSMsgTypeUserTyping:
		var incomingPayload WSTypingPayload
		if payloadBytes, err := json.Marshal(wsMsg.Payload); err == nil {
			if err := json.Unmarshal(payloadBytes, &incomingPayload); err == nil {
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
		response := WSMessage{
			Type:      WSMsgTypeHeartbeat,
			Payload:   "pong",
			Timestamp: time.Now(),
		}

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
