package client

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/engine"
	"github.com/cyber-godzilla/praetor/internal/protocol"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Settings holds user-configurable client settings.
type Settings struct {
	CommandEcho bool
}

// Client is the top-level orchestrator that wires session, engine, protocol,
// and notification subsystems together.
type Client struct {
	Config   *config.Config
	Session  *session.Session
	Engine   *engine.Engine
	Creds    session.CredentialStore
	Settings Settings

	events          chan types.Event
	reconnector     *session.Reconnector
	cancelReconnect chan struct{}
	cancelRun       chan struct{} // cancels the ModeChanges listener goroutine
	cancelMu        sync.Mutex
	lastHealth      int
	lastFatigue     int

	// htmlIndent tracks <ul> nesting across protocol lines for indentation.
	htmlIndent int

	// authUser and authPassCookie are set by Login and used by handleSecret.
	authUser       string
	authPassCookie string
}

// NewClient creates a fully wired Client. Pass scriptDirs for the
// Lua engine, and a CredentialStore for auth.
func NewClient(cfg *config.Config, scriptDirs []string, dataDir string, creds session.CredentialStore) (*Client, error) {
	eng, err := engine.NewEngine(scriptDirs, cfg, dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating engine: %w", err)
	}

	recon := session.NewReconnector(
		cfg.Reconnect.InitialDelay.Duration,
		cfg.Reconnect.MaxDelay.Duration,
		cfg.Reconnect.BackoffMultiplier,
	)

	return &Client{
		Config:      cfg,
		Session:     session.New(),
		Engine:      eng,
		Creds:       creds,
		Settings:    Settings{CommandEcho: true},
		events:      make(chan types.Event, 256),
		reconnector: recon,
		lastHealth:  100,
		lastFatigue: 100,
	}, nil
}

// Events returns a read-only channel of game events for the TUI.
func (c *Client) Events() <-chan types.Event {
	return c.events
}

// Login performs HTTP login and stores the session cookies for auth.
// It uses the raw password to get a server-generated pass cookie token.
// Credentials are NOT automatically saved — the caller decides whether to persist them.
func (c *Client) Login(username, password string) error {
	userCookie, passCookie, err := session.HTTPLogin(
		c.Config.Server.LoginURL, username, password,
	)
	if err != nil {
		return fmt.Errorf("HTTP login: %w", err)
	}
	c.authUser = userCookie
	c.authPassCookie = passCookie
	c.Engine.SetUsername(username)
	return nil
}

// AuthUser returns the authenticated username.
func (c *Client) AuthUser() string {
	return c.authUser
}

// ConnectWebSocket opens the WebSocket connection using cookies from a
// prior Login() call. Call Login() first.
func (c *Client) ConnectWebSocket() error {
	if c.authUser == "" || c.authPassCookie == "" {
		return fmt.Errorf("not authenticated — call Login() first")
	}

	wsURL := fmt.Sprintf("%s://%s:%d/tec",
		c.Config.Server.Protocol,
		c.Config.Server.Host,
		c.Config.Server.Port,
	)

	cookies := []*http.Cookie{
		{Name: "user", Value: c.authUser},
		{Name: "pass", Value: c.authPassCookie},
	}

	c.Session = session.New()
	return c.Session.Connect(wsURL, cookies)
}

// ConnectWithAuth performs HTTP login, then connects the WebSocket with
// the session cookies. This is the standard connection flow.
func (c *Client) ConnectWithAuth(username, password string) error {
	if err := c.Login(username, password); err != nil {
		return err
	}
	return c.ConnectWebSocket()
}

// Run is the main loop: reads lines from the session, processes each one,
// and emits events. It blocks until the session's Lines channel closes.
func (c *Client) Run() {
	c.emit(types.ConnectedEvent{})
	c.reconnector.Reset()

	// Cancel any previous ModeChanges listener goroutine from a prior Run().
	c.cancelMu.Lock()
	if c.cancelRun != nil {
		close(c.cancelRun)
	}
	c.cancelRun = make(chan struct{})
	done := c.cancelRun
	c.cancelMu.Unlock()

	// Listen for Lua-initiated mode changes and forward them as events.
	go func() {
		for {
			select {
			case mc, ok := <-c.Engine.ModeChanges():
				if !ok {
					return
				}
				c.emit(types.ModeChangeEvent{NewMode: mc.NewMode, PrevMode: mc.PrevMode})
				c.emitStatusUpdate()
				c.drainQueue()
			case <-done:
				return
			}
		}
	}()

	for line := range c.Session.Lines() {
		c.processLine(line)
	}

	c.emit(types.DisconnectedEvent{Reason: "connection closed"})
	if c.Config.Reconnect.Enabled {
		go c.reconnectLoop()
	}
}

