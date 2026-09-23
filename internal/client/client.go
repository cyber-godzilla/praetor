package client

import (
	"fmt"
	"log"
	"net/http"
	neturl "net/url"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/commandinput"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/engine"
	"github.com/cyber-godzilla/praetor/internal/protocol"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/cyber-godzilla/praetor/internal/textutil"
	"github.com/cyber-godzilla/praetor/internal/types"
	"github.com/cyber-godzilla/praetor/internal/wiki"
)

// Settings holds user-configurable client settings.
type Settings struct {
	EchoTyped  bool // echo commands the user types
	EchoScript bool // echo commands sent by Lua scripts
}

type inputContinuation struct {
	chain    *inputChain
	commands []commandinput.Command
}

// inputChain owns one typed line with commands still waiting behind ;; or &&.
// Closing cancel wakes a paced wait immediately; membership in inputChains is
// the authoritative active/queued state exposed to the GUI.
type inputChain struct {
	sess   *session.Session
	cancel chan struct{}
}

// inputUnbusyMessages mirrors praetor-scripts/lib_strings.lua. Keep this as a
// slice: TEC has several ordinary responses that end roundtime, and new ones
// can be added without changing the && scheduler.
var inputUnbusyMessages = []string{
	"You are no longer busy.",
	"You are no longer stunned.",
	"You wield",
	"You grab onto",
	"You are already wielding",
	"You successfully train to rank",
}

// Client is the top-level orchestrator that wires session, engine, protocol,
// and notification subsystems together.
type Client struct {
	Config   *config.Config
	Session  *session.Session
	Engine   *engine.Engine
	Creds    session.CredentialStore
	Settings Settings

	events         chan types.Event
	cancelRun      chan struct{} // cancels this connection's listener + drainer
	userDisconnect bool          // set by Disconnect so Run reports a user logout
	cancelMu       sync.Mutex    // guards cancelRun and userDisconnect

	// sessMu guards concurrent access to the Session pointer. A reconnect
	// reassigns it while the previous connection's goroutines may still be
	// reading it, so all production reads/writes go through session()/setSession.
	sessMu sync.Mutex

	// htmlIndent tracks <ul> nesting across protocol lines for indentation.
	htmlIndent int

	// ignore drops lines from listed OOC accounts / Think characters.
	// It is hot-swappable via SetIgnoreOOC / SetIgnoreThink.
	ignore *IgnoreFilter

	// inputVariables is a hot-swappable snapshot used only for text submitted
	// through SendInput. Script sends and other UI command sources bypass it.
	inputMu        sync.RWMutex
	inputVariables map[string]string
	inputChainMu   sync.Mutex
	inputChains    map[*inputChain]struct{}
	inputWaiters   []*inputContinuation

	// authUser and authPassCookie are set by Login and used by handleSecret.
	authUser       string
	authPassCookie string

	// handshakeDone is set once the SKOOT auth handshake has run for this
	// connection. After that, a game line that happens to start "SECRET " is
	// passed through as text instead of being consumed as another handshake.
	// Touched only from the single Run line-loop goroutine.
	handshakeDone bool

	// openURL launches an external URL. Overridable in tests; defaults to opening
	// the system browser asynchronously.
	openURL func(string)

	// desktopNotify launches the platform notification. Overridable in tests.
	desktopNotify func(title, message string, sound bool)
}

// NewClient creates a fully wired Client. Pass scriptDirs for the
// Lua engine, and a CredentialStore for auth.
func NewClient(cfg *config.Config, scriptDirs []string, dataDir string, creds session.CredentialStore) (*Client, error) {
	eng, err := engine.NewEngine(scriptDirs, cfg, dataDir)
	if err != nil {
		return nil, fmt.Errorf("creating engine: %w", err)
	}

	c := &Client{
		Config:         cfg,
		Session:        session.New(),
		Engine:         eng,
		Creds:          creds,
		Settings:       Settings{EchoTyped: true, EchoScript: true},
		events:         make(chan types.Event, 256),
		ignore:         NewIgnoreFilter(),
		inputVariables: cloneVariables(cfg.Commands.Variables),
		openURL:        func(u string) { go OpenBrowser(u) },
		desktopNotify:  sendDesktopNotification,
	}
	eng.SetNotifyHandler(c.sendNotification)
	return c, nil
}

