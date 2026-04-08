package protocol

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// SKOOT channel IDs (sequence numbers identify the data type).
const (
	SkootChannelHelp     = 5  // help URL to open in browser
	SkootChannelMinimap  = 6  // room positions, sizes, colors, brightness
	SkootChannelExits    = 7  // direction,visibility pairs
	SkootChannelStatus   = 8  // "Health,80" / "Fatigue,28" etc.
	SkootChannelLighting = 9  // brightness value (0-255)
	SkootChannelWalls    = 10 // wall/door segments between rooms
)

// ParseSkoot extracts the sequence number and payload from a SKOOT protocol
// line. The expected format is "SKOOT <seq> <payload>".
func ParseSkoot(line string) (seq int, payload string, err error) {
	if !strings.HasPrefix(line, "SKOOT ") {
		return 0, "", fmt.Errorf("not a SKOOT line: %q", line)
	}
	rest := line[6:] // strip "SKOOT "
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		return 0, "", fmt.Errorf("SKOOT line missing payload: %q", line)
	}
	seqStr := rest[:spaceIdx]
	seq, err = strconv.Atoi(seqStr)
	if err != nil {
		return 0, "", fmt.Errorf("SKOOT sequence not numeric: %q", seqStr)
	}
	payload = rest[spaceIdx+1:]
	return seq, payload, nil
}

// InterpretSkoot parses a SKOOT payload into a SKOOTUpdateEvent based on
// the channel (sequence number). Returns nil for unrecognized channels.
func InterpretSkoot(seq int, payload string) *types.SKOOTUpdateEvent {
	switch seq {
	case SkootChannelHelp:
		return &types.SKOOTUpdateEvent{
			Channel:    seq,
			RawPayload: payload,
			HelpURL:    strings.TrimSpace(payload),
		}
	case SkootChannelExits:
		return parseExits(payload)
	case SkootChannelStatus:
		return parseStatus(payload)
	case SkootChannelLighting:
		return parseLighting(payload)
	case SkootChannelMinimap:
		return parseMinimap(payload)
	case SkootChannelWalls:
		return parseWalls(payload)
	default:
		return nil
	}
}

// parseMinimap interprets room data from SKOOT channel 6.
// Format: groups of 5 comma-separated values: x,y,size,#color,brightness
// Example: "0,0,10,#ff0000,19.56,0,-10,10,#ffffff,37.8"
func parseMinimap(payload string) *types.SKOOTUpdateEvent {
	parts := strings.Split(payload, ",")
	if len(parts) < 5 || len(parts)%5 != 0 {
		return nil
	}
	rooms := make([]types.MinimapRoom, 0, len(parts)/5)
	for i := 0; i+4 < len(parts); i += 5 {
		x, err := strconv.Atoi(parts[i])
		if err != nil {
			return nil
		}
		y, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return nil
		}
		size, err := strconv.Atoi(parts[i+2])
		if err != nil {
			return nil
		}
		color := parts[i+3]
		brightness, err := strconv.ParseFloat(parts[i+4], 64)
		if err != nil {
			return nil
		}
		rooms = append(rooms, types.MinimapRoom{
			X:          x,
			Y:          y,
			Size:       size,
			Color:      color,
			Brightness: brightness,
		})
	}
	return &types.SKOOTUpdateEvent{Rooms: rooms}
}

// parseWalls interprets wall/door segment data from SKOOT channel 10.
// Format: groups of 4 comma-separated values: x,y,type,accessible
// Example: "5,10,ver,0,5,-1,ver,1"
// accessible: 1 = accessible/passable (Orchil draws white), 0 = blocked (Orchil draws black)
func parseWalls(payload string) *types.SKOOTUpdateEvent {
	parts := strings.Split(payload, ",")
	if len(parts) < 4 || len(parts)%4 != 0 {
		return nil
	}
	walls := make([]types.MinimapWall, 0, len(parts)/4)
	for i := 0; i+3 < len(parts); i += 4 {
		x, err := strconv.Atoi(parts[i])
		if err != nil {
			return nil
		}
		y, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return nil
		}
		wallType := parts[i+2]
		passableInt, err := strconv.Atoi(parts[i+3])
		if err != nil {
			return nil
		}
		walls = append(walls, types.MinimapWall{
			X:        x,
			Y:        y,
			Type:     wallType,
			Passable: passableInt == 1,
		})
	}
	return &types.SKOOTUpdateEvent{Walls: walls}
}

// parseExits interprets exit data in the format:
// "n,show,ne,none,e,show,se,none,s,show,sw,none,w,show,nw,none,u,none,d,show"
// Each pair is direction,visibility where "show" means available and "none" means unavailable.
func parseExits(payload string) *types.SKOOTUpdateEvent {
	exits := &types.Exits{}
	parts := strings.Split(payload, ",")
	// Process in pairs: direction, visibility
	for i := 0; i+1 < len(parts); i += 2 {
		dir := strings.TrimSpace(parts[i])
		vis := strings.TrimSpace(parts[i+1])
		available := vis == "show"
		switch dir {
		case "n":
			exits.North = available
		case "ne":
			exits.Northeast = available
		case "e":
			exits.East = available
		case "se":
			exits.Southeast = available
		case "s":
			exits.South = available
		case "sw":
			exits.Southwest = available
		case "w":
			exits.West = available
		case "nw":
			exits.Northwest = available
		case "u":
			exits.Up = available
		case "d":
			exits.Down = available
		}
	}
	return &types.SKOOTUpdateEvent{Exits: exits}
}

// parseStatus interprets status bar data in the format "Health,80" or "Fatigue,28".
func parseStatus(payload string) *types.SKOOTUpdateEvent {
	parts := strings.SplitN(payload, ",", 2)
	if len(parts) != 2 {
		return nil
	}
	name := parts[0]
	value, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}

	ev := &types.SKOOTUpdateEvent{}
	switch name {
	case "Health":
		ev.Health = &value
	case "Fatigue":
		ev.Fatigue = &value
	case "Encumbrance":
		ev.Encumbrance = &value
	case "Satiation":
		ev.Satiation = &value
	default:
		return nil
	}
	return ev
}

// parseLighting interprets the environment lighting value.
// Observed values: 150+ (bright outdoor), 61 (bright indoor), 24 (dim), 15 (dark).
func parseLighting(payload string) *types.SKOOTUpdateEvent {
	value, err := strconv.Atoi(strings.TrimSpace(payload))
	if err != nil {
		return nil
	}
	if value < 0 {
		value = 0
	}

	var level types.LightingLevel
	switch {
	case value >= 122:
		level = types.LightBlindinglyBright
	case value >= 86:
		level = types.LightVeryBright
	case value >= 46:
		level = types.LightBright
	case value >= 26:
		level = types.LightFairlyLit
	case value >= 6:
		level = types.LightSomewhatDark
	case value >= 3:
		level = types.LightVeryDark
	case value >= 1:
		level = types.LightExtremelyDark
	default:
		level = types.LightPitchBlack
	}
	return &types.SKOOTUpdateEvent{Lighting: &level, LightingRaw: value}
}
