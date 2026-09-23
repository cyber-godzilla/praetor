package gui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/engine"
)

// playNoteBeat is how long a %note holds before the scene moves on. A note is a
// stage direction the performer has to read mid-scene, so passing through it
// instantly made scripted scenes look as though their waits were being ignored.
const playNoteBeat = 500 * time.Millisecond

// noteColor is Skotos orange — the same accent as the GUI's --skotos-orange and
// the TUI's colorOrange — so a stage note reads as client chrome rather than
// game text. It must stay a bare hex value: the frontend's safeColor sanitizer
// fails closed on anything containing "(", so a var(--skotos-orange) reference
// would be stripped and the note would render uncolored.
const noteColor = "#e8a838"

// PlayState is the live status of a performance, for the input-bar indicator.
// Step is 1-based and advances as the script runs, so the indicator can show
// progress rather than a static "performing" chip — during a long %wait or a
// cue wait, a number that moves is the only thing distinguishing a deliberate
// hold from a wedged client.
type PlayState struct {
	Active bool `json:"active"`
	Paused bool `json:"paused"`
	Step   int  `json:"step"`
	Total  int  `json:"total"`
}

// PlayError is one script validation failure, for the frontend.
type PlayError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// PlayPreview describes a picked script before it is performed. A non-empty
// Errors means the script cannot be played.
type PlayPreview struct {
	Path    string      `json:"path"`
	Name    string      `json:"name"`
	Steps   int         `json:"steps"`
	FixedMs int64       `json:"fixedMs"` // sum of fixed waits only
	HasCues bool        `json:"hasCues"` // has %wait-for or %wait-key: runtime is unbounded
	Errors  []PlayError `json:"errors"`
}

// playSession is one performance. Owned by its driver goroutine except for the
// control channels, which the /pause, /resume, /stop, /next entry points use.
type playSession struct {
	steps  []client.PlayStep
	cancel chan struct{} // closed by /stop and Alt+X
	pause  chan struct{} // signalled by /pause
	resume chan struct{} // signalled by /resume
	next   chan struct{} // signalled by /next
	paused bool          // guarded by playMu
	idx    int           // step currently executing, 0-based; guarded by playMu

	text    chan string // game text for %wait-for; buffered, never blocks the event loop
	matcher *engine.Matcher

	lastSend time.Time // guarded by the driver goroutine, which is its only user
}

