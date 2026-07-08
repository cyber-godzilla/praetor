package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// newTestServer creates an httptest.Server that upgrades to WebSocket and
// calls handler with the connection. It returns the server and a ws:// URL.
func newTestServer(t *testing.T, handler func(*websocket.Conn)) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		handler(conn)
	}))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

func TestSession_ConnectAndReceiveLines(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()

		// Read the client identification message
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read ident error: %v", err)
			return
		}
		if string(msg) != "SKOTOS Praetor 0.1.0\r\n" {
			t.Errorf("expected ident message, got %q", string(msg))
		}

		// Send some lines to the client
		conn.WriteMessage(websocket.TextMessage, []byte("SECRET abc123\r\nYou see a sword.\r\n"))
	})
	defer srv.Close()

	s := New()
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !s.IsConnected() {
		t.Error("expected IsConnected to be true")
	}

	// Read the two lines
	select {
	case line := <-s.Lines():
		if line != "SECRET abc123" {
			t.Errorf("expected 'SECRET abc123', got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first line")
	}

	select {
	case line := <-s.Lines():
		if line != "You see a sword." {
			t.Errorf("expected 'You see a sword.', got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second line")
	}
}

func TestSession_SendCommand(t *testing.T) {
	received := make(chan string, 10)
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			received <- string(msg)
		}
	})
	defer srv.Close()

	s := New()
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Read and discard the ident message
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ident")
	}

	// Send a command
	err = s.Send("look")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	select {
	case msg := <-received:
		if msg != "look\r\n" {
			t.Errorf("expected 'look\\r\\n', got %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for sent command")
	}
}

func TestSession_SendAfterClose(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		// Just drain messages
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer srv.Close()

	s := New()
	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	s.Close()

	err = s.Send("look")
	if err != ErrSessionClosed {
		t.Errorf("expected ErrSessionClosed, got %v", err)
	}

	if s.IsConnected() {
		t.Error("expected IsConnected to be false after Close")
	}
}

func TestSession_DoneSignaledOnClose(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer srv.Close()

	s := New()
	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	s.Close()

	select {
	case <-s.Done():
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Done signal")
	}
}

func TestSession_DoneSignaledOnServerClose(t *testing.T) {
	connClosed := make(chan struct{})
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		// Read ident then close immediately
		conn.ReadMessage()
		conn.Close()
		close(connClosed)
	})
	defer srv.Close()

	s := New()
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Wait for server to close connection
	<-connClosed

	select {
	case <-s.Done():
		// expected - readLoop exits on error and closes done
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Done signal after server disconnect")
	}
}

func TestSession_CookieSupport(t *testing.T) {
	receivedCookies := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookies <- r.Header.Get("Cookie")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	s := New()
	defer s.Close()

	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "token", Value: "xyz"},
	}

	err := s.Connect(wsURL, cookies)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	select {
	case cookie := <-receivedCookies:
		if !strings.Contains(cookie, "session=abc123") {
			t.Errorf("expected cookie to contain 'session=abc123', got %q", cookie)
		}
		if !strings.Contains(cookie, "token=xyz") {
			t.Errorf("expected cookie to contain 'token=xyz', got %q", cookie)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cookies")
	}
}

func TestSession_PartialLineBuffering(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()

		// Read ident
		conn.ReadMessage()

		// Send a partial line, then complete it in a second message
		conn.WriteMessage(websocket.TextMessage, []byte("partial"))
		time.Sleep(50 * time.Millisecond)
		conn.WriteMessage(websocket.TextMessage, []byte(" line\r\n"))
	})
	defer srv.Close()

	s := New()
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	select {
	case line := <-s.Lines():
		if line != "partial line" {
			t.Errorf("expected 'partial line', got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for buffered line")
	}
}

func TestSession_SilentPeerTriggersDisconnect(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		// Read the ident, then go silent: no reads (no auto-pong), no writes.
		conn.ReadMessage()
		time.Sleep(2 * time.Second)
	})
	defer srv.Close()

	s := New()
	s.pongWait = 150 * time.Millisecond
	s.pingPeriod = 40 * time.Millisecond
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	select {
	case <-s.Done():
		// expected: silent peer detected via read deadline expiry
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Done signal after silent peer")
	}
}

func TestSession_DataResetsDeadline(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		conn.ReadMessage()
		for i := 0; i < 10; i++ {
			if err := conn.WriteMessage(websocket.TextMessage, []byte("tick\r\n")); err != nil {
				return
			}
			time.Sleep(40 * time.Millisecond)
		}
	})

	s := New()
	s.pongWait = 150 * time.Millisecond
	s.pingPeriod = 40 * time.Millisecond
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	received := 0
	deadline := time.After(1 * time.Second)
loop:
	for received < 10 {
		select {
		case _, ok := <-s.Lines():
			if !ok {
				t.Fatalf("lines channel closed prematurely after %d lines", received)
			}
			received++
		case <-s.Done():
			t.Fatalf("session closed prematurely after %d lines", received)
		case <-deadline:
			break loop
		}
	}

	if received == 0 {
		t.Fatal("expected to receive at least one line")
	}

	srv.Close()

	select {
	case <-s.Done():
		// expected once server goes away
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Done signal after server close")
	}
}

func TestSession_PongResetsDeadline(t *testing.T) {
	connCh := make(chan *websocket.Conn, 1)
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		connCh <- conn
		conn.ReadMessage()
		// Keep reading so gorilla's default ping handler auto-responds with
		// pongs; send no data of our own.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	s := New()
	s.pongWait = 150 * time.Millisecond
	s.pingPeriod = 40 * time.Millisecond
	defer s.Close()

	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	select {
	case <-s.Done():
		t.Fatal("session closed prematurely; pong should have reset deadline")
	case <-time.After(500 * time.Millisecond):
		// expected: still alive thanks to pong-driven deadline resets
	}

	// Directly close the server-side conn: httptest.Server.Close() does not
	// terminate hijacked (upgraded) connections, so we close it explicitly to
	// simulate the server going away.
	var serverConn *websocket.Conn
	select {
	case serverConn = <-connCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout capturing server-side conn")
	}
	serverConn.Close()

	select {
	case <-s.Done():
		// expected once server goes away
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Done signal after server close")
	}
}

func TestSession_CloseIdempotent(t *testing.T) {
	srv, wsURL := newTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer srv.Close()

	s := New()
	err := s.Connect(wsURL, nil)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Should not panic
	s.Close()
	s.Close()
	s.Close()
}
