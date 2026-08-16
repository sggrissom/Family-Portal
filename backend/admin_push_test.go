package backend

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func newTestPushWorker(t *testing.T, respond roundTripFunc) *PushWorker {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate APNs signing key: %v", err)
	}

	return &PushWorker{
		apnsConfig: &APNsConfig{
			TeamId: "test-team",
			KeyId:  "test-key",
			Key:    privateKey,
		},
		httpClient: &http.Client{Transport: respond},
	}
}

func TestMaskPushToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "full length token", token: strings.Repeat("ab", 32), want: "abababab…abab"},
		{name: "short token left intact", token: "abc123", want: "abc123"},
		{name: "boundary length left intact", token: "123456789012", want: "123456789012"},
		{name: "one over boundary is masked", token: "1234567890123", want: "12345678…0123"},
		{name: "empty token", token: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := maskPushToken(test.token); got != test.want {
				t.Errorf("maskPushToken(%q) = %q, want %q", test.token, got, test.want)
			}
		})
	}
}

// A masked token must never be enough to send a notification with, which is the
// whole reason the admin page shows a hint rather than the token.
func TestMaskPushTokenDropsMostOfTheToken(t *testing.T) {
	token := strings.Repeat("ab", 32)
	masked := maskPushToken(token)

	if strings.Contains(token, masked) {
		t.Errorf("masked token %q is a literal substring of the real token", masked)
	}
	if len(masked) >= len(token) {
		t.Errorf("masked token length %d, want shorter than %d", len(masked), len(token))
	}
}

func TestSendAPNsNotificationRecordsSuccess(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("apns-id", "8A4B2C1D-TEST")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     header,
		}, nil
	})
	worker.db = db

	if err := worker.sendAPNsNotification(device, PushNotificationJob{Content: "hello"}); err != nil {
		t.Fatalf("sendAPNsNotification() error = %v, want nil", err)
	}

	globalPushWorker = worker
	t.Cleanup(func() { globalPushWorker = nil })

	stats := GetPushWorkerStats()
	if stats.Sent != 1 {
		t.Errorf("Sent = %d, want 1", stats.Sent)
	}
	if stats.Failed != 0 {
		t.Errorf("Failed = %d, want 0", stats.Failed)
	}
	if stats.LastSentAt.IsZero() {
		t.Error("LastSentAt is zero, want the time of the delivery")
	}
	if len(stats.RecentAttempts) != 1 {
		t.Fatalf("RecentAttempts length = %d, want 1", len(stats.RecentAttempts))
	}

	attempt := stats.RecentAttempts[0]
	if !attempt.Success {
		t.Error("attempt.Success = false, want true")
	}
	if attempt.ApnsId != "8A4B2C1D-TEST" {
		t.Errorf("attempt.ApnsId = %q, want the apns-id response header", attempt.ApnsId)
	}
	if attempt.Kind != "chat" {
		t.Errorf("attempt.Kind = %q, want \"chat\"", attempt.Kind)
	}
	if attempt.TokenHint == device.Token {
		t.Error("attempt.TokenHint is the raw device token, want it masked")
	}
}

func TestSendAPNsNotificationRecordsFailureReason(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusGone,
			Body:       io.NopCloser(strings.NewReader(`{"reason":"Unregistered"}`)),
			Header:     make(http.Header),
		}, nil
	})
	worker.db = db

	if err := worker.sendAPNsNotification(device, PushNotificationJob{Content: "hello"}); err == nil {
		t.Fatal("sendAPNsNotification() error = nil, want an APNs error")
	}

	globalPushWorker = worker
	t.Cleanup(func() { globalPushWorker = nil })

	stats := GetPushWorkerStats()
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
	if stats.Sent != 0 {
		t.Errorf("Sent = %d, want 0", stats.Sent)
	}
	if stats.LastError != "Unregistered" {
		t.Errorf("LastError = %q, want \"Unregistered\"", stats.LastError)
	}
	// Unregistered is one of the reasons that retires a token, so the counter
	// the admin page reads must move with it.
	if stats.Deactivated != 1 {
		t.Errorf("Deactivated = %d, want 1", stats.Deactivated)
	}

	if len(stats.RecentAttempts) != 1 {
		t.Fatalf("RecentAttempts length = %d, want 1", len(stats.RecentAttempts))
	}
	attempt := stats.RecentAttempts[0]
	if attempt.Success {
		t.Error("attempt.Success = true, want false")
	}
	if attempt.StatusCode != http.StatusGone {
		t.Errorf("attempt.StatusCode = %d, want %d", attempt.StatusCode, http.StatusGone)
	}
	if attempt.Reason != "Unregistered" {
		t.Errorf("attempt.Reason = %q, want \"Unregistered\"", attempt.Reason)
	}
}

