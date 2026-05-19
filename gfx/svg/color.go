package svg

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	. "codeburg.org/lexbit/lurpicui/gfx"
)

func parsePaint(value string) (SVGPaint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return SVGPaint{}, nil
	}
	switch value {
	case "none":
		return SVGPaint{Kind: SVGPaintNone, Opacity: 1}, nil
	case "currentColor":
		return SVGPaint{Kind: SVGPaintCurrentColor, Opacity: 1}, nil
	}
	if strings.HasPrefix(value, "url(") {
		ref, err := parseURLReference(value)
		if err != nil {
			return SVGPaint{}, err
		}
		return SVGPaint{Kind: SVGPaintLinearGradient, Gradient: &SVGGradient{ID: ref}, Opacity: 1}, nil
	}
	color, err := parseColor(value)
	if err != nil {
		return SVGPaint{}, err
	}
	return SVGPaint{Kind: SVGPaintColor, Color: color, Opacity: 1}, nil
}

func parseColor(value string) (Color, error) {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "#"):
		return parseHexColor(value)
	case strings.HasPrefix(value, "rgb(") && strings.HasSuffix(value, ")"):
		parts := strings.Split(value[4:len(value)-1], ",")
		if len(parts) != 3 {
			return Color{}, fmt.Errorf("svg: invalid rgb color %q", value)
		}
		r, err := parseByteComponent(parts[0])
		if err != nil {
			return Color{}, err
		}
		g, err := parseByteComponent(parts[1])
		if err != nil {
			return Color{}, err
		}
		b, err := parseByteComponent(parts[2])
		if err != nil {
			return Color{}, err
		}
		return ColorFromRGBA8(r, g, b, 255), nil
	default:
		return Color{}, fmt.Errorf("svg: unsupported color %q", value)
	}
}

func parseHexColor(value string) (Color, error) {
	s := strings.TrimPrefix(value, "#")
	switch len(s) {
	case 3:
		r := duplicateHexNibble(s[0])
		g := duplicateHexNibble(s[1])
		b := duplicateHexNibble(s[2])
		return ColorFromRGBA8(r, g, b, 255), nil
	case 4:
		r := duplicateHexNibble(s[0])
		g := duplicateHexNibble(s[1])
		b := duplicateHexNibble(s[2])
		a := duplicateHexNibble(s[3])
		return ColorFromRGBA8(r, g, b, a), nil
	case 6:
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return Color{}, fmt.Errorf("svg: invalid color %q", value)
		}
		return ColorFromHex(uint32(v<<8) | 0xFF), nil
	case 8:
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return Color{}, fmt.Errorf("svg: invalid color %q", value)
		}
		return ColorFromHex(uint32(v)), nil
	default:
		return Color{}, fmt.Errorf("svg: invalid hex color %q", value)
	}
}

func duplicateHexNibble(b byte) uint8 {
	v := hexNibble(b)
	return uint8(v<<4 | v)
}

func hexNibble(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	default:
		return 0
	}
}

func parseByteComponent(s string) (uint8, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(math.Round(v)), nil
}

func parseOpacity(value string) (float32, error) {
	v, ok, err := parseLength(value)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("svg: empty opacity")
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v, nil
}