// SendCommand handles user input. Strings starting with "/" are interpreted
// as local commands; everything else is sent to the game server.
func (c *Client) SendCommand(input string) {
	if strings.HasPrefix(input, "/") {
		log.Printf("[SEND:CMD] %s", input)
		c.handleLocalCommand(input)
		return
	}
	log.Printf("[SEND:GAME] %s", input)
	if err := c.Session.Send(input); err != nil {
		log.Printf("[CLIENT] send error: %v", err)
	}

	// Echo the sent command in the output pane as italic text.
	if c.Settings.CommandEcho {
		c.emit(types.GameTextEvent{
			Styled: []types.StyledSegment{{
				Text:   input,
				Italic: true,
			}},
			Timestamp: time.Now(),
		})
	}
}

// CancelReconnect signals the reconnect loop to stop.
func (c *Client) CancelReconnect() {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	if c.cancelReconnect != nil {
		select {
		case <-c.cancelReconnect:
			// already closed
		default:
			close(c.cancelReconnect)
		}
	}
}

// emit sends an event to the events channel without blocking.
func (c *Client) emit(ev types.Event) {
	select {
	case c.events <- ev:
	default:
		// Drop event if channel is full to avoid blocking the main loop.
		log.Printf("[CLIENT] event channel full, dropping event %T", ev)
	}
}

// processLine classifies a raw protocol line and routes it to the appropriate
// handler.
func (c *Client) processLine(line string) {
	msgType := protocol.ClassifyLine(line)

	// Log every raw line with its classification.
	switch msgType {
	case protocol.MsgSkoot:
		log.Printf("[RECV:SKOOT] %s", line)
	case protocol.MsgMapURL:
		log.Printf("[RECV:MAPURL] %s", line)
	case protocol.MsgSecret:
		log.Printf("[RECV:SECRET] (redacted)")
	case protocol.MsgGameText:
		log.Printf("[RECV:TEXT] %s", line)
	}

	switch msgType {
	case protocol.MsgSkoot:
		c.handleSkoot(line)
	case protocol.MsgMapURL:
		url := protocol.ParseMapURL(line)
		c.emit(types.MapURLEvent{URL: url})
	case protocol.MsgSecret:
		c.handleSecret(line)
	case protocol.MsgGameText:
		c.handleGameText(line)
	}
}

// handleSkoot parses SKOOT protocol lines and emits status updates.
func (c *Client) handleSkoot(line string) {
	seq, payload, err := protocol.ParseSkoot(line)
	if err != nil {
		log.Printf("[CLIENT] skoot parse error: %v", err)
		return
	}

	ev := protocol.InterpretSkoot(seq, payload)
	if ev == nil {
		// Unhandled SKOOT channel — log for discovery.
		log.Printf("[CLIENT] unhandled SKOOT ch=%d: %s", seq, payload)
		return
	}

	ev.Channel = seq
	ev.RawPayload = payload
	c.emit(*ev)

	// Open help URLs in the system browser.
	if ev.HelpURL != "" {
		go OpenBrowser(ev.HelpURL)
	}

	// Update engine status values for Lua access.
	c.Engine.Status().Update(ev.Health, ev.Fatigue, ev.Encumbrance, ev.Satiation)

	// Health/fatigue notifications.
	c.checkHealthNotifications(ev)
}

// handleSecret processes SECRET lines by performing the auth handshake.
// Uses the passCookie from the HTTP login (not the raw password).
func (c *Client) handleSecret(line string) {
	secret := protocol.ParseSecret(line)

	if c.authUser == "" || c.authPassCookie == "" {
		log.Printf("[CLIENT] no auth credentials available for SKOOT handshake")
		return
	}

	msgs := session.BuildAuthMessages(c.authUser, c.authPassCookie, secret)
	for _, msg := range msgs {
		if err := c.Session.Send(msg); err != nil {
			log.Printf("[CLIENT] auth send error: %v", err)
			return
		}
	}
}