// A transport failure never reaches APNs, so there is no status or reason to
// report; the attempt still has to be visible or the page shows nothing at all
// for the most confusing kind of failure.
func TestSendAPNsNotificationRecordsTransportFailure(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})
	worker.db = db

	if err := worker.sendAPNsNotification(device, PushNotificationJob{Content: "hello"}); err == nil {
		t.Fatal("sendAPNsNotification() error = nil, want a transport error")
	}

	globalPushWorker = worker
	t.Cleanup(func() { globalPushWorker = nil })

	stats := GetPushWorkerStats()
	if len(stats.RecentAttempts) != 1 {
		t.Fatalf("RecentAttempts length = %d, want 1", len(stats.RecentAttempts))
	}
	attempt := stats.RecentAttempts[0]
	if attempt.StatusCode != 0 {
		t.Errorf("attempt.StatusCode = %d, want 0 for a request that never completed", attempt.StatusCode)
	}
	if !strings.Contains(attempt.Reason, "transport error") {
		t.Errorf("attempt.Reason = %q, want it to name the transport failure", attempt.Reason)
	}
}

// The companion app routes on data.type. A test push carrying "chat_message"
// would send it looking for a message that was never written.
func TestTestPushUsesDistinctPayloadType(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	var captured APNsPayload
	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode APNs payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	worker.db = db

	job := PushNotificationJob{
		MessageId:  99,
		SenderName: "Admin",
		Content:    "verification ping",
		IsTest:     true,
	}
	if err := worker.sendAPNsNotification(device, job); err != nil {
		t.Fatalf("sendAPNsNotification() error = %v, want nil", err)
	}

	if captured.Data.Type != "test" {
		t.Errorf("payload data.type = %q, want \"test\"", captured.Data.Type)
	}
	if captured.Data.MessageId != 0 {
		t.Errorf("payload data.message_id = %d, want 0 so the app cannot open a missing message", captured.Data.MessageId)
	}
	if captured.Aps.Category != "test" {
		t.Errorf("payload aps.category = %q, want \"test\"", captured.Aps.Category)
	}
	if captured.Aps.Alert.Body != "verification ping" {
		t.Errorf("payload alert body = %q, want the test message verbatim", captured.Aps.Alert.Body)
	}
	// A test must not be prefixed with a sender name the way a chat push is.
	if strings.Contains(captured.Aps.Alert.Body, "Admin") {
		t.Errorf("payload alert body = %q, want no sender prefix", captured.Aps.Alert.Body)
	}
}

