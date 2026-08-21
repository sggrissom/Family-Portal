package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// A push notification is the one part of this application that shows family
// content to whoever is holding the phone, without anybody signing in first. So
// the defaults here are deliberately quiet: notifications arrive, but the text
// of a message does not reach the lock screen until the account asks for it.
//
// Preferences are per user rather than per family or per device. A user's own
// phone is the thing being protected, and the same person can be in two
// families.

func RegisterNotificationPreferenceMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetNotificationPreferences)
	vbeam.RegisterProc(app, UpdateNotificationPreferences)
}

// NotificationPreferences is one account's push settings.
type NotificationPreferences struct {
	UserId int `json:"userId"`
	// ChatEnabled controls whether a chat message the user missed produces a
	// notification at all.
	ChatEnabled bool `json:"chatEnabled"`
	// ShowMessageText allows the message itself into the alert, which is what
	// iOS renders on the lock screen. Off by default.
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

// defaultNotificationPreferences is what an account gets before it has ever
// opened the settings page: notifications on, message text withheld.
func defaultNotificationPreferences(userId int) NotificationPreferences {
	return NotificationPreferences{
		UserId:          userId,
		ChatEnabled:     true,
		ShowMessageText: false,
	}
}

// loadNotificationPreferences reads an account's settings, falling back to the
// defaults when it has never saved any. Every stored row carries its own UserId,
// so a zero one is the marker for "nothing written yet" — which matters because
// the zero value of ChatEnabled would otherwise silence a user who never
// touched the setting.
func loadNotificationPreferences(tx *vbolt.Tx, userId int) NotificationPreferences {
	var prefs NotificationPreferences
	vbolt.Read(tx, NotificationPreferencesBkt, userId, &prefs)
	if prefs.UserId == 0 {
		return defaultNotificationPreferences(userId)
	}
	return prefs
}

// deleteNotificationPreferencesTx drops the row for a deleted account.
func deleteNotificationPreferencesTx(tx *vbolt.Tx, userId int) {
	vbolt.Delete(tx, NotificationPreferencesBkt, userId)
}

// allowsEvent reports whether this account wants to be told about an event.
func (prefs NotificationPreferences) allowsEvent(event string) bool {
	switch event {
	case PushEventChatMessage:
		return prefs.ChatEnabled
	case PushEventTest:
		// A test push answers "can this device receive anything at all", which
		// is a question about the delivery path rather than about content. It
		// is only ever sent by an admin naming a specific recipient, so the
		// content preferences deliberately do not apply.
		return true
	}
	return false
}

// Request/Response types

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

// vbeam procedures

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
		// Zero is the "never saved" marker in the bucket, so a user id of zero
		// would write a row that reads back as the defaults.
		err = errors.New("invalid account")
		return
	}

	vbeam.UseWriteTx(ctx)
	vbolt.Write(ctx.Tx, NotificationPreferencesBkt, prefs.UserId, &prefs)
	vbolt.TxCommit(ctx.Tx)

	resp.Preferences = notificationPreferencesResponse(prefs)
	return
}
