package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	appgui "github.com/cyber-godzilla/praetor/internal/gui"
	"github.com/cyber-godzilla/praetor/internal/notes"
	"github.com/cyber-godzilla/praetor/internal/session"
)

func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	srv, handler, _ := newTestServerWithCredentials(t, &session.MockCredentialStore{})
	return srv, handler
}

func newTestServerWithCredentials(t *testing.T, creds session.CredentialStore) (*Server, http.Handler, *client.Client) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Defaults()
	gameClient, err := client.NewClient(cfg, []string{dir}, dir, creds)
	if err != nil {
		t.Fatal(err)
	}
	deps := &appgui.Deps{
		Client:        gameClient,
		Config:        cfg,
		ConfigPath:    dir + "/config.yaml",
		ConfigDir:     dir,
		DataDir:       dir,
		StateDir:      dir,
		SessionsDir:   dir,
		Creds:         creds,
		DesktopNotify: client.NewDesktopNotifier(cfg.Notifications.Desktop),
		Notes:         notes.New(filepath.Join(dir, "notes")),
		Version:       "test",
	}
	hub := NewHub(cfg.UI.Scrollback)
	app := appgui.NewGuiApp(deps, hub)
	auth, _ := NewAuthManager("test password")
	assets := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<html>praetor</html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('praetor')")},
	}
	srv := NewServer(app, auth, hub, fs.FS(assets), log.New(io.Discard, "", 0))
	return srv, srv.Handler(), gameClient
}

