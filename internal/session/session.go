package session

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/protocol"
	"github.com/gorilla/websocket"
)

const (
	// Detection window tuned against the live TEC server, which answers every
	// WebSocket ping with a pong (~77ms RTT), so a live connection always yields
	// an incoming frame within one ping period. A 30s read deadline (3x the ping
	// period) detects a dropped link in ~30s with no false-positive risk.
	defaultPongWait   = 30 * time.Second // read deadline window; connection considered dead if no frame arrives within this
	defaultPingPeriod = 10 * time.Second // how often we send a ping; must be < pongWait
	writeWait         = 10 * time.Second // deadline for a single control-frame write
)

// Session manages a WebSocket connection to the game server, providing
// line-oriented communication through a protocol.LineBuffer.
type Session struct {
	mu         sync.Mutex
	conn       *websocket.Conn
	lines      chan string
	done       chan struct{}
	closed     bool
	buf        *protocol.LineBuffer
	pongWait   time.Duration
	pingPeriod time.Duration
	writeWait  time.Duration
}

// New creates a new Session with initialized channels and line buffer.
func New() *Session {
	return &Session{
		lines:      make(chan string, 256),
		done:       make(chan struct{}),
		buf:        protocol.NewLineBuffer(),
		pongWait:   defaultPongWait,
		pingPeriod: defaultPingPeriod,
		writeWait:  writeWait,
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
	// One-shot: a second Connect would leak the first conn and spawn a second
	// readLoop whose cleanup double-closes s.lines (panic). Callers reconnect by
	// allocating a fresh Session, so enforce that contract explicitly rather than
	// leave a latent trap for a future refactor.
	if s.conn != nil {
		return ErrAlreadyConnected
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

	// Enable OS-level TCP keepalive as a backstop for hard drops (best-effort;
	// a wss connection's UnderlyingConn is not a *net.TCPConn, so skip quietly).
	if tcp, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(s.pingPeriod)
	}

	// Read deadline + pong handler: any incoming frame (data or pong) resets the
	// deadline; if none arrives within pongWait, ReadMessage errors and we detect
	// the dead connection.
	_ = conn.SetReadDeadline(time.Now().Add(s.pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(s.pongWait))
	})

	go s.readLoop()
	go s.pingLoop()
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

	// Bound the write so a stalled link (full OS send buffer) can't wedge the
	// mutex — and with it Close/IsConnected/pingLoop/readLoop cleanup — for the
	// minutes it takes TCP retransmission to give up. On error the caller treats
	// the session as disconnected.
	_ = s.conn.SetWriteDeadline(time.Now().Add(s.writeWait))
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
		// The read loop is the authority on "this connection is dead": close the
		// conn here so an involuntary drop doesn't leak the fd. A later Close()
		// early-returns on s.closed and never reaches its own conn.Close(), so
		// without this the socket would leak until process exit. Idempotent:
		// gorilla tolerates a repeat Close.
		if s.conn != nil {
			s.conn.Close()
		}
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
		_ = s.conn.SetReadDeadline(time.Now().Add(s.pongWait))
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

// pingLoop sends periodic WebSocket ping frames so a dead-but-open connection is
// detected via the read deadline (and to keep the path warm). It exits when the
// session is closed.
func (s *Session) pingLoop() {
	ticker := time.NewTicker(s.pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.closed || s.conn == nil {
				s.mu.Unlock()
				return
			}
			err := s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(s.writeWait))
			s.mu.Unlock()
			if err != nil {
				return // write failed; readLoop will also error out and close the session
			}
		case <-s.done:
			return
		}
	}
}
