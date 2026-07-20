package gui

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/colorwords"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/session"
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
	configMu         sync.RWMutex
	savedConfig      *config.Config
	started          bool
	kudosPromptShown bool

	// colorWords is read on the per-line hot path in the event loop and
	// written by SetColorWords, so it is atomic to avoid a data race.
	colorWords atomic.Bool
	// initialKudosQueue snapshots the queued-kudos count at startup, used for
	// the one-time login prompt without racing the live config.
	initialKudosQueue int
}

// NewGuiApp constructs the facade around bootstrapped Deps and an Emitter.
func NewGuiApp(deps *Deps, emitter Emitter) *GuiApp {
	r := newRenderer()
	r.setScale(deps.Config.UI.MinimapScale)
	a := &GuiApp{
		deps:              deps,
		render:            r,
		emitter:           emitter,
		savedConfig:       cloneConfig(deps.Config),
		initialKudosQueue: len(deps.Config.Kudos.Queue),
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
		if disconnected {
			switch ev.(type) {
			case types.SKOOTUpdateEvent, types.GameTextEvent, types.SuppressedGameTextEvent,
				types.StatusUpdateEvent, types.ModeChangeEvent, types.CommandEvent, types.MapURLEvent:
				continue
			}
		}
		switch e := ev.(type) {
		case types.ConnectedEvent:
			disconnected = false
		case types.GameTextEvent:
			if a.deps.SessionLog != nil {
				a.deps.SessionLog.Log(e.Timestamp, e.Text)
			}
			a.deps.DesktopNotify.CheckText(e.Text)

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
			// the GUI's cached graphics and the one-time kudos prompt so a reconnect
			// starts fresh. This lives here (not in GuiApp.Disconnect) so it covers
			// every disconnect cause — Disconnect() only runs on user logout.
			a.render.reset()
			a.mu.Lock()
			a.kudosPromptShown = false
			a.mu.Unlock()
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
	}
	a.emit(wire)
}

// maybeKudosPrompt emits a one-time "kudos-login" menu request when the player
// first enters the game (rooms present) and there were queued kudos at startup.
func (a *GuiApp) maybeKudosPrompt(roomCount int) {
	if a.kudosPromptShown || roomCount == 0 || a.initialKudosQueue == 0 {
		return
	}
	a.kudosPromptShown = true
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
	Version         string                `json:"version"`
	Debug           bool                  `json:"debug"`
	Accounts        []string              `json:"accounts"`
	CredentialStore CredentialStoreStatus `json:"credentialStore"`
	HasModes        bool                  `json:"hasModes"`
	ModeNames       []string              `json:"modeNames"`
	Config          *config.Config        `json:"config"`
}

// GetInitState returns the initial application state.
func (a *GuiApp) GetInitState() InitState {
	accountState := a.ListAccounts()
	modes := a.client().Engine.ModeNames()
	a.configMu.RLock()
	configSnapshot := cloneConfig(a.cfg())
	a.configMu.RUnlock()
	return InitState{
		Version:         a.deps.Version,
		Debug:           a.deps.Debug,
		Accounts:        accountState.Accounts,
		CredentialStore: accountState.CredentialStore,
		HasModes:        len(modes) > 0,
		ModeNames:       modes,
		Config:          configSnapshot,
	}
}

// GetConfig returns an immutable deep snapshot of the current configuration.
func (a *GuiApp) GetConfig() *config.Config {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return cloneConfig(a.cfg())
}

func cloneConfig(source *config.Config) *config.Config {
	if source == nil {
		return nil
	}
	data, err := json.Marshal(source)
	if err != nil {
		return &config.Config{}
	}
	var cloned config.Config
	if err := json.Unmarshal(data, &cloned); err != nil {
		return &config.Config{}
	}
	return &cloned
}

// ---------------------------------------------------------------------------
// Authentication & connection
// ---------------------------------------------------------------------------

// CredentialStoreStatus is the non-secret health/capability projection shown
// by the login and account-selection screens.
type CredentialStoreStatus struct {
	Backend   string `json:"backend"`
	Available bool   `json:"available"`
	CanStore  bool   `json:"canStore"`
	Message   string `json:"message,omitempty"`
}

// AccountState distinguishes an unavailable credential backend from a valid,
// empty account list.
type AccountState struct {
	Accounts        []string              `json:"accounts"`
	CredentialStore CredentialStoreStatus `json:"credentialStore"`
}

// ConnectResult reports a successful TEC connection independently from the
// optional request to remember credentials.
type ConnectResult struct {
	Connected               bool          `json:"connected"`
	CredentialSaveRequested bool          `json:"credentialSaveRequested"`
	CredentialsSaved        bool          `json:"credentialsSaved"`
	Warning                 string        `json:"warning,omitempty"`
	AccountState            *AccountState `json:"accountState,omitempty"`
}

// CredentialStoreError identifies failures before a stored-account connection
// reaches TEC, allowing web handlers to avoid reporting them as game failures.
type CredentialStoreError struct {
	Operation string
	Err       error
}

func (e *CredentialStoreError) Error() string {
	return fmt.Sprintf("credential store %s: %v", e.Operation, e.Err)
}

func (e *CredentialStoreError) Unwrap() error { return e.Err }

// ListAccounts returns both stored usernames and the credential backend's
// current health. Backend errors are represented explicitly rather than being
// collapsed into an empty list.
func (a *GuiApp) ListAccounts() AccountState {
	if a.deps.Creds == nil {
		return AccountState{
			Accounts: []string{},
			CredentialStore: CredentialStoreStatus{
				Backend: "unconfigured", Message: "Secure credential storage is not configured.",
			},
		}
	}
	descriptor := a.deps.Creds.Descriptor()
	status := CredentialStoreStatus{
		Backend:   descriptor.Backend,
		Available: true,
		CanStore:  descriptor.CanStore,
	}
	if !descriptor.CanStore {
		status.Message = "Remembering accounts is disabled. You can still connect by entering your credentials."
	}
	accounts, err := a.deps.Creds.ListAccounts()
	if err != nil {
		status.Available = false
		status.Message = credentialStoreUnavailableMessage(descriptor.Backend)
		log.Printf("credential store list failed backend=%s: %v", descriptor.Backend, err)
		return AccountState{Accounts: []string{}, CredentialStore: status}
	}
	if accounts == nil {
		accounts = []string{}
	}
	return AccountState{Accounts: accounts, CredentialStore: status}
}

// ConnectNew logs in with an explicit username/password, connects the
// WebSocket, and then optionally stores the credentials. Persistence failures
// are returned as a non-fatal warning so a valid game session remains active.
func (a *GuiApp) ConnectNew(username, password string, store bool) (ConnectResult, error) {
	result := ConnectResult{CredentialSaveRequested: store}
	if err := a.client().Login(username, password); err != nil {
		return result, err
	}
	if err := a.connectAndRun(); err != nil {
		return result, err
	}
	result.Connected = true
	if !store {
		return result, nil
	}
	if a.deps.Creds == nil {
		state := a.ListAccounts()
		result.AccountState = &state
		result.Warning = "Connected, but this account could not be remembered because secure credential storage is not configured."
		return result, nil
	}
	if err := a.deps.Creds.SetAccount(username, password); err != nil {
		descriptor := a.deps.Creds.Descriptor()
		log.Printf("credential store save failed backend=%s: %v", descriptor.Backend, err)
		state := AccountState{
			Accounts: []string{},
			CredentialStore: CredentialStoreStatus{
				Backend: descriptor.Backend, CanStore: descriptor.CanStore,
				Message: credentialStoreUnavailableMessage(descriptor.Backend),
			},
		}
		result.AccountState = &state
		result.Warning = "Connected, but this account could not be remembered because secure credential storage is unavailable."
		return result, nil
	}
	state := a.ListAccounts()
	result.AccountState = &state
	result.CredentialsSaved = true
	return result, nil
}

// ConnectStored looks up a stored password, logs in, and connects.
func (a *GuiApp) ConnectStored(username string) error {
	if a.deps.Creds == nil {
		return &CredentialStoreError{Operation: "read failed", Err: session.ErrCredentialStorageDisabled}
	}
	pass, err := a.deps.Creds.GetAccount(username)
	if err != nil {
		return &CredentialStoreError{Operation: "read failed", Err: err}
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
	if a.deps.Creds == nil {
		return &CredentialStoreError{Operation: "save failed", Err: session.ErrCredentialStorageDisabled}
	}
	if err := a.deps.Creds.SetAccount(username, password); err != nil {
		return &CredentialStoreError{Operation: "save failed", Err: err}
	}
	return nil
}

// RemoveAccount deletes stored credentials for a username.
func (a *GuiApp) RemoveAccount(username string) error {
	if a.deps.Creds == nil {
		return &CredentialStoreError{Operation: "remove failed", Err: session.ErrCredentialStorageDisabled}
	}
	if err := a.deps.Creds.RemoveAccount(username); err != nil {
		return &CredentialStoreError{Operation: "remove failed", Err: err}
	}
	return nil
}

func credentialStoreUnavailableMessage(backend string) string {
	switch backend {
	case session.CredentialBackendKeyring:
		return "The system keyring is unavailable or locked. You can still connect without remembering this account."
	case session.CredentialBackendEncryptedFile:
		return "The encrypted credential store is unavailable. Check the server credential-storage configuration."
	case session.CredentialBackendDisabled:
		return "Remembering accounts is disabled. You can still connect by entering your credentials."
	default:
		return "Secure credential storage is unavailable. You can still connect without remembering this account."
	}
}

// ---------------------------------------------------------------------------
// Input & modes
// ---------------------------------------------------------------------------

// Send routes a line of user input to the client (game command or /slash).
func (a *GuiApp) Send(input string) { a.client().SendCommand(input) }

// ModeNames returns the available Lua mode names.
func (a *GuiApp) ModeNames() []string { return a.client().Engine.ModeNames() }

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
