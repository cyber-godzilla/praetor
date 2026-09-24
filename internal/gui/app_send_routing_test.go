package gui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/gorilla/websocket"
)

var sendRoutingUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// newSendRoutingServer upgrades exactly one client connection and records
// every non-ident text frame it receives onto a buffered channel, mirroring
// internal/client's newRecordingServer (unexported there, so duplicated here
// for the gui package's own routing tests).
func newSendRoutingServer(t *testing.T) (*httptest.Server, string, <-chan string) {
	t.Helper()
	received := make(chan string, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := sendRoutingUpgrader.Upgrade(w, r, nil)
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

// newVerbatimSendRoutingServer is like newSendRoutingServer but records the
// frame WITHOUT trimming "\r\n" — needed to tell "look\r\n" apart from
// "look\n\r\n", which newSendRoutingServer's TrimRight would collapse into the
// same string.
func newVerbatimSendRoutingServer(t *testing.T) (*httptest.Server, string, <-chan string) {
	t.Helper()
	received := make(chan string, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := sendRoutingUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			text := string(msg)
			if strings.HasPrefix(text, "SKOTOS ") {
				continue // ident frame, not a game command
			}
			received <- text
		}
	}))
	return srv, "ws" + strings.TrimPrefix(srv.URL, "http"), received
}

// newSendRoutingApp builds a GuiApp wired to a real *client.Client connected
// to a recording test server, so Send's routing decision (SendCommand vs
// SendBlock) can be observed via what actually reaches the wire — and, for
// slash commands, via CurrentMode(), since /mode never touches the network.
func newSendRoutingApp(t *testing.T) (*GuiApp, <-chan string) {
	t.Helper()
	srv, wsURL, recv := newSendRoutingServer(t)
	t.Cleanup(srv.Close)

	modeDir := t.TempDir()
	modePath := filepath.Join(modeDir, "aggro.lua")
	modeBody := `
local M = {}
M.on_start = function(args)
    if args[1] then send("arg " .. args[1]) end
end
M.reactions = {}
return M
`
	if err := os.WriteFile(modePath, []byte(modeBody), 0o644); err != nil {
		t.Fatalf("write test mode: %v", err)
	}

	c, err := client.NewClient(config.Defaults(), []string{modeDir}, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)

	c.Session = session.New()
	if err := c.Session.Connect(wsURL, nil); err != nil {
		t.Fatalf("connect: %v", err)
	}

	deps := &Deps{
		Config:  config.Defaults(),
		Client:  c,
		Version: "0.2.0",
	}
	a := NewGuiApp(deps, &captureEmitter{})
	return a, recv
}

// newVerbatimSendRoutingApp is newSendRoutingApp wired to the verbatim
// recorder, for tests that must distinguish a trailing newline on the wire.
func newVerbatimSendRoutingApp(t *testing.T) (*GuiApp, <-chan string) {
	t.Helper()
	srv, wsURL, recv := newVerbatimSendRoutingServer(t)
	t.Cleanup(srv.Close)

	c, err := client.NewClient(config.Defaults(), nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)

	c.Session = session.New()
	if err := c.Session.Connect(wsURL, nil); err != nil {
		t.Fatalf("connect: %v", err)
	}

	deps := &Deps{
		Config:  config.Defaults(),
		Client:  c,
		Version: "0.2.0",
	}
	a := NewGuiApp(deps, &captureEmitter{})
	return a, recv
}

