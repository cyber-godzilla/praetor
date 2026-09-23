package client

import (
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/session"
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

func TestInputUnbusyMessagesMatchPraetorScriptsSet(t *testing.T) {
	for _, text := range []string{
		"You are no longer busy.",
		"You are no longer stunned.",
		"You wield a gladius.",
		"You grab onto the wagon.",
		"You are already wielding that.",
		"You successfully train to rank 501.",
	} {
		if !isInputUnbusy(text) {
			t.Errorf("isInputUnbusy(%q) = false", text)
		}
	}
	if isInputUnbusy("You are still busy.") {
		t.Fatal("unrelated busy text matched the unbusy set")
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
