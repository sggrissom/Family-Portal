package backend

import (
	"encoding/json"
	"family/cfg"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestNotificationPreferenceDefaults(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/prefs.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		prefs := loadNotificationPreferences(tx, 42)
		if !prefs.ChatEnabled {
			t.Error("ChatEnabled = false for an account that never saved preferences, want true")
		}
		if prefs.ShowMessageText {
			t.Error("ShowMessageText = true by default, want family content withheld until asked for")
		}
		if prefs.UserId != 42 {
			t.Errorf("UserId = %d, want 42", prefs.UserId)
		}
	})
}

func TestNotificationPreferencesRoundTrip(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/prefs.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })

	saved := NotificationPreferences{UserId: 7, ChatEnabled: false, ShowMessageText: true}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, NotificationPreferencesBkt, saved.UserId, &saved)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		got := loadNotificationPreferences(tx, 7)
		if got.ChatEnabled != false || got.ShowMessageText != true {
			t.Errorf("loaded %+v, want the saved values back rather than the defaults", got)
		}
		if other := loadNotificationPreferences(tx, 8); !other.ChatEnabled {
			t.Error("a different account picked up the saved preferences")
		}
	})
}

func TestAllowsEventIgnoresPreferencesForTestPushes(t *testing.T) {
	silent := NotificationPreferences{UserId: 1, ChatEnabled: false}

	if silent.allowsEvent(PushEventChatMessage) {
		t.Error("a chat push reached an account with chat notifications off")
	}
	if !silent.allowsEvent(PushEventTest) {
		t.Error("an admin test push was suppressed; it verifies the delivery path, not content")
	}
	if silent.allowsEvent("photo_added") {
		t.Error("an unknown event was allowed through")
	}
}

