package backend

import (
	"errors"
	"family/cfg"
	"slices"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterChatMethods(app *vbeam.Application) {
	InitializeChatHub()

	vbeam.RegisterProc(app, SendMessage)
	vbeam.RegisterProc(app, GetChatMessages)
	vbeam.RegisterProc(app, DeleteMessage)
}

type SendMessageRequest struct {
	Content         string `json:"content"`
	ClientMessageId string `json:"clientMessageId"`
	FamilyId        int    `json:"familyId,omitempty"`
}

type SendMessageResponse struct {
	Message ChatMessage `json:"message"`
}

type GetChatMessagesRequest struct {
	Limit    *int `json:"limit,omitempty"`
	Offset   *int `json:"offset,omitempty"`
	FamilyId int  `json:"familyId,omitempty"`
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

type ChatMessage struct {
	Id              int       `json:"id"`
	FamilyId        int       `json:"familyId"`
	UserId          int       `json:"userId"`
	UserName        string    `json:"userName"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"createdAt"`
	ClientMessageId string    `json:"clientMessageId"`
}

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

var ChatMessagesBkt = vbolt.Bucket(&cfg.Info, "chat_messages", vpack.FInt, PackChatMessage)

var ChatMessagesByFamilyIndex = vbolt.Index(&cfg.Info, "chat_messages_by_family", vpack.FInt, vpack.FInt)

var ChatMessagesByUserIndex = vbolt.Index(&cfg.Info, "chat_messages_by_user", vpack.FInt, vpack.FInt)

func GetChatMessageById(tx *vbolt.Tx, messageId int) (message ChatMessage) {
	vbolt.Read(tx, ChatMessagesBkt, messageId, &message)
	return
}

func GetFamilyChatMessages(tx *vbolt.Tx, familyId int, limit int, offset int) (messages []ChatMessage) {
	var messageIds []int
	vbolt.ReadTermTargets(tx, ChatMessagesByFamilyIndex, familyId, &messageIds, vbolt.Window{
		Limit:     limit,
		Offset:    offset,
		Direction: vbolt.IterateReverse,
	})
	if len(messageIds) > 0 {
		slices.Reverse(messageIds)
		vbolt.ReadSlice(tx, ChatMessagesBkt, messageIds, &messages)
	}
	return
}

func GetChatMessageForUser(tx *vbolt.Tx, messageId int, user User, need AccessLevel) (ChatMessage, error) {
	message := GetChatMessageById(tx, messageId)
	if message.Id == 0 {
		return message, errors.New("Message not found")
	}
	if !CanAccessFamily(tx, user, message.FamilyId, need) {
		return message, errors.New("Access denied: message belongs to another family")
	}
	return message, nil
}

func GetChatMessageByIdAndFamily(tx *vbolt.Tx, messageId int, familyId int) (ChatMessage, error) {
	message := GetChatMessageById(tx, messageId)
	if message.Id == 0 {
		return message, errors.New("Message not found")
	}
	if !CanFamilyAccess(tx, familyId, message.FamilyId, AccessView) {
		return message, errors.New("Access denied: message belongs to another family")
	}
	return message, nil
}

func AddChatMessageTx(tx *vbolt.Tx, req SendMessageRequest, familyId int, userId int, userName string) (ChatMessage, error) {
	var message ChatMessage

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
	message, err := GetChatMessageByIdAndFamily(tx, messageId, familyId)
	if err != nil {
		return err
	}

	vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, -1)
	vbolt.SetTargetSingleTerm(tx, ChatMessagesByUserIndex, message.Id, -1)

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

func SendMessage(ctx *vbeam.Context, req SendMessageRequest) (resp SendMessageResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateSendMessageRequest(req); err != nil {
		return
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	message, err := AddChatMessageTx(ctx.Tx, req, familyId, user.Id, user.Name)
	if err != nil {
		return
	}

	queueChatPushNotifications(ctx.Tx, user, message)

	vbolt.TxCommit(ctx.Tx)

	if hub := GetChatHub(); hub != nil {
		hub.BroadcastNewMessage(message.FamilyId, message)
	}

	LogInfo(LogCategoryAPI, "Chat message sent", map[string]interface{}{
		"messageId": message.Id,
		"familyId":  message.FamilyId,
		"userId":    user.Id,
		"length":    len(message.Content),
	})

	resp.Message = message
	return
}

func GetChatMessages(ctx *vbeam.Context, req GetChatMessagesRequest) (resp GetChatMessagesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	limit := 100
	if req.Limit != nil && *req.Limit > 0 && *req.Limit <= 200 {
		limit = *req.Limit
	}

	offset := 0
	if req.Offset != nil && *req.Offset > 0 {
		offset = *req.Offset
	}

	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}

	resp.Messages = GetFamilyChatMessages(ctx.Tx, familyId, limit, offset)

	LogInfo(LogCategoryAPI, "Chat messages retrieved", map[string]interface{}{
		"familyId":     familyId,
		"userId":       user.Id,
		"messageCount": len(resp.Messages),
		"limit":        limit,
		"offset":       offset,
	})

	return
}

func DeleteMessage(ctx *vbeam.Context, req DeleteMessageRequest) (resp DeleteMessageResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if err = validateDeleteMessageRequest(req); err != nil {
		return
	}

	message, err := GetChatMessageForUser(ctx.Tx, req.Id, user, AccessContribute)
	if err != nil {
		return
	}

	if message.UserId != user.Id {
		err = errors.New("You can only delete your own messages")
		return
	}

	vbeam.UseWriteTx(ctx)
	err = DeleteChatMessageTx(ctx.Tx, req.Id, message.FamilyId)
	if err != nil {
		return
	}

	vbolt.TxCommit(ctx.Tx)

	if hub := GetChatHub(); hub != nil {
		hub.BroadcastDeleteMessage(message.FamilyId, req.Id, user.Id)
	}

	LogInfo(LogCategoryAPI, "Chat message deleted", map[string]interface{}{
		"messageId": req.Id,
		"familyId":  message.FamilyId,
		"userId":    user.Id,
	})

	resp.Success = true
	return
}

func queueChatPushNotifications(tx *vbolt.Tx, sender User, message ChatMessage) {
	if !IsPushWorkerEnabled() {
		return
	}

	familyUserIds := GetFamilyUserIds(tx, message.FamilyId)
	if len(familyUserIds) == 0 {
		return
	}

	var onlineUserIds []int
	if hub := GetChatHub(); hub != nil {
		onlineUserIds = hub.GetOnlineUsers(message.FamilyId)
	}

	onlineSet := make(map[int]bool)
	for _, userId := range onlineUserIds {
		onlineSet[userId] = true
	}

	var offlineUserIds []int
	for _, userId := range familyUserIds {
		if userId != sender.Id && !onlineSet[userId] {
			offlineUserIds = append(offlineUserIds, userId)
		}
	}

	if len(offlineUserIds) == 0 {
		return
	}

	job := PushNotificationJob{
		Event:            PushEventChatMessage,
		RecordId:         message.Id,
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
