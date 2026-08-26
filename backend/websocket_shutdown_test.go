package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newShutdownTestHub(t *testing.T) *ChatHub {
	t.Helper()

	previous := globalChatHub
	hub := InitializeChatHub()
	t.Cleanup(func() { globalChatHub = previous })
	return hub
}

func serveTestClient(t *testing.T, hub *ChatHub, familyId int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hub.closing.Load() {
			http.Error(w, "server shutting down", http.StatusServiceUnavailable)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		client := &Client{
			hub:      hub,
			conn:     conn,
			send:     make(chan []byte, 4),
			userId:   1,
			familyId: familyId,
			userName: "Test",
			lastSeen: time.Now(),
			ctx:      ctx,
			cancel:   cancel,
		}
		hub.register <- client
		go client.writePump()
		go client.readPump()
	}))
	t.Cleanup(server.Close)
	return server
}

func TestShutdownClosesChatConnectionsWithGoingAway(t *testing.T) {
	hub := newShutdownTestHub(t)
	server := serveTestClient(t, hub, 42)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	waitForHubClients(t, hub, 1)

	readErrs := make(chan error, 1)
	go func() {
		readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelRead()
		for {
			if _, _, err := conn.Read(readCtx); err != nil {
				readErrs <- err
				return
			}
		}
	}()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelShutdown()
	if !hub.Shutdown(shutdownCtx) {
		t.Error("Shutdown reported clients still connected")
	}

	var readErr error
	select {
	case readErr = <-readErrs:
	case <-time.After(3 * time.Second):
		t.Fatal("the client never saw the connection close")
	}
	if status := websocket.CloseStatus(readErr); status != websocket.StatusGoingAway {
		t.Errorf("close status = %v (err %v), want %v", status, readErr, websocket.StatusGoingAway)
	}
}

func TestShutdownRefusesNewConnections(t *testing.T) {
	hub := newShutdownTestHub(t)
	server := serveTestClient(t, hub, 42)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !hub.Shutdown(ctx) {
		t.Fatal("Shutdown did not complete on an empty hub")
	}

	_, resp, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):], nil)
	if err == nil {
		t.Fatal("the hub accepted a connection after shutdown began")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %v, want %d", resp, http.StatusServiceUnavailable)
	}
}

func TestShutdownOnAnIdleHubIsImmediate(t *testing.T) {
	hub := newShutdownTestHub(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if !hub.Shutdown(ctx) {
		t.Fatal("Shutdown reported failure on an idle hub")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("shutdown of an idle hub took %s", elapsed)
	}
}

func TestShutdownChatHubWithoutAHub(t *testing.T) {
	previous := globalChatHub
	globalChatHub = nil
	t.Cleanup(func() { globalChatHub = previous })

	if !ShutdownChatHub(context.Background()) {
		t.Error("ShutdownChatHub should succeed when there is no hub")
	}
}

func waitForHubClients(t *testing.T, hub *ChatHub, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.connectionCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("hub held %d clients, want %d", hub.connectionCount(), want)
}
