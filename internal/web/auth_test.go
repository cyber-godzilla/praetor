package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthManagerLoginSessionLogout(t *testing.T) {
	a, err := NewAuthManager("shared secret")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	a.now = func() time.Time { return now }

	if _, _, err := a.Login("192.0.2.1:1234", "wrong"); !errors.Is(err, errInvalidPassword) {
		t.Fatalf("wrong password error = %v", err)
	}
	token, csrf, err := a.Login("192.0.2.1:1234", "shared secret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" || csrf == "" || token == csrf {
		t.Fatalf("bad generated values: token=%q csrf=%q", token, csrf)
	}

	req := httptest.NewRequest("GET", "http://praetor.test/api/v1/bootstrap", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	gotCSRF, err := a.Session(req)
	if err != nil || gotCSRF != csrf {
		t.Fatalf("Session = %q, %v; want %q", gotCSRF, err, csrf)
	}

	a.Logout(req)
	if _, err := a.Session(req); !errors.Is(err, errNoSession) {
		t.Fatalf("Session after logout = %v", err)
	}
}

func TestAuthManagerRateLimitAndExpiry(t *testing.T) {
	a, _ := NewAuthManager("secret")
	now := time.Unix(2000, 0)
	a.now = func() time.Time { return now }

	for i := 0; i < a.maxAttempts; i++ {
		if _, _, err := a.Login("192.0.2.2:1", "bad"); !errors.Is(err, errInvalidPassword) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, _, err := a.Login("192.0.2.2:2", "secret"); !errors.Is(err, errRateLimited) {
		t.Fatalf("rate limit = %v", err)
	}

	now = now.Add(a.attemptWindow)
	token, _, err := a.Login("192.0.2.2:2", "secret")
	if err != nil {
		t.Fatalf("login after window: %v", err)
	}
	req := httptest.NewRequest("GET", "http://praetor.test/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	now = now.Add(a.idleTimeout + time.Second)
	if _, err := a.Session(req); !errors.Is(err, errNoSession) {
		t.Fatalf("expired session = %v", err)
	}
}

func TestSameOrigin(t *testing.T) {
	r := httptest.NewRequest("POST", "http://praetor.local:8787/api/v1/auth/login", nil)
	r.Host = "praetor.local:8787"
	r.Header.Set("Origin", "http://praetor.local:8787")
	if !sameOrigin(r) {
		t.Fatal("expected matching origin")
	}
	r.Header.Set("Origin", "http://attacker.local")
	if sameOrigin(r) {
		t.Fatal("accepted cross origin")
	}
	tlsRequest := httptest.NewRequest("POST", "https://praetor.local:8787/api/v1/auth/login", nil)
	tlsRequest.Host = "praetor.local:8787"
	tlsRequest.Header.Set("Origin", "https://praetor.local:8787")
	if !sameOrigin(tlsRequest) {
		t.Fatal("expected matching HTTPS origin")
	}
}

func TestAuthManagerGlobalRateLimit(t *testing.T) {
	a, _ := NewAuthManager("secret")
	a.maxAttempts = 100
	a.maxGlobal = 2

	for i := 0; i < 2; i++ {
		addr := "192.0.2." + string(rune('1'+i)) + ":1234"
		if _, _, err := a.Login(addr, "wrong"); !errors.Is(err, errInvalidPassword) {
			t.Fatalf("failure %d = %v", i, err)
		}
	}
	if _, _, err := a.Login("192.0.2.99:1234", "secret"); !errors.Is(err, errRateLimited) {
		t.Fatalf("global rate limit = %v", err)
	}
}

func TestAuthManagerPrunesExpiredAttemptWindows(t *testing.T) {
	a, _ := NewAuthManager("secret")
	now := time.Unix(2500, 0)
	a.now = func() time.Time { return now }

	if _, _, err := a.Login("192.0.2.1:1", "wrong"); !errors.Is(err, errInvalidPassword) {
		t.Fatal(err)
	}
	if len(a.attempts) != 1 {
		t.Fatalf("attempt entries = %d, want 1", len(a.attempts))
	}
	now = now.Add(a.attemptWindow)
	if _, _, err := a.Login("192.0.2.2:1", "wrong"); !errors.Is(err, errInvalidPassword) {
		t.Fatal(err)
	}
	if _, exists := a.attempts["192.0.2.1"]; exists {
		t.Fatal("expired source-attempt window was retained")
	}
	if len(a.attempts) != 1 {
		t.Fatalf("attempt entries after prune = %d, want 1", len(a.attempts))
	}
}

func TestAuthManagerSessionCapacityPrunesExpiredSessions(t *testing.T) {
	a, _ := NewAuthManager("secret")
	a.maxSessions = 2
	now := time.Unix(3000, 0)
	a.now = func() time.Time { return now }

	if _, _, err := a.Login("192.0.2.1:1", "secret"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Login("192.0.2.2:1", "secret"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Login("192.0.2.3:1", "secret"); !errors.Is(err, errSessionsFull) {
		t.Fatalf("capacity error = %v", err)
	}

	now = now.Add(a.idleTimeout + time.Second)
	if _, _, err := a.Login("192.0.2.3:1", "secret"); err != nil {
		t.Fatalf("login after expiry pruning: %v", err)
	}
}

func TestAuthCookieIsSecureUnderTLS(t *testing.T) {
	a, _ := NewAuthManager("secret")
	req := httptest.NewRequest(http.MethodPost, "https://praetor.test/api/v1/auth/login", nil)
	rr := httptest.NewRecorder()
	a.SetCookie(rr, req, "opaque")
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("TLS cookie = %#v", cookies)
	}
}

func TestLogoutInvalidatesOnlyCallingBrowser(t *testing.T) {
	a, _ := NewAuthManager("secret")
	tokenA, _, _ := a.Login("192.0.2.1:1", "secret")
	tokenB, _, _ := a.Login("192.0.2.2:1", "secret")
	request := func(token string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://praetor.test/", nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		return r
	}
	a.Logout(request(tokenA))
	if _, err := a.Session(request(tokenA)); !errors.Is(err, errNoSession) {
		t.Fatalf("browser A remained authenticated: %v", err)
	}
	if _, err := a.Session(request(tokenB)); err != nil {
		t.Fatalf("browser B was invalidated: %v", err)
	}
}
