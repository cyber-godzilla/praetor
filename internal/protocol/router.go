package protocol

import "strings"

// LineBuffer accumulates partial WebSocket frames and splits on \r\n,
// returning complete lines.
type LineBuffer struct {
	partial string
}

// NewLineBuffer creates a new LineBuffer.
func NewLineBuffer() *LineBuffer {
	return &LineBuffer{}
}

// Write appends data to the internal buffer and returns any complete lines
// (terminated by \r\n). Trailing partial data is buffered for subsequent calls.
func (lb *LineBuffer) Write(data []byte) []string {
	lb.partial += string(data)
	var lines []string
	for {
		idx := strings.Index(lb.partial, "\r\n")
		if idx < 0 {
			break
		}
		lines = append(lines, lb.partial[:idx])
		lb.partial = lb.partial[idx+2:]
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
