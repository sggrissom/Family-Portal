package family

import (
	"net/http"
	"testing"
	"time"
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
