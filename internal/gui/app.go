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
	kudosPromptShown bool

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
	// sendOne overrides batch dispatch in tests; nil means send for real.
	sendOne func(string) error

	// playMu guards the in-flight /play session. play is non-nil exactly while a
	// performance is running or paused.
	playMu sync.Mutex
	play   *playSession
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
	}
	a.colorWords.Store(deps.Config.UI.ColorWords)
	return a
}

// client is a convenience accessor.
func (a *GuiApp) client() *client.Client { return a.deps.Client }
func (a *GuiApp) cfg() *config.Config    { return a.deps.Config }

// emit forwards a batch of wire events to the frontend on the single ordered
// channel. A nil/empty batch is a no-op.
func (a *GuiApp) emit(batch []WireEvent) {
	if len(batch) == 0 || a.emitter == nil {
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
	if a.started {
		a.mu.Unlock()
		return
	}
	a.started = true
	a.mu.Unlock()

	go a.eventLoop()
}

// eventLoop mirrors the bridge goroutine in cmd/praetor/main.go: it batches
// events, performs side effects (session log, desktop notifications), renders
// the minimap/compass, and forwards everything to the frontend in order.
func (a *GuiApp) eventLoop() {
	events := a.client().Events()
	for event := range events {
		batch := []types.Event{event}
	drain:
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					break drain
				}
				batch = append(batch, ev)
			default:
				break drain
			}
		}
		a.processBatch(batch)
	}
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
				types.StatusUpdateEvent, types.ModeChangeEvent, types.CommandEvent, types.MapURLEvent:
				continue
			}
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
			if a.deps.SessionLog != nil {
				a.deps.SessionLog.Log(e.Timestamp, e.Text)
			}
			a.deps.DesktopNotify.CheckText(e.Text)
			// Echoed lines (user-typed or script-sent) must not satisfy a
			// %wait-for cue: a script waiting on a phrase it just sent itself
			// would match instantly.
			if !e.IsEcho {
				a.feedPlayText(e.Text)
			}

		case types.SKOOTUpdateEvent:
			// Side effects.
			if e.Health != nil {
				a.deps.DesktopNotify.CheckHealth(*e.Health)
			}
			if e.Fatigue != nil {
				a.deps.DesktopNotify.CheckFatigue(*e.Fatigue)
			}
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

		case types.ModeChangeEvent:
			a.deps.DesktopNotify.Prune()

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

// GetConfig returns the current configuration.
func (a *GuiApp) GetConfig() *config.Config { return a.cfg() }

// ---------------------------------------------------------------------------
// Authentication & connection
// ---------------------------------------------------------------------------

// ListAccounts returns stored account usernames.
func (a *GuiApp) ListAccounts() []string {
	accounts, err := a.deps.Creds.ListAccounts()
	if err != nil {
		return nil
	}
	return accounts
}

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
	if err := a.client().ConnectWebSocket(); err != nil {
		return err
	}
	go a.client().Run()
	return nil
}

// Disconnect performs a user-initiated logout, tearing down the current game
// session. The resulting disconnected event (empty reason) drives the frontend
// back to the bootup screen. Safe to call when not connected.
func (a *GuiApp) Disconnect() {
	a.client().Disconnect()
}

// SaveAccount stores credentials for later ConnectStored use.
func (a *GuiApp) SaveAccount(username, password string) error {
	return a.deps.Creds.SetAccount(username, password)
}

// RemoveAccount deletes stored credentials for a username.
func (a *GuiApp) RemoveAccount(username string) error {
	return a.deps.Creds.RemoveAccount(username)
}

// ---------------------------------------------------------------------------
// Input & modes
// ---------------------------------------------------------------------------

// Send handles one submission from the command input. A block containing
// newlines (paste or modifier+Enter) goes out whole via SendBlock; a single line
// keeps the existing path, which also interprets slash commands.
func (a *GuiApp) Send(input string) {
	// Every path that reaches the game funnels through here — the command
	// input, numpad navigation, sidebar buttons, action sets, the status bar.
	// The frontend lockout covers only the command input, so the authoritative
	// gate lives here: during a performance nothing may interleave with the
	// script. The four control commands (/pause, /resume, /stop, /next) use
	// their own bindings and never reach Send, so this is purely additive.
	if a.PlayActive() {
		log.Printf("[PLAY] rejected input during performance: %q", input)
		return
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
		if err := a.client().SendBlock(trimmed); err != nil {
			a.emit([]WireEvent{{Kind: KindNotify, Notify: &NotifyPayload{
				Title: "Send failed", Message: err.Error(),
			}}})
		}
		return
	}
	a.client().SendCommand(trimmed)
}

// ModeNames returns the available Lua mode names.
func (a *GuiApp) ModeNames() []string { return a.client().Engine.ModeNames() }

// ModeSpecs returns the declared metadata for the available Lua modes, so the
// frontend can refresh its command hint after a script reload without a
// restart.
func (a *GuiApp) ModeSpecs() []engine.ModeSpec { return a.client().Engine.ModeSpecs() }

// CurrentMode returns the active mode name.
func (a *GuiApp) CurrentMode() string { return a.client().Engine.CurrentMode() }

// SetMode validates and switches the active mode. "disable"/"" always allowed.
func (a *GuiApp) SetMode(name string, args []string) error {
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

// RefreshGraphics re-emits the current minimap and compass (e.g. after the
// frontend panel is resized or the scale changes).
func (a *GuiApp) RefreshGraphics() {
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
