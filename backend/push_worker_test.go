package backend

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.hasen.dev/vbolt"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSendAPNsNotificationDeactivatesRejectedTokens(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		reason     string
		wantActive bool
	}{
		{name: "bad device token", statusCode: http.StatusBadRequest, reason: "BadDeviceToken"},
		{name: "unregistered token", statusCode: http.StatusGone, reason: "Unregistered"},
		{name: "temporary APNs failure", statusCode: http.StatusServiceUnavailable, reason: "ServiceUnavailable", wantActive: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openPushNotificationTestDB(t)
			device := createActivePushDevice(t, db)
			privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatalf("generate APNs signing key: %v", err)
			}

			worker := &PushWorker{
				db: db,
				apnsConfig: &APNsConfig{
					TeamId: "test-team",
					KeyId:  "test-key",
					Key:    privateKey,
				},
				httpClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if !strings.HasSuffix(request.URL.Path, "/"+device.Token) {
						t.Errorf("request path = %q, want device token suffix", request.URL.Path)
					}
					return &http.Response{
						StatusCode: test.statusCode,
						Body:       io.NopCloser(strings.NewReader(`{"reason":"` + test.reason + `"}`)),
						Header:     make(http.Header),
					}, nil
				})},
			}

			err = worker.sendAPNsNotification(device, PushNotificationJob{Content: "hello"})
			if err == nil {
				t.Fatal("sendAPNsNotification() error = nil, want APNs error")
			}

			vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
				stored := GetPushDeviceTokenById(tx, device.Id)
				if stored.IsActive != test.wantActive {
					t.Errorf("token active = %t, want %t", stored.IsActive, test.wantActive)
				}
			})
		})
	}
}

func createActivePushDevice(t *testing.T, db *vbolt.DB) PushDeviceToken {
	t.Helper()
	var device PushDeviceToken
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		var err error
		device, err = upsertPushDeviceToken(tx, 42, RegisterPushDeviceRequest{
			Token:       strings.Repeat("ab", 32),
			Platform:    "ios",
			Environment: "sandbox",
			BundleId:    "dev.family.portal",
		})
		if err != nil {
			t.Fatalf("create push token: %v", err)
		}
		vbolt.TxCommit(tx)
	})
	return device
}
