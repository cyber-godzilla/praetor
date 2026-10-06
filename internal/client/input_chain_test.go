package client

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func receiveCommand(t *testing.T, received <-chan string, want string) {
	t.Helper()
	select {
	case got := <-received:
		if got != want {
			t.Fatalf("server received %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("server never received %q", want)
	}
}

func receiveNoCommand(t *testing.T, received <-chan string, wait time.Duration) {
	t.Helper()
	select {
	case got := <-received:
		t.Fatalf("server unexpectedly received %q", got)
	case <-time.After(wait):
	}
}

func receiveCommandAt(t *testing.T, received <-chan string, want string) time.Time {
	t.Helper()
	select {
	case got := <-received:
		if got != want {
			t.Fatalf("server received %q, want %q", got, want)
		}
		return time.Now()
	case <-time.After(2 * time.Second):
		t.Fatalf("server never received %q", want)
		return time.Time{}
	}
}

func waitForInputChainInactive(t *testing.T, c *Client) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for c.InputChainActive() {
		if time.Now().After(deadline) {
			t.Fatal("input chain remained active after its final command was sent")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestInputUnbusyMessagesMatchPraetorScriptsSet(t *testing.T) {
	for _, text := range []string{
		"You are no longer busy.",
		"You are no longer stunned.",
		"You wield a gladius.",
		"You grab onto the wagon.",
		"You are already wielding that.",
		"You successfully train to rank 501.",
		"You stop walking.",
	} {
		if !isInputUnbusy(text) {
			t.Errorf("isInputUnbusy(%q) = false", text)
		}
	}
	if isInputUnbusy("You are still busy.") {
		t.Fatal("unrelated busy text matched the unbusy set")
	}
}

func TestClient_SendInput_EachRecognizedUnbusyMessageAdvancesAChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetUnbusyDelay(0)

	messages := []string{
		"You are no longer busy.",
		"You are no longer stunned.",
		"You wield a gladius.",
		"You grab onto the wagon.",
		"You are already wielding that.",
		"You successfully train to rank 501.",
		"You stop walking.",
	}
	for i, message := range messages {
		first := fmt.Sprintf("first-%d", i)
		second := fmt.Sprintf("second-%d", i)
		if err := c.SendInput(first + "&&" + second); err != nil {
			t.Fatalf("SendInput for %q: %v", message, err)
		}
		receiveCommand(t, received, first)
		c.processLine(message)
		receiveCommand(t, received, second)
	}
}

func TestNewClientUsesConfiguredChainDelays(t *testing.T) {
	cfg := config.Defaults()
	cfg.Commands.SemicolonDelayMS = 1250
	cfg.Commands.UnbusyDelayMS = 275
	c, err := NewClient(cfg, nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)

	if got := c.SemicolonDelay(); got != 1250*time.Millisecond {
		t.Fatalf("SemicolonDelay() = %s, want 1.25s from config", got)
	}
	if got := c.UnbusyDelay(); got != 275*time.Millisecond {
		t.Fatalf("UnbusyDelay() = %s, want 275ms from config", got)
	}
}

func TestClient_SendInput_DoubleAmpersandWaitsForUnbusy(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	if err := c.SendInput("stand&&climb wall&&look"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "stand")
	if !c.InputChainActive() {
		t.Fatal("InputChainActive = false with && commands queued")
	}
	receiveNoCommand(t, received, 150*time.Millisecond)

	c.processLine("You are still busy.")
	receiveNoCommand(t, received, 150*time.Millisecond)

	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "climb wall")
	receiveNoCommand(t, received, 150*time.Millisecond)

	c.processLine("You wield a gladius in your right hand.")
	receiveCommand(t, received, "look")
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after the final command was sent")
	}
}

func TestClient_InputChainStatusAdvancesAcrossDoubleAmpersandWaits(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetUnbusyDelay(0)

	if err := c.SendInput("stand&&climb wall&&look"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "stand")
	status := c.InputChainStatus()
	if !status.Active || status.Step != 2 || status.Total != 3 || status.State != "unbusy" {
		t.Fatalf("status after first send = %#v, want step 2/3 waiting for unbusy", status)
	}

	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "climb wall")
	status = c.InputChainStatus()
	if !status.Active || status.Step != 3 || status.Total != 3 || status.State != "unbusy" {
		t.Fatalf("status after second send = %#v, want step 3/3 waiting for unbusy", status)
	}

	c.processLine("You stop walking.")
	receiveCommand(t, received, "look")
	waitForInputChainInactive(t, c)
}

