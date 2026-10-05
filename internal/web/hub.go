package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"

	appgui "github.com/cyber-godzilla/praetor/internal/gui"
)

const (
	subscriberQueueSize = 64
	maxSubscribers      = 64
)

type Subscription struct {
	ID       uint64
	Messages <-chan Envelope
}

// Hub is both the GuiApp emitter and the fan-out point for authenticated web
// subscribers. Projection mutation, sequence allocation, snapshot capture, and
// subscriber registration share one lock, closing the usual late-join gap.
type Hub struct {
	mu sync.Mutex

	serverID        string
	sequence        uint64
	nextID          uint64
	projector       *Projection
	clients         map[uint64]chan Envelope
	closed          bool
	observer        func([]appgui.WireEvent)
	config          json.RawMessage
	revision        uint64
	modeNames       []string
	accounts        []string
	credentialStore appgui.CredentialStoreStatus
}

func NewHub(historyLimit int) *Hub {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("web hub: crypto/rand failed: " + err.Error())
	}
	return &Hub{
		serverID:  hex.EncodeToString(b),
		projector: NewProjection(historyLimit),
		clients:   make(map[uint64]chan Envelope),
	}
}

// Emit implements internal/gui.Emitter.
func (h *Hub) Emit(event string, data any) {
	if event != appgui.EventChannel {
		return
	}
	events, ok := data.([]appgui.WireEvent)
	if !ok || len(events) == 0 {
		return
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.sequence++
	h.projector.Apply(events)
	h.broadcastLocked(Envelope{
		Type:         "events",
		Protocol:     ProtocolVersion,
		ServerID:     h.serverID,
		Sequence:     h.sequence,
		FromSequence: h.sequence,
		ToSequence:   h.sequence,
		Events:       events,
	})
	observer := h.observer
	h.mu.Unlock()
	if observer != nil {
		observer(events)
	}
}

func (h *Hub) SetObserver(observer func([]appgui.WireEvent)) {
	h.mu.Lock()
	h.observer = observer
	h.mu.Unlock()
}

func (h *Hub) ServerID() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.serverID
}

func (h *Hub) Subscribe() (Subscription, Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || len(h.clients) >= maxSubscribers {
		return Subscription{}, Envelope{}
	}
	h.nextID++
	id := h.nextID
	ch := make(chan Envelope, subscriberQueueSize)
	h.clients[id] = ch
	snapshot := Envelope{
		Type:            "snapshot",
		Protocol:        ProtocolVersion,
		ServerID:        h.serverID,
		Sequence:        h.sequence,
		Events:          h.projector.SnapshotEvents(),
		Config:          append(json.RawMessage(nil), h.config...),
		Revision:        h.revision,
		ModeNames:       append([]string(nil), h.modeNames...),
		Accounts:        cloneAccounts(h.accounts),
		CredentialStore: cloneCredentialStoreStatus(h.credentialStore),
	}
	return Subscription{ID: id, Messages: ch}, snapshot
}

func (h *Hub) Unsubscribe(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.clients[id]; ok {
		delete(h.clients, id)
		close(ch)
	}
}

func (h *Hub) BroadcastState(env Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.applyStateLocked(env)
	h.sequence++
	env.Protocol = ProtocolVersion
	env.ServerID = h.serverID
	env.Sequence = h.sequence
	h.broadcastLocked(env)
}

// Close releases all browser subscriptions. It is idempotent and makes future
// emits/subscriptions inert so process shutdown cannot race event delivery.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for id, ch := range h.clients {
		delete(h.clients, id)
		close(ch)
	}
}

func (h *Hub) SetInitialState(config json.RawMessage, revision uint64, modeNames, accounts []string, credentialStore appgui.CredentialStoreStatus) {
	h.mu.Lock()
	h.config = append(json.RawMessage(nil), config...)
	h.revision = revision
	h.modeNames = append([]string(nil), modeNames...)
	h.accounts = append([]string(nil), accounts...)
	h.credentialStore = credentialStore
	h.mu.Unlock()
}

func (h *Hub) applyStateLocked(env Envelope) {
	switch env.Type {
	case "config":
		if raw, ok := env.Config.(json.RawMessage); ok {
			h.config = append(json.RawMessage(nil), raw...)
		}
		h.revision = env.Revision
	case "modes":
		h.modeNames = append([]string(nil), env.ModeNames...)
	case "accounts":
		if env.Accounts != nil {
			h.accounts = append([]string(nil), (*env.Accounts)...)
		}
		if env.CredentialStore != nil {
			h.credentialStore = *env.CredentialStore
		}
	}
}

func cloneCredentialStoreStatus(status appgui.CredentialStoreStatus) *appgui.CredentialStoreStatus {
	copy := status
	return &copy
}

func cloneAccounts(accounts []string) *[]string {
	copy := append([]string(nil), accounts...)
	if copy == nil {
		copy = []string{}
	}
	return &copy
}

func (h *Hub) broadcastLocked(env Envelope) {
	for id, ch := range h.clients {
		select {
		case ch <- env:
		default:
			delete(h.clients, id)
			close(ch)
		}
	}
}