// Events returns a read-only channel of game events for the TUI.
func (c *Client) Events() <-chan types.Event {
	return c.events
}

// session returns the current Session pointer under the guard.
func (c *Client) session() *session.Session {
	c.sessMu.Lock()
	defer c.sessMu.Unlock()
	return c.Session
}

// setSession replaces the current Session pointer under the guard.
func (c *Client) setSession(s *session.Session) {
	c.inputChainMu.Lock()
	c.sessMu.Lock()
	c.Session = s
	c.sessMu.Unlock()
	// A chain belongs to the connection that sent its first command. Replacing
	// the session cancels both && waiters and ;; timers so neither can send onto
	// the next connection.
	for chain := range c.inputChains {
		close(chain.cancel)
	}
	c.inputChains = nil
	c.inputWaiters = nil
	c.inputChainMu.Unlock()
}

// SetIgnoreOOC replaces the OOC ignorelist (account names).
func (c *Client) SetIgnoreOOC(names []string) {
	c.ignore.SetOOC(names)
}

// SetIgnoreThink replaces the Think ignorelist (character names).
func (c *Client) SetIgnoreThink(names []string) {
	c.ignore.SetThink(names)
}

// SetInputVariables replaces the typed-input variable snapshot. The copy keeps
// command expansion race-free while the GUI persists a newly edited map.
func (c *Client) SetInputVariables(variables map[string]string) {
	c.inputMu.Lock()
	c.inputVariables = cloneVariables(variables)
	c.inputMu.Unlock()
}

func cloneVariables(variables map[string]string) map[string]string {
	cloned := make(map[string]string, len(variables))
	for name, value := range variables {
		cloned[name] = value
	}
	return cloned
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

	s := session.New()
	c.setSession(s)
	return s.Connect(wsURL, cookies)
}

// Run is the main loop: reads lines from the session, processes each one,
// and emits events. It blocks until the session's Lines channel closes.
func (c *Client) Run() {
	// Fresh connection: reset the HTML indent tracker (owned solely by this
	// goroutine via processLine), the handshake state, and any stale
	// user-disconnect flag.
	c.htmlIndent = 0
	c.handshakeDone = false

	sess := c.session()

	// Drop anything the engine queued while offline (between the previous
	// disconnect and now): those commands must not burst onto a fresh login.
	c.Engine.Queue().Clear()
	c.clearInputChains(sess)

	c.cancelMu.Lock()
	c.userDisconnect = false
	if c.cancelRun != nil {
		close(c.cancelRun)
	}
	c.cancelRun = make(chan struct{})
	done := c.cancelRun
	c.cancelMu.Unlock()

	// Announce the connection only after the lifecycle state (userDisconnect
	// reset, cancelRun) is set up. Emitting earlier let a Disconnect() that
	// raced the consumer of this event have its userDisconnect flag clobbered by
	// the reset above, mislabeling a user logout as a dropped connection.
	c.emit(types.ConnectedEvent{})

	// The listener and drainer are joined at teardown (below) so no goroutine from
	// this connection can outlive it — a lingering drainer sharing the queue with
	// a reconnected session could otherwise consume and lose its commands.
	var wg sync.WaitGroup

	// Listen for Lua-initiated mode changes and forward them as events.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case mc, ok := <-c.Engine.ModeChanges():
				if !ok {
					return
				}
				c.emitOrStop(types.ModeChangeEvent{NewMode: mc.NewMode, PrevMode: mc.PrevMode}, done)
				c.emitStatusUpdate()
			case <-done:
				return
			}
		}
	}()

	// One long-lived drainer owns command sending for this connection: it paces
	// min-interval locally, preserves FIFO order, and drains on enqueue (so timer
	// sends go out on an idle link). It stops when done closes.
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.drainLoop(sess, done)
	}()

	for line := range sess.Lines() {
		c.processLine(line)
	}

	// The line loop exited for some reason — a user Disconnect or an involuntary
	// drop. Tear down this connection's world so nothing from the old session
	// leaks into a reconnect: stop the listener + drainer, and drop any commands
	// the still-running engine queued while offline (they must not be transmitted
	// onto the next login). Guard against double-close: Disconnect may already
	// have closed and nil'd cancelRun.
	c.cancelMu.Lock()
	userInitiated := c.userDisconnect
	if c.cancelRun == done {
		close(c.cancelRun)
		c.cancelRun = nil
	}
	c.cancelMu.Unlock()

	// Join the listener + drainer before finalizing: once Run returns, the shell
	// may reconnect and start a new Run on the shared queue, so this connection's
	// goroutines must be fully stopped first.
	wg.Wait()

	c.Engine.Queue().Clear()
	c.clearInputChains(sess)

	reason := "connection closed"
	if userInitiated {
		reason = "" // user-initiated logout — no banner on the bootup screen
	}
	c.emit(types.DisconnectedEvent{Reason: reason})
}

