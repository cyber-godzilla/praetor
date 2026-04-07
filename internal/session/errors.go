package session

import "errors"

// ErrSessionClosed is returned when an operation is attempted on a closed session.
var ErrSessionClosed = errors.New("session is closed")
