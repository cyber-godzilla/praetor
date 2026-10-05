package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sessionCookieName = "praetor_web_session"

var (
	errInvalidPassword = errors.New("invalid password")
	errRateLimited     = errors.New("too many attempts")
	errSessionsFull    = errors.New("too many browser sessions")
	errNoSession       = errors.New("no authenticated session")
)

type browserSession struct {
	csrf      string
	id        string
	expiresAt time.Time
}

type attemptWindow struct {
	started time.Time
	count   int
}

// AuthManager verifies the startup PSK and owns opaque, in-memory browser
// sessions. Only digests of the password and session-cookie values are kept.
type AuthManager struct {
	mu sync.Mutex

	passwordDigest [sha256.Size]byte
	sessions       map[[sha256.Size]byte]browserSession
	attempts       map[string]attemptWindow
	globalAttempts attemptWindow
	idleTimeout    time.Duration
	attemptWindow  time.Duration
	maxAttempts    int
	maxGlobal      int
	maxSessions    int
	now            func() time.Time
	random         func([]byte) (int, error)
}

func NewAuthManager(password string) (*AuthManager, error) {
	if password == "" {
		return nil, errors.New("PRAETOR_WEB_PASSWORD must not be empty")
	}
	return &AuthManager{
		passwordDigest: sha256.Sum256([]byte(password)),
		sessions:       make(map[[sha256.Size]byte]browserSession),
		attempts:       make(map[string]attemptWindow),
		idleTimeout:    12 * time.Hour,
		attemptWindow:  time.Minute,
		maxAttempts:    5,
		maxGlobal:      50,
		maxSessions:    64,
		now:            time.Now,
		random:         rand.Read,
	}, nil
}

func (a *AuthManager) Login(remoteAddr, password string) (token, csrf string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	key := remoteHost(remoteAddr)
	window := a.attempts[key]
	if window.started.IsZero() || now.Sub(window.started) >= a.attemptWindow {
		window = attemptWindow{started: now}
	}
	global := a.globalAttempts
	if global.started.IsZero() || now.Sub(global.started) >= a.attemptWindow {
		global = attemptWindow{started: now}
		a.pruneAttemptsLocked(now)
	}
	if window.count >= a.maxAttempts || global.count >= a.maxGlobal {
		a.globalAttempts = global
		return "", "", errRateLimited
	}

	candidate := sha256.Sum256([]byte(password))
	if subtle.ConstantTimeCompare(candidate[:], a.passwordDigest[:]) != 1 {
		window.count++
		global.count++
		a.attempts[key] = window
		a.globalAttempts = global
		return "", "", errInvalidPassword
	}
	delete(a.attempts, key)
	a.globalAttempts = global
	a.pruneSessionsLocked(now)
	if len(a.sessions) >= a.maxSessions {
		return "", "", errSessionsFull
	}

	token, err = a.randomToken()
	if err != nil {
		return "", "", err
	}
	csrf, err = a.randomToken()
	if err != nil {
		return "", "", err
	}
	browserID, err := a.randomToken()
	if err != nil {
		return "", "", err
	}
	a.sessions[sha256.Sum256([]byte(token))] = browserSession{
		csrf:      csrf,
		id:        browserID,
		expiresAt: now.Add(a.idleTimeout),
	}
	return token, csrf, nil
}

func (a *AuthManager) pruneAttemptsLocked(now time.Time) {
	for key, window := range a.attempts {
		if window.started.IsZero() || now.Sub(window.started) >= a.attemptWindow {
			delete(a.attempts, key)
		}
	}
}

func (a *AuthManager) pruneSessionsLocked(now time.Time) {
	for digest, session := range a.sessions {
		if !now.Before(session.expiresAt) {
			delete(a.sessions, digest)
		}
	}
}

func (a *AuthManager) Session(r *http.Request) (string, error) {
	csrf, _, err := a.SessionInfo(r)
	return csrf, err
}

// SessionInfo validates and refreshes a browser session, returning its CSRF
// token and non-sensitive diagnostic ID. The opaque cookie value is never
// exposed to handlers or logs.
func (a *AuthManager) SessionInfo(r *http.Request) (string, string, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", "", errNoSession
	}
	digest := sha256.Sum256([]byte(cookie.Value))

	a.mu.Lock()
	defer a.mu.Unlock()
	sess, ok := a.sessions[digest]
	if !ok {
		return "", "", errNoSession
	}
	now := a.now()
	if !now.Before(sess.expiresAt) {
		delete(a.sessions, digest)
		return "", "", errNoSession
	}
	sess.expiresAt = now.Add(a.idleTimeout)
	a.sessions[digest] = sess
	return sess.csrf, sess.id, nil
}

func (a *AuthManager) Logout(r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	a.mu.Lock()
	delete(a.sessions, digest)
	a.mu.Unlock()
}

func (a *AuthManager) ValidateCSRF(r *http.Request, want string) bool {
	got := r.Header.Get("X-Praetor-CSRF")
	if got == "" || want == "" {
		return false
	}
	gotDigest := sha256.Sum256([]byte(got))
	wantDigest := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotDigest[:], wantDigest[:]) == 1
}

func (a *AuthManager) SetCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *AuthManager) ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *AuthManager) randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := a.random(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(origin, scheme+"://"+r.Host)
}
