package gui

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/colorwords"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/engine"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// GuiApp is the Wails-bound application facade. Its exported methods are
// callable from the frontend; it pushes game events to the frontend via the
// Emitter. It holds no Wails types, so it is fully unit testable.
type GuiApp struct {
	deps    *Deps
	render  *renderer
	emitter Emitter

	mu               sync.Mutex
	started          bool
	closing          bool
	kudosPromptShown bool

	// The intake loop must stay cheaper than the socket/core producer: it only
	// appends to this queue and wakes the processing loop. Rendering, session
	// logging, notification matching, and the Wails bridge all happen on the
	// processing loop, so a slow webview cannot back up the client's event channel
	// and ultimately stop the WebSocket reader from consuming pong frames.
	eventMu        sync.Mutex
	eventQueue     []types.Event
	eventWake      chan struct{}
	eventStop      chan struct{}
	eventWG        sync.WaitGroup
	runWG          sync.WaitGroup
	stopOnce       sync.Once
	stopping       atomic.Bool
	eventWarned    atomic.Bool
	eventLimit     int
	overflowWarned atomic.Bool

	// activityMu makes producer preflights and installation atomic. Wails invokes
	// bound methods concurrently, so checking play/send/chain state without this
	// reservation permits two producers to pass their checks together.
	activityMu sync.Mutex

	// colorWords is read on the per-line hot path in the event loop and
	// written by SetColorWords, so it is atomic to avoid a data race.
	colorWords atomic.Bool
	// kudosQueueAtConnect snapshots the queued-kudos count for the current
	// connection. It is refreshed on ConnectedEvent under mu so the event loop
	// can check it without racing Wails config mutations.
	kudosQueueAtConnect int

	// sendMu guards the in-flight /send driver. sendCancel is non-nil exactly
	// while a send is running; closing it stops the driver before its next batch.
	sendMu     sync.Mutex
	sendCancel chan struct{}
	sendDone   chan struct{}
	// sendOne overrides batch dispatch in tests; nil means send for real.
	sendOne func(string) error

	// playMu guards the in-flight /play session. play is non-nil exactly while a
	// performance is running or paused.
	playMu sync.Mutex
	play   *playSession
	// playDone remains non-nil until the driver goroutine has returned, even
	// after StopPlay removes the visible play state. New producers wait for it so
	// they cannot overlap an already-committed final socket write.
	playDone chan struct{}
	// Test seams; nil means use the real implementation.
	playSend  func(string) error
	playAfter func(time.Duration) <-chan time.Time
	playRand  func(min, max time.Duration) time.Duration
	// playCheck overrides the pre-flight guard in tests; nil means the real check.
	playCheck func() error
}

// NewGuiApp constructs the facade around bootstrapped Deps and an Emitter.
func NewGuiApp(deps *Deps, emitter Emitter) *GuiApp {
	r := newRenderer()
	r.setScale(deps.Config.UI.MinimapScale)
	a := &GuiApp{
		deps:                deps,
		render:              r,
		emitter:             emitter,
		kudosQueueAtConnect: len(deps.Config.Kudos.Queue),
		eventWake:           make(chan struct{}, 1),
		eventStop:           make(chan struct{}),
		eventLimit:          guiEventBacklogLimit(deps.Config.UI.Scrollback),
	}
	a.colorWords.Store(deps.Config.UI.ColorWords)
	return a
}

func guiEventBacklogLimit(scrollback int) int {
	// Keep one extra burst beyond retained scrollback. An unlimited/very large
	// configured history still gets a safety ceiling so a wedged webview cannot
	// consume memory without bound.
	if scrollback <= 0 || scrollback > 20_000 {
		scrollback = 20_000
	}
	limit := scrollback + 1024
	if limit < 4096 {
		limit = 4096
	}
	return limit
}

// client is a convenience accessor.
func (a *GuiApp) client() *client.Client { return a.deps.Client }
func (a *GuiApp) cfg() *config.Config    { return a.deps.Config }

