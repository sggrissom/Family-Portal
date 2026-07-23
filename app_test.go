package family

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestNewHTTPServerAppliesTransportLimits(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := NewHTTPServer(":8666", handler)

	if server.Addr != ":8666" {
		t.Fatalf("Addr = %q, want %q", server.Addr, ":8666")
	}
	if server.Handler == nil {
		t.Fatal("Handler should be configured")
	}
	if server.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, 10*time.Second)
	}
	if server.IdleTimeout != 2*time.Minute {
		t.Errorf("IdleTimeout = %s, want %s", server.IdleTimeout, 2*time.Minute)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Errorf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, 1<<20)
	}
	if server.ReadTimeout != 0 || server.WriteTimeout != 0 {
		t.Error("global read/write timeouts should not constrain uploads or WebSockets")
	}
}

func TestReadinessHandler(t *testing.T) {
	dir := t.TempDir()
	db := vbolt.Open(filepath.Join(dir, "ready.db"))
	t.Cleanup(func() { _ = db.Close() })

	t.Run("ready", func(t *testing.T) {
		response := httptest.NewRecorder()
		readinessHandler(db, dir).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		if response.Body.String() != "ok" {
			t.Errorf("body = %q, want %q", response.Body.String(), "ok")
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
		}
	})

	t.Run("missing database", func(t *testing.T) {
		response := httptest.NewRecorder()
		readinessHandler(nil, dir).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("unwritable storage path", func(t *testing.T) {
		filePath := filepath.Join(dir, "not-a-directory")
		if err := os.WriteFile(filePath, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		readinessHandler(db, filePath).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
	})
}

func TestRunHTTPServerDrainsActiveRequests(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusNoContent)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewHTTPServer(listener.Addr().String(), handler)
	listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- RunHTTPServer(ctx, server) }()

	responseDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + server.Addr)
		if err == nil {
			resp.Body.Close()
		}
		responseDone <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach server")
	}
	cancel()
	close(releaseRequest)

	if err := <-responseDone; err != nil {
		t.Fatalf("active request failed during shutdown: %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("RunHTTPServer() error = %v", err)
	}

	client := &http.Client{Timeout: 250 * time.Millisecond}
	resp, err := client.Get("http://" + server.Addr)
	if err == nil {
		resp.Body.Close()
		t.Fatal("server accepted a new request after shutdown completed")
	}
}