// TestSend_PlainCommandTrailingNewlineDoesNotLeak covers the assertion the
// final review asked for: a plain (non-slash) single-line command typed or
// pasted with a trailing newline, e.g. "look\n", must not leak that newline
// into what is sent — it must reach the wire as exactly "look\r\n", not
// "look\n\r\n". Send() already trims trailing "\r\n" before handing the value
// to SendCommand; this pins that behavior with a recorder that can actually
// tell the two apart (newSendRoutingServer's TrimRight cannot).
func TestSend_PlainCommandTrailingNewlineDoesNotLeak(t *testing.T) {
	a, recv := newVerbatimSendRoutingApp(t)

	a.Send("look\n")

	select {
	case msg := <-recv:
		want := "look\r\n"
		if msg != want {
			t.Fatalf("server received %q, want %q — trailing newline leaked into the sent command", msg, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received the command")
	}
}

// TestSend_TrailingNewlineStillRoutesSlashCommand covers the fix-round-1
// finding: a single command pasted with a trailing newline ("/mode aggro\n",
// common when copying from a document) must still be treated as single-line
// input and interpreted as a slash command, not shipped to SendBlock as
// literal prose.
func TestSend_TrailingNewlineStillRoutesSlashCommand(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	a.Send("/mode aggro\n")

	if got := a.client().Engine.CurrentMode(); got != "aggro" {
		t.Fatalf("CurrentMode() = %q, want %q — trailing newline should not have routed to SendBlock", got, "aggro")
	}
	select {
	case msg := <-recv:
		t.Fatalf("server received %q — /mode must be handled locally, never sent over the wire", msg)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestSend_PlainSlashCommandRoutesAsCommand is the baseline: a slash command
// with no trailing newline at all must keep working exactly as before.
func TestSend_PlainSlashCommandRoutesAsCommand(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	a.Send("/mode aggro")

	if got := a.client().Engine.CurrentMode(); got != "aggro" {
		t.Fatalf("CurrentMode() = %q, want %q", got, "aggro")
	}
	select {
	case msg := <-recv:
		t.Fatalf("server received %q — /mode must be handled locally, never sent over the wire", msg)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestSend_InteriorNewlineRoutesAsBlock covers real multi-line input: an
// interior newline means the user is sending a block, which must go out as
// ONE message with the newline embedded (never split, never interpreted).
func TestSend_InteriorNewlineRoutesAsBlock(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	a.Send("line one\nline two")

	select {
	case msg := <-recv:
		want := "line one\nline two"
		if msg != want {
			t.Fatalf("server received %q, want %q", msg, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received the block")
	}
	if got := a.client().Engine.CurrentMode(); got == "line" {
		t.Fatalf("CurrentMode() = %q — block input must not be interpreted as a command", got)
	}
}

// TestSend_InteriorNewlinePlusTrailingNewlineStillRoutesAsBlock checks that
// trimming only strips trailing terminators before the routing decision: an
// interior newline surviving that trim still means "block", even when the
// input also happens to end with one.
func TestSend_InteriorNewlinePlusTrailingNewlineStillRoutesAsBlock(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	a.Send("line one\nline two\n")

	select {
	case msg := <-recv:
		want := "line one\nline two"
		if msg != want {
			t.Fatalf("server received %q, want %q", msg, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received the block")
	}
}

func TestSendInput_ExpandsVariablesAndDoubleSemicolon(t *testing.T) {
	a, recv := newSendRoutingApp(t)
	a.client().SetInputVariables(map[string]string{"target": "scarred bandit"})

	if err := a.SendInput("kill ${target};;look;;inventory"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}

	var receivedAt []time.Time
	for _, want := range []string{"kill scarred bandit", "look", "inventory"} {
		select {
		case got := <-recv:
			receivedAt = append(receivedAt, time.Now())
			if got != want {
				t.Fatalf("server received %q, want %q", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("server never received %q", want)
		}
	}
	for i := 1; i < len(receivedAt); i++ {
		if gap := receivedAt[i].Sub(receivedAt[i-1]); gap < client.InputCommandDelay-50*time.Millisecond {
			t.Fatalf("command gap %d = %s, want approximately %s or longer", i, gap, client.InputCommandDelay)
		} else {
			t.Logf("observed ;; command gap %d: %s", i, gap)
		}
	}
}

func TestSendInput_ActionModeCommandValidatesAndPassesExpandedArgs(t *testing.T) {
	a, recv := newSendRoutingApp(t)
	a.client().SetInputVariables(map[string]string{
		"mode":   "AGGRO",
		"target": "scarred bandit",
	})

	if err := a.SendInput("/mode ${mode} ${target}"); err != nil {
		t.Fatalf("SendInput(valid /mode): %v", err)
	}
	if got := a.client().Engine.CurrentMode(); got != "aggro" {
		t.Fatalf("CurrentMode() = %q, want canonical %q", got, "aggro")
	}
	queued, _, ok := a.client().Engine.Queue().DequeueGen()
	if !ok || queued.Command != "arg scarred" {
		// Slash-command args follow shell-style whitespace splitting, so the mode
		// receives "scarred" and "bandit" as separate arguments.
		t.Fatalf("queued mode output = %+v, %v; want first arg %q", queued, ok, "arg scarred")
	}
	select {
	case got := <-recv:
		t.Fatalf("server received local Action-set command %q", got)
	case <-time.After(200 * time.Millisecond):
	}

	if err := a.SendInput("/mode missing target"); err == nil || !strings.Contains(err.Error(), `unknown mode "missing"`) {
		t.Fatalf("SendInput(unknown /mode) error = %v, want unknown-mode error", err)
	}
	if got := a.client().Engine.CurrentMode(); got != "aggro" {
		t.Fatalf("invalid Action-set mode changed CurrentMode() to %q", got)
	}
}

func TestSendInput_ActionChainValidatesModeBeforeSendingAnything(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	err := a.SendInput("look;;/mode missing")
	if err == nil || !strings.Contains(err.Error(), `unknown mode "missing"`) {
		t.Fatalf("SendInput error = %v, want unknown-mode error", err)
	}
	select {
	case got := <-recv:
		t.Fatalf("server received %q before the later /mode failed validation", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSendInput_UsesCurrentVariablesOnEveryCall(t *testing.T) {
	a, recv := newSendRoutingApp(t)
	a.client().SetInputVariables(map[string]string{"target": "first bandit"})
	if err := a.SendInput("attack ${target}"); err != nil {
		t.Fatalf("first SendInput: %v", err)
	}
	a.client().SetInputVariables(map[string]string{"target": "second bandit"})
	if err := a.SendInput("attack ${target}"); err != nil {
		t.Fatalf("second SendInput: %v", err)
	}

	for _, want := range []string{"attack first bandit", "attack second bandit"} {
		select {
		case got := <-recv:
			if got != want {
				t.Fatalf("server received %q, want %q", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("server never received %q", want)
		}
	}
}

func TestSendInput_ValidationIsAtomic(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	err := a.SendInput("look;;kill ${missing}")
	if err == nil {
		t.Fatal("SendInput returned nil, want unknown-variable error")
	}
	select {
	case got := <-recv:
		t.Fatalf("server received %q before the later command failed validation", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSendInput_MultilineBlockExpandsVariablesWithoutChains(t *testing.T) {
	a, recv := newSendRoutingApp(t)
	a.client().SetInputVariables(map[string]string{"target": "scarred bandit"})

	input := "say ${target};;look\nsecond line ${target}&&wait"
	want := "say scarred bandit;;look\nsecond line scarred bandit&&wait"
	if err := a.SendInput(input); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	select {
	case got := <-recv:
		if got != want {
			t.Fatalf("server received %q, want expanded block %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received multiline block")
	}
}

func TestSendInput_MultilineVariableValidationIsAtomic(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	err := a.SendInput("first line\nkill ${missing}")
	if err == nil {
		t.Fatal("SendInput returned nil, want unknown-variable error")
	}
	select {
	case got := <-recv:
		t.Fatalf("server received %q before multiline validation failed", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestStartFileSend_ExpandsVariablesWithoutChains(t *testing.T) {
	a, recv := newSendRoutingApp(t)
	a.client().SetInputVariables(map[string]string{"target": "scarred bandit"})

	path := writeScript(t, "say ${target};;look\nsecond line ${target}&&wait\n")
	if err := a.StartFileSend(path); err != nil {
		t.Fatalf("StartFileSend: %v", err)
	}
	select {
	case got := <-recv:
		want := "say scarred bandit;;look\nsecond line scarred bandit&&wait"
		if got != want {
			t.Fatalf("server received %q, want expanded file block %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received /send block")
	}
}

func TestStartFileSend_RejectsUnknownVariableBeforeSending(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	path := writeScript(t, "first line\nkill ${missing}\n")
	err := a.StartFileSend(path)
	if err == nil {
		t.Fatal("StartFileSend returned nil, want unknown-variable error")
	}
	if a.sendActive() {
		t.Fatal("send became active after variable validation failed")
	}
	select {
	case got := <-recv:
		t.Fatalf("server received %q before /send variable validation failed", got)
	case <-time.After(200 * time.Millisecond):
	}
}