func TestClient_AbortInputChainsCancelsQueuedCommands(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "paced", input: "first;;second"},
		{name: "unbusy", input: "first&&second"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, wsURL, received := newRecordingServer(t)
			defer srv.Close()
			c := newDiscTestClient(t)
			connectTestSession(t, c, wsURL)

			if err := c.SendInput(tc.input); err != nil {
				t.Fatalf("SendInput: %v", err)
			}
			receiveCommand(t, received, "first")
			if !c.InputChainActive() {
				t.Fatal("InputChainActive = false with a command queued")
			}
			if got := c.AbortInputChains(); got != 1 {
				t.Fatalf("AbortInputChains() = %d, want 1", got)
			}
			if c.InputChainActive() {
				t.Fatal("InputChainActive = true after abort")
			}

			// An unbusy line must not revive an && continuation, and waiting past
			// the pacing interval must not revive a ;; continuation.
			c.processLine("You are no longer busy.")
			receiveNoCommand(t, received, InputCommandDelay+150*time.Millisecond)
		})
	}
}

func TestClient_SendInput_SingleCommandNeverCreatesActiveChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	if err := c.SendInput("look"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "look")
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after a single command")
	}
}

func TestClient_SendInput_UnbusyAdvancesOnePendingChainFIFO(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	if err := c.SendInput("first&&first-next"); err != nil {
		t.Fatalf("first SendInput: %v", err)
	}
	if err := c.SendInput("second&&second-next"); err != nil {
		t.Fatalf("second SendInput: %v", err)
	}
	receiveCommand(t, received, "first")
	receiveCommand(t, received, "second")

	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "first-next")
	receiveNoCommand(t, received, 150*time.Millisecond)

	c.processLine("You are no longer stunned.")
	receiveCommand(t, received, "second-next")
}

func TestClient_SendInput_MixesUnbusyAndPacedSeparators(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	if err := c.SendInput("stand&&climb;;look"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "stand")
	c.processLine("You grab onto the rope.")
	receiveCommand(t, received, "climb")
	receiveNoCommand(t, received, InputCommandDelay-100*time.Millisecond)
	receiveCommand(t, received, "look")
}

func TestClient_SendInput_UsesConfiguredSemicolonDelay(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	c.SetSemicolonDelay(150 * time.Millisecond)
	started := time.Now()
	if err := c.SendInput("first;;second"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "first")
	// A setting change affects future submissions, not the timing contract of
	// a chain that is already running.
	c.SetSemicolonDelay(700 * time.Millisecond)
	receiveNoCommand(t, received, 100*time.Millisecond)
	receiveCommand(t, received, "second")

	if elapsed := time.Since(started); elapsed < 125*time.Millisecond || elapsed > 550*time.Millisecond {
		t.Fatalf("configured ;; delay took %s, want approximately 150ms", elapsed)
	}
	if got := c.SemicolonDelay(); got != 700*time.Millisecond {
		t.Fatalf("SemicolonDelay() = %s, want 700ms for the next chain", got)
	}
}

func TestClient_SendInput_UsesConfiguredUnbusyDelay(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	c.SetUnbusyDelay(150 * time.Millisecond)
	if err := c.SendInput("first&&second"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "first")
	// A setting change affects future submissions, not the timing contract of
	// a chain that is already running.
	c.SetUnbusyDelay(700 * time.Millisecond)
	started := time.Now()
	c.processLine("You are no longer busy.")
	receiveNoCommand(t, received, 100*time.Millisecond)
	receiveCommand(t, received, "second")

	if elapsed := time.Since(started); elapsed < 125*time.Millisecond || elapsed > 550*time.Millisecond {
		t.Fatalf("configured && response delay took %s, want approximately 150ms", elapsed)
	}
	if got := c.UnbusyDelay(); got != 700*time.Millisecond {
		t.Fatalf("UnbusyDelay() = %s, want 700ms for the next chain", got)
	}
}

