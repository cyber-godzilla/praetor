package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	appgui "github.com/cyber-godzilla/praetor/internal/gui"
)

// Hard transport limits are intentionally server-owned so a config edit cannot
// turn one browser into unbounded memory or upstream input pressure.
const (
	maxJSONBody    = 64 << 10
	maxCommandBody = 16 << 10
)

type Server struct {
	app    *appgui.GuiApp
	auth   *AuthManager
	hub    *Hub
	assets fs.FS
	log    *log.Logger

	mux      *http.ServeMux
	opMu     sync.Mutex
	revision uint64
	conn     string
	requests atomic.Uint64

	upgrader websocket.Upgrader
}

type bootstrapResponse struct {
	appgui.InitState
	CSRF           string `json:"csrf"`
	Protocol       int    `json:"protocol"`
	ServerID       string `json:"serverId"`
	ConfigRevision uint64 `json:"configRevision"`
}

func NewServer(app *appgui.GuiApp, auth *AuthManager, hub *Hub, assets fs.FS, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	s := &Server{
		app:      app,
		auth:     auth,
		hub:      hub,
		assets:   assets,
		log:      logger,
		mux:      http.NewServeMux(),
		revision: 1,
		conn:     "disconnected",
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			CheckOrigin:      sameOrigin,
		},
	}
	if hub != nil {
		hub.SetObserver(s.observeEvents)
		if app != nil {
			init := app.GetInitState()
			hub.SetInitialState(cloneJSON(init.Config), s.revision, init.ModeNames, init.Accounts, init.CredentialStore)
		}
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.protected(true, s.handleLogout))
	s.mux.HandleFunc("GET /api/v1/bootstrap", s.protected(false, s.handleBootstrap))
	s.mux.HandleFunc("GET /api/v1/events", s.protected(false, s.handleEvents))

	s.mux.HandleFunc("POST /api/v1/game/connect", s.protected(true, s.handleConnect))
	s.mux.HandleFunc("POST /api/v1/game/connect-stored", s.protected(true, s.handleConnectStored))
	s.mux.HandleFunc("POST /api/v1/game/disconnect", s.protected(true, s.handleDisconnect))
	s.mux.HandleFunc("POST /api/v1/commands", s.protected(true, s.handleCommand))
	s.mux.HandleFunc("GET /api/v1/accounts", s.protected(false, s.handleAccounts))
	s.mux.HandleFunc("PUT /api/v1/accounts/{name}", s.protected(true, s.handleSaveAccount))
	s.mux.HandleFunc("DELETE /api/v1/accounts/{name}", s.protected(true, s.handleRemoveAccount))
	s.mux.HandleFunc("GET /api/v1/modes", s.protected(false, s.handleModes))
	s.mux.HandleFunc("PUT /api/v1/mode", s.protected(true, s.handleSetMode))
	s.mux.HandleFunc("POST /api/v1/scripts/reload", s.protected(true, s.handleReloadScripts))
	s.mux.HandleFunc("POST /api/v1/graphics/refresh", s.protected(true, s.handleRefreshGraphics))
	s.mux.HandleFunc("PUT /api/v1/settings/{operation}", s.protected(true, s.handleSetting))

	s.mux.HandleFunc("GET /api/v1/kudos", s.protected(false, s.handleGetKudos))
	s.mux.HandleFunc("PUT /api/v1/kudos", s.protected(true, s.handleSetKudos))
	s.mux.HandleFunc("POST /api/v1/kudos/favorites", s.protected(true, s.handleAddKudosFavorite))
	s.mux.HandleFunc("POST /api/v1/kudos/queue", s.protected(true, s.handleAddKudosQueue))
	s.mux.HandleFunc("GET /api/v1/persistent", s.protected(false, s.handlePersistentData))
	s.mux.HandleFunc("POST /api/v1/persistent/export", s.protected(true, s.handlePersistentExport))
	s.mux.HandleFunc("DELETE /api/v1/persistent", s.protected(true, s.handlePersistentClear))
	s.mux.HandleFunc("GET /api/v1/wiki", s.protected(false, s.handleWiki))
	s.mux.HandleFunc("GET /api/v1/maps", s.protected(false, s.handleMaps))
	s.mux.HandleFunc("POST /api/v1/calc/rank-bonus", s.protected(true, s.handleRankBonus))
	s.mux.HandleFunc("POST /api/v1/calc/train-cost", s.protected(true, s.handleTrainCost))

	s.mux.HandleFunc("/", s.handleStatic)
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

type protectedHandler func(http.ResponseWriter, *http.Request, string)

type browserIDContextKey struct{}