// A chat push must keep the payload the companion app already expects.
func TestChatPushKeepsChatPayloadType(t *testing.T) {
	db := openPushNotificationTestDB(t)
	device := createActivePushDevice(t, db)

	var captured APNsPayload
	worker := newTestPushWorker(t, func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode APNs payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	worker.db = db

	job := PushNotificationJob{
		MessageId:  7,
		SenderId:   3,
		SenderName: "Dad",
		Content:    "dinner is ready",
	}
	if err := worker.sendAPNsNotification(device, job); err != nil {
		t.Fatalf("sendAPNsNotification() error = %v, want nil", err)
	}

	if captured.Data.Type != "chat_message" {
		t.Errorf("payload data.type = %q, want \"chat_message\"", captured.Data.Type)
	}
	if captured.Data.MessageId != 7 {
		t.Errorf("payload data.message_id = %d, want 7", captured.Data.MessageId)
	}
	if captured.Aps.Alert.Body != "Dad: dinner is ready" {
		t.Errorf("payload alert body = %q, want the sender-prefixed form", captured.Aps.Alert.Body)
	}
}

// The history is a debugging aid, not a log: it must stay bounded and keep the
// newest attempts rather than the first ones seen.
func TestRecentAttemptsAreBoundedAndNewestFirst(t *testing.T) {
	worker := &PushWorker{}
	globalPushWorker = worker
	t.Cleanup(func() { globalPushWorker = nil })

	total := maxRecentPushAttempts + 10
	for i := 0; i < total; i++ {
		worker.recordAttempt(PushAttempt{TokenId: i, Success: true})
	}

	stats := GetPushWorkerStats()
	if len(stats.RecentAttempts) != maxRecentPushAttempts {
		t.Fatalf("RecentAttempts length = %d, want %d", len(stats.RecentAttempts), maxRecentPushAttempts)
	}
	if stats.Sent != total {
		t.Errorf("Sent = %d, want %d; the counter must not be capped with the history", stats.Sent, total)
	}
	if got := stats.RecentAttempts[0].TokenId; got != total-1 {
		t.Errorf("first attempt TokenId = %d, want %d (newest first)", got, total-1)
	}
	if got := stats.RecentAttempts[len(stats.RecentAttempts)-1].TokenId; got != total-maxRecentPushAttempts {
		t.Errorf("last attempt TokenId = %d, want %d", got, total-maxRecentPushAttempts)
	}
}

// The admin page loads whether or not push is configured, so an uninitialized
// worker has to produce a usable zero snapshot rather than panicking.
func TestGetPushWorkerStatsWithoutWorker(t *testing.T) {
	globalPushWorker = nil

	stats := GetPushWorkerStats()
	if stats.Enabled {
		t.Error("Enabled = true, want false when the worker was never initialized")
	}
	if stats.RecentAttempts == nil {
		t.Error("RecentAttempts = nil, want an empty slice so the client renders an empty table")
	}
	if len(stats.RecentAttempts) != 0 {
		t.Errorf("RecentAttempts length = %d, want 0", len(stats.RecentAttempts))
	}
}

// Every proc on the push admin page exposes device tokens or can send a
// notification, so all three must reject a non-admin caller.
func TestPushAdminProcsRequireAdmin(t *testing.T) {
	db := openPushNotificationTestDB(t)
	appDb = db

	var adminUser User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminUser = AddUserTx(tx, CreateAccountRequest{
			Name:            "Admin User",
			Email:           "push-admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}, []byte("hash"))
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)
		vbolt.TxCommit(tx)
	})

	regularUser := User{Id: 2, Email: "regular@example.com"}

	procs := map[string]func(*vbeam.Context) error{
		"GetPushStatus": func(ctx *vbeam.Context) error {
			_, err := GetPushStatus(ctx, Empty{})
			return err
		},
		"ListPushDevices": func(ctx *vbeam.Context) error {
			_, err := ListPushDevices(ctx, Empty{})
			return err
		},
		"SendTestPushNotification": func(ctx *vbeam.Context) error {
			_, err := SendTestPushNotification(ctx, SendTestPushRequest{})
			return err
		},
	}

	for name, call := range procs {
		t.Run(name+" denies non-admin", func(t *testing.T) {
			ctx := &vbeam.Context{}
			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				ctx.Tx = tx
				token, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
				ctx.Token = token

				err := call(ctx)
				if err == nil {
					t.Fatal("error = nil, want an authorization error")
				}
				if err.Error() != "Unauthorized: Admin access required" {
					t.Errorf("error = %q, want the admin authorization error", err.Error())
				}
			})
		})
	}

	t.Run("GetPushStatus allows admin", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			token, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = token

			resp, err := GetPushStatus(ctx, Empty{})
			if err != nil {
				t.Fatalf("GetPushStatus() error = %v, want nil", err)
			}
			// Issues must be a slice rather than nil so the client can call
			// .length on it without a guard.
			if resp.Issues == nil {
				t.Error("Issues = nil, want an empty slice")
			}
		})
	})
}

// An admin should get a clear reason rather than a silent no-op when push is
// not configured at all, which is the normal state in local development.
func TestSendTestPushNotificationRequiresRunningWorker(t *testing.T) {
	db := openPushNotificationTestDB(t)
	appDb = db
	globalPushWorker = nil

	var adminUser User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminUser = AddUserTx(tx, CreateAccountRequest{
			Name:            "Admin User",
			Email:           "push-admin-2@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}, []byte("hash"))
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)
		vbolt.TxCommit(tx)
	})

	ctx := &vbeam.Context{}
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		ctx.Tx = tx
		token, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
		ctx.Token = token

		_, err := SendTestPushNotification(ctx, SendTestPushRequest{})
		if err == nil {
			t.Fatal("SendTestPushNotification() error = nil, want an error when push is not configured")
		}
		if !strings.Contains(err.Error(), "push worker is not running") {
			t.Errorf("error = %q, want it to name the stopped worker", err.Error())
		}
	})
}
