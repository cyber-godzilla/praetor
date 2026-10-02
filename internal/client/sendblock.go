package client

import (
	"log"
	"strings"
	"time"

	"github.com/cyber-godzilla/praetor/internal/types"
)

const (
	// batchThreshold is the line count above which a block is chunked. At or
	// below it the block goes out in one message, exactly like typed input.
	batchThreshold = 50
	// batchChunk is the lines per batch once chunking kicks in.
	batchChunk = 20
)

// SplitSendBatches splits text into batches for /send, each ready to hand to
// SendBlock. Rules, in order of precedence:
//
//   - A completely empty line (whitespace-only) always ends its batch, and is
//     kept as that batch's last line — a blank line is often what terminates a
//     writing prompt in-game, so it must reach the server in place.
//   - Blocks longer than batchThreshold are additionally chunked every
//     batchChunk lines. At or below the threshold the block stays whole.
//   - Input that is empty or a single blank line yields no batches at all: a file
//     with no content is treated as empty rather than as one blank line to send.
//     Deliberate blank lines inside or at the end of real content are preserved
//     ("text\n\n" keeps its trailing blank line).
//
// Input is CRLF-normalized and a single trailing line terminator is dropped, so
// a file ending "text\n" does not produce a spurious empty final batch. A file
// ending "text\n\n" keeps its deliberate trailing blank line.
func SplitSendBatches(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	chunked := len(lines) > batchThreshold

	var batches []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			batches = append(batches, strings.Join(cur, "\n"))
			cur = nil
		}
	}

	for _, ln := range lines {
		cur = append(cur, ln)
		if strings.TrimSpace(ln) == "" {
			flush()
			continue
		}
		if chunked && len(cur) >= batchChunk {
			flush()
		}
	}
	flush()
	return batches
}

// SendBlock sends text to the game as a SINGLE WebSocket message with its
// newlines embedded, matching the Orchil client (conn.send(message+"\n") at
// orchil.js:1086) — the server splits the block itself. Sending line-by-line
// instead would change the framing players' scripts and the game's prompts rely
// on, so do not "improve" this into a loop.
//
// SendBlock only CRLF-normalizes; it does NOT trim a trailing newline. The
// caller owns trailing-newline policy. In particular, SplitSendBatches
// deliberately ends a batch ON a blank line and keeps that blank line as the
// batch's last line, because a blank line often terminates a writing prompt
// in-game — SendBlock must transmit it, not strip it. Callers that want to
// drop an incidental trailing newline (e.g. typed/pasted single input) must
// trim it themselves before calling SendBlock.
//
// Slash commands are deliberately not interpreted here: a multi-line block is
// prose, so a line beginning "/" is text, not a client command. Single-line
// input still routes through SendCommand.
func (c *Client) SendBlock(text string) error {
	block := strings.ReplaceAll(text, "\r\n", "\n")

	log.Printf("[SEND:BLOCK] %d line(s)", strings.Count(block, "\n")+1)
	if err := c.session().Send(block); err != nil {
		log.Printf("[CLIENT] block send error: %v", err)
		return err
	}

	if c.EchoTyped() {
		for _, ln := range strings.Split(block, "\n") {
			c.emit(types.GameTextEvent{
				Styled:    []types.StyledSegment{{Text: ln, Italic: true}},
				Timestamp: time.Now(),
				IsEcho:    true,
			})
		}
	}
	return nil
}
