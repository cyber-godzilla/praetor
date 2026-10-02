package client

import (
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func receiveStatusUpdate(t *testing.T, c *Client) {
	t.Helper()
	select {
	case event := <-c.Events():
		if _, ok := event.(types.StatusUpdateEvent); !ok {
			t.Fatalf("event = %T, want StatusUpdateEvent", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status update")
	}
}

func TestEmitStatusUpdateIfChangedSkipsUnchangedSnapshot(t *testing.T) {
	c := newDiscTestClient(t)
	defer c.Engine.Close()
	c.emitStatusUpdate()
	receiveStatusUpdate(t, c)

	c.emitStatusUpdateIfChanged()
	select {
	case event := <-c.Events():
		t.Fatalf("unchanged status emitted %T", event)
	case <-time.After(20 * time.Millisecond):
	}

	c.Engine.State().SetFromString("test", "changed")
	c.emitStatusUpdateIfChanged()
	receiveStatusUpdate(t, c)
}