// emit forwards a batch of wire events to the frontend on the single ordered
// channel. A nil/empty batch is a no-op.
func (a *GuiApp) emit(batch []WireEvent) {
	if len(batch) == 0 || a.emitter == nil || a.stopping.Load() {
		return
	}
	a.emitter.Emit(EventChannel, batch)
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Start begins draining the client event stream. The frontend calls this once
// it has registered its EventsOn listener, avoiding a race where early events
// are emitted before anyone is subscribed. Safe to call once; repeat calls
// are ignored.
func (a *GuiApp) Start() {
	a.mu.Lock()
	if a.started || a.closing {
		a.mu.Unlock()
		return
	}
	a.started = true
	a.eventWG.Add(2)
	a.mu.Unlock()

	go func() {
		defer a.eventWG.Done()
		a.eventLoop()
	}()
	go func() {
		defer a.eventWG.Done()
		a.eventProcessLoop()
	}()
}

// eventLoop is deliberately an intake-only loop. It must never perform disk,
// rendering, notification, or native-webview work: blocking this loop can fill
// Client.Events and propagate backpressure all the way to WebSocket ReadMessage.
func (a *GuiApp) eventLoop() {
	events := a.client().Events()
	for {
		select {
		case <-a.eventStop:
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			a.handleCoreEventSideEffects(event)
			a.eventMu.Lock()
			var dropped bool
			a.eventQueue, dropped = enqueueGUIEvent(a.eventQueue, event, a.eventLimit)
			if dropped && a.overflowWarned.CompareAndSwap(false, true) {
				log.Printf("[GUI] display backlog exceeded %d events; dropping render-only events while preserving logs and script reactions", a.eventLimit)
			}
			pending := len(a.eventQueue)
			a.eventMu.Unlock()
			if pending >= 2048 && a.eventWarned.CompareAndSwap(false, true) {
				log.Printf("[GUI] event backlog reached %d items; webview processing is falling behind", pending)
			}
			select {
			case a.eventWake <- struct{}{}:
			default:
			}
		}
	}
}

// enqueueGUIEvent keeps the display queue bounded without letting a protected
// lifecycle/notification event at the head pin every render-only event behind
// it. Once full, incoming bulk traffic is discarded in O(1); a rare protected
// event may evict the oldest bulk event so it can still reach the frontend. In
// the pathological case where every queued event is protected, retain the most
// recent bounded window; side effects already ran before this display queue.
func enqueueGUIEvent(queue []types.Event, event types.Event, limit int) ([]types.Event, bool) {
	if limit <= 0 || len(queue) < limit {
		return append(queue, event), false
	}
	if isBulkGUIEvent(event) {
		return queue, true
	}
	for i, queued := range queue {
		if !isBulkGUIEvent(queued) {
			continue
		}
		copy(queue[i:], queue[i+1:])
		clear(queue[len(queue)-1:])
		queue = queue[:len(queue)-1]
		return append(queue, event), true
	}
	copy(queue, queue[1:])
	queue[len(queue)-1] = event
	return queue, true
}

func isBulkGUIEvent(event types.Event) bool {
	switch event.(type) {
	case types.GameTextEvent, types.SuppressedGameTextEvent, types.StatusUpdateEvent,
		types.SKOOTUpdateEvent, types.MapURLEvent:
		return true
	default:
		return false
	}
}

// handleCoreEventSideEffects runs before the bounded display queue. None of
// these operations waits for the native webview: session logging is queued,
// play cue delivery is non-blocking, and desktop notifications launch outside
// the caller. This preserves behavior even if old render-only events are shed.
func (a *GuiApp) handleCoreEventSideEffects(event types.Event) {
	switch e := event.(type) {
	case types.GameTextEvent:
		if a.deps.SessionLog != nil {
			a.deps.SessionLog.Log(e.Timestamp, e.Text)
		}
		if a.stopping.Load() {
			return
		}
		if a.deps.DesktopNotify != nil {
			a.deps.DesktopNotify.CheckText(e.Text)
		}
		if !e.IsEcho {
			a.feedPlayText(e.Text)
		}
	case types.SKOOTUpdateEvent:
		if a.stopping.Load() || a.deps.DesktopNotify == nil {
			return
		}
		if e.Health != nil {
			a.deps.DesktopNotify.CheckHealth(*e.Health)
		}
		if e.Fatigue != nil {
			a.deps.DesktopNotify.CheckFatigue(*e.Fatigue)
		}
	case types.ModeChangeEvent:
		if !a.stopping.Load() && a.deps.DesktopNotify != nil {
			a.deps.DesktopNotify.Prune()
		}
	}
}

// eventProcessLoop drains bounded-size batches so a large burst cannot create
// one enormous Wails payload. eventQueue itself is allowed to absorb a transient
// GUI stall; the frontend's configured scrollback remains the retention bound.
func (a *GuiApp) eventProcessLoop() {
	const maxBatch = 512
	for {
		select {
		case <-a.eventStop:
			return
		case <-a.eventWake:
		}

		for {
			a.eventMu.Lock()
			n := len(a.eventQueue)
			if n == 0 {
				a.eventMu.Unlock()
				break
			}
			if n > maxBatch {
				n = maxBatch
			}
			batch := append([]types.Event(nil), a.eventQueue[:n]...)
			clear(a.eventQueue[:n])
			a.eventQueue = a.eventQueue[n:]
			if len(a.eventQueue) == 0 {
				a.eventQueue = nil
			}
			a.eventMu.Unlock()
			a.processBatch(batch)
			if a.eventWarned.Load() {
				a.eventMu.Lock()
				pending := len(a.eventQueue)
				a.eventMu.Unlock()
				if pending < 512 {
					a.eventWarned.Store(false)
				}
			}
		}
	}
}

// Shutdown synchronously stops every producer that can outlive the native
// window. It is safe to call more than once. The emitter must be disabled by the
// platform shell before this method runs, so an in-flight native call is joined
// before Wails destroys the webview.
func (a *GuiApp) Shutdown() {
	a.stopOnce.Do(func() {
		a.mu.Lock()
		a.closing = true
		a.mu.Unlock()
		a.stopping.Store(true)

		a.activityMu.Lock()
		a.StopPlay()
		a.waitForStoppedPlay()
		a.abortSendAndWait()
		a.AbortInputChains()
		a.client().Disconnect()
		a.activityMu.Unlock()

		// Keep the intake loop alive until Client.Run has completed. Its final
		// lifecycle event is guaranteed and could otherwise block on a full channel.
		a.runWG.Wait()
		// Run's final event may be buffered in Client.Events. Give the intake loop
		// ownership of every accepted event before stopping it; anything the
		// processing loop has not completed is synchronously drained below.
		deadline := time.Now().Add(2 * time.Second)
		for a.client().PendingEvents() > 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		close(a.eventStop)
		a.eventWG.Wait()
		a.eventMu.Lock()
		remaining := append([]types.Event(nil), a.eventQueue...)
		a.eventQueue = nil
		a.eventMu.Unlock()
		if len(remaining) > 0 {
			a.processBatch(remaining)
		}
	})
}

// processBatch runs side effects and converts a batch of core events into
// wire events, then emits them.
func (a *GuiApp) processBatch(batch []types.Event) {
	wire := make([]WireEvent, 0, len(batch))
	// Once a disconnect is seen, drop in-game events for the rest of the batch so
	// a trailing SKOOT/text doesn't repopulate the just-reset caches. A Connected
	// event later in the same batch (reconnect) re-enables them. Mirrors the
	// frontend guard so both sides agree by construction.
	disconnected := false
	for _, ev := range batch {
		showNewUserWelcome := false
		if disconnected {
			switch ev.(type) {
			case types.SKOOTUpdateEvent, types.GameTextEvent, types.SuppressedGameTextEvent,
				types.StatusUpdateEvent, types.ModeChangeEvent, types.MapURLEvent:
				continue
			}
		}
		if a.stopping.Load() {
			// Side effects already ran in the intake loop. Native rendering is the
			// only part disabled during teardown.
			continue
		}
		switch e := ev.(type) {
		case types.ConnectedEvent:
			a.mu.Lock()
			a.kudosPromptShown = false
			a.kudosQueueAtConnect = len(a.cfg().Kudos.Queue)
			a.mu.Unlock()
			showNewUserWelcome = a.claimNewUserWelcome()
			disconnected = false
		case types.GameTextEvent:
		case types.SKOOTUpdateEvent:
			a.maybeKudosPrompt(len(e.Rooms))

			// Graphics: render minimap and/or compass from this update.
			if len(e.Rooms) > 0 || len(e.Walls) > 0 {
				if img := a.render.updateMinimap(e.Rooms, e.Walls); img != nil {
					wire = append(wire, WireEvent{Kind: KindMinimap, Image: img})
				}
			}
			if e.Exits != nil {
				if img := a.render.updateExits(*e.Exits); img != nil {
					wire = append(wire, WireEvent{Kind: KindCompass, Image: img})
				}
			}

			// Debug panel: forward the raw SKOOT payload when in debug mode.
			if a.deps.Debug {
				wire = append(wire, WireEvent{Kind: KindDebug, Debug: &DebugPayload{
					Channel: e.Channel,
					Payload: e.RawPayload,
				}})
			}

		case types.DisconnectedEvent:
			// Session ended (user logout, server close, or a dropped link). Clear
			// connection-scoped graphics for every disconnect cause. Kudos state is
			// refreshed by the next Connected event, when the live queue can be
			// snapshotted for the new session.
			a.render.reset()
			disconnected = true
		}

		// Apply color-word coloring (if enabled) before conversion, mirroring
		// the TUI order (color words -> highlights -> IP mask; highlights and
		// masking happen in the frontend renderer).
		if a.colorWords.Load() {
			ev = withColorWords(ev)
		}

		// Convert the event itself (SKOOT bars, text, status, conn, etc.).
		if w, ok := toWire(ev); ok {
			wire = append(wire, w)
		}
		if showNewUserWelcome {
			// Append after the Connected wire event so the frontend has entered the
			// game screen before it handles the modal request.
			wire = append(wire, WireEvent{Kind: KindOpenMenu, OpenMenu: "new-user"})
		}
	}
	a.emit(wire)
}

// claimNewUserWelcome persists and claims the one-time welcome popup after a
// new GUI user's first successful connection. The frontend supplies explicit
// links; nothing is opened automatically. Legacy configs are migrated to
// completed during config.Load.
func (a *GuiApp) claimNewUserWelcome() bool {
	if a.deps.ConfigPath == "" {
		return false
	}

	a.mu.Lock()
	if a.cfg().Onboarding.WelcomeShown {
		a.mu.Unlock()
		return false
	}
	a.cfg().Onboarding.WelcomeShown = true
	if err := config.Save(a.cfg(), a.deps.ConfigPath); err != nil {
		a.cfg().Onboarding.WelcomeShown = false
		a.mu.Unlock()
		log.Printf("[ONBOARDING] save completion marker: %v", err)
		return false
	}
	a.mu.Unlock()
	return true
}

// maybeKudosPrompt emits one "kudos-login" menu request per connection when
// the player first enters the game (rooms present) and that connection's fresh
// queue snapshot contains pending kudos.
func (a *GuiApp) maybeKudosPrompt(roomCount int) {
	a.mu.Lock()
	if a.kudosPromptShown || roomCount == 0 || a.kudosQueueAtConnect == 0 {
		a.mu.Unlock()
		return
	}
	a.kudosPromptShown = true
	a.mu.Unlock()
	a.emit([]WireEvent{{Kind: KindOpenMenu, OpenMenu: "kudos-login"}})
}

// withColorWords returns a copy of the event with color-word coloring applied
// to its styled segments. Non-text events are returned unchanged.
func withColorWords(ev types.Event) types.Event {
	switch e := ev.(type) {
	case types.GameTextEvent:
		e.Styled = colorwords.ApplyColorWords(e.Styled)
		return e
	case types.SuppressedGameTextEvent:
		e.OriginalStyled = colorwords.ApplyColorWords(e.OriginalStyled)
		return e
	default:
		return ev
	}
}

// ---------------------------------------------------------------------------
// Init state
// ---------------------------------------------------------------------------

// InitState is the snapshot the frontend fetches on load to render the initial
// screen (account select vs. login) and seed its settings.
type InitState struct {
	Version   string            `json:"version"`
	Debug     bool              `json:"debug"`
	Accounts  []string          `json:"accounts"`
	HasModes  bool              `json:"hasModes"`
	ModeNames []string          `json:"modeNames"`
	ModeSpecs []engine.ModeSpec `json:"modeSpecs"`
	Config    *config.Config    `json:"config"`
}

// GetInitState returns the initial application state.
func (a *GuiApp) GetInitState() InitState {
	accounts, err := a.deps.Creds.ListAccounts()
	if err != nil {
		accounts = nil
	}
	modes := a.client().Engine.ModeNames()
	return InitState{
		Version:   a.deps.Version,
		Debug:     a.deps.Debug,
		Accounts:  accounts,
		HasModes:  len(modes) > 0,
		ModeNames: modes,
		ModeSpecs: a.client().Engine.ModeSpecs(),
		Config:    a.cfg(),
	}
}

// ---------------------------------------------------------------------------
// Authentication & connection
// ---------------------------------------------------------------------------

// ConnectNew logs in with an explicit username/password, optionally stores the
// credentials, then connects the WebSocket and starts the game loop. Returns
// an error string that the frontend can display; the game loop runs in the
// background on success.
func (a *GuiApp) ConnectNew(username, password string, store bool) error {
	if err := a.client().Login(username, password); err != nil {
		return err
	}
	if store {
		if err := a.deps.Creds.SetAccount(username, password); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}
	}
	return a.connectAndRun()
}