func TestClient_SendInput_CollectsRepeatedTimingSamples(t *testing.T) {
	const (
		samples = 5
		delay   = 80 * time.Millisecond
		minimum = 65 * time.Millisecond
	)

	t.Run("semicolon", func(t *testing.T) {
		srv, wsURL, received := newRecordingServer(t)
		defer srv.Close()
		c := newDiscTestClient(t)
		connectTestSession(t, c, wsURL)
		c.SetSemicolonDelay(delay)

		for i := range samples {
			first := fmt.Sprintf("paced-first-%d", i)
			second := fmt.Sprintf("paced-second-%d", i)
			if err := c.SendInput(first + ";;" + second); err != nil {
				t.Fatalf("sample %d SendInput: %v", i, err)
			}
			started := receiveCommandAt(t, received, first)
			finished := receiveCommandAt(t, received, second)
			if elapsed := finished.Sub(started); elapsed < minimum {
				t.Fatalf("sample %d ;; delay = %s, want at least %s", i, elapsed, minimum)
			}
		}
	})

	t.Run("unbusy", func(t *testing.T) {
		srv, wsURL, received := newRecordingServer(t)
		defer srv.Close()
		c := newDiscTestClient(t)
		connectTestSession(t, c, wsURL)
		c.SetUnbusyDelay(delay)

		for i := range samples {
			first := fmt.Sprintf("unbusy-first-%d", i)
			second := fmt.Sprintf("unbusy-second-%d", i)
			if err := c.SendInput(first + "&&" + second); err != nil {
				t.Fatalf("sample %d SendInput: %v", i, err)
			}
			receiveCommand(t, received, first)
			started := time.Now()
			c.processLine("You are no longer busy.")
			finished := receiveCommandAt(t, received, second)
			if elapsed := finished.Sub(started); elapsed < minimum {
				t.Fatalf("sample %d && delay = %s, want at least %s", i, elapsed, minimum)
			}
		}
	})

	t.Run("repeat", func(t *testing.T) {
		srv, wsURL, received := newRecordingServer(t)
		defer srv.Close()
		c := newDiscTestClient(t)
		connectTestSession(t, c, wsURL)
		c.SetUnbusyDelay(delay)

		for i := range samples {
			command := fmt.Sprintf("repeat-%d", i)
			success := fmt.Sprintf("done-%d", i)
			input := fmt.Sprintf(`$(repeat %q until %q)`, command, success)
			if err := c.SendInput(input); err != nil {
				t.Fatalf("sample %d SendInput: %v", i, err)
			}
			receiveCommand(t, received, command)
			started := time.Now()
			c.processLine("You are no longer busy.")
			finished := receiveCommandAt(t, received, command)
			if elapsed := finished.Sub(started); elapsed < minimum {
				t.Fatalf("sample %d repeat delay = %s, want at least %s", i, elapsed, minimum)
			}
			c.processLine(success)
			if c.InputChainActive() {
				t.Fatalf("sample %d repeat remained active after success", i)
			}
		}
	})
}

func TestClient_AbortInputChainsCancelsPostUnbusyDelay(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetUnbusyDelay(250 * time.Millisecond)

	if err := c.SendInput("first&&second"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "first")
	c.processLine("You are no longer busy.")
	if got := c.AbortInputChains(); got != 1 {
		t.Fatalf("AbortInputChains() = %d, want 1", got)
	}
	receiveNoCommand(t, received, 350*time.Millisecond)
}

func TestClient_SendInput_UnbusyChainDoesNotCrossReconnect(t *testing.T) {
	srvA, urlA, recvA := newRecordingServer(t)
	defer srvA.Close()
	srvB, urlB, recvB := newRecordingServer(t)
	defer srvB.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, urlA)
	if err := c.SendInput("first&&stale-second"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, recvA, "first")

	next := session.New()
	if err := next.Connect(urlB, nil); err != nil {
		t.Fatalf("connect B: %v", err)
	}
	c.setSession(next)
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after replacing the session")
	}
	c.processLine("You are no longer busy.")

	receiveNoCommand(t, recvA, 200*time.Millisecond)
	receiveNoCommand(t, recvB, 200*time.Millisecond)
}

