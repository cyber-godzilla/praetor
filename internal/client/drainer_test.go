package client

import (
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// The single-drainer redesign guarantees: an enqueued command drains on an idle
// connection (no incoming game text needed), sends keep FIFO order across mixed
// delays, min-interval pacing holds, and a mode switch recalls a command a
// drainer is still sleeping on. These are exercised end-to-end through Run().

func TestClient_Drainer_SendsOnIdleConnection(t *testing.T) {
	srv, wsURL, recv := newRecordingServer(t)
	defer srv.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	go c.Run()
	waitForConnected(t, c)

	// No incoming server text and no manual drain trigger: the drainer must wake
	// on the enqueue itself.
	c.Engine.Queue().Enqueue("wave", 1)

	select {
	case cmd := <-recv:
		if cmd != "wave" {
			t.Fatalf("got %q, want wave", cmd)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("queued command was never drained on an idle connection")
	}
	c.Disconnect()
}

func TestClient_Drainer_PreservesOrderAcrossDelays(t *testing.T) {
	srv, wsURL, recv := newRecordingServer(t)
	defer srv.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	go c.Run()
	waitForConnected(t, c)

	// "first" has a longer delay than "second", which is enqueued after it. A
	// single sequential drainer must still send them in enqueue order.
	c.Engine.Queue().Enqueue("first", 200)
	c.Engine.Queue().Enqueue("second", 1)

	got := make([]string, 0, 2)
	for len(got) < 2 {
		select {
		case cmd := <-recv:
			got = append(got, cmd)
		case <-time.After(3 * time.Second):
			t.Fatalf("only received %v before timeout", got)
		}
	}
	if got[0] != "first" || got[1] != "second" {
		t.Fatalf("send order = %v, want [first second]", got)
	}
	c.Disconnect()
}

func TestClient_Drainer_EnforcesMinInterval(t *testing.T) {
	srv, wsURL, recv := newRecordingServer(t)
	defer srv.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	go c.Run()
	waitForConnected(t, c)

	min := c.Engine.Queue().MinInterval()

	c.Engine.Queue().Enqueue("a", 1)
	c.Engine.Queue().Enqueue("b", 1)
	c.Engine.Queue().Enqueue("c", 1)

	times := make([]time.Time, 0, 3)
	for len(times) < 3 {
		select {
		case <-recv:
			times = append(times, time.Now())
		case <-time.After(3 * time.Second):
			t.Fatalf("only received %d commands before timeout", len(times))
		}
	}

	// Allow a little scheduling slop below the configured minimum.
	slop := min / 5
	if gap := times[1].Sub(times[0]); gap < min-slop {
		t.Fatalf("gap a->b = %v, want >= %v", gap, min-slop)
	}
	if gap := times[2].Sub(times[1]); gap < min-slop {
		t.Fatalf("gap b->c = %v, want >= %v", gap, min-slop)
	}
	c.Disconnect()
}

func TestClient_Drainer_DropsCommandRetiredByModeSwitch(t *testing.T) {
	srv, wsURL, recv := newRecordingServer(t)
	defer srv.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, wsURL)
	go c.Run()
	waitForConnected(t, c)

	// The drainer pops "stale" and parks on its long delay.
	c.Engine.Queue().Enqueue("stale", 300)
	time.Sleep(80 * time.Millisecond)

	// A mode switch clears the queue and advances the generation; the sleeping
	// command must be recalled, not sent.
	c.Engine.SetMode("disable", nil)

	// A command enqueued after the switch proves the drainer is alive and lets us
	// assert ordering: the sentinel must arrive and "stale" must never precede it.
	c.Engine.Queue().Enqueue("sentinel", 1)

	select {
	case cmd := <-recv:
		if cmd != "sentinel" {
			t.Fatalf("first send = %q, want sentinel (stale command leaked past a mode switch)", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainer did not send the post-switch sentinel")
	}
	c.Disconnect()
}

// A text flood must not cost a DisconnectedEvent: dropping it would leave the UI
// rendering a live-looking session forever. Lifecycle events are guaranteed even
// when the droppable bulk buffer is saturated.
func TestClient_Emit_GuaranteesDisconnectedUnderFlood(t *testing.T) {
	c := newDiscTestClient(t)

	// Saturate the events buffer with droppable bulk events, no consumer.
	for i := 0; i < 300; i++ {
		c.emit(types.GameTextEvent{Text: "flood"})
	}

	sent := make(chan struct{})
	go func() {
		c.emit(types.DisconnectedEvent{Reason: "dropped"})
		close(sent)
	}()

	// Let the goroutine attempt its emit while the buffer is full and unconsumed:
	// a droppable emit would return (and close sent) here; a guaranteed one blocks.
	time.Sleep(100 * time.Millisecond)

	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-c.Events():
			if _, ok := ev.(types.DisconnectedEvent); ok {
				<-sent
				return
			}
		case <-deadline:
			t.Fatal("DisconnectedEvent was dropped under a bulk-event flood")
		}
	}
}

// After a disconnect, commands the still-running engine queues while offline must
// not be transmitted onto the next login, and mode changes must still reach the
// UI on the reconnected session (proving the fresh listener/drainer took over).
func TestClient_Reconnect_ClearsOfflineQueueAndKeepsModeChanges(t *testing.T) {
	srvA, urlA := newDiscServer(t, make(chan struct{}))
	defer srvA.Close()

	c := newDiscTestClient(t)
	connectTestSession(t, c, urlA)
	go c.Run()
	waitForConnected(t, c)

	c.Disconnect()
	waitForDisconnected(t, c)

	// The engine (still alive by design) queues a command while offline.
	c.Engine.Queue().Enqueue("offline-cmd", 1)

	// Reconnect to a fresh recording server.
	srvB, urlB, recvB := newRecordingServer(t)
	defer srvB.Close()
	nb := session.New()
	if err := nb.Connect(urlB, nil); err != nil {
		t.Fatalf("connect B: %v", err)
	}
	c.setSession(nb)
	go c.Run()
	waitForConnected(t, c)

	// The offline command must have been dropped on connect, not sent to B.
	select {
	case cmd := <-recvB:
		t.Fatalf("server B received offline-queued command %q; it leaked across the reconnect", cmd)
	case <-time.After(400 * time.Millisecond):
		// expected: nothing leaks
	}

	// A mode change on the new connection must still reach the UI, and its
	// on-switch send must reach server B — proving the fresh listener + drainer
	// are wired to the reconnected session.
	c.Engine.Queue().Enqueue("post-reconnect", 1)
	select {
	case cmd := <-recvB:
		if cmd != "post-reconnect" {
			t.Fatalf("server B received %q, want post-reconnect", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainer on the reconnected session never sent")
	}

	// The mode-change listener on the new connection is live.
	c.Engine.SetMode("disable", nil)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-c.Events():
			if _, ok := ev.(types.ModeChangeEvent); ok {
				c.Disconnect()
				return
			}
		case <-deadline:
			t.Fatal("no ModeChangeEvent after reconnect; listener did not follow the new session")
		}
	}
}
