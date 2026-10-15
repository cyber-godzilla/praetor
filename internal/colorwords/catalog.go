package colorwords

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// StyleOption is one selectable Colorword style. Solid styles use one color;
// alternating and rainbow styles cycle Colors across the rendered text.
type StyleOption struct {
	Name   string   `json:"name"`
	Label  string   `json:"label"`
	Family string   `json:"family"`
	Kind   string   `json:"kind"`
	Colors []string `json:"colors"`
}

var (
	styleCatalogOnce sync.Once
	styleCatalog     []StyleOption
	styleLookup      map[string]StyleOption
)

// Catalog returns the complete selectable Colorword catalog. It includes the
// ordinary named colors plus the rainbow and alternating-plural special styles.
// Colors are the terminal-safe values users will actually see when selected.
func Catalog() []StyleOption {
	initStyleCatalog()
	out := make([]StyleOption, len(styleCatalog))
	for i, style := range styleCatalog {
		out[i] = style
		out[i].Colors = append([]string(nil), style.Colors...)
	}
	return out
}

// LookupStyle resolves a selectable style name case-insensitively.
func LookupStyle(name string) (StyleOption, bool) {
	initStyleCatalog()
	style, ok := styleLookup[normalizeStyleName(name)]
	if !ok {
		return StyleOption{}, false
	}
	style.Colors = append([]string(nil), style.Colors...)
	return style, true
}

// ApplyStyle colors every non-horizontal-rule segment with a named Colorword
// style while retaining the server's bold, italic, and underline attributes.
func ApplyStyle(segments []types.StyledSegment, name string) []types.StyledSegment {
	style, ok := LookupStyle(name)
	if !ok || len(style.Colors) == 0 {
		return segments
	}
	if style.Kind == "solid" {
		result := make([]types.StyledSegment, len(segments))
		copy(result, segments)
		for i := range result {
			if !result[i].IsHR && result[i].Text != "" {
				result[i].Color = style.Colors[0]
			}
		}
		return result
	}

	var result []types.StyledSegment
	colorIndex := 0
	for _, segment := range segments {
		if segment.IsHR || segment.Text == "" {
			result = append(result, segment)
			continue
		}
		for _, character := range segment.Text {
			result = append(result, types.StyledSegment{
				Text:      string(character),
				Bold:      segment.Bold,
				Italic:    segment.Italic,
				Underline: segment.Underline,
				Color:     style.Colors[colorIndex%len(style.Colors)],
			})
			if !unicode.IsSpace(character) && character != '-' {
				colorIndex++
			}
		}
	}
	return result
}

func initStyleCatalog() {
	styleCatalogOnce.Do(func() {
		styleLookup = make(map[string]StyleOption, len(colorWordMap)+len(colorPluralMap)+len(rainbowWords))
		for name, color := range colorWordMap {
			addStyle(StyleOption{
				Name: name, Label: styleLabel(name), Family: colorFamily(color),
				Kind: "solid", Colors: []string{brightenIfDark(color)},
			})
		}
		for name, shades := range colorPluralMap {
			addStyle(StyleOption{
				Name: name, Label: styleLabel(name), Family: "Special",
				Kind: "alternating", Colors: []string{brightenIfDark(shades[0]), brightenIfDark(shades[1])},
			})
		}
		for name := range rainbowWords {
			addStyle(StyleOption{
				Name: name, Label: styleLabel(name), Family: "Special",
				Kind: "rainbow", Colors: append([]string(nil), rainbowColors[:7]...),
			})
		}
		sort.Slice(styleCatalog, func(i, j int) bool {
			if styleCatalog[i].Family != styleCatalog[j].Family {
				return familyRank(styleCatalog[i].Family) < familyRank(styleCatalog[j].Family)
			}
			return styleCatalog[i].Label < styleCatalog[j].Label
		})
	})
}

func addStyle(style StyleOption) {
	style.Name = normalizeStyleName(style.Name)
	styleCatalog = append(styleCatalog, style)
	styleLookup[style.Name] = style
}

func normalizeStyleName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func styleLabel(name string) string {
	runes := []rune(strings.TrimSpace(name))
	upperNext := true
	for i, r := range runes {
		if upperNext && unicode.IsLetter(r) {
			runes[i] = unicode.ToUpper(r)
			upperNext = false
		}
		if unicode.IsSpace(r) || r == '-' {
			upperNext = true
		}
	}
	return string(runes)
}

// colorFamily groups the dictionary by visible hue rather than by the source
// data's historical sections. That makes a family query such as "red" include
// Crimson, Scarlet, Ruby, and compound red names alike.
func colorFamily(hexColor string) string {
	raw := strings.TrimPrefix(hexColor, "#")
	if len(raw) != 6 {
		return "Other"
	}
	value, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return "Other"
	}
	r := float64((value>>16)&0xff) / 255
	g := float64((value>>8)&0xff) / 255
	b := float64(value&0xff) / 255
	maxValue := max(r, g, b)
	minValue := min(r, g, b)
	delta := maxValue - minValue
	lightness := (maxValue + minValue) / 2
	saturation := 0.0
	if delta != 0 {
		if lightness > 0.5 {
			saturation = delta / (2 - maxValue - minValue)
		} else {
			saturation = delta / (maxValue + minValue)
		}
	}
	if lightness < 0.16 {
		return "Blacks"
	}
	if lightness > 0.76 && saturation < 0.35 {
		return "Whites"
	}
	if saturation < 0.14 {
		return "Grays"
	}

	hue := 0.0
	switch maxValue {
	case r:
		hue = 60 * ((g - b) / delta)
	case g:
		hue = 60 * (2 + (b-r)/delta)
	case b:
		hue = 60 * (4 + (r-g)/delta)
	}
	if hue < 0 {
		hue += 360
	}
	if hue >= 15 && hue < 50 && lightness < 0.40 {
		return "Browns"
	}
	switch {
	case hue < 15 || hue >= 345:
		return "Reds"
	case hue < 45:
		return "Oranges"
	case hue < 70:
		return "Yellows"
	case hue < 165:
		return "Greens"
	case hue < 250:
		return "Blues"
	case hue < 290:
		return "Purples"
	default:
		return "Pinks"
	}
}

func familyRank(family string) int {
	order := []string{"Reds", "Oranges", "Yellows", "Greens", "Blues", "Purples", "Pinks", "Browns", "Grays", "Whites", "Blacks", "Special", "Other"}
	for i, candidate := range order {
		if candidate == family {
			return i
		}
	}
	return len(order)
}