func TestClient_SendInput_WaitDirectiveDelaysAndCanBeStopped(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(10 * time.Millisecond)

	started := time.Now()
	if err := c.SendInput(`$(wait 0.15);;look`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	if !c.InputChainActive() {
		t.Fatal("InputChainActive = false during explicit wait")
	}
	receiveNoCommand(t, received, 100*time.Millisecond)
	receiveCommand(t, received, "look")
	if elapsed := time.Since(started); elapsed < 140*time.Millisecond {
		t.Fatalf("wait completed after %s, want at least 150ms (allowing timer margin)", elapsed)
	}
	waitForInputChainInactive(t, c)

	if err := c.SendInput(`$(wait 0.2);;stale`); err != nil {
		t.Fatalf("SendInput(second wait): %v", err)
	}
	if got := c.AbortInputChains(); got != 1 {
		t.Fatalf("AbortInputChains() = %d, want 1", got)
	}
	receiveNoCommand(t, received, 300*time.Millisecond)
}

func TestClient_SendInput_ExplicitWaitAddsToSurroundingPacing(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(60 * time.Millisecond)

	if err := c.SendInput(`first;;$(wait 0.08);;second`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	started := receiveCommandAt(t, received, "first")
	finished := receiveCommandAt(t, received, "second")
	// Two 60ms separators plus the explicit 80ms wait should total about 200ms.
	if elapsed := finished.Sub(started); elapsed < 170*time.Millisecond {
		t.Fatalf("combined pacing took %s, want at least 170ms", elapsed)
	}
}

func TestClient_SendInput_WaitForDirectiveAdvancesOnFutureSubstring(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(10 * time.Millisecond)

	// Text observed before the directive begins must not be retained as a cue.
	c.processLine("The latch clicks in the old scene.")
	if err := c.SendInput(`$(wait-for "The latch clicks");;open door`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveNoCommand(t, received, 100*time.Millisecond)
	c.processLine("From nearby, The latch clicks softly.")
	receiveCommand(t, received, "open door")
	// The recording server can observe the write just before SendCommand returns
	// and the chain goroutine performs its final bookkeeping. Wait for that
	// bounded cleanup instead of making receipt of the socket write a barrier.
	waitForInputChainInactive(t, c)
}

func TestClient_SendInput_WaitForRemainsActiveUntilMatchedOrStopped(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	if err := c.SendInput(`$(wait-for "a cue that never arrives");;stale`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveNoCommand(t, received, 250*time.Millisecond)
	if !c.InputChainActive() {
		t.Fatal("wait-for timed out or completed without a matching future line")
	}
	if got := c.AbortInputChains(); got != 1 {
		t.Fatalf("AbortInputChains() = %d, want 1", got)
	}
}

func TestClient_SendInput_WaitForIsCaseSensitive(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)

	if err := c.SendInput(`$(wait-for "Ready");;look`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	c.processLine("ready")
	receiveNoCommand(t, received, 100*time.Millisecond)
	c.processLine("Ready")
	receiveCommand(t, received, "look")
}

func TestClient_SendInput_WaitForTimeoutCancelsWholeChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)

	if err := c.SendInput(`$(wait-for "never" timeout 0.05);;stale`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	select {
	case event := <-c.Events():
		got, ok := event.(types.ErrorEvent)
		if !ok || got.Context != "PraetorScript" || got.Err == nil || !strings.Contains(got.Err.Error(), "timed out") {
			t.Fatalf("timeout event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("wait-for timeout did not emit an error")
	}
	receiveNoCommand(t, received, 100*time.Millisecond)
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after wait-for timeout")
	}
}

func TestClient_SendInput_WaitForCancelStopsWholeChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	if err := c.SendInput(`$(wait-for "open" cancel-on "locked" timeout 1);;stale`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	c.processLine("The gate is locked.")
	receiveNoCommand(t, received, 100*time.Millisecond)
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after wait-for cancellation text")
	}
}

func TestClient_SendInput_OneLineCanCompleteMultipleWaitForReactions(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)

	if err := c.SendInput(`$(wait-for "ready");;first`); err != nil {
		t.Fatalf("first SendInput: %v", err)
	}
	if err := c.SendInput(`$(wait-for "ready");;second`); err != nil {
		t.Fatalf("second SendInput: %v", err)
	}
	c.processLine("both are ready now")
	got := map[string]bool{}
	for range 2 {
		select {
		case command := <-received:
			got[command] = true
		case <-time.After(2 * time.Second):
			t.Fatal("server did not receive both wait-for continuations")
		}
	}
	if !got["first"] || !got["second"] || len(got) != 2 {
		t.Fatalf("server received %#v, want first and second in either order", got)
	}
	waitForInputChainInactive(t, c)
}

func TestClient_SendInput_WaitForAfterCommandArmsBeforeSend(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(10 * time.Millisecond)

	if err := c.SendInput(`search chest;;$(wait-for "You find a key");;get key`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "search chest")
	// This arrives well before the normal ;; delay. The wait-for must already
	// be live or a fast server response would be lost.
	c.processLine("You find a key in the chest.")
	receiveCommand(t, received, "get key")
}

func TestClient_SendInput_NotifyDirectiveUsesUserNotificationPath(t *testing.T) {
	c := newDiscTestClient(t)
	// Explicit user notifications are allowed even when Lua/script notifications
	// are disabled; only the shared sound preference applies.
	c.SetScriptNotificationPreferences(false, true)
	desktop := make(chan struct {
		title   string
		message string
		sound   bool
	}, 1)
	c.desktopNotify = func(title, message string, sound bool) {
		desktop <- struct {
			title   string
			message string
			sound   bool
		}{title, message, sound}
	}
	if err := c.SendInput(`$(notify "Training" "Complete")`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	select {
	case got := <-desktop:
		if got.title != "Training" || got.message != "Complete" || !got.sound {
			t.Fatalf("desktop notification = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("desktop notification was not sent")
	}
	select {
	case event := <-c.Events():
		got, ok := event.(types.NotificationEvent)
		if !ok || got.Title != "Training" || got.Message != "Complete" {
			t.Fatalf("notification event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("GUI notification event was not emitted")
	}
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after notify-only chain completed")
	}
}

func TestClient_SendInput_NotifyUsesCurrentSoundPreference(t *testing.T) {
	c := newDiscTestClient(t)
	desktop := make(chan struct {
		message string
		sound   bool
	}, 2)
	c.desktopNotify = func(_ string, message string, sound bool) {
		desktop <- struct {
			message string
			sound   bool
		}{message, sound}
	}

	for _, tc := range []struct {
		message string
		sound   bool
	}{
		{message: "silent", sound: false},
		{message: "audible", sound: true},
	} {
		c.SetScriptNotificationPreferences(false, tc.sound)
		if err := c.SendInput(fmt.Sprintf(`$(notify %q)`, tc.message)); err != nil {
			t.Fatalf("SendInput(%q): %v", tc.message, err)
		}
		select {
		case got := <-desktop:
			if got.message != tc.message || got.sound != tc.sound {
				t.Fatalf("desktop notification = %#v, want message %q sound %v", got, tc.message, tc.sound)
			}
		case <-time.After(time.Second):
			t.Fatalf("desktop notification %q was not sent", tc.message)
		}
		select {
		case <-c.Events():
		case <-time.After(time.Second):
			t.Fatalf("GUI notification %q was not emitted", tc.message)
		}
	}
}

func TestClient_AbortInputChainsCancelsWaitForAndRepeat(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		initial string
		lines   []string
	}{
		{
			name:  "wait-for",
			input: `$(wait-for "ready");;stale`,
			lines: []string{"ready"},
		},
		{
			name:    "repeat",
			input:   `$(repeat "climb" until "done");;stale`,
			initial: "climb",
			lines:   []string{"You are no longer busy.", "done"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, wsURL, received := newRecordingServer(t)
			defer srv.Close()
			c := newDiscTestClient(t)
			connectTestSession(t, c, wsURL)
			c.SetSemicolonDelay(0)
			c.SetUnbusyDelay(0)

			if err := c.SendInput(tc.input); err != nil {
				t.Fatalf("SendInput: %v", err)
			}
			if tc.initial != "" {
				receiveCommand(t, received, tc.initial)
			}
			if got := c.AbortInputChains(); got != 1 {
				t.Fatalf("AbortInputChains() = %d, want 1", got)
			}
			for _, line := range tc.lines {
				c.processLine(line)
			}
			receiveNoCommand(t, received, 150*time.Millisecond)
			if c.InputChainActive() {
				t.Fatal("InputChainActive = true after abort")
			}
		})
	}
}

func TestClient_AbortInputChainsCancelsMixedConcurrentWork(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(100 * time.Millisecond)
	c.SetUnbusyDelay(0)

	inputs := []string{
		"paced;;paced-next",
		"unbusy&&unbusy-next",
		`$(wait 5);;wait-next`,
		`$(wait-for "cue");;reaction-next`,
		`$(repeat "repeat" until "done");;repeat-next`,
	}
	for _, input := range inputs {
		if err := c.SendInput(input); err != nil {
			t.Fatalf("SendInput(%q): %v", input, err)
		}
	}

	initial := map[string]bool{}
	for range 3 {
		select {
		case command := <-received:
			initial[command] = true
		case <-time.After(2 * time.Second):
			t.Fatal("server did not receive all immediate commands")
		}
	}
	for _, want := range []string{"paced", "unbusy", "repeat"} {
		if !initial[want] {
			t.Fatalf("immediate commands = %#v, missing %q", initial, want)
		}
	}
	if got := c.AbortInputChains(); got != len(inputs) {
		t.Fatalf("AbortInputChains() = %d, want %d", got, len(inputs))
	}
	c.processLine("You are no longer busy.")
	c.processLine("cue")
	c.processLine("done")
	receiveNoCommand(t, received, 250*time.Millisecond)
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after aborting all mixed work")
	}
}

func TestClient_SendInput_RepeatRetriesUntilSuccessThenAdvances(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(10 * time.Millisecond)
	c.SetUnbusyDelay(150 * time.Millisecond)

	input := `$(repeat "climb wall" until "You reach the top" cancel-on "You fall");;look`
	if err := c.SendInput(input); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "climb wall")
	c.processLine("You are no longer busy.")
	receiveNoCommand(t, received, 100*time.Millisecond)
	receiveCommand(t, received, "climb wall")
	c.processLine("At last, You reach the top safely.")
	receiveCommand(t, received, "look")

	c.processLine("You are no longer busy.")
	receiveNoCommand(t, received, 100*time.Millisecond)
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after repeat succeeded and continuation completed")
	}
}

func TestClient_SendInput_RepeatCancelStopsWholeChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(10 * time.Millisecond)

	input := `$(repeat "climb wall" until "You reach the top" cancel-on "You fall");;look`
	if err := c.SendInput(input); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "climb wall")
	c.processLine("You fall to the ground.")
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after cancel substring")
	}
	c.processLine("You are no longer busy.")
	c.processLine("You reach the top.")
	receiveNoCommand(t, received, 150*time.Millisecond)
}

