package session

import "time"

// Reconnector implements exponential backoff for reconnection attempts.
// The delay starts at initialDelay and is multiplied by multiplier after
// each attempt, capping at maxDelay.
type Reconnector struct {
	initialDelay time.Duration
	maxDelay     time.Duration
	currentDelay time.Duration
	multiplier   int
	attempt      int
}

// NewReconnector creates a Reconnector with the given initial delay, maximum
// delay, and multiplier for exponential backoff.
func NewReconnector(initial, max time.Duration, multiplier int) *Reconnector {
	return &Reconnector{
		initialDelay: initial,
		maxDelay:     max,
		currentDelay: initial,
		multiplier:   multiplier,
		attempt:      0,
	}
}

// NextDelay returns the current delay and advances to the next attempt.
// The delay increases exponentially (multiplied by the multiplier each time)
// up to the configured maximum.
func (r *Reconnector) NextDelay() time.Duration {
	delay := r.currentDelay
	r.attempt++

	next := r.currentDelay * time.Duration(r.multiplier)
	if next > r.maxDelay {
		next = r.maxDelay
	}
	r.currentDelay = next

	return delay
}

// Reset sets the attempt counter back to zero and resets the delay to the
// initial value. Call this after a successful connection.
func (r *Reconnector) Reset() {
	r.attempt = 0
	r.currentDelay = r.initialDelay
}

// Attempt returns the current attempt number (0-based before first NextDelay call).
func (r *Reconnector) Attempt() int {
	return r.attempt
}
