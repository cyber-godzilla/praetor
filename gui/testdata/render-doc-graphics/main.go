// Command render-doc-graphics creates the minimap and compass fixtures used by
// the documentation screenshot test. It deliberately goes through the real
// SKOOT parser and production raster renderers; the browser test only supplies
// the resulting PNGs to its fake Wails bridge.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/cyber-godzilla/praetor/internal/compass"
	"github.com/cyber-godzilla/praetor/internal/minimap"
	"github.com/cyber-godzilla/praetor/internal/protocol"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Captured SKOOT 6/10 room-and-wall frame, reduced to protocol-only data. It
// contains no player, account, command, or output text. The exit frame is the
// real SKOOT 7 sample already used by the protocol regression tests.
const (
	roomsFrame = "SKOOT 6 -10,-10,20,#ff0000,215.0,10,-10,20,#ffffff,200.0,-30,-10,20,#ffffff,204.308,-10,10,20,#ffffff,120.0,-10,-30,20,#ffffff,140.0,30,-10,20,#ffffff,120.0,30,10,20,#ffffff,120.0,30,-30,20,#ffffff,120.0,10,-30,20,#ffffff,120.0,-50,-10,20,#ffffff,175.0,-50,10,20,#ffffff,35.0,-50,-30,20,#ffffff,91.54,-30,-30,20,#ffffff,135.0,-30,10,20,#ffffff,135.0,10,-50,20,#ffffff,120.0,20,10,10,#ffffff,20.0,20,30,10,#ffffff,20.0,-70,-10,20,#ffffff,135.0,-50,-70,40,#ffffff,29.308"
	wallsFrame = "SKOOT 10 10,0,hor,1,-11,0,hor,1,0,10,ver,1,0,-11,ver,1,30,0,hor,1,30,10,nw,1,30,-10,ne,1,20,-11,ver,1,-31,0,hor,1,-30,10,ne,1,-30,-10,nw,1,-20,-11,ver,1,-20,10,ver,1,0,30,ver,0,10,-30,ne,1,50,10,nw,0,29,15,hor,1,30,30,ne,1,-51,0,hor,1,-51,20,hor,0,-40,-31,ver,1,-20,30,ver,0"
	exitsFrame = "SKOOT 7 n,show,ne,none,e,show,se,none,s,show,sw,none,w,show,nw,none,u,none,d,show"
)

func main() {
	output := flag.String("output", "gui/frontend/artifacts/ui-screenshots/graphics", "output directory")
	flag.Parse()

	rooms := mustInterpret(roomsFrame)
	walls := mustInterpret(wallsFrame)
	exits := mustInterpret(exitsFrame)
	if rooms.Rooms == nil || walls.Walls == nil || exits.Exits == nil {
		panic("SKOOT fixture did not produce rooms, walls, and exits")
	}

	mini := minimap.NewMinimap()
	mini.SetScale(0.8)
	mini.Update(rooms.Rooms, walls.Walls)
	if err := writePNG(filepath.Join(*output, "minimap.png"), mini.BuildImage()); err != nil {
		panic(err)
	}
	// Match internal/gui's current production compass render width exactly.
	if err := writePNG(filepath.Join(*output, "compass.png"), compass.BuildImage(*exits.Exits, 14)); err != nil {
		panic(err)
	}
}

func mustInterpret(line string) *types.SKOOTUpdateEvent {
	channel, payload, err := protocol.ParseSkoot(line)
	if err != nil {
		panic(err)
	}
	event := protocol.InterpretSkoot(channel, payload)
	if event == nil {
		panic(fmt.Sprintf("SKOOT channel %d fixture was rejected", channel))
	}
	return event
}

func writePNG(path string, img *image.RGBA) error {
	if img == nil {
		return fmt.Errorf("renderer returned no image for %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