func TestClient_SendInput_RepeatMaxAttemptsCancelsWholeChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)
	c.SetUnbusyDelay(0)

	if err := c.SendInput(`$(repeat "climb" until "done" max 2);;stale`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "climb")
	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "climb")
	c.processLine("You are no longer busy.")
	receiveNoCommand(t, received, 100*time.Millisecond)
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-c.Events():
			got, ok := event.(types.ErrorEvent)
			if !ok {
				continue // command echoes share this channel
			}
			if got.Context != "PraetorScript" || got.Err == nil || !strings.Contains(got.Err.Error(), "maximum of 2 attempts") {
				t.Fatalf("max-attempt event = %#v", event)
			}
			goto gotError
		case <-deadline:
			t.Fatal("repeat max did not emit an error")
		}
	}
gotError:
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after repeat reached max attempts")
	}
}

func TestClient_SendInput_RepeatCountSendsExactlyThenAdvances(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)
	c.SetUnbusyDelay(0)

	if err := c.SendInput(`$(repeat "search" count 3);;look`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "search")
	status := c.InputChainStatus()
	if status.State != "repeat" || status.Attempts != 1 || status.MaxAttempts != 3 {
		t.Fatalf("initial count status = %#v", status)
	}

	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "search")
	status = c.InputChainStatus()
	if status.Attempts != 2 || status.MaxAttempts != 3 {
		t.Fatalf("second count status = %#v", status)
	}

	c.processLine("You stop walking.")
	receiveCommand(t, received, "search")
	receiveNoCommand(t, received, 50*time.Millisecond)
	if !c.InputChainActive() {
		t.Fatal("count repeat completed before the final attempt became unbusy")
	}

	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "look")
	waitForInputChainInactive(t, c)

	c.processLine("You are no longer busy.")
	receiveNoCommand(t, received, 100*time.Millisecond)
}

