package web

import (
	"sync"
	"testing"
	"time"

	appgui "github.com/cyber-godzilla/praetor/internal/gui"
)

func TestProjectionSnapshotAndDisconnectReset(t *testing.T) {
	p := NewProjection(100)
	h := 80
	p.Apply([]appgui.WireEvent{
		{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "connected"}},
		{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "hello"}},
		{Kind: appgui.KindBars, Bars: &appgui.BarsPayload{HasHealth: true, Health: h}},
		{Kind: appgui.KindMinimap, Image: &appgui.ImagePayload{DataURI: "data:image/png;base64,a"}},
	})
	p.Apply([]appgui.WireEvent{
		{Kind: appgui.KindBars, Bars: &appgui.BarsPayload{HasFatigue: true, Fatigue: 10}},
	})

	snap := p.SnapshotEvents()
	if len(snap) != 4 {
		t.Fatalf("snapshot len = %d, want 4: %#v", len(snap), snap)
	}
	if snap[0].Conn == nil || snap[0].Conn.State != "connected" {
		t.Fatalf("first event is not connected: %#v", snap[0])
	}
	var bars *appgui.BarsPayload
	for _, ev := range snap {
		if ev.Kind == appgui.KindBars {
			bars = ev.Bars
		}
	}
	if bars == nil || !bars.HasHealth || bars.Health != 80 || !bars.HasFatigue || bars.Fatigue != 10 {
		t.Fatalf("merged bars = %#v", bars)
	}

	p.Apply([]appgui.WireEvent{{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "disconnected"}}})
	snap = p.SnapshotEvents()
	if len(snap) != 1 || snap[0].Conn == nil || snap[0].Conn.State != "disconnected" {
		t.Fatalf("disconnect snapshot = %#v", snap)
	}
}

func TestProjectionDoesNotCarryTrailingDisconnectedEventsIntoNextSession(t *testing.T) {
	p := NewProjection(100)
	p.Apply([]appgui.WireEvent{
		{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "connected"}},
		{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "old session"}},
		{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "disconnected"}},
		{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "trailing old event"}},
	})
	p.Apply([]appgui.WireEvent{
		{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "connected"}},
		{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "new session"}},
	})

	snapshot := p.SnapshotEvents()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot len = %d, want connection plus new text: %#v", len(snapshot), snapshot)
	}
	if snapshot[1].Text == nil || snapshot[1].Text.Text != "new session" {
		t.Fatalf("snapshot retained stale session data: %#v", snapshot)
	}
}

func TestHubEvictsSlowSubscriberWithoutBlockingOthers(t *testing.T) {
	h := NewHub(100)
	slow, _ := h.Subscribe()
	fast, _ := h.Subscribe()
	defer h.Unsubscribe(fast.ID)

	for i := 0; i < subscriberQueueSize+2; i++ {
		h.Emit(appgui.EventChannel, []appgui.WireEvent{{
			Kind: appgui.KindText,
			Text: &appgui.TextPayload{Text: "line"},
		}})
		select {
		case <-fast.Messages:
		case <-time.After(time.Second):
			t.Fatal("fast subscriber did not receive an event")
		}
	}

	h.mu.Lock()
	_, slowPresent := h.clients[slow.ID]
	_, fastPresent := h.clients[fast.ID]
	h.mu.Unlock()
	if slowPresent {
		t.Fatal("slow subscriber was not evicted")
	}
	if !fastPresent {
		t.Fatal("draining subscriber was incorrectly evicted")
	}
}

func TestHubConcurrentSubscribeAndEmitPreservesJoinBoundary(t *testing.T) {
	h := NewHub(1000)
	const emissions = 100
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < emissions; i++ {
			h.Emit(appgui.EventChannel, []appgui.WireEvent{{
				Kind: appgui.KindText,
				Text: &appgui.TextPayload{Text: "line"},
			}})
			time.Sleep(time.Microsecond)
		}
	}()

	sub, snapshot := h.Subscribe()
	defer h.Unsubscribe(sub.ID)
	received := make(chan uint64, emissions)
	go func() {
		for message := range sub.Messages {
			received <- message.Sequence
		}
	}()
	wg.Wait()

	last := snapshot.Sequence
	for last < emissions {
		select {
		case sequence := <-received:
			if sequence != last+1 {
				t.Fatalf("sequence after snapshot = %d, want %d", sequence, last+1)
			}
			last = sequence
		case <-time.After(time.Second):
			t.Fatalf("timed out at sequence %d", last)
		}
	}
}

func TestHubBoundsConcurrentSubscribers(t *testing.T) {
	h := NewHub(100)
	ids := make([]uint64, 0, maxSubscribers)
	for i := 0; i < maxSubscribers; i++ {
		subscription, _ := h.Subscribe()
		if subscription.ID == 0 {
			t.Fatalf("subscriber %d was rejected before capacity", i)
		}
		ids = append(ids, subscription.ID)
	}
	if extra, _ := h.Subscribe(); extra.ID != 0 {
		t.Fatalf("subscriber over capacity was accepted: %#v", extra)
	}
	for _, id := range ids {
		h.Unsubscribe(id)
	}
	if subscription, _ := h.Subscribe(); subscription.ID == 0 {
		t.Fatal("capacity was not released after unsubscribe")
	} else {
		h.Unsubscribe(subscription.ID)
	}
}

func TestHubCloseDuringEventDelivery(t *testing.T) {
	h := NewHub(100)
	subscription, _ := h.Subscribe()
	h.Emit(appgui.EventChannel, []appgui.WireEvent{{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "queued"}}})
	h.Close()
	h.Close()
	h.Emit(appgui.EventChannel, []appgui.WireEvent{{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "ignored"}}})

	for range subscription.Messages {
		// The already queued envelope may be observed before the closed channel.
	}
	if after, _ := h.Subscribe(); after.ID != 0 {
		t.Fatalf("subscription accepted after close: %#v", after)
	}
}

func TestHubSnapshotThenOrderedEvents(t *testing.T) {
	h := NewHub(100)
	h.Emit(appgui.EventChannel, []appgui.WireEvent{
		{Kind: appgui.KindConn, Conn: &appgui.ConnPayload{State: "connected"}},
		{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "before"}},
	})

	sub, snapshot := h.Subscribe()
	defer h.Unsubscribe(sub.ID)
	if snapshot.Type != "snapshot" || snapshot.Sequence != 1 || len(snapshot.Events) != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}

	h.Emit(appgui.EventChannel, []appgui.WireEvent{
		{Kind: appgui.KindText, Text: &appgui.TextPayload{Text: "after"}},
	})
	msg := <-sub.Messages
	if msg.Type != "events" || msg.FromSequence != 2 || msg.ToSequence != 2 {
		t.Fatalf("event envelope = %#v", msg)
	}
}