// Disconnect performs a user-initiated logout: it flags the disconnect as
// user-initiated (so Run emits an empty reason), cancels the ModeChanges
// listener, closes the WebSocket, and resets engine state so a subsequent
// connection starts fresh. Safe to call when already disconnected.
func (c *Client) Disconnect() {
	c.cancelMu.Lock()
	c.userDisconnect = true
	if c.cancelRun != nil {
		close(c.cancelRun)
		c.cancelRun = nil
	}
	c.cancelMu.Unlock()

	// Close the socket; this unblocks Run()'s read loop, which emits the
	// DisconnectedEvent with the empty (logout) reason.
	sess := c.session()
	c.clearInputChains(sess)
	sess.Close()

	// Reset the engine to a clean slate for the next session: switching to the
	// default mode cancels timers, clears per-mode state, ends the metric
	// session, and drains the command queue; then wipe metrics history.
	c.Engine.SetMode("disable", nil)
	c.Engine.Metrics().Reset()
}

// SendCommand handles user input. Strings starting with "/" are interpreted
// as local commands; everything else is sent to the game server.
func (c *Client) SendCommand(input string) {
	_ = c.sendCommand(c.session(), input)
}

// sendCommand is SendCommand bound to a particular connection. Delayed input
// chains capture the active session when invoked so their remaining commands
// cannot leak into a later reconnect.
func (c *Client) sendCommand(sess *session.Session, input string) error {
	if strings.HasPrefix(input, "/") {
		log.Printf("[SEND:CMD] %s", input)
		c.handleLocalCommand(input)
		return nil
	}
	log.Printf("[SEND:GAME] %s", input)
	sendErr := sess.Send(input)
	if sendErr != nil {
		log.Printf("[CLIENT] send error: %v", sendErr)
	}

	// Echo the sent command in the output pane as italic text.
	if c.Settings.EchoTyped {
		c.emit(types.GameTextEvent{
			Styled: []types.StyledSegment{{
				Text:   input,
				Italic: true,
			}},
			Timestamp: time.Now(),
			IsEcho:    true,
		})
	}
	return sendErr
}

// InputCommandDelay is the fixed delay between commands separated by ;; in one
// input line. It is intentionally a constant until command-chain pacing
// becomes a user-facing setting.
const InputCommandDelay = 900 * time.Millisecond

// SendInput processes one single-line command submitted by a user. It expands
// ${name} variables plus ;; (paced) and && (wait-for-unbusy) separators
// atomically before sending any result. Variables are snapshotted afresh for
// every call, so saved changes are immediately visible to typed input and
// action sets. Generated commands skip further input expansion, so variables
// cannot recurse and an expanded value cannot inject a separator.
func (c *Client) SendInput(input string) error {
	c.inputMu.RLock()
	variables := cloneVariables(c.inputVariables)
	c.inputMu.RUnlock()

	commands, err := commandinput.Expand(input, variables)
	if err != nil {
		return err
	}
	if len(commands) == 0 {
		return nil
	}

	sess := c.session()
	if len(commands) == 1 {
		return c.sendCommand(sess, commands[0].Text)
	}
	chain := c.startInputChain(sess)
	if chain == nil {
		return nil
	}
	return c.dispatchInputCommand(chain, commands)
}