func (s *Server) protected(mutating bool, next protectedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrf, browserID, err := s.auth.SessionInfo(r)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
			return
		}
		if mutating {
			if !sameOrigin(r) {
				s.writeError(w, http.StatusForbidden, "origin_rejected", "Request origin rejected.")
				return
			}
			if !s.auth.ValidateCSRF(r, csrf) {
				s.writeError(w, http.StatusForbidden, "csrf_rejected", "Request verification failed.")
				return
			}
			s.log.Printf("web mutation browser=%s method=%s path=%s", browserID, r.Method, r.URL.Path)
		}
		r = r.WithContext(context.WithValue(r.Context(), browserIDContextKey{}, browserID))
		next(w, r, csrf)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		s.writeError(w, http.StatusForbidden, "origin_rejected", "Request origin rejected.")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid login request.")
		return
	}
	token, _, err := s.auth.Login(r.RemoteAddr, req.Password)
	if err != nil {
		if errors.Is(err, errRateLimited) {
			w.Header().Set("Retry-After", "60")
			s.writeError(w, http.StatusTooManyRequests, "login_limited", "Too many login attempts. Try again later.")
			return
		}
		if errors.Is(err, errSessionsFull) {
			s.writeError(w, http.StatusServiceUnavailable, "session_capacity", "Browser session capacity reached. Try again later.")
			return
		}
		s.writeError(w, http.StatusUnauthorized, "login_failed", "Authentication failed.")
		return
	}
	s.auth.SetCookie(w, r, token)
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, _ string) {
	s.auth.Logout(r)
	s.auth.ClearCookie(w, r)
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleBootstrap(w http.ResponseWriter, _ *http.Request, csrf string) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, http.StatusOK, bootstrapResponse{
		InitState:      s.app.GetInitState(),
		CSRF:           csrf,
		Protocol:       ProtocolVersion,
		ServerID:       s.hub.ServerID(),
		ConfigRevision: s.revision,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, _ string) {
	if !sameOrigin(r) {
		s.writeError(w, http.StatusForbidden, "origin_rejected", "Request origin rejected.")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sub, snapshot := s.hub.Subscribe()
	if sub.ID == 0 {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "browser capacity reached"),
			time.Now().Add(5*time.Second))
		return
	}
	defer s.hub.Unsubscribe(sub.ID)

	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	if _, err := s.auth.Session(r); err != nil {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "authentication expired"),
			time.Now().Add(5*time.Second))
		return
	}
	if err := writeWebSocketJSON(conn, snapshot); err != nil {
		return
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-sub.Messages:
			if !ok {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "resync required"),
					time.Now().Add(5*time.Second))
				return
			}
			// Logout revokes the browser session immediately for subsequent
			// application data; do not wait for the next heartbeat to notice.
			if _, err := s.auth.Session(r); err != nil {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "authentication expired"),
					time.Now().Add(5*time.Second))
				return
			}
			if err := writeWebSocketJSON(conn, msg); err != nil {
				return
			}
		case <-ticker.C:
			if _, err := s.auth.Session(r); err != nil {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "authentication expired"),
					time.Now().Add(5*time.Second))
				return
			}
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func writeWebSocketJSON(conn *websocket.Conn, value any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(value)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		s.writeError(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	if s.assets == nil {
		http.Error(w, "web assets unavailable", http.StatusServiceUnavailable)
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(s.assets, name)
	if err != nil {
		name = "index.html"
		data, err = fs.ReadFile(s.assets, name)
	}
	if err != nil {
		http.Error(w, "web assets unavailable", http.StatusServiceUnavailable)
		return
	}
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (s *Server) observeEvents(events []appgui.WireEvent) {
	for _, event := range events {
		if event.Kind != appgui.KindConn || event.Conn == nil {
			continue
		}
		s.opMu.Lock()
		s.conn = event.Conn.State
		s.opMu.Unlock()
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		s.log.Printf("web response encode: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	id := fmt.Sprintf("web-%d", s.requests.Add(1))
	s.writeJSON(w, status, APIError{Error: APIErrorBody{Code: code, Message: message, RequestID: id}})
}

func decodeJSON(r *http.Request, dst any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBody+1))
	if err != nil {
		return err
	}
	if len(data) > maxJSONBody {
		return errors.New("request body is too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func decodeValue[T any](raw json.RawMessage) (T, error) {
	var value T
	err := json.Unmarshal(raw, &value)
	return value, err
}

func cloneJSON(value any) json.RawMessage {
	b, _ := json.Marshal(value)
	return b
}

func settingValue[T any](raw json.RawMessage, apply func(T) error) error {
	v, err := decodeValue[T](raw)
	if err != nil {
		return err
	}
	return apply(v)
}