func TestClient_SendInput_RepeatCountCancelStopsWholeChain(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)
	c.SetUnbusyDelay(0)

	if err := c.SendInput(`$(repeat "search" count 3 cancel-on "You find nothing");;look`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "search")
	c.processLine("You find nothing of interest.")
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after count repeat cancellation")
	}
	c.processLine("You are no longer busy.")
	receiveNoCommand(t, received, 100*time.Millisecond)
}

func TestClient_InputChainStatusTracksWaitAndRepeat(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetUnbusyDelay(0)

	if err := c.SendInput(`$(wait-for "ready" timeout 2);;look`); err != nil {
		t.Fatalf("SendInput(wait-for): %v", err)
	}
	status := c.InputChainStatus()
	if !status.Active || status.Chains != 1 || status.Step != 1 || status.Total != 2 || status.State != "wait-for" || status.Detail != "ready" {
		t.Fatalf("wait-for status = %#v", status)
	}
	if status.DurationMS != 2000 || status.RemainingMS <= 0 || status.RemainingMS > 2000 {
		t.Fatalf("wait-for timing = duration %dms, remaining %dms", status.DurationMS, status.RemainingMS)
	}
	c.processLine("ready")
	receiveCommand(t, received, "look")

	if err := c.SendInput(`$(repeat "climb" until "done" max 3)`); err != nil {
		t.Fatalf("SendInput(repeat): %v", err)
	}
	receiveCommand(t, received, "climb")
	status = c.InputChainStatus()
	if status.State != "repeat" || status.Detail != "climb" || status.Attempts != 1 || status.MaxAttempts != 3 {
		t.Fatalf("initial repeat status = %#v", status)
	}
	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "climb")
	status = c.InputChainStatus()
	if status.Attempts != 2 || status.MaxAttempts != 3 {
		t.Fatalf("retried repeat status = %#v", status)
	}
	c.processLine("done")
	if status = c.InputChainStatus(); status.Active {
		t.Fatalf("finished status = %#v", status)
	}
}