func (c *Client) startInputChain(sess *session.Session) *inputChain {
	chain := &inputChain{sess: sess, cancel: make(chan struct{})}
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	if c.session() != sess {
		return nil
	}
	if c.inputChains == nil {
		c.inputChains = make(map[*inputChain]struct{})
	}
	c.inputChains[chain] = struct{}{}
	return chain
}

func (c *Client) scheduleInputCommands(chain *inputChain, commands []commandinput.Command) {
	if len(commands) == 0 {
		c.finishInputChain(chain)
		return
	}
	switch commands[0].Wait {
	case commandinput.WaitDelay:
		go func() {
			timer := time.NewTimer(InputCommandDelay)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = c.dispatchInputCommand(chain, commands)
			case <-chain.cancel:
				return
			}
		}()
	case commandinput.WaitUnbusy:
		c.enqueueInputWaiter(chain, commands)
	default:
		_ = c.dispatchInputCommand(chain, commands)
	}
}

func (c *Client) dispatchInputCommand(chain *inputChain, commands []commandinput.Command) error {
	if len(commands) == 0 {
		c.finishInputChain(chain)
		return nil
	}
	if !c.inputChainIsActive(chain) || c.session() != chain.sess {
		c.finishInputChain(chain)
		return nil
	}

	// Register an && continuation before sending the command that can produce
	// its unbusy response. A fast server may reply before Send returns.
	var waiter *inputContinuation
	remaining := commands[1:]
	if len(remaining) > 0 && remaining[0].Wait == commandinput.WaitUnbusy {
		waiter = c.enqueueInputWaiter(chain, remaining)
	}
	if err := c.sendCommand(chain.sess, commands[0].Text); err != nil {
		if waiter != nil {
			c.removeInputWaiter(waiter)
		}
		c.finishInputChain(chain)
		return err
	}
	if len(remaining) == 0 {
		c.finishInputChain(chain)
	} else if waiter == nil {
		c.scheduleInputCommands(chain, remaining)
	}
	return nil
}

func (c *Client) inputChainIsActive(chain *inputChain) bool {
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	_, ok := c.inputChains[chain]
	return ok
}

func (c *Client) enqueueInputWaiter(chain *inputChain, commands []commandinput.Command) *inputContinuation {
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	if _, ok := c.inputChains[chain]; !ok || c.session() != chain.sess {
		return nil
	}
	pending := &inputContinuation{chain: chain, commands: commands}
	c.inputWaiters = append(c.inputWaiters, pending)
	return pending
}

func (c *Client) removeInputWaiter(target *inputContinuation) {
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	for i, pending := range c.inputWaiters {
		if pending == target {
			c.inputWaiters = append(c.inputWaiters[:i], c.inputWaiters[i+1:]...)
			return
		}
	}
}

func (c *Client) finishInputChain(chain *inputChain) {
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	delete(c.inputChains, chain)
	kept := c.inputWaiters[:0]
	for _, pending := range c.inputWaiters {
		if pending.chain != chain {
			kept = append(kept, pending)
		}
	}
	c.inputWaiters = kept
}

// InputChainActive reports whether any typed input has commands still queued
// behind a ;; delay or && unbusy response.
func (c *Client) InputChainActive() bool {
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	return len(c.inputChains) > 0
}

// AbortInputChains immediately discards every queued typed-input continuation.
// It returns the number of chains canceled. Commands already sent are not
// recalled, but pending ;; timers are woken and && waiters are removed.
func (c *Client) AbortInputChains() int {
	c.inputChainMu.Lock()
	defer c.inputChainMu.Unlock()
	count := len(c.inputChains)
	for chain := range c.inputChains {
		close(chain.cancel)
		delete(c.inputChains, chain)
	}
	c.inputWaiters = nil
	return count
}

