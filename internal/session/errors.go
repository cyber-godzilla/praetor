package session

import "errors"

// ErrSessionClosed is returned when an operation is attempted on a closed session.
var ErrSessionClosed = errors.New("session is closed")

// ErrAlreadyConnected is returned by Connect when the session already has a
// connection. Sessions are one-shot; reconnect by creating a new Session.
var ErrAlreadyConnected = errors.New("session already connected; create a new Session")