func TestClient_InputChainStatusKeepsOriginalPacingDuration(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(200 * time.Millisecond)

	if err := c.SendInput("first;;second"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "first")
	first := c.InputChainStatus()
	time.Sleep(60 * time.Millisecond)
	second := c.InputChainStatus()
	if first.State != "pacing" || first.DurationMS != 200 || second.DurationMS != 200 {
		t.Fatalf("pacing statuses = first %#v, second %#v", first, second)
	}
	if second.RemainingMS >= first.RemainingMS {
		t.Fatalf("remaining time did not advance internally: first %dms, second %dms", first.RemainingMS, second.RemainingMS)
	}
	receiveCommand(t, received, "second")
}

func TestClient_InputChainStatusCountsConcurrentChainsAndShowsOldest(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)

	if err := c.SendInput(`$(wait-for "first");;one`); err != nil {
		t.Fatalf("SendInput(first): %v", err)
	}
	if err := c.SendInput(`$(wait-for "second");;two`); err != nil {
		t.Fatalf("SendInput(second): %v", err)
	}
	status := c.InputChainStatus()
	if status.Chains != 2 || status.Detail != "first" {
		t.Fatalf("concurrent status = %#v, want oldest of two chains", status)
	}
	c.processLine("first")
	receiveCommand(t, received, "one")
	status = c.InputChainStatus()
	if status.Chains != 1 || status.Detail != "second" {
		t.Fatalf("remaining status = %#v, want second chain", status)
	}
	c.processLine("second")
	receiveCommand(t, received, "two")
}

func TestClient_SendInput_RepeatCancelWinsWhenLineAlsoMatchesSuccess(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)

	input := `$(repeat "climb" until "done" cancel-on "cancel");;stale`
	if err := c.SendInput(input); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "climb")
	c.processLine("cancel and done")
	receiveNoCommand(t, received, 150*time.Millisecond)
	if c.InputChainActive() {
		t.Fatal("InputChainActive = true after cancel substring won")
	}
}