// handleGameText parses HTML, classifies text, emits the event, then runs it
// through the engine and drains the command queue.
func (c *Client) handleGameText(line string) {
	// Filter out any lines that are actually SKOOT/MAPURL data that slipped
	// through classification (e.g., embedded in multi-line content).
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "SKOOT ") || strings.HasPrefix(trimmed, "MAPURL ") {
		log.Printf("[CLIENT] filtered misclassified protocol line: %s", trimmed)
		return
	}

	result := protocol.ParseHTMLWithIndent(line, c.htmlIndent)
	c.htmlIndent = result.IndentLevel

	// Filter protocol lines that arrived wrapped in HTML — ClassifyLine
	// missed them because the raw line started with a tag, not "SKOOT ".
	stripped := strings.TrimSpace(result.Text)
	if strings.HasPrefix(stripped, "SKOOT ") || strings.HasPrefix(stripped, "MAPURL ") {
		log.Printf("[CLIENT] filtered HTML-wrapped protocol line: %s", stripped)
		if strings.HasPrefix(stripped, "SKOOT ") {
			c.handleSkoot(stripped)
		}
		return
	}

	// Emit a blank line before section breaks (</pre> boundaries).
	if result.SectionBreak {
		c.emit(types.GameTextEvent{
			Timestamp: time.Now(),
		})
	}

	c.emit(types.GameTextEvent{
		Text:      result.Text,
		Styled:    result.Segments,
		Timestamp: time.Now(),
		Raw:       line,
	})

	// Emit a blank line after horizontal rules for visual separation.
	if result.HasHR {
		c.emit(types.GameTextEvent{
			Timestamp: time.Now(),
		})
	}

	if result.Text != "" {
		c.Engine.Process(result.Text)
		c.drainQueue()
		c.emitStatusUpdate()
	}
}

// emitStatusUpdate sends a StatusUpdateEvent with current mode, display state, and metrics.
func (c *Client) emitStatusUpdate() {
	displayVals := c.Engine.State().DisplayValues()
	var items []types.StateDisplayItem
	for _, dv := range displayVals {
		items = append(items, types.StateDisplayItem{Label: dv.Label, Value: dv.Value})
	}

	ev := types.StatusUpdateEvent{
		Mode:         c.Engine.CurrentMode(),
		DisplayState: items,
	}

	// Snapshot current metrics session.
	if cur := c.Engine.Metrics().Current(); cur != nil {
		snap := &types.MetricSnapshot{
			Mode:  cur.Mode,
			Start: cur.StartTime,
			End:   cur.EndTime,
		}
		for _, e := range cur.Entries {
			snap.Entries = append(snap.Entries, types.MetricSnapshotEntry{
				Label: e.Label,
				Value: e.Value,
			})
		}
		ev.MetricsCurrent = snap
	}

	// Snapshot history.
	for _, h := range c.Engine.Metrics().History() {
		snap := types.MetricSnapshot{
			Mode:  h.Mode,
			Start: h.StartTime,
			End:   h.EndTime,
		}
		for _, e := range h.Entries {
			snap.Entries = append(snap.Entries, types.MetricSnapshotEntry{
				Label: e.Label,
				Value: e.Value,
			})
		}
		ev.MetricsHistory = append(ev.MetricsHistory, snap)
	}

	c.emit(ev)
}

// drainQueue pops all commands from the engine queue and sends them, respecting
// delays and the minimum interval between sends.
func (c *Client) drainQueue() {
	queue := c.Engine.Queue()

	// Drain in a goroutine so processLine doesn't block.
	go func() {
		for {
			cmd, ok := queue.Dequeue()
			if !ok {
				return
			}

			// Respect the command delay.
			if cmd.Delay > 0 {
				time.Sleep(cmd.Delay)
			}

			// Enforce minimum interval between sends.
			elapsed := queue.TimeSinceLastSend()
			if elapsed < queue.MinInterval() {
				time.Sleep(queue.MinInterval() - elapsed)
			}

			if err := c.Session.Send(cmd.Command); err != nil {
				log.Printf("[CLIENT] queue send error: %v", err)
				return
			}
			queue.RecordSend()

			c.emit(types.CommandEvent{Command: cmd.Command})

			// Echo engine commands in the output if command echo is enabled.
			if c.Settings.CommandEcho {
				c.emit(types.GameTextEvent{
					Styled: []types.StyledSegment{{
						Text:   cmd.Command,
						Italic: true,
					}},
					Timestamp: time.Now(),
				})
			}
		}
	}()
}

// checkHealthNotifications sends push notifications when health or fatigue
// cross critical thresholds.
func (c *Client) checkHealthNotifications(ev *types.SKOOTUpdateEvent) {
	if ev.Health != nil {
		health := *ev.Health
		if health <= 25 && c.lastHealth > 25 {
			c.sendNotification("Urgent", fmt.Sprintf("Health critical: %d%%", health))
		} else if health <= 75 && c.lastHealth > 75 {
			c.sendNotification("Ready To Start Cooldown", fmt.Sprintf("Health at %d%%", health))
		}
		c.lastHealth = health
	}

	if ev.Fatigue != nil {
		fatigue := *ev.Fatigue
		if fatigue <= 0 && c.lastFatigue > 0 {
			c.sendNotification("Ready To Start Cooldown", "Fatigue depleted")
		}
		c.lastFatigue = fatigue
	}
}