func isInputUnbusy(text string) bool {
	for _, message := range inputUnbusyMessages {
		if strings.Contains(text, message) {
			return true
		}
	}
	return false
}

// advanceInputOnUnbusy releases exactly one waiter. FIFO consumption keeps two
// overlapping typed chains from both treating one server response as theirs.
func (c *Client) advanceInputOnUnbusy(text string) {
	if !isInputUnbusy(text) {
		return
	}

	c.inputChainMu.Lock()
	sess := c.session()
	var pending *inputContinuation
	for len(c.inputWaiters) > 0 {
		candidate := c.inputWaiters[0]
		c.inputWaiters = c.inputWaiters[1:]
		if candidate.chain.sess == sess {
			if _, ok := c.inputChains[candidate.chain]; !ok {
				continue
			}
			pending = candidate
			break
		}
	}
	c.inputChainMu.Unlock()

	if pending != nil {
		_ = c.dispatchInputCommand(pending.chain, pending.commands)
	}
}

func (c *Client) clearInputChains(sess *session.Session) {
	c.inputChainMu.Lock()
	for chain := range c.inputChains {
		if chain.sess == sess {
			close(chain.cancel)
			delete(c.inputChains, chain)
		}
	}
	kept := c.inputWaiters[:0]
	for _, pending := range c.inputWaiters {
		if pending.chain.sess != sess {
			kept = append(kept, pending)
		}
	}
	c.inputWaiters = kept
	c.inputChainMu.Unlock()
}

// emit delivers an event to the events channel. Guaranteed events block until
// delivered: lifecycle events (connect, disconnect, mode change) because a
// dropped DisconnectedEvent leaves the UI rendering a live-looking session, and
// game text because it is unique and unrecoverable — dropping it truncates a
// large server message mid-block and loses the line from the session log too.
// Blocking here propagates backpressure through the line channel to TCP, which
// is the correct response to a UI that cannot keep up.
//
// Status, SKOOT bar, and command events stay droppable: those are coalescible
// snapshots where the newest supersedes the old, so a dropped one costs nothing.
func (c *Client) emit(ev types.Event) {
	switch ev.(type) {
	case types.ConnectedEvent, types.DisconnectedEvent, types.ModeChangeEvent,
		types.GameTextEvent, types.SuppressedGameTextEvent:
		c.events <- ev
	default:
		select {
		case c.events <- ev:
		default:
			log.Printf("[CLIENT] event channel full, dropping event %T", ev)
		}
	}
}