// ConnectStored looks up a stored password, logs in, and connects.
func (a *GuiApp) ConnectStored(username string) error {
	pass, err := a.deps.Creds.GetAccount(username)
	if err != nil {
		return fmt.Errorf("stored credentials not found: %w", err)
	}
	if err := a.client().Login(username, pass); err != nil {
		return err
	}
	return a.connectAndRun()
}

// connectAndRun opens the WebSocket and launches the blocking Run loop in a
// goroutine. It returns once the socket is established (or errors).
func (a *GuiApp) connectAndRun() error {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return fmt.Errorf("application is shutting down")
	}
	// Reserve the Run goroutine before dialing so Shutdown cannot observe a zero
	// count, return, and close the engine while this connection is being opened.
	a.runWG.Add(1)
	a.mu.Unlock()

	if err := a.client().ConnectWebSocket(); err != nil {
		a.runWG.Done()
		return err
	}
	go func() {
		defer a.runWG.Done()
		a.client().Run()
	}()
	return nil
}

// Disconnect performs a user-initiated logout, tearing down the current game
// session. The resulting disconnected event (empty reason) drives the frontend
// back to the bootup screen. Safe to call when not connected.
func (a *GuiApp) Disconnect() {
	a.client().Disconnect()
}

// RemoveAccount deletes stored credentials for a username.
func (a *GuiApp) RemoveAccount(username string) error {
	return a.deps.Creds.RemoveAccount(username)
}

