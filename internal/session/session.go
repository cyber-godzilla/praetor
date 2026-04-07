package session

import (
	"net/http"
	"sync"

	"github.com/cyber-godzilla/praetor/internal/protocol"
	"github.com/gorilla/websocket"
)

// Session manages a WebSocket connection to the game server, providing
// line-oriented communication through a protocol.LineBuffer.
type Session struct {
	mu     sync.Mutex
	conn   *websocket.Conn
	lines  chan string
	done   chan struct{}
	closed bool
	buf    *protocol.LineBuffer
}

// New creates a new Session with initialized channels and line buffer.
func New() *Session {
	return &Session{
		lines: make(chan string, 256),
		done:  make(chan struct{}),
		buf:   protocol.NewLineBuffer(),
	}
}

// Connect establishes a WebSocket connection to the given URL with optional
// cookies for authentication. After connecting, it sends the TecClient
// identification string and starts the read loop.
func (s *Session) Connect(url string, cookies []*http.Cookie) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSessionClosed
	}

	header := http.Header{}
	if len(cookies) > 0 {
		req := &http.Request{Header: http.Header{}}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		header.Set("Cookie", req.Header.Get("Cookie"))
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		return err
	}
	s.conn = conn

	// Send client identification
	err = conn.WriteMessage(websocket.TextMessage, []byte("SKOTOS Praetor 0.1.0\r\n"))
	if err != nil {
		conn.Close()
		s.conn = nil
		return err
	}

	go s.readLoop()
	return nil
}

// Lines returns a read-only channel that emits complete lines received from
// the game server.
func (s *Session) Lines() <-chan string {
	return s.lines
}

// Send writes a command to the WebSocket connection, appending \r\n.
func (s *Session) Send(command string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.conn == nil {
		return ErrSessionClosed
	}

	return s.conn.WriteMessage(websocket.TextMessage, []byte(command+"\r\n"))
}

// Close shuts down the session, closing the WebSocket connection and
// signaling the done channel. It is safe to call multiple times.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	if s.conn != nil {
		s.conn.Close()
	}
	close(s.done)
}

// Done returns a channel that is closed when the session terminates.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// IsConnected returns true if the session has an active connection.
func (s *Session) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil && !s.closed
}

// readLoop reads WebSocket messages, feeds them through the LineBuffer,
// and sends complete lines to the lines channel. It runs until the
// connection is closed or an error occurs.
func (s *Session) readLoop() {
	defer func() {
		s.mu.Lock()
		wasClosed := s.closed
		s.closed = true
		s.mu.Unlock()
		if !wasClosed {
			close(s.done)
		}
		close(s.lines)
	}()

	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		lines := s.buf.Write(msg)
		for _, line := range lines {
			select {
			case s.lines <- line:
			case <-s.done:
				return
			}
		}
	}
}