func TestQuietPayloadKeepsFamilyContentOffTheLockScreen(t *testing.T) {
	job := PushNotificationJob{
		Event:      PushEventChatMessage,
		RecordId:   12,
		FamilyId:   3,
		SenderId:   5,
		SenderName: "Dad",
		Content:    "the spare key is under the mat",
	}

	payload := buildAPNsPayload(job, defaultNotificationPreferences(5))

	alert := payload.Aps.Alert.Title + " " + payload.Aps.Alert.Body
	for _, secret := range []string{"Dad", "spare key", "under the mat"} {
		if strings.Contains(alert, secret) {
			t.Errorf("alert %q contains %q, want nothing about the family by default", alert, secret)
		}
	}
	if payload.Aps.Alert.Body == "" {
		t.Error("alert body is empty; a notification with no text is not worth delivering")
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if strings.Contains(string(encoded), "spare key") {
		t.Errorf("payload %s carries the message content", encoded)
	}
}

func TestPreviewPreferenceRestoresTheMessageText(t *testing.T) {
	job := PushNotificationJob{
		Event:      PushEventChatMessage,
		RecordId:   12,
		SenderName: "Dad",
		Content:    "dinner is ready",
	}

	prefs := defaultNotificationPreferences(5)
	prefs.ShowMessageText = true
	payload := buildAPNsPayload(job, prefs)

	if payload.Aps.Alert.Body != "Dad: dinner is ready" {
		t.Errorf("alert body = %q, want the sender-prefixed message", payload.Aps.Alert.Body)
	}
}

func TestLongPreviewIsTruncated(t *testing.T) {
	prefs := defaultNotificationPreferences(1)
	prefs.ShowMessageText = true

	payload := buildAPNsPayload(PushNotificationJob{
		Event:      PushEventChatMessage,
		SenderName: "Dad",
		Content:    strings.Repeat("x", 500),
	}, prefs)

	if len(payload.Aps.Alert.Body) > maxAlertBodyLength {
		t.Errorf("alert body length = %d, want at most %d", len(payload.Aps.Alert.Body), maxAlertBodyLength)
	}
}

func TestPayloadCarriesVersionEventAndDestination(t *testing.T) {
	payload := buildAPNsPayload(PushNotificationJob{
		Event:      PushEventChatMessage,
		RecordId:   12,
		FamilyId:   3,
		SenderId:   5,
		SenderName: "Dad",
		Content:    "hi",
	}, defaultNotificationPreferences(5))

	if payload.Data.Version != pushPayloadVersion {
		t.Errorf("data.v = %d, want %d", payload.Data.Version, pushPayloadVersion)
	}
	if payload.Data.Type != PushEventChatMessage {
		t.Errorf("data.type = %q, want %q", payload.Data.Type, PushEventChatMessage)
	}
	if payload.Data.RecordId != 12 {
		t.Errorf("data.record_id = %d, want 12", payload.Data.RecordId)
	}
	if payload.Data.Destination != "/chat" {
		t.Errorf("data.destination = %q, want the in-app path for the chat screen", payload.Data.Destination)
	}
	if payload.Data.FamilyId != 3 {
		t.Errorf("data.family_id = %d, want 3", payload.Data.FamilyId)
	}
	if payload.Data.MessageId != 12 {
		t.Errorf("data.message_id = %d, want the record id repeated for older builds", payload.Data.MessageId)
	}
}

func TestTestPushCarriesNoRecordToOpen(t *testing.T) {
	payload := buildAPNsPayload(PushNotificationJob{
		Event:   PushEventTest,
		Content: "verification ping",
	}, defaultNotificationPreferences(1))

	if payload.Data.Type != PushEventTest {
		t.Errorf("data.type = %q, want %q", payload.Data.Type, PushEventTest)
	}
	if payload.Data.MessageId != 0 {
		t.Errorf("data.message_id = %d, want 0", payload.Data.MessageId)
	}
	if payload.Aps.Alert.Body != "verification ping" {
		t.Errorf("alert body = %q, want the admin's text verbatim", payload.Aps.Alert.Body)
	}
}

func TestQueueRefusesAnUnknownEvent(t *testing.T) {
	previous := globalPushWorker
	globalPushWorker = &PushWorker{jobQueue: make(chan PushNotificationJob, 1)}
	t.Cleanup(func() { globalPushWorker = previous })

	if err := QueuePushNotification(PushNotificationJob{Event: "photo_added"}); err == nil {
		t.Fatal("QueuePushNotification() error = nil, want a refusal for an unknown event")
	}
	if len(globalPushWorker.jobQueue) != 0 {
		t.Error("the unknown job was queued anyway")
	}
}

func TestProcessPushJobSkipsRecipientsWhoTurnedChatOff(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	silent := NotificationPreferences{UserId: device.UserId, ChatEnabled: false}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, NotificationPreferencesBkt, silent.UserId, &silent)
		vbolt.TxCommit(tx)
	})

	sends := 0
	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		sends++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	worker.db = db

	worker.processPushJob(PushNotificationJob{
		Event:            PushEventChatMessage,
		RecordId:         1,
		SenderName:       "Dad",
		Content:          "hello",
		RecipientUserIds: []int{device.UserId},
	})

	if sends != 0 {
		t.Errorf("APNs requests = %d, want 0 for a recipient with chat notifications off", sends)
	}

	globalPushWorker = worker
	t.Cleanup(func() { globalPushWorker = nil })
	if stats := GetPushWorkerStats(); stats.Suppressed != 1 {
		t.Errorf("Suppressed = %d, want 1 so the admin page can tell this apart from a failure", stats.Suppressed)
	}
}

func TestProcessPushJobStillDeliversATestPush(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	silent := NotificationPreferences{UserId: device.UserId, ChatEnabled: false}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, NotificationPreferencesBkt, silent.UserId, &silent)
		vbolt.TxCommit(tx)
	})

	sends := 0
	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		sends++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	worker.db = db

	worker.processPushJob(PushNotificationJob{
		Event:            PushEventTest,
		Content:          "verification ping",
		RecipientUserIds: []int{device.UserId},
	})

	if sends != 1 {
		t.Errorf("APNs requests = %d, want 1", sends)
	}
}
