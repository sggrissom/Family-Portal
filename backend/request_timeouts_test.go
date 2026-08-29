package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestDeadlinesByRoute(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		read  time.Duration
		write time.Duration
	}{
		{"RPC call", "/rpc/GetDashboard", defaultReadTimeout, defaultWriteTimeout},
		{"login", "/api/login", defaultReadTimeout, defaultWriteTimeout},
		{"photo upload", "/api/upload-photo", uploadReadTimeout, defaultWriteTimeout},
		{"family import", "/api/import-bundle", importReadTimeout, defaultWriteTimeout},
		{"family export", "/api/export-bundle", defaultReadTimeout, downloadWriteTimeout},
		{"database snapshot", SnapshotPath, defaultReadTimeout, downloadWriteTimeout},
		{"photo download", "/api/photo/1234/medium", defaultReadTimeout, downloadWriteTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			read, write := requestDeadlines(httptest.NewRequest(http.MethodGet, tt.path, nil))
			if read != tt.read {
				t.Errorf("read deadline = %s, want %s", read, tt.read)
			}
			if write != tt.write {
				t.Errorf("write deadline = %s, want %s", write, tt.write)
			}
		})
	}
}

func TestWebSocketUpgradeGetsNoDeadline(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	read, write := requestDeadlines(req)
	if read != 0 || write != 0 {
		t.Errorf("deadlines = (%s, %s), want (0, 0) for a WebSocket upgrade", read, write)
	}
}

func TestUploadBudgetsCoverTheirSizeLimits(t *testing.T) {
	const (
		mobileUplinkBytesPerSecond    = 200 << 10
		broadbandUplinkBytesPerSecond = 1 << 20
	)

	cases := []struct {
		name   string
		limit  int64
		budget time.Duration
		uplink int64
	}{
		{"photo upload", maxPhotoRequestBytes, uploadReadTimeout, mobileUplinkBytesPerSecond},
		{"family import", maxImportRequestBytes, importReadTimeout, broadbandUplinkBytesPerSecond},
	}

	for _, tc := range cases {
		needed := time.Duration(tc.limit/tc.uplink) * time.Second
		if tc.budget < needed {
			t.Errorf("%s: budget %s cannot carry %d bytes at %d B/s (needs %s)",
				tc.name, tc.budget, tc.limit, tc.uplink, needed)
		}
	}
}

func TestWriteBudgetOutlastsTheSlowestHandler(t *testing.T) {
	const slowestUpstreamCall = 60 * time.Second

	if defaultWriteTimeout <= slowestUpstreamCall {
		t.Errorf("defaultWriteTimeout = %s, must exceed the %s AI call it has to outlive",
			defaultWriteTimeout, slowestUpstreamCall)
	}
}

func TestRequestTimeoutWrapperServesWithoutAConnection(t *testing.T) {
	served := false
	wrapper := NewRequestTimeoutWrapper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	wrapper.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rpc/Anything", nil))

	if !served {
		t.Fatal("the wrapper swallowed the request")
	}
	if recorder.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
