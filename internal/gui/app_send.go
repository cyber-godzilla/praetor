package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
)

// sendBatchDelay separates consecutive batches of a /send. Orchil paces nothing,
// so this exists only to keep a large file from arriving as one enormous burst —
// it is a safety margin, not protocol pacing.
const sendBatchDelay = 250 * time.Millisecond

// SendPreview describes a picked file so the frontend can confirm before sending.
type SendPreview struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Lines   int    `json:"lines"`
	Batches int    `json:"batches"`
}

// PickSendFile opens the file picker, reads the chosen file, and returns a
// preview. A cancelled picker returns the zero SendPreview and a nil error.
func (a *GuiApp) PickSendFile() (SendPreview, error) {
	if a.deps.Dialogs == nil {
		return SendPreview{}, nil
	}
	path, err := a.deps.Dialogs.PickFile("Select a file to send", "")
	if err != nil || path == "" {
		return SendPreview{}, err
	}
	batches, lines, err := readSendFile(path)
	if err != nil {
		return SendPreview{}, err
	}
	return SendPreview{
		Path:    path,
		Name:    filepath.Base(path),
		Lines:   lines,
		Batches: len(batches),
	}, nil
}

// readSendFile loads a file and splits it into batches, also reporting the total
// line count across those batches.
func readSendFile(path string) (batches []string, lines int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	batches = client.SplitSendBatches(string(data))
	for _, b := range batches {
		lines += strings.Count(b, "\n") + 1
	}
	return batches, lines, nil
}

// StartFileSend reads path, expands the current input variables without
// interpreting command chains, and begins sending its batches. Only one send
// runs at a time: starting a second aborts the first. Refused while a /play
// performance is active — see sendActive/PlayActive below for why the two
// drivers must never run concurrently.
func (a *GuiApp) StartFileSend(path string) error {
	if a.PlayActive() {
		return fmt.Errorf("a performance is running — a /send would interleave with it on the wire; press Alt+X, or wait for the performance to finish")
	}
	if a.InputChainActive() {
		return fmt.Errorf("a typed command chain is still queued — a /send would interleave with it on the wire; stop the chain, press Alt+X, or wait for it to finish")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	expanded, err := a.client().ExpandInputVariables(string(data))
	if err != nil {
		return err
	}
	batches := client.SplitSendBatches(expanded)
	if len(batches) == 0 {
		return fmt.Errorf("%s is empty", filepath.Base(path))
	}
	a.AbortSend()
	a.startSend(batches)
	return nil
}

// sendActive reports whether a /send is currently in flight. Used by
// playPreflight to refuse starting a performance while a send is running — the
// two drivers must never write to the socket at once, or their output
// interleaves on the wire. Takes and releases sendMu only; callers must never
// hold playMu while calling this (see the lock-ordering note on PlayActive).
func (a *GuiApp) sendActive() bool {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()
	return a.sendCancel != nil
}

// startSend spawns the driver goroutine for an already-split block. Split from
// StartFileSend so tests can drive it without touching the filesystem.
func (a *GuiApp) startSend(batches []string) {
	cancel := make(chan struct{})
	a.sendMu.Lock()
	a.sendCancel = cancel
	a.sendMu.Unlock()
	go a.runSend(batches, cancel)
}

// AbortSend cancels an in-flight send. It reports whether one was actually
// running, so the caller can tell the user what stopped.
func (a *GuiApp) AbortSend() bool {
	a.sendMu.Lock()
	defer a.sendMu.Unlock()
	if a.sendCancel == nil {
		return false
	}
	close(a.sendCancel)
	a.sendCancel = nil
	return true
}

// runSend walks the batches, honoring cancellation between each one. Already
// transmitted batches cannot be recalled — abort only stops what is still queued.
func (a *GuiApp) runSend(batches []string, cancel chan struct{}) {
	defer func() {
		a.sendMu.Lock()
		if a.sendCancel == cancel {
			a.sendCancel = nil
		}
		a.sendMu.Unlock()
	}()

	for i, b := range batches {
		if i > 0 {
			select {
			case <-time.After(sendBatchDelay):
			case <-cancel:
				a.notify("Send aborted", fmt.Sprintf("%d of %d batches sent.", i, len(batches)))
				return
			}
		}
		select {
		case <-cancel:
			a.notify("Send aborted", fmt.Sprintf("%d of %d batches sent.", i, len(batches)))
			return
		default:
		}
		if err := a.sendBatch(b); err != nil {
			a.notify("Send failed", fmt.Sprintf("batch %d of %d: %v", i+1, len(batches), err))
			return
		}
	}
	a.notify("Send complete", fmt.Sprintf("%d batch(es) sent.", len(batches)))
}

// sendBatch dispatches one batch, through the test seam when set.
func (a *GuiApp) sendBatch(b string) error {
	if a.sendOne != nil {
		return a.sendOne(b)
	}
	return a.client().SendBlock(b)
}

// notify surfaces a message to the frontend as a toast.
func (a *GuiApp) notify(title, message string) {
	a.emit([]WireEvent{{Kind: KindNotify, Notify: &NotifyPayload{Title: title, Message: message}}})
}