func TestClient_SendInput_RepeatSharesUnbusyFIFOWithDoubleAmpersand(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetUnbusyDelay(0)

	if err := c.SendInput(`$(repeat "climb wall" until "You reach the top")`); err != nil {
		t.Fatalf("SendInput(repeat): %v", err)
	}
	if err := c.SendInput(`stand&&look`); err != nil {
		t.Fatalf("SendInput(chain): %v", err)
	}
	receiveCommand(t, received, "climb wall")
	receiveCommand(t, received, "stand")

	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "climb wall")
	receiveNoCommand(t, received, 100*time.Millisecond)

	c.processLine("You reach the top.")
	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "look")
}

func TestClient_SendInput_RepeatRejectsLocalCommandBeforeAnythingSends(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)

	for _, input := range []string{
		`look;;$(repeat "/help" until "done")`,
		`look;;$(repeat "/help" count 2)`,
	} {
		if err := c.SendInput(input); err == nil {
			t.Fatalf("SendInput(%q) returned nil for repeat with local command", input)
		}
	}
	receiveNoCommand(t, received, 150*time.Millisecond)
}

func TestClient_SendInput_ComposesVariablesWaitRepeatNotifyAndPacing(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetInputVariables(map[string]string{"target": "wall"})
	c.SetSemicolonDelay(10 * time.Millisecond)
	desktop := make(chan string, 1)
	c.desktopNotify = func(_ string, message string, _ bool) { desktop <- message }

	input := `prepare ${target};;$(wait 0.03);;$(repeat "climb ${target}" until "at the top");;$(notify "Finished ${target}");;look`
	if err := c.SendInput(input); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	receiveCommand(t, received, "prepare wall")
	receiveCommand(t, received, "climb wall")
	c.processLine("You arrive at the top.")
	select {
	case got := <-desktop:
		if got != "Finished wall" {
			t.Fatalf("desktop message = %q, want %q", got, "Finished wall")
		}
	case <-time.After(time.Second):
		t.Fatal("combined chain did not emit its notification")
	}
	receiveCommand(t, received, "look")
	waitForInputChainInactive(t, c)
}

func TestClient_SendInput_RepeatedShortChainsDoNotLeakStateOrDuplicateCommands(t *testing.T) {
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(time.Millisecond)

	for i := range 20 {
		first := fmt.Sprintf("first-%d", i)
		second := fmt.Sprintf("second-%d", i)
		if err := c.SendInput(first + ";;" + second); err != nil {
			t.Fatalf("chain %d SendInput: %v", i, err)
		}
		receiveCommand(t, received, first)
		receiveCommand(t, received, second)
		waitForInputChainInactive(t, c)
	}
	receiveNoCommand(t, received, 50*time.Millisecond)
}

func TestClient_SendInput_ReactionsDoNotCrossReconnect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		initial string
		lines   []string
	}{
		{
			name:  "wait-for",
			input: `$(wait-for "cue");;stale-next`,
			lines: []string{"cue"},
		},
		{
			name:    "repeat",
			input:   `$(repeat "climb wall" until "done");;stale-next`,
			initial: "climb wall",
			lines:   []string{"You are no longer busy.", "done"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srvA, urlA, recvA := newRecordingServer(t)
			defer srvA.Close()
			srvB, urlB, recvB := newRecordingServer(t)
			defer srvB.Close()

			c := newDiscTestClient(t)
			connectTestSession(t, c, urlA)
			if err := c.SendInput(tc.input); err != nil {
				t.Fatalf("SendInput: %v", err)
			}
			if tc.initial != "" {
				receiveCommand(t, recvA, tc.initial)
			}

			next := session.New()
			if err := next.Connect(urlB, nil); err != nil {
				t.Fatalf("connect B: %v", err)
			}
			c.setSession(next)
			if c.InputChainActive() {
				t.Fatal("InputChainActive = true after replacing the session")
			}
			for _, line := range tc.lines {
				c.processLine(line)
			}

			receiveNoCommand(t, recvA, 200*time.Millisecond)
			receiveNoCommand(t, recvB, 200*time.Millisecond)
		})
	}
}