// sendNotification sends a desktop notification and emits a NotificationEvent.
func (c *Client) sendNotification(title, message string) {
	go sendDesktopNotification(title, message)
	c.emit(types.NotificationEvent{Title: title, Message: message})
}

// handleLocalCommand parses and executes slash commands.
func (c *Client) handleLocalCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/mode", "/sm":
		// /mode <name> [args...] (alias: /sm)
		if len(parts) < 2 {
			log.Printf("[CLIENT] /mode requires a mode name")
			return
		}
		mode := parts[1]
		var args []string
		if len(parts) > 2 {
			args = parts[2:]
		}
		c.Engine.SetMode(mode, args)

	case "/toggle":
		if len(parts) < 2 {
			log.Printf("[CLIENT] /toggle requires a label")
			return
		}
		label := parts[1]
		key, ok := c.Engine.State().ResolveLabel(label)
		if !ok {
			log.Printf("[CLIENT] unknown state label: %s", label)
			return
		}
		c.Engine.State().Toggle(key)
		c.emitStatusUpdate()

	case "/set":
		if len(parts) < 3 {
			log.Printf("[CLIENT] /set requires a label and value")
			return
		}
		label := parts[1]
		value := strings.Join(parts[2:], " ")
		key, ok := c.Engine.State().ResolveLabel(label)
		if !ok {
			log.Printf("[CLIENT] unknown state label: %s", label)
			return
		}
		c.Engine.State().SetFromString(key, value)
		c.emitStatusUpdate()

	case "/reconnect":
		c.Reconnect()

	default:
		log.Printf("[CLIENT] unknown command: %s", cmd)
	}
}

// Reconnect terminates the current session and starts a fresh connection,
// equivalent to refreshing the Orchil browser window.
func (c *Client) Reconnect() {
	log.Printf("[CLIENT] manual reconnect requested")
	if c.Session != nil {
		c.Session.Close()
	}
	c.emit(types.DisconnectedEvent{Reason: "manual reconnect"})
	go c.reconnectLoop()
}

// reconnectLoop attempts to reconnect with exponential backoff. It can be
// cancelled via CancelReconnect().
func (c *Client) reconnectLoop() {
	c.cancelMu.Lock()
	c.cancelReconnect = make(chan struct{})
	c.cancelMu.Unlock()

	for {
		delay := c.reconnector.NextDelay()
		attempt := c.reconnector.Attempt()

		c.emit(types.ReconnectingEvent{
			Attempt:   attempt,
			NextDelay: delay,
		})

		// Wait for delay or cancellation.
		select {
		case <-time.After(delay):
		case <-c.cancelReconnect:
			log.Printf("[CLIENT] reconnect cancelled")
			return
		}

		// Re-login to get fresh session cookies using the stored authUser.
		pass, err := c.Creds.GetAccount(c.authUser)
		if err != nil {
			log.Printf("[CLIENT] reconnect: no credentials for %q: %v", c.authUser, err)
			c.emit(types.ErrorEvent{Context: "reconnect", Err: err})
			continue
		}

		if err := c.Login(c.authUser, pass); err != nil {
			log.Printf("[CLIENT] reconnect attempt %d login failed: %v", attempt, err)
			c.emit(types.ErrorEvent{Context: "reconnect", Err: err})
			continue
		}

		// Build WebSocket URL.
		wsURL := fmt.Sprintf("%s://%s:%d/tec",
			c.Config.Server.Protocol,
			c.Config.Server.Host,
			c.Config.Server.Port,
		)

		cookies := []*http.Cookie{
			{Name: "user", Value: c.authUser},
			{Name: "pass", Value: c.authPassCookie},
		}

		// Create a fresh session.
		c.Session = session.New()

		if err := c.Session.Connect(wsURL, cookies); err != nil {
			log.Printf("[CLIENT] reconnect attempt %d failed: %v", attempt, err)
			c.emit(types.ErrorEvent{Context: "reconnect", Err: err})
			continue
		}

		// Connection succeeded — Run() will emit ConnectedEvent and reset the
		// reconnector.
		c.Run()
		return
	}
}

// OpenBrowser opens a URL in the system's default browser.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		log.Printf("[CLIENT] cannot open browser on %s: %s", runtime.GOOS, url)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[CLIENT] failed to open browser: %v", err)
		return
	}
	go cmd.Wait() // reap child process
}
