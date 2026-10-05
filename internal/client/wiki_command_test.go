package client

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestWikiCommandUsesNewWikiHost(t *testing.T) {
	c := newDiscTestClient(t)
	var opened []string
	c.openURL = func(u string) { opened = append(opened, u) }

	if err := c.SendInput("/wiki stats"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	if len(opened) != 1 || opened[0] != "https://tec-wiki.com/stats" {
		t.Fatalf("opened = %v, want new wiki stats URL", opened)
	}
}

func TestMapsCommandUsesNewWikiHost(t *testing.T) {
	c := newDiscTestClient(t)
	var opened []string
	c.openURL = func(u string) { opened = append(opened, u) }

	if err := c.SendInput("/maps monlon-ravines"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	if len(opened) != 1 || opened[0] != "https://tec-wiki.com/hg:monlon-ravines" {
		t.Fatalf("opened = %v, want new wiki map URL", opened)
	}
}

func TestBareWikiCommandStillOpensBookmarkMenu(t *testing.T) {
	c := newDiscTestClient(t)
	if err := c.SendInput("/wiki"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}

	select {
	case event := <-c.Events():
		if _, ok := event.(types.WikiOpenMenuEvent); !ok {
			t.Fatalf("event = %T, want WikiOpenMenuEvent", event)
		}
	default:
		t.Fatal("bare /wiki did not emit WikiOpenMenuEvent")
	}
}
