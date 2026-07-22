package client

import (
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestIsSafeExternalURL(t *testing.T) {
	ok := []string{"https://ok", "http://ok/path?q=1", "https://wiki.example.com/x"}
	bad := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"ssh://host",
		"ftp://host/x",
		"notaurl",
		"//host/x", // scheme-relative, no scheme
		"http://",  // no host
		"",
	}
	for _, u := range ok {
		if !isSafeExternalURL(u) {
			t.Errorf("isSafeExternalURL(%q) = false, want true", u)
		}
	}
	for _, u := range bad {
		if isSafeExternalURL(u) {
			t.Errorf("isSafeExternalURL(%q) = true, want false", u)
		}
	}
}

func TestClient_HelpURL_RejectsNonHTTPAndSurfacesText(t *testing.T) {
	c := newDiscTestClient(t)

	var opened []string
	c.openURL = func(u string) { opened = append(opened, u) }

	// A server-supplied file:// URL must not be launched.
	c.openHelpURL("file:///etc/passwd")
	if len(opened) != 0 {
		t.Fatalf("opener was called for a file:// URL: %v", opened)
	}

	// The refused URL is surfaced as text so the user can copy it deliberately.
	if !drainForText(c, "/etc/passwd") {
		t.Error("refused help URL was not surfaced as output text")
	}

	// A valid https URL is opened.
	c.openHelpURL("https://help.example.com/x")
	if len(opened) != 1 || opened[0] != "https://help.example.com/x" {
		t.Fatalf("valid help URL not opened: %v", opened)
	}
}

func TestClient_PostHandshake_SecretLineIsGameText(t *testing.T) {
	c := newDiscTestClient(t)

	// Pre-handshake: a SECRET line is the auth token, not game text.
	c.handshakeDone = false
	c.processLine("SECRET pre-auth-token")
	if drainForText(c, "pre-auth-token") {
		t.Error("pre-handshake SECRET line was shown as game text")
	}

	// Post-handshake: a game line beginning \"SECRET \" is delivered as text.
	c.handshakeDone = true
	c.processLine("SECRET hello there")
	if !drainForText(c, "hello there") {
		t.Error("post-handshake SECRET line was not delivered as game text")
	}
}

func drainForText(c *Client, substr string) bool {
	for {
		select {
		case ev := <-c.Events():
			if g, ok := ev.(types.GameTextEvent); ok {
				for _, s := range g.Styled {
					if strings.Contains(s.Text, substr) {
						return true
					}
				}
			}
		default:
			return false
		}
	}
}