// ---------------------------------------------------------------------------
// Input & modes
// ---------------------------------------------------------------------------

// Send handles a command from direct UI controls such as numpad navigation and
// status buttons. It deliberately bypasses typed-input variables and command
// chaining; Action-set buttons use SendInput instead.
func (a *GuiApp) Send(input string) {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	a.waitForStoppedPlay()
	if err := a.send(input, false); err != nil {
		a.emit([]WireEvent{{Kind: KindNotify, Notify: &NotifyPayload{
			Title: "Send failed", Message: err.Error(),
		}}})
	}
}

// SendInput handles one submission from the command input. Both single-line and
// multi-line input receive ${name} and ${name:fallback} substitution. A block
// containing newlines goes out whole via SendBlock without interpreting
// separators; a single line additionally receives ;; / && chaining and $()
// control steps before dispatch.
func (a *GuiApp) SendInput(input string) error {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	a.waitForStoppedPlay()
	if a.sendActive() {
		return fmt.Errorf("a /send is in flight — wait for it to finish or press Alt+X before starting another input sequence")
	}
	return a.send(input, true)
}

// InputChainActive reports whether typed input still has commands, timers, or
// reactions queued by ;;, &&, or $().
func (a *GuiApp) InputChainActive() bool {
	return a.client() != nil && a.client().InputChainActive()
}