func TestCredentialStoreStatusIsNotCollapsedIntoAnEmptyAccountList(t *testing.T) {
	storeErr := fmt.Errorf("secret service unavailable")
	_, handler, _ := newTestServerWithCredentials(
		t,
		&session.MockCredentialStore{Err: storeErr},
	)
	cookie, csrf := loginRequest(t, handler)

	accountsReq := httptest.NewRequest(http.MethodGet, "http://praetor.test/api/v1/accounts", nil)
	accountsReq.Host = "praetor.test"
	accountsReq.AddCookie(cookie)
	accountsResponse := httptest.NewRecorder()
	handler.ServeHTTP(accountsResponse, accountsReq)
	if accountsResponse.Code != http.StatusOK {
		t.Fatalf("accounts status=%d body=%s", accountsResponse.Code, accountsResponse.Body.String())
	}
	var state appgui.AccountState
	if err := json.Unmarshal(accountsResponse.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.CredentialStore.Available || !state.CredentialStore.CanStore || state.CredentialStore.Message == "" {
		t.Fatalf("credential status = %+v", state.CredentialStore)
	}

	saveReq := httptest.NewRequest(
		http.MethodPut,
		"http://praetor.test/api/v1/accounts/alice",
		bytes.NewBufferString(`{"password":"password"}`),
	)
	saveReq.Host = "praetor.test"
	saveReq.Header.Set("Origin", "http://praetor.test")
	saveReq.Header.Set("X-Praetor-CSRF", csrf)
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.AddCookie(cookie)
	saveResponse := httptest.NewRecorder()
	handler.ServeHTTP(saveResponse, saveReq)
	if saveResponse.Code != http.StatusServiceUnavailable || !strings.Contains(saveResponse.Body.String(), "credential_store_unavailable") {
		t.Fatalf("save status=%d body=%s", saveResponse.Code, saveResponse.Body.String())
	}
}

func TestProtectedRoutesRejectUnauthenticatedRequests(t *testing.T) {
	_, handler := newTestServer(t)
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/bootstrap"},
		{http.MethodGet, "/api/v1/events"},
		{http.MethodPost, "/api/v1/game/connect"},
		{http.MethodPost, "/api/v1/game/connect-stored"},
		{http.MethodPost, "/api/v1/game/disconnect"},
		{http.MethodPost, "/api/v1/commands"},
		{http.MethodGet, "/api/v1/accounts"},
		{http.MethodGet, "/api/v1/modes"},
		{http.MethodPut, "/api/v1/mode"},
		{http.MethodPost, "/api/v1/scripts/reload"},
		{http.MethodPut, "/api/v1/settings/echo-typed"},
		{http.MethodGet, "/api/v1/kudos"},
		{http.MethodGet, "/api/v1/persistent"},
		{http.MethodGet, "/api/v1/notes"},
		{http.MethodPut, "/api/v1/notes"},
		{http.MethodDelete, "/api/v1/notes/example"},
		{http.MethodGet, "/api/v1/wiki"},
		{http.MethodGet, "/api/v1/maps"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "http://praetor.test"+test.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMutatingRouteRequiresOriginAndCSRF(t *testing.T) {
	_, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	makeRequest := func(origin, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://praetor.test/api/v1/game/disconnect", bytes.NewBufferString(`{}`))
		req.Host = "praetor.test"
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Praetor-CSRF", token)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}
	if rr := makeRequest("http://attacker.test", csrf); rr.Code != http.StatusForbidden {
		t.Fatalf("wrong origin status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := makeRequest("http://praetor.test", "wrong"); rr.Code != http.StatusForbidden {
		t.Fatalf("wrong csrf status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := makeRequest("http://praetor.test", csrf); rr.Code != http.StatusOK {
		t.Fatalf("valid mutation status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func loginRequest(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://praetor.test/api/v1/auth/login", bytes.NewBufferString(`{"password":"test password"}`))
	req.Host = "praetor.test"
	req.RemoteAddr = "192.0.2.5:1234"
	req.Header.Set("Origin", "http://praetor.test")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login cookies = %#v", cookies)
	}

	bootstrap := httptest.NewRequest(http.MethodGet, "http://praetor.test/api/v1/bootstrap", nil)
	bootstrap.Host = "praetor.test"
	bootstrap.AddCookie(cookies[0])
	br := httptest.NewRecorder()
	handler.ServeHTTP(br, bootstrap)
	if br.Code != http.StatusOK {
		t.Fatalf("bootstrap status=%d body=%s", br.Code, br.Body.String())
	}
	var body struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(br.Body.Bytes(), &body); err != nil || body.CSRF == "" {
		t.Fatalf("bootstrap body=%s err=%v", br.Body.String(), err)
	}
	return cookies[0], body.CSRF
}

func TestServerAuthStaticAndProtectedBoundary(t *testing.T) {
	_, handler := newTestServer(t)

	static := httptest.NewRecorder()
	handler.ServeHTTP(static, httptest.NewRequest(http.MethodGet, "http://praetor.test/", nil))
	if static.Code != http.StatusOK || !strings.Contains(static.Body.String(), "praetor") {
		t.Fatalf("static status=%d body=%s", static.Code, static.Body.String())
	}
	if static.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing security headers")
	}

	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "http://praetor.test/api/v1/bootstrap", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized bootstrap status=%d", unauth.Code)
	}

	badOrigin := httptest.NewRequest(http.MethodPost, "http://praetor.test/api/v1/auth/login", bytes.NewBufferString(`{"password":"test password"}`))
	badOrigin.Host = "praetor.test"
	badOrigin.Header.Set("Origin", "http://attacker.test")
	bad := httptest.NewRecorder()
	handler.ServeHTTP(bad, badOrigin)
	if bad.Code != http.StatusForbidden {
		t.Fatalf("bad-origin login status=%d", bad.Code)
	}

	loginRequest(t, handler)

	apiRoot := httptest.NewRecorder()
	handler.ServeHTTP(apiRoot, httptest.NewRequest(http.MethodGet, "http://praetor.test/api", nil))
	if apiRoot.Code != http.StatusNotFound || strings.Contains(apiRoot.Body.String(), "<html>") {
		t.Fatalf("/api used SPA fallback: status=%d body=%s", apiRoot.Code, apiRoot.Body.String())
	}
}

func TestLoginBodyLimit(t *testing.T) {
	_, handler := newTestServer(t)
	body := `{"password":"` + strings.Repeat("x", maxJSONBody) + `"}`
	req := httptest.NewRequest(http.MethodPost, "http://praetor.test/api/v1/auth/login", strings.NewReader(body))
	req.Host = "praetor.test"
	req.Header.Set("Origin", "http://praetor.test")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("oversized login status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestServerRevisionedSettingBroadcast(t *testing.T) {
	srv, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	sub, _ := srv.hub.Subscribe()
	defer srv.hub.Unsubscribe(sub.ID)

	apply := func(revision int) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"expectedRevision":%d,"value":false}`, revision)
		req := httptest.NewRequest(http.MethodPut, "http://praetor.test/api/v1/settings/echo-typed", bytes.NewBufferString(body))
		req.Host = "praetor.test"
		req.Header.Set("Origin", "http://praetor.test")
		req.Header.Set("X-Praetor-CSRF", csrf)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	if rr := apply(1); rr.Code != http.StatusOK {
		t.Fatalf("setting status=%d body=%s", rr.Code, rr.Body.String())
	}
	msg := <-sub.Messages
	if msg.Type != "config" || msg.Revision != 2 {
		t.Fatalf("broadcast = %#v", msg)
	}
	if rr := apply(1); rr.Code != http.StatusConflict {
		t.Fatalf("stale setting status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestServerMobileWebSettings(t *testing.T) {
	tests := []struct {
		operation string
		value     any
		matches   func(config.UIConfig) bool
	}{
		{
			operation: "mobile-show-toolbar",
			value:     false,
			matches:   func(ui config.UIConfig) bool { return !ui.MobileShowToolbar },
		},
		{
			operation: "mobile-show-tab-bar",
			value:     false,
			matches:   func(ui config.UIConfig) bool { return !ui.MobileShowTabBar },
		},
		{
			operation: "mobile-hide-navigation-on-input",
			value:     true,
			matches:   func(ui config.UIConfig) bool { return ui.MobileHideNavigationOnInput },
		},
		{
			operation: "mobile-lowercase-first-letter",
			value:     true,
			matches:   func(ui config.UIConfig) bool { return ui.MobileLowercaseFirstLetter },
		},
		{
			operation: "mobile-output-font-size",
			value:     6,
			matches:   func(ui config.UIConfig) bool { return ui.MobileOutputFontSize == 6 },
		},
		{
			operation: "input-spellcheck",
			value:     false,
			matches:   func(ui config.UIConfig) bool { return !ui.InputSpellcheck },
		},
	}

	for _, test := range tests {
		t.Run(test.operation, func(t *testing.T) {
			srv, handler := newTestServer(t)
			cookie, csrf := loginRequest(t, handler)
			sub, _ := srv.hub.Subscribe()
			defer srv.hub.Unsubscribe(sub.ID)

			value, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			body := fmt.Sprintf(`{"expectedRevision":1,"value":%s}`, value)
			req := httptest.NewRequest(
				http.MethodPut,
				"http://praetor.test/api/v1/settings/"+test.operation,
				bytes.NewBufferString(body),
			)
			req.Host = "praetor.test"
			req.Header.Set("Origin", "http://praetor.test")
			req.Header.Set("X-Praetor-CSRF", csrf)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
			}
			if !test.matches(srv.app.GetConfig().UI) {
				t.Fatalf("setting was not applied: %+v", srv.app.GetConfig().UI)
			}
			message := <-sub.Messages
			if message.Type != "config" || message.Revision != 2 {
				t.Fatalf("broadcast = %#v", message)
			}
		})
	}
}

func TestServerUpdateCheckSetting(t *testing.T) {
	srv, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	req := httptest.NewRequest(
		http.MethodPut,
		"http://praetor.test/api/v1/settings/update-check",
		bytes.NewBufferString(`{"expectedRevision":1,"value":false}`),
	)
	req.Host = "praetor.test"
	req.Header.Set("Origin", "http://praetor.test")
	req.Header.Set("X-Praetor-CSRF", csrf)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if srv.app.GetConfig().Updates.Check {
		t.Fatal("update check setting was not applied")
	}
}

func TestWebNotesRoundTrip(t *testing.T) {
	_, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	request := func(method, path, body string, mutating bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://praetor.test"+path, strings.NewReader(body))
		req.Host = "praetor.test"
		req.AddCookie(cookie)
		if mutating {
			req.Header.Set("Origin", "http://praetor.test")
			req.Header.Set("X-Praetor-CSRF", csrf)
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	saved := request(
		http.MethodPut,
		"/api/v1/notes",
		`{"originalTitle":"","title":"Field Notes","body":"first line"}`,
		true,
	)
	if saved.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saved.Code, saved.Body.String())
	}

	listed := request(http.MethodGet, "/api/v1/notes", "", false)
	var summaries []notes.Summary
	if listed.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &summaries); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].Title != "Field Notes" {
		t.Fatalf("notes = %+v", summaries)
	}

	got := request(http.MethodGet, "/api/v1/notes/Field%20Notes", "", false)
	var note notes.Note
	if got.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
	}
	if err := json.Unmarshal(got.Body.Bytes(), &note); err != nil {
		t.Fatal(err)
	}
	if note.Title != "Field Notes" || note.Body != "first line" {
		t.Fatalf("note = %+v", note)
	}

	deleted := request(http.MethodDelete, "/api/v1/notes/Field%20Notes", "", true)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	missing := request(http.MethodGet, "/api/v1/notes/Field%20Notes", "", false)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestConcurrentSettingWritesAllowOneRevisionWinner(t *testing.T) {
	_, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	const writers = 12
	statuses := make(chan int, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(value bool) {
			defer wg.Done()
			body := fmt.Sprintf(`{"expectedRevision":1,"value":%t}`, value)
			req := httptest.NewRequest(http.MethodPut, "http://praetor.test/api/v1/settings/echo-typed", bytes.NewBufferString(body))
			req.Host = "praetor.test"
			req.Header.Set("Origin", "http://praetor.test")
			req.Header.Set("X-Praetor-CSRF", csrf)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			statuses <- rr.Code
		}(i%2 == 0)
	}
	wg.Wait()
	close(statuses)
	winners := 0
	conflicts := 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			winners++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status %d", status)
		}
	}
	if winners != 1 || conflicts != writers-1 {
		t.Fatalf("winners=%d conflicts=%d", winners, conflicts)
	}
}

func TestKudosReplacementRequiresCurrentConfigRevision(t *testing.T) {
	_, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	apply := func(revision int) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"expectedRevision":%d,"value":{"Favorites":["Marcus"],"Queue":[]}}`, revision)
		req := httptest.NewRequest(http.MethodPut, "http://praetor.test/api/v1/kudos", bytes.NewBufferString(body))
		req.Host = "praetor.test"
		req.Header.Set("Origin", "http://praetor.test")
		req.Header.Set("X-Praetor-CSRF", csrf)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	if response := apply(1); response.Code != http.StatusOK {
		t.Fatalf("current Kudos update status=%d body=%s", response.Code, response.Body.String())
	}
	if response := apply(1); response.Code != http.StatusConflict {
		t.Fatalf("stale Kudos update status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCommandRequiresConnectedSessionAndEnforcesLength(t *testing.T) {
	srv, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	submit := func(input string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"input": input})
		req := httptest.NewRequest(http.MethodPost, "http://praetor.test/api/v1/commands", bytes.NewReader(body))
		req.Host = "praetor.test"
		req.Header.Set("Origin", "http://praetor.test")
		req.Header.Set("X-Praetor-CSRF", csrf)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}
	if rr := submit("look"); rr.Code != http.StatusConflict {
		t.Fatalf("disconnected command status=%d body=%s", rr.Code, rr.Body.String())
	}
	srv.opMu.Lock()
	srv.conn = "connected"
	srv.opMu.Unlock()
	if rr := submit(strings.Repeat("x", maxCommandBody+1)); rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized command status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr := submit("look"); rr.Code != http.StatusAccepted {
		t.Fatalf("connected command status=%d body=%s", rr.Code, rr.Body.String())
	}
}
