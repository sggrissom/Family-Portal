package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterNotificationPreferenceMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetNotificationPreferences)
	vbeam.RegisterProc(app, UpdateNotificationPreferences)
}

type NotificationPreferences struct {
	UserId          int       `json:"userId"`
	ChatEnabled     bool      `json:"chatEnabled"`
	ShowMessageText bool      `json:"showMessageText"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func PackNotificationPreferences(self *NotificationPreferences, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.UserId, buf)
	vpack.Bool(&self.ChatEnabled, buf)
	vpack.Bool(&self.ShowMessageText, buf)
	vpack.Time(&self.UpdatedAt, buf)
}

var NotificationPreferencesBkt = vbolt.Bucket(&cfg.Info, "notification_preferences", vpack.FInt, PackNotificationPreferences)

func defaultNotificationPreferences(userId int) NotificationPreferences {
	return NotificationPreferences{
		UserId:          userId,
		ChatEnabled:     true,
		ShowMessageText: false,
	}
}

func loadNotificationPreferences(tx *vbolt.Tx, userId int) NotificationPreferences {
	var prefs NotificationPreferences
	vbolt.Read(tx, NotificationPreferencesBkt, userId, &prefs)
	if prefs.UserId == 0 {
		return defaultNotificationPreferences(userId)
	}
	return prefs
}

func deleteNotificationPreferencesTx(tx *vbolt.Tx, userId int) {
	vbolt.Delete(tx, NotificationPreferencesBkt, userId)
}

func (prefs NotificationPreferences) allowsEvent(event string) bool {
	switch event {
	case PushEventChatMessage:
		return prefs.ChatEnabled
	case PushEventTest:
		return true
	}
	return false
}

type NotificationPreferencesResponse struct {
	ChatEnabled     bool `json:"chatEnabled"`
	ShowMessageText bool `json:"showMessageText"`
}

type UpdateNotificationPreferencesRequest struct {
	ChatEnabled     bool `json:"chatEnabled"`
	ShowMessageText bool `json:"showMessageText"`
}

type UpdateNotificationPreferencesResponse struct {
	Preferences NotificationPreferencesResponse `json:"preferences"`
}

func notificationPreferencesResponse(prefs NotificationPreferences) NotificationPreferencesResponse {
	return NotificationPreferencesResponse{
		ChatEnabled:     prefs.ChatEnabled,
		ShowMessageText: prefs.ShowMessageText,
	}
}

func GetNotificationPreferences(ctx *vbeam.Context, req Empty) (resp NotificationPreferencesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	resp = notificationPreferencesResponse(loadNotificationPreferences(ctx.Tx, user.Id))
	return
}

func UpdateNotificationPreferences(ctx *vbeam.Context, req UpdateNotificationPreferencesRequest) (resp UpdateNotificationPreferencesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	prefs := NotificationPreferences{
		UserId:          user.Id,
		ChatEnabled:     req.ChatEnabled,
		ShowMessageText: req.ShowMessageText,
		UpdatedAt:       time.Now(),
	}
	if prefs.UserId == 0 {
		err = errors.New("invalid account")
		return
	}

	vbeam.UseWriteTx(ctx)
	vbolt.Write(ctx.Tx, NotificationPreferencesBkt, prefs.UserId, &prefs)
	vbolt.TxCommit(ctx.Tx)

	resp.Preferences = notificationPreferencesResponse(prefs)
	return
}
