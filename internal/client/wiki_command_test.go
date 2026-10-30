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

func TestMapAliasUsesNewWikiHost(t *testing.T) {
	c := newDiscTestClient(t)
	var opened []string
	c.openURL = func(u string) { opened = append(opened, u) }

	if err := c.SendInput("/map monlon-ravines"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	if len(opened) != 1 || opened[0] != "https://tec-wiki.com/hg:monlon-ravines" {
		t.Fatalf("opened = %v, want new wiki map URL", opened)
	}
}

func TestUnknownSlashCommandReportsPlayerVisibleError(t *testing.T) {
	c := newDiscTestClient(t)

	if err := c.SendInput("/does-not-exist argument"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}

	select {
	case event := <-c.Events():
		gameText, ok := event.(types.GameTextEvent)
		if !ok {
			t.Fatalf("event = %T, want GameTextEvent", event)
		}
		if len(gameText.Styled) != 1 || gameText.Styled[0].Text != `unknown command "/does-not-exist" (type /help for commands)` {
			t.Fatalf("styled = %#v, want unknown-command feedback", gameText.Styled)
		}
		if !gameText.IsEcho {
			t.Fatal("unknown-command feedback should be marked as an echo")
		}
	default:
		t.Fatal("unknown slash command did not emit player-visible feedback")
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
