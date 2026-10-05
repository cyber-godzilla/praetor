package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	appgui "github.com/cyber-godzilla/praetor/internal/gui"
)

func TestEventWebSocketRequiresAuthenticationAndSameOrigin(t *testing.T) {
	_, handler := newTestServer(t)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/events"

	_, response, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{httpServer.URL},
	})
	if err == nil {
		t.Fatal("unauthenticated websocket unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated websocket response = %#v, err=%v", response, err)
	}

	cookie, _ := loginRequest(t, handler)
	_, response, err = websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{"http://attacker.test"},
		"Cookie": []string{cookie.String()},
	})
	if err == nil {
		t.Fatal("wrong-origin websocket unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong-origin websocket response = %#v, err=%v", response, err)
	}
}

func TestEventWebSocketSnapshotThenLiveEvents(t *testing.T) {
	srv, handler := newTestServer(t)
	srv.hub.Emit(appgui.EventChannel, []appgui.WireEvent{{
		Kind: appgui.KindConn,
		Conn: &appgui.ConnPayload{State: "connected"},
	}, {
		Kind: appgui.KindText,
		Text: &appgui.TextPayload{Text: "before"},
	}})

	cookie, _ := loginRequest(t, handler)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/events"
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{httpServer.URL},
		"Cookie": []string{cookie.String()},
	})
	if err != nil {
		t.Fatalf("websocket dial response=%#v: %v", response, err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var snapshot Envelope
	if err := conn.ReadJSON(&snapshot); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot.Type != "snapshot" || snapshot.Sequence != 1 || len(snapshot.Events) != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}

	srv.hub.Emit(appgui.EventChannel, []appgui.WireEvent{{
		Kind: appgui.KindText,
		Text: &appgui.TextPayload{Text: "after"},
	}})
	var live Envelope
	if err := conn.ReadJSON(&live); err != nil {
		t.Fatalf("read live event: %v", err)
	}
	if live.Type != "events" || live.Sequence != 2 || len(live.Events) != 1 || live.Events[0].Text.Text != "after" {
		t.Fatalf("live event = %#v", live)
	}
}

func TestTwoWebSocketClientsConvergeAcrossLateJoin(t *testing.T) {
	srv, handler := newTestServer(t)
	cookieA, _ := loginRequest(t, handler)
	cookieB, _ := loginRequest(t, handler)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/events"
	dial := func(cookie *http.Cookie) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
			"Origin": []string{httpServer.URL},
			"Cookie": []string{cookie.String()},
		})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		return conn
	}
	read := func(conn *websocket.Conn) Envelope {
		var envelope Envelope
		if err := conn.ReadJSON(&envelope); err != nil {
			t.Fatalf("read envelope: %v", err)
		}
		return envelope
	}

	first := dial(cookieA)
	defer first.Close()
	if snapshot := read(first); snapshot.Type != "snapshot" || snapshot.Sequence != 0 {
		t.Fatalf("first snapshot = %#v", snapshot)
	}
	srv.hub.Emit(appgui.EventChannel, []appgui.WireEvent{{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "connected"}}, {
		Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "late join history"},
	}})
	if live := read(first); live.Type != "events" || live.Sequence != 1 {
		t.Fatalf("first live event = %#v", live)
	}

	second := dial(cookieB)
	defer second.Close()
	secondSnapshot := read(second)
	if secondSnapshot.Type != "snapshot" || secondSnapshot.Sequence != 1 || len(secondSnapshot.Events) != 2 {
		t.Fatalf("late-join snapshot = %#v", secondSnapshot)
	}

	srv.hub.Emit(appgui.EventChannel, []appgui.WireEvent{{
		Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "shared live event"},
	}})
	for name, conn := range map[string]*websocket.Conn{"first": first, "second": second} {
		if live := read(conn); live.Type != "events" || live.Sequence != 2 || live.Events[0].Text.Text != "shared live event" {
			t.Fatalf("%s converged event = %#v", name, live)
		}
	}
}

func TestLogoutStopsSubsequentWebSocketEvents(t *testing.T) {
	srv, handler := newTestServer(t)
	cookie, csrf := loginRequest(t, handler)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/events"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{httpServer.URL},
		"Cookie": []string{cookie.String()},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var snapshot Envelope
	if err := conn.ReadJSON(&snapshot); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	logout := httptest.NewRequest(http.MethodPost, httpServer.URL+"/api/v1/auth/logout", strings.NewReader(`{}`))
	logout.Header.Set("Origin", httpServer.URL)
	logout.Header.Set("X-Praetor-CSRF", csrf)
	logout.Header.Set("Content-Type", "application/json")
	logout.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, logout)
	if response.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", response.Code, response.Body.String())
	}

	srv.hub.Emit(appgui.EventChannel, []appgui.WireEvent{{
		Kind: appgui.KindText,
		Text: &appgui.TextPayload{Text: "must not reach logged-out browser"},
	}})
	var live Envelope
	if err := conn.ReadJSON(&live); err == nil {
		t.Fatalf("logged-out browser received event: %#v", live)
	}
}
