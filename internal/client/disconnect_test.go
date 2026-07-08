package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/cyber-godzilla/praetor/internal/types"
	"github.com/gorilla/websocket"
)

var discUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// newDiscTestClient builds a minimal Client wired to a fresh engine. Creds are
// nil (Disconnect/Run never touch them).
func newDiscTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(config.Defaults(), nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// newDiscServer upgrades, drains client messages, and holds the connection open
// until the given signal fires (then closes the socket) or the client leaves.
func newDiscServer(t *testing.T, closeSig <-chan struct{}) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := discUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		<-closeSig
		conn.Close()
	}))
	return srv, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func connectTestSession(t *testing.T, c *Client, wsURL string) {
	t.Helper()
	c.Session = session.New()
	if err := c.Session.Connect(wsURL, nil); err != nil {
		t.Fatalf("connect: %v", err)
	}
}

func waitForConnected(t *testing.T, c *Client) {
	t.Helper()
	for {
		select {
		case ev := <-c.Events():
			if _, ok := ev.(types.ConnectedEvent); ok {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ConnectedEvent")
		}
	}
}

func waitForDisconnected(t *testing.T, c *Client) types.DisconnectedEvent {
	t.Helper()
	for {
		select {
		case ev := <-c.Events():
			if d, ok := ev.(types.DisconnectedEvent); ok {
				return d
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for DisconnectedEvent")
		}
	}
}

// newRecordingServer upgrades exactly one client connection and records every
// non-ident text frame it receives onto a buffered channel. The initial
// "SKOTOS ..." identification frame sent by Session.Connect is filtered out
// so the channel only carries game commands.
func newRecordingServer(t *testing.T) (*httptest.Server, string, <-chan string) {
	t.Helper()
	received := make(chan string, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := discUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			text := strings.TrimRight(string(msg), "\r\n")
			if strings.HasPrefix(text, "SKOTOS ") {
				continue // ident frame, not a game command
			}
			received <- text
		}
	}))
	return srv, "ws" + strings.TrimPrefix(srv.URL, "http"), received
}

// TestClient_DrainQueue_DoesNotSendStaleCommandToReconnectedSession guards
// against a data race / cross-session leak: a command dequeued while c.Session
// still pointed at the original (server A) session must be sent to server A,
// even if c.Session is reassigned (reconnect to server B) while the drain
// goroutine is mid-sleep on the command's delay.
func TestClient_DrainQueue_DoesNotSendStaleCommandToReconnectedSession(t *testing.T) {
	srvA, urlA, recvA := newRecordingServer(t)
	defer srvA.Close()
	srvB, urlB, recvB := newRecordingServer(t)
	defer srvB.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, urlA)

	// Enqueue a command with enough delay to span the reassignment below.
	c.Engine.Queue().Enqueue("stale-cmd", 300)
	c.drainQueue()

	// Mimic a reconnect racing with the still-sleeping drain goroutine: swap
	// c.Session to a brand-new session connected to server B.
	c.Session = session.New()
	if err := c.Session.Connect(urlB, nil); err != nil {
		t.Fatalf("connect B: %v", err)
	}

	select {
	case cmd := <-recvA:
		if cmd != "stale-cmd" {
			t.Fatalf("server A received %q, want %q", cmd, "stale-cmd")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server A to receive the stale command")
	}

	select {
	case cmd := <-recvB:
		t.Fatalf("server B unexpectedly received %q; stale command leaked into the new session", cmd)
	case <-time.After(500 * time.Millisecond):
		// expected: nothing arrives at B
	}
}

func TestClient_Disconnect_UserInitiated_EmptyReason(t *testing.T) {
	// Server holds the connection open; the client closes it via Disconnect.
	never := make(chan struct{})
	srv, wsURL := newDiscServer(t, never)
	defer srv.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	go c.Run()
	waitForConnected(t, c)

	c.Disconnect()

	ev := waitForDisconnected(t, c)
	if ev.Reason != "" {
		t.Errorf("Reason = %q, want empty for a user-initiated logout", ev.Reason)
	}
}

func TestClient_Disconnect_Dropped_HasReason(t *testing.T) {
	drop := make(chan struct{})
	srv, wsURL := newDiscServer(t, drop)
	defer srv.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	go c.Run()
	waitForConnected(t, c)

	close(drop) // server drops the socket

	ev := waitForDisconnected(t, c)
	if ev.Reason != "connection closed" {
		t.Errorf("Reason = %q, want %q for a dropped connection", ev.Reason, "connection closed")
	}
}
