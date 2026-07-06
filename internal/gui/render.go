package gui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"sync"

	"github.com/cyber-godzilla/praetor/internal/compass"
	"github.com/cyber-godzilla/praetor/internal/minimap"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// renderer owns the minimap state and last-known exits, and renders both the
// minimap and compass to PNG data URIs for the frontend. All access is
// serialized because SKOOT updates arrive from the client event goroutine
// while the frontend may request a re-render (e.g. on resize).
// compassBaseW is the compass render width at scale 1.0. Rendered larger than
// it typically displays so CSS downscaling stays crisp; the scale multiplier
// lets the user grow it further.
const compassBaseW = 200

type renderer struct {
	mu           sync.Mutex
	mini         minimap.Minimap
	haveExits    bool
	exits        types.Exits
	compassScale float64
}

func newRenderer() *renderer {
	return &renderer{
		mini:         minimap.NewMinimap(),
		compassScale: 1.0,
	}
}

// compassWidth returns the current compass render width in pixels.
func (r *renderer) compassWidth() int {
	w := int(float64(compassBaseW) * r.compassScale)
	if w < 60 {
		w = 60
	}
	return w
}

// setCompassScale adjusts the compass render scale.
func (r *renderer) setCompassScale(s float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s > 0 {
		r.compassScale = s
	}
}

// updateMinimap replaces the minimap's room/wall set and returns the rendered
// PNG payload, or nil if there is nothing to draw.
func (r *renderer) updateMinimap(rooms []types.MinimapRoom, walls []types.MinimapWall) *ImagePayload {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mini.Update(rooms, walls)
	return encodeImage(r.mini.BuildImage())
}

// updateExits stores the latest exit set and returns the rendered compass PNG.
func (r *renderer) updateExits(exits types.Exits) *ImagePayload {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.haveExits = true
	r.exits = exits
	return encodeImage(compass.BuildImage(exits, r.compassWidth()))
}

// setScale adjusts the minimap scale (from config / UI).
func (r *renderer) setScale(s float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mini.SetScale(s)
}

// encodeImage PNG-encodes an RGBA image into a base64 data URI payload.
// Returns nil for a nil image (nothing to render).
func encodeImage(img *image.RGBA) *ImagePayload {
	if img == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	b := img.Bounds()
	return &ImagePayload{
		DataURI: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
		Width:   b.Dx(),
		Height:  b.Dy(),
	}
}