// InputChainStatus reports the current PraetorScript step for the compact GUI
// indicator. When several chains are active, the oldest is shown and Chains
// tells the frontend how many more are running.
func (a *GuiApp) InputChainStatus() client.InputChainStatus {
	if a.client() == nil {
		return client.InputChainStatus{}
	}
	return a.client().InputChainStatus()
}

// AbortInputChains drops every queued typed-input continuation. Commands that
// already reached the server cannot be recalled.
func (a *GuiApp) AbortInputChains() int {
	if a.client() == nil {
		return 0
	}
	return a.client().AbortInputChains()
}

func (a *GuiApp) send(input string, processInput bool) error {
	// Every path that reaches the game funnels through here — the command
	// input, numpad navigation, sidebar buttons, action sets, the status bar.
	// The frontend lockout covers only the command input, so the authoritative
	// gate lives here: during a performance nothing may interleave with the
	// script. The four control commands (/pause, /resume, /stop, /next) use
	// their own bindings and never reach Send, so this is purely additive.
	if a.PlayActive() {
		log.Printf("[PLAY] rejected input during performance: %q", input)
		return fmt.Errorf("a performance is running — only /pause, /resume, /stop, /next (or Alt+X) are accepted")
	}
	// Route on the input minus any trailing line terminators: a single command
	// pasted with a trailing newline ("/mode aggro\n") is still single-line
	// input and must keep reaching SendCommand, which interprets slash commands.
	// Only an interior newline means the user is really sending a block.
	trimmed := strings.TrimRight(input, "\r\n")
	if strings.ContainsAny(trimmed, "\n\r") {
		// Pass the trimmed value, not the raw input: SendBlock no longer strips a
		// trailing newline itself (a /send batch's deliberate trailing blank line
		// must survive), so a paste's incidental trailing newline would otherwise
		// go out as an extra blank line here. /send's own batches bypass Send
		// entirely (they call SendBlock directly from sendBatch), so that path
		// keeps its trailing blank intact.
		if processInput {
			expanded, err := a.client().ExpandInputVariables(trimmed)
			if err != nil {
				return err
			}
			trimmed = expanded
		}
		if err := a.client().SendBlock(trimmed); err != nil {
			return err
		}
		return nil
	}
	if processInput {
		return a.client().SendInput(trimmed)
	}
	a.client().SendCommand(trimmed)
	return nil
}