// PickPlayFile opens the picker, parses the chosen script, and returns a
// preview. A cancelled picker returns the zero PlayPreview and a nil error.
func (a *GuiApp) PickPlayFile() (PlayPreview, error) {
	if a.deps.Dialogs == nil {
		return PlayPreview{}, nil
	}
	path, err := a.deps.Dialogs.PickFile("Select a script to play", "")
	if err != nil || path == "" {
		return PlayPreview{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PlayPreview{}, err
	}
	steps, perrs := client.ParsePlayScript(string(data))

	p := PlayPreview{Path: path, Name: filepath.Base(path), Steps: len(steps)}
	for _, e := range perrs {
		p.Errors = append(p.Errors, PlayError{Line: e.Line, Message: e.Msg})
	}
	for _, s := range steps {
		switch s.Kind {
		case client.StepWait:
			p.FixedMs += s.Dur.Milliseconds()
		case client.StepWaitRandom:
			p.FixedMs += s.Dur.Milliseconds() // lower bound; the estimate is labelled approximate
		case client.StepWaitFor, client.StepWaitKey:
			p.HasCues = true
		}
	}
	return p, nil
}

// playPreflight refuses a performance that cannot run cleanly. It is nil-safe
// throughout: a missing client is treated as disconnected rather than panicking,
// which matters because this runs on the caller's goroutine and a panic here
// would surface as a dead binding rather than a message.
func (a *GuiApp) playPreflight() error {
	if a.playCheck != nil {
		return a.playCheck()
	}
	c := a.client()
	if c == nil || c.Session == nil || !c.Session.IsConnected() {
		return fmt.Errorf("not connected — nothing was played")
	}
	// A /send in flight is exactly the interleaving hazard the mode check below
	// exists for: two drivers writing to the socket at once corrupt both. Checked
	// via sendActive(), which takes and releases sendMu on its own — never inline
	// the field read here, and never call this while holding playMu (StartPlay
	// calls playPreflight before it acquires playMu, so this is safe as written).
	if a.sendActive() {
		return fmt.Errorf(
			"a /send is in flight — it would interleave with the performance on the wire; press Alt+X, or wait for the send to finish")
	}
	if c.InputChainActive() {
		return fmt.Errorf(
			"a typed command chain is still queued — it would interleave with the performance; stop the chain, press Alt+X, or wait for it to finish")
	}
	if c.Engine != nil {
		if mode := c.Engine.CurrentMode(); mode != "" && mode != "disable" {
			return fmt.Errorf(
				"mode %q is running — its queued commands would interleave with the performance; press Alt+X first",
				mode)
		}
	}
	return nil
}

// StartPlay parses path and begins performing it. It refuses an invalid script
// outright: validation exists so faults surface before anything is sent.
func (a *GuiApp) StartPlay(path string) error {
	if err := a.playPreflight(); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	steps, perrs := client.ParsePlayScript(string(data))
	if len(perrs) > 0 {
		return fmt.Errorf("%s has %d error(s); first: %s", filepath.Base(path), len(perrs), perrs[0].Error())
	}
	if len(steps) == 0 {
		return fmt.Errorf("%s has nothing to play", filepath.Base(path))
	}

	a.playMu.Lock()
	if a.play != nil {
		a.playMu.Unlock()
		return fmt.Errorf("a performance is already running — /stop it first")
	}
	s := &playSession{
		steps:   steps,
		cancel:  make(chan struct{}),
		pause:   make(chan struct{}, 1),
		resume:  make(chan struct{}, 1),
		next:    make(chan struct{}, 1),
		text:    make(chan string, 64),
		matcher: engine.NewMatcher(),
	}
	a.play = s
	a.playMu.Unlock()

	go a.runPlay(s)
	return nil
}

// PlayStatus reports the live state of a performance for the input-bar
// indicator. The zero value (Active false) means nothing is playing.
func (a *GuiApp) PlayStatus() PlayState {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.play == nil {
		return PlayState{}
	}
	return PlayState{
		Active: true,
		Paused: a.play.paused,
		Step:   a.play.idx + 1, // 1-based for display
		Total:  len(a.play.steps),
	}
}

// PlayActive reports whether a performance is running or paused. Also used by
// StartFileSend to refuse a /send while a performance is active (the reverse
// of playPreflight's sendActive check). Takes and releases playMu only — never
// hold sendMu while calling this, and never hold playMu while calling
// sendActive: the two mutexes are never acquired together, in either order, or
// a lock-ordering deadlock becomes possible where none exists today.
func (a *GuiApp) PlayActive() bool {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	return a.play != nil
}

// PausePlay holds the performance. Reports whether one was running to hold.
func (a *GuiApp) PausePlay() bool {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.play == nil || a.play.paused {
		return false
	}
	a.play.paused = true
	select {
	case a.play.pause <- struct{}{}:
	default:
	}
	return true
}

// ResumePlay continues a held performance from the pending step.
func (a *GuiApp) ResumePlay() bool {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.play == nil || !a.play.paused {
		return false
	}
	a.play.paused = false
	select {
	case a.play.resume <- struct{}{}:
	default:
	}
	return true
}

// StopPlay halts the performance and drops its state.
func (a *GuiApp) StopPlay() bool {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.play == nil {
		return false
	}
	close(a.play.cancel)
	a.play = nil
	return true
}

// NextPlayStep releases a %wait-key hold. Ignored when nothing is holding.
func (a *GuiApp) NextPlayStep() bool {
	a.playMu.Lock()
	defer a.playMu.Unlock()
	if a.play == nil {
		return false
	}
	select {
	case a.play.next <- struct{}{}:
	default:
	}
	return true
}

// runPlay executes the script. The index advances only after a step COMPLETES,
// so pausing mid-instruction leaves it on that instruction and /resume re-runs
// it from the start — a %wait restarts, a %wait-for re-arms its matcher.
func (a *GuiApp) runPlay(s *playSession) {
	defer a.finishPlay(s)

	for idx := 0; idx < len(s.steps); {
		// Stop is checked EVERY iteration, not only inside waits: a run of plain
		// text lines would otherwise ignore /stop until the next wait step.
		// A line already in flight still completes — like a sent command, it
		// cannot be recalled — but no further step begins.
		select {
		case <-s.cancel:
			return
		default:
		}

		// Publish the position for PlayStatus before the step runs, so the
		// indicator names the step actually in progress — including the long
		// %wait or cue wait the driver is about to park on.
		a.playMu.Lock()
		s.idx = idx
		a.playMu.Unlock()

		if !a.waitWhilePaused(s) {
			return
		}

		// Drop pause/next tokens left behind by a step that could not consume
		// them. `paused` (guarded by playMu) is the source of truth for whether
		// to hold; these channels exist only to interrupt a step that blocks.
		// Without this drain, a signal aimed at one step silently satisfies a
		// later, unrelated one — an early /next skipping a %wait-key, or a stale
		// /pause making a later %wait restart.
		select {
		case <-s.pause:
		default:
		}
		select {
		case <-s.next:
		default:
		}

		done, stopped := a.runPlayStep(s, s.steps[idx])
		if stopped {
			return
		}
		if done {
			idx++
		}
		// !done means paused mid-step: loop without advancing, so the step re-runs.
	}
	a.notify("Performance complete", fmt.Sprintf("%d step(s) played.", len(s.steps)))
}

// waitWhilePaused blocks until /resume or /stop. Returns false when stopped.
//
// Unlike s.pause and s.next (drained in runPlay before each step), s.resume is
// deliberately NOT drained anywhere. That is safe only because this is a
// RE-CHECKING LOOP, not a single select: a stale resume token consumed here
// just costs one extra iteration — paused is re-read, found true, and the loop
// blocks again on the next resume/cancel. If this loop is ever collapsed into
// a single non-looping select (e.g. "select once on resume/cancel and
// return"), that guarantee disappears and a stale token would let a later
// pause fall straight through as a spurious wake. Keep the loop, or add a
// drain, if this ever changes.
func (a *GuiApp) waitWhilePaused(s *playSession) bool {
	for {
		a.playMu.Lock()
		paused := s.paused
		a.playMu.Unlock()
		if !paused {
			return true
		}
		select {
		case <-s.resume:
		case <-s.cancel:
			return false
		}
	}
}

// runPlayStep executes one step. It returns (completed, stopped): completed is
// false when a pause interrupted the step, so the caller re-runs it.
func (a *GuiApp) runPlayStep(s *playSession, step client.PlayStep) (completed, stopped bool) {
	switch step.Kind {
	case client.StepText:
		if gap := time.Since(s.lastSend); !s.lastSend.IsZero() {
			// Commands.MinInterval is config.Duration, a YAML wrapper — the real
			// time.Duration is its .Duration field.
			if floor := a.cfg().Commands.MinInterval.Duration; gap < floor {
				if done, stopped := a.playSleep(s, floor-gap); !done {
					return done, stopped
				}
			}
		}
		if err := a.playDispatch(step.Text); err != nil {
			a.notify("Performance failed", fmt.Sprintf("line %d: %v", step.Line, err))
			return false, true
		}
		s.lastSend = time.Now()
		return true, false

	case client.StepNote:
		// The spec puts %note in the OUTPUT PANE, not a toast: a cue you need
		// mid-scene must stay on screen and in scrollback, not fade after 5s.
		a.emit([]WireEvent{{Kind: KindText, Text: &TextPayload{
			Text:      step.Text,
			Segments:  []Segment{{Text: step.Text, Color: noteColor, Italic: true}},
			Timestamp: unixMillis(time.Now()),
		}}})
		// Then hold a beat, interruptibly, so /pause and /stop stay responsive.
		// The beat runs AFTER the print so the note lands with the line it
		// annotates rather than at the same instant as the following line. A
		// pause landing inside the beat re-runs the step and reprints the note —
		// cosmetic, and the better trade of the two.
		return a.playSleep(s, playNoteBeat)

	case client.StepWait:
		return a.playSleep(s, step.Dur)

	case client.StepWaitRandom:
		return a.playSleep(s, a.playRandDur(step.Dur, step.DurMax))

	case client.StepWaitKey:
		select {
		case <-s.next:
			return true, false
		case <-s.pause:
			return false, false
		case <-s.cancel:
			return false, true
		}

	case client.StepWaitFor:
		return a.playWaitFor(s, step)

	default:
		return true, false
	}
}

// playWaitFor blocks until game text matches the step's pattern, bounded by the
// step's own timeout or the configured default. Matching is CASE-INSENSITIVE,
// deliberately diverging from Lua reaction matching: a reaction that misses
// fires again on the next line, while a missed cue stalls the performance and
// then halts the scene, so the forgiving behavior is worth the inconsistency.
func (a *GuiApp) playWaitFor(s *playSession, step client.PlayStep) (completed, stopped bool) {
	timeout := step.Dur
	if timeout <= 0 {
		timeout = a.cfg().Play.WaitForTimeout.Duration
	}
	cp := s.matcher.Compile(strings.ToLower(step.Text))
	deadline := a.playTimer(timeout)

	// Drain text buffered before this step began: a cue that arrived while an
	// earlier line was being sent has already passed and must not satisfy us.
	for {
		select {
		case <-s.text:
			continue
		default:
		}
		break
	}

	for {
		select {
		case line := <-s.text:
			if s.matcher.Match(cp, strings.ToLower(line)) {
				return true, false
			}
		case <-deadline:
			a.notify("Cue not seen", fmt.Sprintf(
				"line %d gave up waiting for %q after %s. /resume to keep waiting.",
				step.Line, step.Text, timeout))
			// Halt but KEEP state: pause the session in place so /resume re-runs
			// this same step and re-arms the matcher.
			a.playMu.Lock()
			s.paused = true
			a.playMu.Unlock()
			return false, false
		case <-s.pause:
			return false, false
		case <-s.cancel:
			return false, true
		}
	}
}

// feedPlayText hands one line of game text to a waiting %wait-for. It never
// blocks: the caller is the GUI event loop, and a full buffer means the
// performance is not currently waiting on a cue.
func (a *GuiApp) feedPlayText(text string) {
	a.playMu.Lock()
	s := a.play
	a.playMu.Unlock()
	if s == nil {
		return
	}
	select {
	case s.text <- text:
	default:
	}
}

// playSleep waits out a duration, interruptible by pause and stop.
func (a *GuiApp) playSleep(s *playSession, d time.Duration) (completed, stopped bool) {
	select {
	case <-a.playTimer(d):
		return true, false
	case <-s.pause:
		return false, false
	case <-s.cancel:
		return false, true
	}
}

// finishPlay clears the session unless a newer one already replaced it.
func (a *GuiApp) finishPlay(s *playSession) {
	a.playMu.Lock()
	if a.play == s {
		a.play = nil
	}
	a.playMu.Unlock()
}

// --- seams -----------------------------------------------------------------

func (a *GuiApp) playDispatch(line string) error {
	if a.playSend != nil {
		return a.playSend(line)
	}
	// Script lines are prose, not client commands: SendBlock deliberately does
	// not interpret a leading "/", so a script can never fire /mode or /stop at
	// the client instead of reaching the game.
	return a.client().SendBlock(line)
}

func (a *GuiApp) playTimer(d time.Duration) <-chan time.Time {
	if a.playAfter != nil {
		return a.playAfter(d)
	}
	return time.After(d)
}

func (a *GuiApp) playRandDur(min, max time.Duration) time.Duration {
	if a.playRand != nil {
		return a.playRand(min, max)
	}
	if max <= min {
		return min
	}
	return min + time.Duration(rand.Int63n(int64(max-min)))
}
