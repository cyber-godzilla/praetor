package protocol

import (
	"bytes"
	"log"
	"strings"
)

// maxPartialSize caps the un-terminated remainder the LineBuffer will hold. It
// is orders of magnitude above any real game line; a server that never sends
// \r\n would otherwise grow the buffer without bound (an OOM vector).
const maxPartialSize = 1 << 20 // 1 MiB

// LineBuffer accumulates partial WebSocket frames and splits on \r\n,
// returning complete lines. Framing is byte-level, so a UTF-8 rune split across
// two frames reassembles correctly.
type LineBuffer struct {
	partial []byte
}

// NewLineBuffer creates a new LineBuffer.
func NewLineBuffer() *LineBuffer {
	return &LineBuffer{}
}

// Write appends data to the internal buffer and returns any complete lines
// (terminated by \r\n). Trailing partial data is buffered for subsequent calls.
// If the buffered remainder ever exceeds maxPartialSize with no terminator, it
// is flushed as a single line (rather than dropped, so game text is never
// silently lost) — this can split a multi-byte rune at the flush point in the
// pathological >1 MiB-line case.
func (lb *LineBuffer) Write(data []byte) []string {
	lb.partial = append(lb.partial, data...)
	var lines []string
	for {
		idx := bytes.Index(lb.partial, []byte("\r\n"))
		if idx < 0 {
			break
		}
		lines = append(lines, string(lb.partial[:idx]))
		lb.partial = lb.partial[idx+2:]
	}
	if len(lb.partial) > maxPartialSize {
		log.Printf("[PROTOCOL] flushing oversized partial line (%d bytes, no CRLF)", len(lb.partial))
		lines = append(lines, string(lb.partial))
		lb.partial = nil // release the large backing array
	}
	return lines
}

// MessageType identifies the protocol-level category of a line received from
// the game server.
type MessageType int

const (
	// MsgGameText is regular game output (HTML or plain text).
	MsgGameText MessageType = iota
	// MsgSecret is a secret/session token line ("SECRET ...").
	MsgSecret
	// MsgSkoot is a SKOOT status update line ("SKOOT ...").
	MsgSkoot
	// MsgMapURL is a map URL line ("MAPURL ...").
	MsgMapURL
)

// ClassifyLine determines the MessageType of a raw protocol line.
// It checks for known prefixes followed by a space. "SKOOTS" is treated as
// game text since only "SKOOT " is a valid protocol prefix.
func ClassifyLine(line string) MessageType {
	if strings.HasPrefix(line, "SECRET ") {
		return MsgSecret
	}
	if strings.HasPrefix(line, "SKOOT ") {
		return MsgSkoot
	}
	if strings.HasPrefix(line, "MAPURL ") {
		return MsgMapURL
	}
	return MsgGameText
}

// ParseSecret strips the "SECRET " prefix and returns the secret value.
func ParseSecret(line string) string {
	return strings.TrimPrefix(line, "SECRET ")
}

// ParseMapURL strips the "MAPURL " prefix and returns the URL.
func ParseMapURL(line string) string {
	return strings.TrimPrefix(line, "MAPURL ")
}