// ModeNames returns the available Lua mode names.
func (a *GuiApp) ModeNames() []string { return a.client().Engine.ModeNames() }

// ModeSpecs returns the declared metadata for the available Lua modes, so the
// frontend can refresh its command hint after a script reload without a
// restart.
func (a *GuiApp) ModeSpecs() []engine.ModeSpec { return a.client().Engine.ModeSpecs() }

// SetMode validates and switches the active mode. "disable"/"" always allowed.
func (a *GuiApp) SetMode(name string, args []string) error {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	a.waitForStoppedPlay()
	if a.PlayActive() {
		return fmt.Errorf("a performance is running — stop it before changing modes")
	}
	if a.sendActive() {
		return fmt.Errorf("a /send is in flight — wait for it to finish or press Alt+X before changing modes")
	}
	if a.InputChainActive() {
		return fmt.Errorf("a typed command chain is still queued — stop it or press Alt+X before changing modes")
	}
	if name != "disable" && name != "" && !a.client().Engine.HasMode(name) {
		cur := a.client().Engine.CurrentMode()
		if cur == "" || cur == "disable" {
			a.client().Engine.SetMode("disable", nil)
		}
		return fmt.Errorf("unknown mode %q", name)
	}
	a.client().Engine.SetMode(name, args)
	return nil
}

// ReloadScripts hot-reloads all Lua modes.
func (a *GuiApp) ReloadScripts() error {
	return a.client().Engine.ReloadAllModes()
}

// ---------------------------------------------------------------------------
// Graphics
// ---------------------------------------------------------------------------

// refreshGraphics re-emits the current minimap and compass after the scale changes.
func (a *GuiApp) refreshGraphics() {
	var wire []WireEvent
	a.render.mu.Lock()
	img := encodeImage(a.render.mini.BuildImage())
	haveExits := a.render.haveExits
	exits := a.render.exits
	a.render.mu.Unlock()
	if img != nil {
		wire = append(wire, WireEvent{Kind: KindMinimap, Image: img})
	}
	if haveExits {
		if cimg := a.render.updateExits(exits); cimg != nil {
			wire = append(wire, WireEvent{Kind: KindCompass, Image: cimg})
		}
	}
	a.emit(wire)
}
