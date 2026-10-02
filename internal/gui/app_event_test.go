package gui

import (
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

type blockingEmitter struct {
	entered chan struct{}
	release chan struct{}
}

func (e *blockingEmitter) Emit(string, any) {
	select {
	case e.entered <- struct{}{}:
	default:
	}
	<-e.release
}

func TestEventIntakeDoesNotBlockWhenEmitterStalls(t *testing.T) {
	previousLogWriter := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(previousLogWriter)
	cfg := config.Defaults()
	c, err := client.NewClient(cfg, nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)
	emitter := &blockingEmitter{entered: make(chan struct{}, 1), release: make(chan struct{})}
	a := NewGuiApp(&Deps{
		Client:        c,
		Config:        cfg,
		DesktopNotify: client.NewDesktopNotifier(cfg.Notifications.Desktop),
	}, emitter)
	a.eventLimit = 128 // force overflow without producing thousands of log lines
	a.Start()

	// The first echo reaches the native bridge and blocks there.
	c.SendCommand("first")
	select {
	case <-emitter.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("event processor never reached the blocking emitter")
	}

	// More than Client.Events' 256-slot capacity must still be accepted while
	// the processor is blocked. Before the split intake/processing loops this
	// producer wedged once that channel filled.
	produced := make(chan struct{})
	go func() {
		defer close(produced)
		for i := 0; i < 600; i++ {
			c.SendCommand(fmt.Sprintf("queued-%d", i))
		}
	}()
	select {
	case <-produced:
	case <-time.After(2 * time.Second):
		t.Fatal("client producer blocked behind a stalled GUI emitter")
	}
	a.eventMu.Lock()
	pending := len(a.eventQueue)
	a.eventMu.Unlock()
	if pending > a.eventLimit {
		t.Fatalf("display backlog grew to %d events, limit %d", pending, a.eventLimit)
	}

	close(emitter.release)
	a.Shutdown()
	a.Shutdown() // idempotent teardown is required by Wails' two callbacks.
}

func TestEventBacklogStaysBoundedWithProtectedEventAtHead(t *testing.T) {
	previousLogWriter := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(previousLogWriter)
	cfg := config.Defaults()
	c, err := client.NewClient(cfg, nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)
	emitter := &blockingEmitter{entered: make(chan struct{}, 1), release: make(chan struct{})}
	a := NewGuiApp(&Deps{
		Client:        c,
		Config:        cfg,
		DesktopNotify: client.NewDesktopNotifier(cfg.Notifications.Desktop),
	}, emitter)
	a.eventLimit = 128
	a.Start()

	// Block the processor in the native bridge, then put a protected menu event
	// at the head of the intake queue before the render-only flood arrives.
	c.SendCommand("first")
	select {
	case <-emitter.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("event processor never reached the blocking emitter")
	}
	c.SendCommand("/wiki")
	deadline := time.Now().Add(2 * time.Second)
	for {
		a.eventMu.Lock()
		ready := len(a.eventQueue) > 0 && !isBulkGUIEvent(a.eventQueue[0])
		a.eventMu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("protected event did not reach the head of the display queue")
		}
		time.Sleep(time.Millisecond)
	}

	for i := 0; i < 600; i++ {
		c.SendCommand(fmt.Sprintf("queued-%d", i))
	}
	deadline = time.Now().Add(2 * time.Second)
	for c.PendingEvents() > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	a.eventMu.Lock()
	pending := len(a.eventQueue)
	protectedHead := pending > 0 && !isBulkGUIEvent(a.eventQueue[0])
	a.eventMu.Unlock()
	if !protectedHead {
		t.Fatal("protected head event was discarded under display pressure")
	}
	if pending > a.eventLimit {
		t.Fatalf("display backlog grew to %d events behind a protected head, limit %d", pending, a.eventLimit)
	}

	close(emitter.release)
	a.Shutdown()
}

func TestEventBacklogStaysBoundedWithOnlyProtectedEvents(t *testing.T) {
	const limit = 8
	queue := make([]types.Event, limit)
	for i := range queue {
		queue[i] = types.WikiOpenMenuEvent{}
	}
	queue, dropped := enqueueGUIEvent(queue, types.DisconnectedEvent{}, limit)
	if !dropped {
		t.Fatal("full protected-event queue did not report an eviction")
	}
	if len(queue) != limit {
		t.Fatalf("protected-event queue grew to %d, limit %d", len(queue), limit)
	}
	if _, ok := queue[len(queue)-1].(types.DisconnectedEvent); !ok {
		t.Fatalf("newest protected event was not retained: %T", queue[len(queue)-1])
	}
}

func TestShutdownJoinsActiveClientRun(t *testing.T) {
	cfg := config.Defaults()
	c, err := client.NewClient(cfg, nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)
	c.Session = newConnectedTestSession(t)
	emitter := &captureEmitter{}
	a := NewGuiApp(&Deps{
		Client:        c,
		Config:        cfg,
		DesktopNotify: client.NewDesktopNotifier(cfg.Notifications.Desktop),
	}, emitter)
	a.Start()
	a.runWG.Add(1)
	go func() {
		defer a.runWG.Done()
		c.Run()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for len(emitter.snapshot()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(emitter.snapshot()) == 0 {
		t.Fatal("client Run loop did not emit its connected event")
	}

	done := make(chan struct{})
	go func() {
		a.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not join the active client Run loop")
	}
	if c.Session.IsConnected() {
		t.Fatal("session remains connected after Shutdown")
	}
}

func TestStartAndShutdownAreRaceSafe(t *testing.T) {
	for i := 0; i < 25; i++ {
		cfg := config.Defaults()
		c, err := client.NewClient(cfg, nil, t.TempDir(), nil)
		if err != nil {
			t.Fatalf("iteration %d NewClient: %v", i, err)
		}
		a := NewGuiApp(&Deps{Client: c, Config: cfg}, &captureEmitter{})
		start := make(chan struct{})
		done := make(chan struct{}, 2)
		go func() { <-start; a.Start(); done <- struct{}{} }()
		go func() { <-start; a.Shutdown(); done <- struct{}{} }()
		close(start)
		<-done
		<-done
		c.Engine.Close()
	}
}
