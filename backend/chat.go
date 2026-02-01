package backend

import (
	"errors"
	"family/cfg"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterChatMethods(app *vbeam.Application) {
	// Initialize chat hub for WebSocket functionality
	InitializeChatHub()

	// Register REST API procedures
	vbeam.RegisterProc(app, SendMessage)
	vbeam.RegisterProc(app, GetChatMessages)
	vbeam.RegisterProc(app, DeleteMessage)
}

// Request/Response types
type SendMessageRequest struct {
	Content         string `json:"content"`
	ClientMessageId string `json:"clientMessageId"`
}

type SendMessageResponse struct {
	Message ChatMessage `json:"message"`
}

type GetChatMessagesRequest struct {
	Limit  *int `json:"limit,omitempty"`
	Offset *int `json:"offset,omitempty"`
}

type GetChatMessagesResponse struct {
	Messages []ChatMessage `json:"messages"`
}

type DeleteMessageRequest struct {
	Id int `json:"id"`
}

type DeleteMessageResponse struct {
	Success bool `json:"success"`
}

// Database types
type ChatMessage struct {
	Id              int       `json:"id"`
	FamilyId        int       `json:"familyId"`
	UserId          int       `json:"userId"`
	UserName        string    `json:"userName"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"createdAt"`
	ClientMessageId string    `json:"clientMessageId"`
}

// Packing function for vbolt serialization
func PackChatMessage(self *ChatMessage, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.UserId, buf)
	vpack.String(&self.UserName, buf)
	vpack.String(&self.Content, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.String(&self.ClientMessageId, buf)
}

// Buckets for vbolt database storage
var ChatMessagesBkt = vbolt.Bucket(&cfg.Info, "chat_messages", vpack.FInt, PackChatMessage)

// ChatMessagesByFamilyIndex: term = family_id, target = message_id
// This allows efficient lookup of messages by family
var ChatMessagesByFamilyIndex = vbolt.Index(&cfg.Info, "chat_messages_by_family", vpack.FInt, vpack.FInt)

// ChatMessagesByUserIndex: term = user_id, target = message_id
// This allows efficient lookup of messages by user
var ChatMessagesByUserIndex = vbolt.Index(&cfg.Info, "chat_messages_by_user", vpack.FInt, vpack.FInt)

// Database helper functions
func GetChatMessageById(tx *vbolt.Tx, messageId int) (message ChatMessage) {
	vbolt.Read(tx, ChatMessagesBkt, messageId, &message)
	return
}

func GetFamilyChatMessages(tx *vbolt.Tx, familyId int, limit int) (messages []ChatMessage) {
	var messageIds []int
	vbolt.ReadTermTargets(tx, ChatMessagesByFamilyIndex, familyId, &messageIds, vbolt.Window{Limit: limit})
	if len(messageIds) > 0 {
		vbolt.ReadSlice(tx, ChatMessagesBkt, messageIds, &messages)
	}
	return
}

func GetChatMessageByIdAndFamily(tx *vbolt.Tx, messageId int, familyId int) (ChatMessage, error) {
	message := GetChatMessageById(tx, messageId)
	if message.Id == 0 {
		return message, errors.New("Message not found")
	}
	if message.FamilyId != familyId {
		return message, errors.New("Access denied: message belongs to another family")
	}
	return message, nil
}

func AddChatMessageTx(tx *vbolt.Tx, req SendMessageRequest, familyId int, userId int, userName string) (ChatMessage, error) {
	var message ChatMessage

	// Create message record
	message.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
	message.FamilyId = familyId
	message.UserId = userId
	message.UserName = userName
	message.Content = strings.TrimSpace(req.Content)
	message.CreatedAt = time.Now()
	message.ClientMessageId = req.ClientMessageId

	vbolt.Write(tx, ChatMessagesBkt, message.Id, &message)

	updateChatMessageIndices(tx, message)

	return message, nil
}

func updateChatMessageIndices(tx *vbolt.Tx, message ChatMessage) {
	vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, message.FamilyId)
	vbolt.SetTargetSingleTerm(tx, ChatMessagesByUserIndex, message.Id, message.UserId)
}

func DeleteChatMessageTx(tx *vbolt.Tx, messageId int, familyId int) error {
	// Get existing message and validate ownership
	message, err := GetChatMessageByIdAndFamily(tx, messageId, familyId)
	if err != nil {
		return err
	}

	// Remove from indices
	vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, -1)
	vbolt.SetTargetSingleTerm(tx, ChatMessagesByUserIndex, message.Id, -1)

	// Delete the record
	vbolt.Delete(tx, ChatMessagesBkt, message.Id)

	return nil
}

