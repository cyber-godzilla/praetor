package gui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/engine"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/gorilla/websocket"
)

// playTestApp returns an app whose sends are captured and whose waits complete
// immediately, so playback order is asserted without sleeping.
func playTestApp(t *testing.T) (*GuiApp, chan string) {
	t.Helper()
	a := newTestApp(t)
	sent := make(chan string, 64)
	a.playSend = func(s string) error { sent <- s; return nil }
	a.playAfter = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Time{}
		return ch
	}
	a.playRand = func(min, max time.Duration) time.Duration { return min }
	// Tests in this package build GuiApp via newTestApp, which leaves deps.Client
	// nil, so the real pre-flight would refuse every performance as disconnected.
	// Default the seam to "pass" here; tests that specifically want to exercise
	// the pre-flight override it themselves.
	a.playCheck = func() error { return nil }
	return a, sent
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scene.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

var playTestUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// newConnectedTestSession spins up a local WebSocket server and returns a real
// session.Session already Connect()-ed to it. This exists so playPreflight's
// !IsConnected() branch can be exercised through the ACTUAL Session type
// (session.New() alone would already report disconnected, which only covers
// the nil/never-dialed case, not "was dialed and IsConnected() says so").
func newConnectedTestSession(t *testing.T) *session.Session {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := playTestUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Keep the connection open (reading and discarding client frames, e.g.
		// the ident line and pings) until the test tears it down.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	s := session.New()
	t.Cleanup(s.Close)
	if err := s.Connect(wsURL, nil); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return s
}

// The nil-client test above only exercises playPreflight's outermost
// nil-safety short-circuit. These three tests drive the REAL function (no
// playCheck stub) through its two substantive refusal branches plus the
// idle-mode pass-through, so a future refactor that inverts or drops either
// condition breaks a test instead of shipping silently.

func TestPlayPreflight_DisconnectedSessionRefuses(t *testing.T) {
	a := newTestApp(t)
	// A real, never-Connect()-ed Session: IsConnected() is false for it, the
	// same state production sees before login.
	a.deps.Client = &client.Client{Session: session.New()}

	err := a.playPreflight()
	if err == nil {
		t.Fatal("playPreflight returned nil for a disconnected session — want an error")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error = %q, want it to mention being disconnected (so a reword can't silently swap this with the mode refusal)", err.Error())
	}
}

func TestPlayPreflight_ActiveModeRefusesAndNamesIt(t *testing.T) {
	sess := newConnectedTestSession(t)

	eng, err := engine.NewEngine(nil, nil, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(eng.Close)
	eng.SetMode("attack", nil)

	a := newTestApp(t)
	a.deps.Client = &client.Client{Session: sess, Engine: eng}

	err = a.playPreflight()
	if err == nil {
		t.Fatal("playPreflight returned nil while a non-idle mode is running — want an error")
	}
	if !strings.Contains(err.Error(), "attack") {
		t.Errorf("error = %q, want it to name the active mode %q", err.Error(), "attack")
	}
}

func TestPlayPreflight_IdleModesAreAccepted(t *testing.T) {
	for _, idle := range []string{"", "disable"} {
		t.Run(fmt.Sprintf("mode=%q", idle), func(t *testing.T) {
			sess := newConnectedTestSession(t)

			eng, err := engine.NewEngine(nil, nil, "")
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			t.Cleanup(eng.Close)
			if idle != "" {
				eng.SetMode(idle, nil)
			}
			if got := eng.CurrentMode(); got != idle {
				t.Fatalf("CurrentMode() = %q, want %q", got, idle)
			}

			a := newTestApp(t)
			a.deps.Client = &client.Client{Session: sess, Engine: eng}

			if err := a.playPreflight(); err != nil {
				t.Errorf("playPreflight() = %v, want nil for idle mode %q", err, idle)
			}
		})
	}
}

// TestPlayPreflight_RefusesWhileSendActive is the Fix-1 regression test for
// the final review's interleaving finding: playPreflight checked connectivity
// and the Lua mode but never whether a /send was in flight, so starting a
// /send then a /play let both drivers write to the socket at once (the
// reviewer's SEND:s1 PLAY:p1..p5 SEND:s2 reproduction). This drives the REAL
// playPreflight (no playCheck stub) so a future refactor that drops the
// sendActive() check breaks this test instead of shipping silently.
func TestPlayPreflight_RefusesWhileSendActive(t *testing.T) {
	sess := newConnectedTestSession(t)
	eng, err := engine.NewEngine(nil, nil, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(eng.Close)

	a := newTestApp(t)
	a.deps.Client = &client.Client{Session: sess, Engine: eng}

	// Keep a send in flight: one batch dispatched, the rest blocked on release.
	release := make(chan struct{})
	a.sendOne = func(string) error { <-release; return nil }
	a.startSend([]string{"one", "two"})
	t.Cleanup(func() { close(release) })
	t.Cleanup(func() { a.AbortSend() })

	// Give the driver a moment to record itself as active before asserting.
	deadline := time.Now().Add(2 * time.Second)
	for !a.sendActive() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !a.sendActive() {
		t.Fatal("sendActive() is false after startSend — test setup didn't produce an in-flight send")
	}

	err = a.playPreflight()
	if err == nil {
		t.Fatal("playPreflight returned nil while a /send is in flight — want an error")
	}
	if !strings.Contains(err.Error(), "send") {
		t.Errorf("error = %q, want it to mention the in-flight send", err.Error())
	}
}

func TestStartPlay_SendsTextInOrderAndSkipsNotesAndComments(t *testing.T) {
	a, sent := playTestApp(t)
	path := writeScript(t, "# comment\nfirst\n%note:local only\n%wait:5s\nsecond\n")

	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	for _, want := range []string{"first", "second"} {
		select {
		case got := <-sent:
			if got != want {
				t.Fatalf("sent %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
	select {
	case extra := <-sent:
		t.Fatalf("sent extra line %q — comments and notes must not be sent", extra)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPausePlay_HoldsAndResumeReRunsTheInstruction(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	sent := make(chan string, 8)
	a.playSend = func(s string) error { sent <- s; return nil }
	a.playRand = func(min, max time.Duration) time.Duration { return min }

	// A wait long enough that the test controls when it would finish.
	waits := make(chan chan time.Time, 4)
	a.playAfter = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		waits <- ch
		return ch
	}

	path := writeScript(t, "%wait:30s\nafter the wait\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	<-waits // the first wait is now pending
	if !a.PausePlay() {
		t.Fatal("PausePlay returned false while playing")
	}
	select {
	case s := <-sent:
		t.Fatalf("sent %q while paused", s)
	case <-time.After(100 * time.Millisecond):
	}

	if !a.ResumePlay() {
		t.Fatal("ResumePlay returned false while paused")
	}
	// Resume must re-run the interrupted %wait, i.e. request a NEW timer.
	select {
	case ch := <-waits:
		ch <- time.Time{} // let the re-run wait elapse
	case <-time.After(2 * time.Second):
		t.Fatal("resume did not re-run the interrupted %wait")
	}

	select {
	case got := <-sent:
		if got != "after the wait" {
			t.Fatalf("sent %q, want %q", got, "after the wait")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("playback did not continue after resume")
	}
}

func TestStopPlay_DropsStateAndHalts(t *testing.T) {
	a, sent := playTestApp(t)
	waits := make(chan chan time.Time, 4)
	a.playAfter = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		waits <- ch
		return ch
	}
	path := writeScript(t, "%wait:30s\nnever sent\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}
	<-waits

	if !a.StopPlay() {
		t.Fatal("StopPlay returned false while playing")
	}
	if a.PlayActive() {
		t.Error("PlayActive is true after StopPlay")
	}
	if a.ResumePlay() {
		t.Error("ResumePlay succeeded after StopPlay — state must be dropped")
	}
	select {
	case s := <-sent:
		t.Fatalf("sent %q after stop", s)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestWaitKey_HoldsUntilNext(t *testing.T) {
	a, sent := playTestApp(t)
	path := writeScript(t, "%wait-key\nafter the key\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	select {
	case s := <-sent:
		t.Fatalf("sent %q before /next", s)
	case <-time.After(150 * time.Millisecond):
	}

	if !a.NextPlayStep() {
		t.Fatal("NextPlayStep returned false while holding on %wait-key")
	}
	select {
	case got := <-sent:
		if got != "after the key" {
			t.Fatalf("sent %q, want %q", got, "after the key")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("playback did not continue after /next")
	}
}

func TestStartPlay_RefusesInvalidScript(t *testing.T) {
	a, _ := playTestApp(t)
	path := writeScript(t, "%bogus:1s\ngood line\n")
	if err := a.StartPlay(path); err == nil {
		t.Fatal("StartPlay accepted a script with a validation error")
	}
	if a.PlayActive() {
		t.Error("PlayActive is true after refusing an invalid script")
	}
}

// TestStopPlay_HaltsBetweenConsecutiveTextSteps is a regression test for a
// defect where /stop was only observed inside a blocking step (playSleep or
// %wait-key's select). A run of plain text/%note lines with no waits between
// them never touched s.cancel at all, so StopPlay could return true and
// PlayActive could report false while the driver kept right on sending.
func TestStopPlay_HaltsBetweenConsecutiveTextSteps(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	sent := make(chan string, 8)
	proceed := make(chan struct{})
	a.playSend = func(s string) error {
		sent <- s
		<-proceed // hold this send "in flight" until the test releases it
		return nil
	}

	// Five plain text lines, no wait steps between them.
	path := writeScript(t, "one\ntwo\nthree\nfour\nfive\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	// Wait for the first line to be in flight (dispatch started, not yet returned).
	select {
	case got := <-sent:
		if got != "one" {
			t.Fatalf("sent %q, want %q", got, "one")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first line")
	}

	// Stop while the first line is still in flight.
	if !a.StopPlay() {
		t.Fatal("StopPlay returned false while playing")
	}
	close(proceed) // let the in-flight send (and only it) return

	// The in-flight line was already committed and may complete, but no
	// further step may begin after /stop is observed.
	select {
	case extra := <-sent:
		t.Fatalf("sent %q after StopPlay — /stop must be observed before the next step begins", extra)
	case <-time.After(300 * time.Millisecond):
	}
}

// TestNextPlayStep_EarlyCallDoesNotSkipWaitKey is a regression test for a
// defect where a /next arriving before any %wait-key was reached left a
// token in the buffered `next` channel, which later silently released the
// hold without the performer actually confirming it.
func TestNextPlayStep_EarlyCallDoesNotSkipWaitKey(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	sent := make(chan string, 8)
	proceed := make(chan struct{})
	a.playSend = func(s string) error {
		sent <- s
		<-proceed
		return nil
	}

	path := writeScript(t, "line one\n%wait-key\nafter the key\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	// "line one" is in flight — well before %wait-key is reached.
	select {
	case got := <-sent:
		if got != "line one" {
			t.Fatalf("sent %q, want %q", got, "line one")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first line")
	}

	// A /next this early must not be banked for the later %wait-key.
	if !a.NextPlayStep() {
		t.Fatal("NextPlayStep returned false while a performance is running")
	}
	close(proceed) // let "line one" finish and the driver reach %wait-key

	select {
	case got := <-sent:
		t.Fatalf("sent %q — an early /next incorrectly skipped %%wait-key", got)
	case <-time.After(200 * time.Millisecond):
	}

	// A /next while actually holding on %wait-key does release it.
	if !a.NextPlayStep() {
		t.Fatal("NextPlayStep returned false while holding on %wait-key")
	}
	select {
	case got := <-sent:
		if got != "after the key" {
			t.Fatalf("sent %q, want %q", got, "after the key")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("playback did not continue after /next")
	}
}

// TestPausePlay_StaleTokenDoesNotRestartALaterWait is a regression test for a
// defect where a /pause landing during a text send (a step that never
// selects on s.pause) left a token in the buffered `pause` channel. A later
// %wait step's playSleep would consume that stale token on its first select,
// report itself as "paused mid-step", and get re-run from the top — silently
// requesting a second timer for the same step.
func TestPausePlay_StaleTokenDoesNotRestartALaterWait(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	sent := make(chan string, 8)
	proceed := make(chan struct{})
	a.playSend = func(s string) error {
		sent <- s
		<-proceed
		return nil
	}
	var timerCalls int32
	a.playAfter = func(time.Duration) <-chan time.Time {
		atomic.AddInt32(&timerCalls, 1)
		return make(chan time.Time, 1) // never fed: the step should request exactly one and then hold
	}
	a.playRand = func(min, max time.Duration) time.Duration { return min }

	path := writeScript(t, "line one\n%wait:5s\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	// "line one" is in flight.
	select {
	case got := <-sent:
		if got != "line one" {
			t.Fatalf("sent %q, want %q", got, "line one")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first line")
	}

	// Pause and resume while the text step is in flight — before any step
	// selects on s.pause, so the token is left unconsumed by the pause itself.
	if !a.PausePlay() {
		t.Fatal("PausePlay returned false while playing")
	}
	if !a.ResumePlay() {
		t.Fatal("ResumePlay returned false while paused")
	}
	close(proceed) // let "line one" finish and the driver reach %wait

	// Give the driver time to reach (and, if buggy, re-enter) the %wait step.
	time.Sleep(200 * time.Millisecond)

	if n := atomic.LoadInt32(&timerCalls); n != 1 {
		t.Fatalf("playAfter called %d time(s) for the %%wait step, want 1 — a stale /pause token restarted it", n)
	}
}

func TestPickPlayFile_ReportsPreviewAndErrors(t *testing.T) {
	a := newTestApp(t)
	path := writeScript(t, "one\n%wait:5s\n%wait-key\n%bogus:x\n")
	a.deps.Dialogs = &fakeDialogs{file: path}

	got, err := a.PickPlayFile()
	if err != nil {
		t.Fatalf("PickPlayFile: %v", err)
	}
	if len(got.Errors) != 1 || got.Errors[0].Line != 4 {
		t.Fatalf("Errors = %+v, want one error on line 4", got.Errors)
	}
	if got.Name != "scene.txt" {
		t.Errorf("Name = %q, want scene.txt", got.Name)
	}
	if !got.HasCues {
		t.Error("HasCues = false, want true (%wait-key is unbounded)")
	}
	if got.FixedMs != 5000 {
		t.Errorf("FixedMs = %d, want 5000", got.FixedMs)
	}
}

func TestWaitFor_ProceedsOnMatchingText(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	sent := make(chan string, 8)
	a.playSend = func(s string) error { sent <- s; return nil }
	// playTestApp's default playAfter fires instantly, which is correct for
	// %wait/%wait-random (those tests want no real sleeping) but wrong here:
	// it would time out this %wait-for before the cue could ever arrive, since
	// the mock ignores the requested duration entirely. Use a deadline that
	// never fires so only the cue (fed below) can complete the step.
	a.playAfter = func(time.Duration) <-chan time.Time { return make(chan time.Time) }
	a.playRand = func(min, max time.Duration) time.Duration { return min }
	path := writeScript(t, "%wait-for:Aldric draws\nafter the cue\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	select {
	case s := <-sent:
		t.Fatalf("sent %q before the cue arrived", s)
	case <-time.After(150 * time.Millisecond):
	}

	// Case differs from the pattern on purpose: matching is case-insensitive.
	a.feedPlayText("ALDRIC DRAWS his blade.")

	select {
	case got := <-sent:
		if got != "after the cue" {
			t.Fatalf("sent %q, want %q", got, "after the cue")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("playback did not continue after the cue")
	}
}

func TestWaitFor_TimeoutHaltsButKeepsState(t *testing.T) {
	a, sent := playTestApp(t)
	// playAfter fires immediately, so the cue wait times out at once.
	path := writeScript(t, "%wait-for:never happens\nnever sent\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	select {
	case s := <-sent:
		t.Fatalf("sent %q after a timed-out cue — playback must halt", s)
	case <-time.After(300 * time.Millisecond):
	}

	// State is KEPT so a late cue costs a /resume, not the whole scene.
	if !a.PlayActive() {
		t.Fatal("PlayActive is false after a %wait-for timeout — state must be kept")
	}
	if !a.ResumePlay() {
		t.Error("ResumePlay failed after a %wait-for timeout")
	}
}

func TestStartPlay_RefusesWhenPreflightFails(t *testing.T) {
	a, _ := playTestApp(t)
	a.playCheck = func() error { return fmt.Errorf("not connected — nothing was played") }

	if err := a.StartPlay(writeScript(t, "hello\n")); err == nil {
		t.Fatal("StartPlay succeeded despite a failing pre-flight check")
	}
	if a.PlayActive() {
		t.Error("PlayActive is true after a refused start")
	}
}

// The real pre-flight must be nil-safe: newTestApp leaves deps.Client nil, and a
// panic here would take down the whole event loop in production too.
func TestPlayPreflight_NilClientIsTreatedAsDisconnected(t *testing.T) {
	a := newTestApp(t)
	err := a.playPreflight()
	if err == nil {
		t.Fatal("playPreflight returned nil with no client — want a disconnected error")
	}
}

func TestStartPlay_ProceedsWhenPreflightPasses(t *testing.T) {
	a, sent := playTestApp(t)
	a.playCheck = func() error { return nil }

	if err := a.StartPlay(writeScript(t, "hello\n")); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}
	select {
	case got := <-sent:
		if got != "hello" {
			t.Fatalf("sent %q, want %q", got, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nothing was sent after a passing pre-flight")
	}
}

// TestStartPlay_SlashLineReachesGameNotLocalCommand is the fix-round-1
// regression test: a play-script line beginning "/" is ordinary StepText to
// the parser (it has no % sigil), and must reach the game VERBATIM, exactly
// like a line starting with "@". Before this fix, playDispatch's default path
// called client.SendCommand, which intercepts any leading "/" as a local
// command — "/mode aggro" fired a real mode switch mid-performance instead of
// reaching the game, and any other "/"-prefixed line was silently swallowed.
// This uses the real client/session/engine (via newSendRoutingApp, the same
// recording-WebSocket harness /send's routing tests use) specifically because
// the bug lived in real dispatch plumbing that playTestApp's playSend stub
// bypasses entirely.
// TestSend_RejectedDuringPerformance is the Fix-2 regression test from the
// final review: GuiApp.Send is the single point every input path funnels
// through — the command input, numpad navigation, the sidebar compass and
// "sizeup here", action-set buttons, the status bar's cond/lighting, vitals
// cond, and the kudos modal all call it directly, and only the command input
// had its own frontend lockout. Gating Send itself is what makes the lockout
// hold for the other seven call sites without touching seven frontend files.
// Uses the real client/session/engine (the newSendRoutingApp harness /send's
// routing tests use) so this exercises the actual wire path, not a stub.
func TestSend_RejectedDuringPerformance(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	path := writeScript(t, "%wait-key\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}
	if !a.PlayActive() {
		t.Fatal("PlayActive is false right after StartPlay")
	}

	a.Send("look")

	select {
	case msg := <-recv:
		t.Fatalf("server received %q — Send must reject input while a performance is active", msg)
	case <-time.After(300 * time.Millisecond):
	}

	// A slash command must be blocked too, not just plain text: Send must
	// return before it ever inspects the input for a leading "/".
	a.Send("/mode aggro")
	if got := a.client().Engine.CurrentMode(); got == "aggro" {
		t.Fatalf("CurrentMode() = %q — Send must not process a slash command during a performance either", got)
	}

	// Sanity check: once the performance ends, Send works again.
	if !a.StopPlay() {
		t.Fatal("StopPlay returned false")
	}
	a.Send("look")
	select {
	case msg := <-recv:
		if msg != "look" {
			t.Fatalf("server received %q, want %q", msg, "look")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send did not reach the game after the performance ended")
	}
}

// TestWaitFor_StaleCueBufferedDuringWaitKeyDoesNotSatisfyLaterWaitFor is the
// Fix-3 regression test the final review supplied a recipe for: playWaitFor
// drains text buffered before its own step began, so a cue seen earlier in
// the scene cannot instantly satisfy a %wait-for reached later. This is
// untested despite being the same defect class as three earlier Critical bugs
// in this branch (see the stale-token tests above for /pause and /next).
//
// The recipe: hold on %wait-key, feed the cue text while still holding (so it
// lands in s.text before the %wait-for step exists), release with /next, and
// confirm the stale cue does NOT complete the %wait-for — only a cue fed AFTER
// the step begins may.
func TestWaitFor_StaleCueBufferedDuringWaitKeyDoesNotSatisfyLaterWaitFor(t *testing.T) {
	a, sent := playTestApp(t)
	// Never fires: only the live cue fed below may complete the %wait-for.
	a.playAfter = func(time.Duration) <-chan time.Time { return make(chan time.Time) }

	path := writeScript(t, "%wait-key\n%wait-for:the bell tolls\nafter the cue\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	// Let the driver actually reach and start holding on %wait-key before
	// touching /next — otherwise an early /next can race runPlay's own
	// top-of-iteration token drain (see TestNextPlayStep_EarlyCallDoesNotSkipWaitKey
	// above) and get silently dropped before %wait-key ever consumes it, which
	// would hang this test rather than exercise the drain this test targets.
	select {
	case s := <-sent:
		t.Fatalf("sent %q before %%wait-key was released", s)
	case <-time.After(150 * time.Millisecond):
	}

	// Feed the cue while still holding on %wait-key — buffered BEFORE the
	// %wait-for step begins, and must be drained rather than consumed.
	a.feedPlayText("the bell tolls across the city")

	if !a.NextPlayStep() {
		t.Fatal("NextPlayStep returned false while holding on %wait-key")
	}

	select {
	case s := <-sent:
		t.Fatalf("sent %q — a cue buffered before %%wait-for began must NOT satisfy it", s)
	case <-time.After(250 * time.Millisecond):
	}

	// Now feed the cue live — this one lands after the step began and must
	// complete the wait.
	a.feedPlayText("the bell tolls across the city")
	select {
	case got := <-sent:
		if got != "after the cue" {
			t.Fatalf("sent %q, want %q", got, "after the cue")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("playback did not continue after the live cue")
	}
}

func TestStartPlay_SlashLineReachesGameNotLocalCommand(t *testing.T) {
	a, recv := newSendRoutingApp(t)

	path := writeScript(t, "/mode aggro\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	select {
	case msg := <-recv:
		if msg != "/mode aggro" {
			t.Fatalf("server received %q, want %q — script line must reach the game verbatim", msg, "/mode aggro")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received the script line — /mode must not be handled locally by a play script")
	}

	if got := a.client().Engine.CurrentMode(); got == "aggro" {
		t.Fatalf("CurrentMode() = %q — a play-script line must never fire a client command like /mode", got)
	}
}

// findNoteSegment polls the capture emitter for a wire text event carrying the
// given note text, returning its first styled segment.
func findNoteSegment(t *testing.T, a *GuiApp, want string) Segment {
	t.Helper()
	ce, ok := a.emitter.(*captureEmitter)
	if !ok {
		t.Fatalf("emitter is %T, want *captureEmitter", a.emitter)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range ce.snapshot() {
			batch, ok := e.data.([]WireEvent)
			if !ok {
				continue
			}
			for _, w := range batch {
				if w.Kind == KindText && w.Text != nil && w.Text.Text == want && len(w.Text.Segments) > 0 {
					return w.Text.Segments[0]
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no wire text event carrying the note %q ever reached the output pane", want)
	return Segment{}
}

// A %note is a stage direction for the performer: it belongs in the output pane
// in Skotos orange, and must never be transmitted to the game.
func TestNote_PrintsInSkotosOrangeAndIsNotSentToGame(t *testing.T) {
	a, sent := playTestApp(t)
	if err := a.StartPlay(writeScript(t, "%note:Reached the note\n")); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	seg := findNoteSegment(t, a, "Reached the note")
	if seg.Color != noteColor {
		t.Errorf("note color = %q, want %q (--skotos-orange)", seg.Color, noteColor)
	}
	if !seg.Italic {
		t.Error("note segment is not italic")
	}

	select {
	case s := <-sent:
		t.Fatalf("note text %q was sent to the game — it must stay local", s)
	case <-time.After(100 * time.Millisecond):
	}
}

// The input-bar indicator needs live progress: a static "performing" chip can't
// distinguish a long %wait from a wedged client, which is the confusion the
// indicator exists to end.
func TestPlayStatus_ReportsStepProgressAndPauseState(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	a.playSend = func(string) error { return nil }
	a.playRand = func(min, max time.Duration) time.Duration { return min }

	waits := make(chan chan time.Time, 4)
	a.playAfter = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		waits <- ch
		return ch
	}

	if st := a.PlayStatus(); st.Active {
		t.Fatalf("PlayStatus() = %+v with nothing playing, want inactive", st)
	}

	// The first line sends with no floor (lastSend is zero), so the first timer
	// request comes from the %wait — parking the driver on a known step.
	if err := a.StartPlay(writeScript(t, "one\n%wait:30s\ntwo\n")); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}
	select {
	case <-waits:
	case <-time.After(2 * time.Second):
		t.Fatal("driver never reached the %wait")
	}

	st := a.PlayStatus()
	if !st.Active {
		t.Error("PlayStatus().Active = false during a performance")
	}
	if st.Paused {
		t.Error("PlayStatus().Paused = true before any /pause")
	}
	if st.Step != 2 || st.Total != 3 {
		t.Errorf("PlayStatus() step %d/%d, want 2/3 (parked on the %%wait)", st.Step, st.Total)
	}

	if !a.PausePlay() {
		t.Fatal("PausePlay returned false during a performance")
	}
	if st := a.PlayStatus(); !st.Paused || !st.Active {
		t.Errorf("after /pause PlayStatus() = %+v, want active and paused", st)
	}

	if !a.StopPlay() {
		t.Fatal("StopPlay returned false during a performance")
	}
	if st := a.PlayStatus(); st.Active {
		t.Errorf("after /stop PlayStatus() = %+v, want inactive", st)
	}
}

// The game enforces a minimum gap between commands, so the send floor is
// measured from the last SEND, never from the last step. Two consequences the
// script author relies on, both pinned here with real timing:
//
//   - command, %note, command — the note cannot shrink the gap below
//     min_interval. The note's own beat counts toward that floor rather than
//     stacking on top of it.
//   - %wait:1s, %note, command — an explicit wait that already exceeds the
//     floor adds nothing further. The author's timing wins.
func TestSendFloor_IsMeasuredFromTheLastSendNotTheLastStep(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }

	stamps := make(chan time.Time, 8)
	a.playSend = func(string) error { stamps <- time.Now(); return nil }
	// Deliberately NOT stubbing playAfter: this test is about real elapsed time.

	floor := a.cfg().Commands.MinInterval.Duration
	if floor != 500*time.Millisecond {
		t.Fatalf("test assumes the 500ms default min_interval, got %s", floor)
	}

	if err := a.StartPlay(writeScript(t,
		"one\n%note:absorbed by the floor\ntwo\n%wait:1s\n%note:after an explicit wait\nthree\n")); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	at := make([]time.Time, 0, 3)
	for len(at) < 3 {
		select {
		case ts := <-stamps:
			at = append(at, ts)
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of 3 commands were dispatched", len(at))
		}
	}

	// one -> two: only a note between them, so the floor governs. If the note's
	// beat were ever made additive, this would jump to roughly 1s.
	if gap := at[1].Sub(at[0]); gap < 400*time.Millisecond || gap > 900*time.Millisecond {
		t.Errorf("command->note->command gap = %s, want ~%s (the min_interval floor)", gap, floor)
	}

	// two -> three: %wait:1s plus the note's 500ms beat, and no floor on top
	// because the gap already exceeds it. ~1.5s, not ~2s.
	if gap := at[2].Sub(at[1]); gap < 1350*time.Millisecond || gap > 1950*time.Millisecond {
		t.Errorf("wait+note gap = %s, want ~1.5s (1s wait + 500ms note beat, no extra floor)", gap)
	}
}

// A %note holds a beat rather than passing instantly: without one, a scripted
// scene reads as though its waits were being ignored.
func TestNote_HoldsABeat(t *testing.T) {
	a := newTestApp(t)
	a.playCheck = func() error { return nil }
	a.playSend = func(string) error { return nil }
	a.playRand = func(min, max time.Duration) time.Duration { return min }

	durs := make(chan time.Duration, 8)
	a.playAfter = func(d time.Duration) <-chan time.Time {
		durs <- d
		ch := make(chan time.Time, 1)
		ch <- time.Time{}
		return ch
	}

	if err := a.StartPlay(writeScript(t, "%note:beat\n")); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	select {
	case d := <-durs:
		if d != playNoteBeat {
			t.Errorf("note beat = %s, want %s", d, playNoteBeat)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("%note completed without holding a beat")
	}
}

func TestStartPlay_DoesNotExpandInputVariables(t *testing.T) {
	a, recv := newSendRoutingApp(t)
	a.client().SetInputVariables(map[string]string{"target": "scarred bandit"})

	path := writeScript(t, "say ${target}\n")
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}

	select {
	case got := <-recv:
		if got != "say ${target}" {
			t.Fatalf("server received %q, want play-script text unchanged", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received play-script text")
	}
}
