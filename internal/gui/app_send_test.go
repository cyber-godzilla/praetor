package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
)

func TestConcurrentPlayAndSendStartsReserveOneProducer(t *testing.T) {
	a, _ := playTestApp(t)
	c, err := client.NewClient(config.Defaults(), nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)
	a.deps.Client = c
	a.playCheck = func() error {
		if a.sendActive() {
			return fmt.Errorf("send active")
		}
		return nil
	}
	playPath := writeScript(t, "%wait-key\n")
	sendPath := filepath.Join(t.TempDir(), "send.txt")
	if err := os.WriteFile(sendPath, []byte("send"), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseSend := make(chan struct{})
	a.sendOne = func(string) error {
		<-releaseSend
		return nil
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- a.StartPlay(playPath) }()
	go func() { <-start; results <- a.StartFileSend(sendPath) }()
	close(start)
	first, second := <-results, <-results
	succeeded := 0
	for _, result := range []error{first, second} {
		if result == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent starts produced %d successes, want exactly one (errors: %v, %v)", succeeded, first, second)
	}

	a.StopPlay()
	a.AbortSend()
	close(releaseSend)
	a.activityMu.Lock()
	a.waitForStoppedPlay()
	a.abortSendAndWait()
	a.activityMu.Unlock()
}

func TestPickSendFile_CountsLinesAndBatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.txt")
	body := strings.TrimSuffix(strings.Repeat("word\n", 60), "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	a := newTestApp(t)
	a.deps.Dialogs = &fakeDialogs{file: path}

	got, err := a.PickSendFile()
	if err != nil {
		t.Fatalf("PickSendFile: %v", err)
	}
	if got.Lines != 60 {
		t.Errorf("Lines = %d, want 60", got.Lines)
	}
	if got.Batches != 3 { // 60 > 50, so chunked: 20 + 20 + 20
		t.Errorf("Batches = %d, want 3", got.Batches)
	}
	if got.Name != "scene.txt" {
		t.Errorf("Name = %q, want scene.txt", got.Name)
	}
}

func TestPickSendFile_CancelledPickerReturnsZero(t *testing.T) {
	a := newTestApp(t)
	a.deps.Dialogs = &fakeDialogs{file: ""}

	got, err := a.PickSendFile()
	if err != nil {
		t.Fatalf("PickSendFile: %v", err)
	}
	if got.Path != "" || got.Lines != 0 {
		t.Errorf("cancelled picker returned %+v, want zero value", got)
	}
}

func TestAbortSend_StopsRemainingBatches(t *testing.T) {
	a := newTestApp(t)

	// 200 lines => 10 batches, 250ms apart: far longer than this test waits.
	batches := make([]string, 10)
	for i := range batches {
		batches[i] = "line"
	}
	sent := make(chan int, len(batches))
	a.sendOne = func(b string) error { sent <- 1; return nil }

	a.startSend(batches)
	time.Sleep(50 * time.Millisecond) // first batch goes immediately

	if !a.AbortSend() {
		t.Fatal("AbortSend returned false while a send was in flight")
	}
	time.Sleep(600 * time.Millisecond) // long enough for 2 more batch delays

	if n := len(sent); n != 1 {
		t.Fatalf("%d batches sent after abort, want 1", n)
	}
	if a.AbortSend() {
		t.Error("AbortSend returned true with no send in flight")
	}
}

func TestStartFileSendWaitsForCancelledDriverToExit(t *testing.T) {
	a := newTestApp(t)
	c, err := client.NewClient(config.Defaults(), nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)
	a.deps.Client = c
	firstPath := filepath.Join(t.TempDir(), "first.txt")
	secondPath := filepath.Join(t.TempDir(), "second.txt")
	if err := os.WriteFile(firstPath, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	a.sendOne = func(batch string) error {
		if batch == "first" {
			close(firstEntered)
			<-releaseFirst
		}
		return nil
	}
	if err := a.StartFileSend(firstPath); err != nil {
		t.Fatalf("first StartFileSend: %v", err)
	}
	<-firstEntered

	secondReturned := make(chan error, 1)
	go func() { secondReturned <- a.StartFileSend(secondPath) }()
	select {
	case err := <-secondReturned:
		t.Fatalf("replacement returned before the in-flight send exited: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFirst)
	select {
	case err := <-secondReturned:
		if err != nil {
			t.Fatalf("replacement StartFileSend: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("replacement did not start after the cancelled driver exited")
	}
}

// TestStartFileSend_RefusesWhilePlayActive is the Fix-1 regression test for the
// final review's interleaving finding: starting a /send while a /play
// performance is running must be refused, symmetric to playPreflight refusing
// a /play while a /send is in flight (see TestPlayPreflight_RefusesWhileSendActive
// in app_play_test.go). Without this, the reviewer's SEND:s1 PLAY:p1..p5 SEND:s2
// reproduction corrupts the wire.
func TestStartFileSend_RefusesWhilePlayActive(t *testing.T) {
	a, _ := playTestApp(t)
	path := writeScript(t, "%wait-key\n") // holds indefinitely, keeping PlayActive true
	if err := a.StartPlay(path); err != nil {
		t.Fatalf("StartPlay: %v", err)
	}
	t.Cleanup(func() { a.StopPlay() })

	dir := t.TempDir()
	sendPath := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(sendPath, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.StartFileSend(sendPath); err == nil {
		t.Fatal("StartFileSend succeeded while a performance was active — want a refusal")
	}
	if a.sendActive() {
		t.Error("sendActive() is true after a refused StartFileSend")
	}
}