// emitOrStop is emit for callers that must stay cancellable while blocked. A
// guaranteed event (game text echo) would otherwise pin drainLoop inside a
// channel send where it can no longer observe stop, hanging Run's wg.Wait() at
// teardown if the consumer has stopped draining.
func (c *Client) emitOrStop(ev types.Event, stop <-chan struct{}) {
	select {
	case c.events <- ev:
	case <-stop:
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
		// Only the pre-handshake SECRET line is the auth token; after the
		// handshake, game text starting "SECRET " must not be swallowed or
		// trigger a garbage auth re-send.
		if c.handshakeDone {
			c.handleGameText(line)
		} else {
			c.handleSecret(line)
		}
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

	// Open help URLs in the system browser, but only after validating the scheme:
	// the URL is server-supplied, so a hostile/compromised (or MITM'd cleartext)
	// server could otherwise auto-launch file:// or custom-scheme URLs.
	if ev.HelpURL != "" {
		c.openHelpURL(ev.HelpURL)
	}

	// Update engine status values for Lua access.
	c.Engine.Status().Update(ev.Health, ev.Fatigue, ev.Encumbrance, ev.Satiation)
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
	sess := c.session()
	for _, msg := range msgs {
		if err := sess.Send(msg); err != nil {
			log.Printf("[CLIENT] auth send error: %v", err)
			return
		}
	}
	// Handshake complete for this connection: later "SECRET " lines are game text.
	c.handshakeDone = true
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

	// Lines from ignored OOC accounts / Think characters are emitted as
	// SuppressedGameTextEvent (placeholder + original) instead of
	// GameTextEvent. The session log, desktop notify, and engine all key
	// off GameTextEvent, so the early return suppresses them as required.
	if ch, name, hit := c.ignore.Match(result.Text); hit {
		c.emitSuppressed(result, ch, name)
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
		c.emitStatusUpdate()
		c.advanceInputOnUnbusy(result.Text)
	}
}

// emitSuppressed builds a SuppressedGameTextEvent for the given parse
// result and channel/name capture, then emits it. Engine processing
// and the regular GameTextEvent are skipped by the caller.
func (c *Client) emitSuppressed(result protocol.HTMLResult, ch IgnoreChannel, name string) {
	placeholderText := fmt.Sprintf("[suppressed: %s %s]", name, ch.String())
	placeholderStyled := []types.StyledSegment{
		{
			Text:   placeholderText,
			Color:  "#888888",
			Italic: true,
		},
	}
	c.emit(types.SuppressedGameTextEvent{
		Channel:           types.IgnoreChannel(ch),
		SourceName:        name,
		PlaceholderText:   placeholderText,
		PlaceholderStyled: placeholderStyled,
		OriginalText:      result.Text,
		OriginalStyled:    result.Segments,
		Timestamp:         time.Now(),
	})
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

// drainLoop is the single command sender for one connection. It is the only
// goroutine that pops the queue and calls Send, which makes min-interval pacing
// a race-free local time.Since and keeps sends in FIFO order across differing
// per-command delays. It parks on the queue's Notify signal when idle (so a
// timer-enqueued command still goes out on a quiet connection) and exits when
// stop closes. sess is captured for this connection's lifetime, so a reconnect
// that reassigns c.Session never redirects an in-flight command.
func (c *Client) drainLoop(sess *session.Session, stop <-chan struct{}) {
	queue := c.Engine.Queue()
	var lastSend time.Time

	for {
		cmd, gen, ok := queue.DequeueGen()
		if !ok {
			// Queue empty: park until something is enqueued or we're stopped.
			select {
			case <-queue.Notify():
				continue
			case <-stop:
				return
			}
		}

		// Respect the command's own delay, interruptibly.
		if cmd.Delay > 0 {
			select {
			case <-time.After(cmd.Delay):
			case <-stop:
				return
			}
		}

		// Enforce the minimum interval since the last send. The first send (zero
		// lastSend) goes immediately.
		if !lastSend.IsZero() {
			if gap := time.Since(lastSend); gap < queue.MinInterval() {
				select {
				case <-time.After(queue.MinInterval() - gap):
				case <-stop:
					return
				}
			}
		}

		// If a mode switch cleared the queue while we were sleeping, this command
		// belongs to a retired generation — drop it instead of sending stale
		// old-mode output onto the wire.
		if queue.Generation() != gen {
			continue
		}

		if err := sess.Send(cmd.Command); err != nil {
			log.Printf("[CLIENT] queue send error: %v", err)
			// A WebSocket write error is terminal. Close the session so the
			// involuntary-disconnect flow runs promptly instead of waiting out
			// the read deadline, and stop draining this dead connection.
			sess.Close()
			return
		}
		lastSend = time.Now()

		// Echo engine commands in the output if script echo is enabled.
		if c.Settings.EchoScript {
			c.emitOrStop(types.GameTextEvent{
				Styled: []types.StyledSegment{{
					Text:   cmd.Command,
					Italic: true,
				}},
				Timestamp: time.Now(),
				IsEcho:    true,
			}, stop)
		}
	}
}

// sendNotification sends a desktop notification and emits a NotificationEvent.
func (c *Client) sendNotification(title, message string) {
	if c.desktopNotify != nil {
		go c.desktopNotify(title, message, c.Config.Notifications.Desktop.Sound)
	}
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

	case "/wiki":
		// Strip "/wiki " prefix; trim spaces.
		rest := strings.TrimSpace(strings.TrimPrefix(input, parts[0]))
		if rest == "" {
			// Bare /wiki — ask the TUI to open the bookmark menu.
			c.emit(types.WikiOpenMenuEvent{})
			return
		}
		slug, ok := wiki.Lookup(rest)
		if !ok {
			c.emit(types.GameTextEvent{
				Styled: []types.StyledSegment{{
					Text:   fmt.Sprintf("unknown wiki bookmark %q (type /wiki for the list)", rest),
					Color:  "#e8a838",
					Italic: true,
				}},
				Timestamp: time.Now(),
				IsEcho:    true,
			})
			return
		}
		url := wiki.URL(slug)
		go OpenBrowser(url)
		c.emit(types.GameTextEvent{
			Styled: []types.StyledSegment{{
				Text:   "opening wiki: " + url,
				Color:  "#e8a838",
				Italic: true,
			}},
			Timestamp: time.Now(),
			IsEcho:    true,
		})

	case "/maps":
		rest := strings.TrimSpace(strings.TrimPrefix(input, parts[0]))
		if rest == "" {
			c.emit(types.MapsOpenMenuEvent{})
			return
		}
		slug, ok := wiki.LookupMap(rest)
		if !ok {
			// Suggest close matches via Levenshtein.
			suggestions := suggestMaps(rest, 3)
			if len(suggestions) > 0 {
				c.emit(types.GameTextEvent{
					Styled: []types.StyledSegment{{
						Text:   fmt.Sprintf("unknown map %q. did you mean: %s? (or /maps for the list)", rest, strings.Join(suggestions, ", ")),
						Color:  "#e8a838",
						Italic: true,
					}},
					Timestamp: time.Now(),
					IsEcho:    true,
				})
			} else {
				c.emit(types.GameTextEvent{
					Styled: []types.StyledSegment{{
						Text:   fmt.Sprintf("unknown map %q (type /maps for the list)", rest),
						Color:  "#e8a838",
						Italic: true,
					}},
					Timestamp: time.Now(),
					IsEcho:    true,
				})
			}
			return
		}
		url := wiki.URL(slug)
		go OpenBrowser(url)
		c.emit(types.GameTextEvent{
			Styled: []types.StyledSegment{{
				Text:   "opening map: " + url,
				Color:  "#e8a838",
				Italic: true,
			}},
			Timestamp: time.Now(),
			IsEcho:    true,
		})

	case "/calc", "/rb":
		c.emit(types.CalcOpenMenuEvent{})

	default:
		log.Printf("[CLIENT] unknown command: %s", cmd)
	}
}

// suggestMaps returns up to 3 known map keys whose Levenshtein distance
// from the input is small, sorted by ascending distance.
func suggestMaps(input string, maxDist int) []string {
	type cand struct {
		key  string
		dist int
	}
	var cands []cand
	lower := strings.ToLower(input)
	for _, k := range wiki.MapKeys() {
		d := textutil.Levenshtein(lower, strings.ToLower(k))
		if d <= maxDist {
			cands = append(cands, cand{k, d})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
	if len(cands) > 3 {
		cands = cands[:3]
	}
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.key
	}
	return out
}

// isSafeExternalURL reports whether a URL is safe to hand to the system opener:
// it must parse cleanly, use the http or https scheme, and name a host. This
// fails closed on file://, javascript:, ssh://, scheme-relative, and empty URLs.
func isSafeExternalURL(raw string) bool {
	u, err := neturl.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

// openHelpURL opens a server-supplied help URL if it passes scheme validation;
// otherwise it logs the refusal and surfaces the URL as output text so the user
// can copy it deliberately instead of it being auto-launched.
func (c *Client) openHelpURL(raw string) {
	if !isSafeExternalURL(raw) {
		log.Printf("[CLIENT] refused to open non-http(s) help URL: %s", raw)
		c.emit(types.GameTextEvent{
			Styled: []types.StyledSegment{{
				Text:   "help link (not opened): " + raw,
				Color:  "#e8a838",
				Italic: true,
			}},
			Timestamp: time.Now(),
			IsEcho:    true,
		})
		return
	}
	c.openURL(raw)
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
