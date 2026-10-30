package client

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestHTMLWrappedSKOOTUpdatesStatus(t *testing.T) {
	c := newDiscTestClient(t)

	c.processLine(`<span class="protocol">SKOOT 8 Fatigue,42</span>`)

	select {
	case event := <-c.Events():
		update, ok := event.(types.SKOOTUpdateEvent)
		if !ok {
			t.Fatalf("event = %T, want SKOOTUpdateEvent", event)
		}
		if update.Channel != 8 || update.RawPayload != "Fatigue,42" {
			t.Fatalf("update = %#v, want channel 8 Fatigue,42", update)
		}
		if update.Fatigue == nil || *update.Fatigue != 42 {
			t.Fatalf("fatigue = %v, want 42", update.Fatigue)
		}
	default:
		t.Fatal("HTML-wrapped SKOOT did not emit a status update")
	}

	_, fatigue, _, _ := c.Engine.Status().Get()
	if fatigue != 42 {
		t.Fatalf("engine fatigue = %d, want 42", fatigue)
	}
}
