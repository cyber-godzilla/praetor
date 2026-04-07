package protocol

import (
	"testing"
)

// --- LineBuffer Tests ---

func TestLineBuffer_PartialLine_NoEmission(t *testing.T) {
	lb := NewLineBuffer()
	lines := lb.Write([]byte("partial data"))
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %d: %v", len(lines), lines)
	}
}

func TestLineBuffer_CompleteLine_Emission(t *testing.T) {
	lb := NewLineBuffer()
	lines := lb.Write([]byte("complete line\r\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != "complete line" {
		t.Errorf("expected 'complete line', got %q", lines[0])
	}
}

func TestLineBuffer_MultipleLinesInOneWrite(t *testing.T) {
	lb := NewLineBuffer()
	lines := lb.Write([]byte("line one\r\nline two\r\nline three\r\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	expected := []string{"line one", "line two", "line three"}
	for i, e := range expected {
		if lines[i] != e {
			t.Errorf("line %d: expected %q, got %q", i, e, lines[i])
		}
	}
}

func TestLineBuffer_TrailingPartialBuffered(t *testing.T) {
	lb := NewLineBuffer()
	lines := lb.Write([]byte("first line\r\npartial"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != "first line" {
		t.Errorf("expected 'first line', got %q", lines[0])
	}

	// Now complete the partial line
	lines = lb.Write([]byte(" data\r\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != "partial data" {
		t.Errorf("expected 'partial data', got %q", lines[0])
	}
}

func TestLineBuffer_EmptyWrite(t *testing.T) {
	lb := NewLineBuffer()
	lines := lb.Write([]byte(""))
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %d", len(lines))
	}
}

func TestLineBuffer_MultiplePartialWrites(t *testing.T) {
	lb := NewLineBuffer()

	lines := lb.Write([]byte("hel"))
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %d", len(lines))
	}

	lines = lb.Write([]byte("lo wor"))
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %d", len(lines))
	}

	lines = lb.Write([]byte("ld\r\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", lines[0])
	}
}

func TestLineBuffer_SplitCRLF(t *testing.T) {
	lb := NewLineBuffer()

	// CR arrives in one write, LF in the next
	lines := lb.Write([]byte("test line\r"))
	if len(lines) != 0 {
		t.Errorf("expected no lines from partial CRLF, got %d: %v", len(lines), lines)
	}

	lines = lb.Write([]byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != "test line" {
		t.Errorf("expected 'test line', got %q", lines[0])
	}
}

// --- ClassifyLine Tests ---

func TestClassifyLine_Secret(t *testing.T) {
	if got := ClassifyLine("SECRET abc123"); got != MsgSecret {
		t.Errorf("expected MsgSecret, got %d", got)
	}
}

func TestClassifyLine_Skoot(t *testing.T) {
	if got := ClassifyLine("SKOOT 1 exits N NE"); got != MsgSkoot {
		t.Errorf("expected MsgSkoot, got %d", got)
	}
}

func TestClassifyLine_MapURL(t *testing.T) {
	if got := ClassifyLine("MAPURL http://example.com/map"); got != MsgMapURL {
		t.Errorf("expected MsgMapURL, got %d", got)
	}
}

func TestClassifyLine_GameText(t *testing.T) {
	if got := ClassifyLine("You see a sword on the ground."); got != MsgGameText {
		t.Errorf("expected MsgGameText, got %d", got)
	}
}

func TestClassifyLine_SkootsIsGameText(t *testing.T) {
	// "SKOOTS ..." should be MsgGameText, not MsgSkoot
	if got := ClassifyLine("SKOOTS something here"); got != MsgGameText {
		t.Errorf("expected MsgGameText for 'SKOOTS', got %d", got)
	}
}

func TestClassifyLine_SecretNoPayload(t *testing.T) {
	// "SECRET " with a space but nothing after is still MsgSecret
	if got := ClassifyLine("SECRET "); got != MsgSecret {
		t.Errorf("expected MsgSecret, got %d", got)
	}
}

func TestClassifyLine_EmptyLine(t *testing.T) {
	if got := ClassifyLine(""); got != MsgGameText {
		t.Errorf("expected MsgGameText for empty line, got %d", got)
	}
}

// --- ParseSecret Tests ---

func TestParseSecret(t *testing.T) {
	if got := ParseSecret("SECRET abc123"); got != "abc123" {
		t.Errorf("expected 'abc123', got %q", got)
	}
}

func TestParseSecret_WithSpaces(t *testing.T) {
	if got := ParseSecret("SECRET some long value"); got != "some long value" {
		t.Errorf("expected 'some long value', got %q", got)
	}
}

// --- ParseMapURL Tests ---

func TestParseMapURL(t *testing.T) {
	if got := ParseMapURL("MAPURL http://example.com/map"); got != "http://example.com/map" {
		t.Errorf("expected 'http://example.com/map', got %q", got)
	}
}

func TestParseMapURL_WithSpaces(t *testing.T) {
	if got := ParseMapURL("MAPURL http://example.com/map?a=1&b=2"); got != "http://example.com/map?a=1&b=2" {
		t.Errorf("expected URL with query params, got %q", got)
	}
}
