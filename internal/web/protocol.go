package web

import (
	"encoding/json"

	appgui "github.com/cyber-godzilla/praetor/internal/gui"
)

const ProtocolVersion = 1

// Envelope is the versioned server-to-browser WebSocket protocol. Events are
// ordered game/application updates; Snapshot is present only on the first
// message for a subscription. Config and mode/account updates are authoritative
// shared-state changes initiated by any connected browser.
type Envelope struct {
	Type            string                        `json:"type"`
	Protocol        int                           `json:"protocol"`
	ServerID        string                        `json:"serverId"`
	Sequence        uint64                        `json:"sequence,omitempty"`
	FromSequence    uint64                        `json:"fromSequence,omitempty"`
	ToSequence      uint64                        `json:"toSequence,omitempty"`
	Events          []appgui.WireEvent            `json:"events,omitempty"`
	Config          any                           `json:"config,omitempty"`
	Revision        uint64                        `json:"revision,omitempty"`
	ModeNames       []string                      `json:"modeNames,omitempty"`
	Accounts        *[]string                     `json:"accounts,omitempty"`
	CredentialStore *appgui.CredentialStoreStatus `json:"credentialStore,omitempty"`
	Result          *OperationResult              `json:"result,omitempty"`
}

type OperationResult struct {
	Operation string `json:"operation"`
	OK        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
}

type APIError struct {
	Error APIErrorBody `json:"error"`
}

type APIErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

type settingRequest struct {
	ExpectedRevision *uint64         `json:"expectedRevision,omitempty"`
	Value            json.RawMessage `json:"value"`
}