func validateSendMessageRequest(req SendMessageRequest) error {
	if strings.TrimSpace(req.Content) == "" {
		return errors.New("Message content is required")
	}
	if len(strings.TrimSpace(req.Content)) > 1000 {
		return errors.New("Message content cannot exceed 1000 characters")
	}
	return nil
}

func validateDeleteMessageRequest(req DeleteMessageRequest) error {
	if req.Id <= 0 {
		return errors.New("Message ID is required")
	}
	return nil
}

// vbeam procedures
func SendMessage(ctx *vbeam.Context, req SendMessageRequest) (resp SendMessageResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateSendMessageRequest(req); err != nil {
		return
	}

	// Add message to database
	vbeam.UseWriteTx(ctx)
	message, err := AddChatMessageTx(ctx.Tx, req, user.FamilyId, user.Id, user.Name)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	// Broadcast the new message to websocket clients
	if hub := GetChatHub(); hub != nil {
		hub.BroadcastNewMessage(user.FamilyId, message)
	}

	// Queue push notifications for offline users
	queueChatPushNotifications(ctx.Tx, user, message)

	// Log the message sending
	LogInfo(LogCategoryAPI, "Chat message sent", map[string]interface{}{
		"messageId": message.Id,
		"familyId":  user.FamilyId,
		"userId":    user.Id,
		"length":    len(message.Content),
	})

	resp.Message = message
	return
}

func GetChatMessages(ctx *vbeam.Context, req GetChatMessagesRequest) (resp GetChatMessagesResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Set default limit
	limit := 100
	if req.Limit != nil && *req.Limit > 0 && *req.Limit <= 200 {
		limit = *req.Limit
	}

	// Get messages for this family
	resp.Messages = GetFamilyChatMessages(ctx.Tx, user.FamilyId, limit)

	// Log the request
	LogInfo(LogCategoryAPI, "Chat messages retrieved", map[string]interface{}{
		"familyId":     user.FamilyId,
		"userId":       user.Id,
		"messageCount": len(resp.Messages),
		"limit":        limit,
	})

	return
}

func DeleteMessage(ctx *vbeam.Context, req DeleteMessageRequest) (resp DeleteMessageResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateDeleteMessageRequest(req); err != nil {
		return
	}

	// Get the message to verify ownership
	message, err := GetChatMessageByIdAndFamily(ctx.Tx, req.Id, user.FamilyId)
	if err != nil {
		return
	}

	// Only allow users to delete their own messages
	if message.UserId != user.Id {
		err = errors.New("You can only delete your own messages")
		return
	}

	// Delete message from database
	vbeam.UseWriteTx(ctx)
	err = DeleteChatMessageTx(ctx.Tx, req.Id, user.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	// Broadcast the message deletion to websocket clients
	if hub := GetChatHub(); hub != nil {
		hub.BroadcastDeleteMessage(user.FamilyId, req.Id, user.Id)
	}

	// Log the message deletion
	LogInfo(LogCategoryAPI, "Chat message deleted", map[string]interface{}{
		"messageId": req.Id,
		"familyId":  user.FamilyId,
		"userId":    user.Id,
	})

	resp.Success = true
	return
}

// queueChatPushNotifications queues push notifications for offline family members
func queueChatPushNotifications(tx *vbolt.Tx, sender User, message ChatMessage) {
	// Check if push notifications are enabled
	if !IsPushWorkerEnabled() {
		return
	}

	// Get all family user IDs
	familyUserIds := GetFamilyUserIds(tx, sender.FamilyId)
	if len(familyUserIds) == 0 {
		return
	}

	// Get online users from the WebSocket hub
	var onlineUserIds []int
	if hub := GetChatHub(); hub != nil {
		onlineUserIds = hub.GetOnlineUsers(sender.FamilyId)
	}

	// Create a set of online users for fast lookup
	onlineSet := make(map[int]bool)
	for _, userId := range onlineUserIds {
		onlineSet[userId] = true
	}

	// Filter to offline users (excluding sender)
	var offlineUserIds []int
	for _, userId := range familyUserIds {
		if userId != sender.Id && !onlineSet[userId] {
			offlineUserIds = append(offlineUserIds, userId)
		}
	}

	if len(offlineUserIds) == 0 {
		return
	}

	// Queue push notification job
	job := PushNotificationJob{
		MessageId:        message.Id,
		FamilyId:         sender.FamilyId,
		SenderId:         sender.Id,
		SenderName:       sender.Name,
		Content:          message.Content,
		RecipientUserIds: offlineUserIds,
	}

	if err := QueuePushNotification(job); err != nil {
		LogWarn(LogCategoryAPI, "Failed to queue push notification", map[string]interface{}{
			"messageId": message.Id,
			"error":     err.Error(),
		})
	}
}
