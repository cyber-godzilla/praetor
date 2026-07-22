package web

import appgui "github.com/cyber-godzilla/praetor/internal/gui"

// Projection reduces transient WireEvent batches into the state needed by a
// browser that joins after the game session has already begun. Hub owns it and
// calls these methods while holding the hub mutex.
type Projection struct {
	historyLimit int
	history      []appgui.WireEvent
	conn         *appgui.WireEvent
	status       *appgui.WireEvent
	bars         *appgui.WireEvent
	minimap      *appgui.WireEvent
	compass      *appgui.WireEvent
}

func NewProjection(historyLimit int) *Projection {
	if historyLimit < 100 {
		historyLimit = 5000
	}
	if historyLimit > 20000 {
		historyLimit = 20000
	}
	return &Projection{historyLimit: historyLimit}
}

func (p *Projection) Apply(events []appgui.WireEvent) {
	for _, ev := range events {
		// Connection transitions establish the lifetime of all other projected
		// game state. In particular, ignore a trailing event that was already in
		// the client channel when a disconnect was processed; otherwise that old
		// line could reappear in a late-join snapshot after the next connection.
		if ev.Kind == appgui.KindConn {
			if ev.Conn != nil && ev.Conn.State == "disconnected" {
				p.resetSession()
			}
			copy := ev
			p.conn = &copy
			continue
		}
		if p.conn == nil || p.conn.Conn == nil || p.conn.Conn.State != "connected" {
			continue
		}

		switch ev.Kind {
		case appgui.KindText, appgui.KindSuppressed, appgui.KindDebug:
			p.history = append(p.history, ev)
			if overflow := len(p.history) - p.historyLimit; overflow > 0 {
				copy(p.history, p.history[overflow:])
				p.history = p.history[:p.historyLimit]
			}
		case appgui.KindStatus:
			copy := ev
			p.status = &copy
		case appgui.KindBars:
			p.mergeBars(ev)
		case appgui.KindMinimap:
			copy := ev
			p.minimap = &copy
		case appgui.KindCompass:
			copy := ev
			p.compass = &copy
		}
	}
}

func (p *Projection) SnapshotEvents() []appgui.WireEvent {
	out := make([]appgui.WireEvent, 0, len(p.history)+5)
	if p.conn != nil {
		out = append(out, *p.conn)
	} else {
		out = append(out, appgui.WireEvent{
			Kind: appgui.KindConn,
			Conn: &appgui.ConnPayload{State: "disconnected"},
		})
	}
	out = append(out, p.history...)
	for _, latest := range []*appgui.WireEvent{p.status, p.bars, p.minimap, p.compass} {
		if latest != nil {
			out = append(out, *latest)
		}
	}
	return out
}

func (p *Projection) resetSession() {
	p.history = nil
	p.status = nil
	p.bars = nil
	p.minimap = nil
	p.compass = nil
}

func (p *Projection) mergeBars(ev appgui.WireEvent) {
	if ev.Bars == nil {
		return
	}
	if p.bars == nil || p.bars.Bars == nil {
		copy := ev
		payload := *ev.Bars
		copy.Bars = &payload
		p.bars = &copy
		return
	}
	merged := *p.bars.Bars
	incoming := ev.Bars
	if incoming.HasHealth {
		merged.HasHealth, merged.Health = true, incoming.Health
	}
	if incoming.HasFatigue {
		merged.HasFatigue, merged.Fatigue = true, incoming.Fatigue
	}
	if incoming.HasEncumbrance {
		merged.HasEncumbrance, merged.Encumbrance = true, incoming.Encumbrance
	}
	if incoming.HasSatiation {
		merged.HasSatiation, merged.Satiation = true, incoming.Satiation
	}
	if incoming.HasLighting {
		merged.HasLighting = true
		merged.Lighting = incoming.Lighting
		merged.LightingRaw = incoming.LightingRaw
	}
	copy := ev
	copy.Bars = &merged
	p.bars = &copy
}
