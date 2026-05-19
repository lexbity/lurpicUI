package svg

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	. "codeburg.org/lexbit/lurpicui/gfx"
)

func parsePercentOrNumber(value string) (float32, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "%") {
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")), 32)
		if err != nil {
			return 0, err
		}
		return float32(v / 100), nil
	}
	v, ok, err := parseLength(value)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("svg: empty number")
	}
	return v, nil
}

func parseLength(value string) (float32, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, nil
	}
	end := 0
	for end < len(value) && isLengthRune(value[end]) {
		end++
	}
	if end == 0 {
		return 0, false, fmt.Errorf("svg: invalid length %q", value)
	}
	v, err := strconv.ParseFloat(value[:end], 32)
	if err != nil {
		return 0, false, err
	}
	return float32(v), true, nil
}

func isLengthRune(b byte) bool {
	return (b >= '0' && b <= '9') || b == '+' || b == '-' || b == '.' || b == 'e' || b == 'E'
}

func parseViewBox(value string) (Rect, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Rect{}, false, nil
	}
	parts := parseFloatList(value)
	if len(parts) != 4 {
		return Rect{}, false, fmt.Errorf("svg: invalid viewBox %q", value)
	}
	return RectFromXYWH(parts[0], parts[1], parts[2], parts[3]), true, nil
}

func parsePreserveAspectRatio(value string) (SVGPreserveAspectRatio, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return SVGPreserveAspectRatio{Align: SVGAspectRatioAlignXMidYMid, MeetOrSlice: SVGMeetOrSliceMeet}, nil
	}
	fields := strings.Fields(value)
	switch fields[0] {
	case "none":
		return SVGPreserveAspectRatio{Align: SVGAspectRatioAlignNone, MeetOrSlice: SVGMeetOrSliceMeet}, nil
	case "xMinYMin":
		return parsePARAlign(fields, SVGAspectRatioAlignXMinYMin)
	case "xMidYMin":
		return parsePARAlign(fields, SVGAspectRatioAlignXMidYMin)
	case "xMaxYMin":
		return parsePARAlign(fields, SVGAspectRatioAlignXMaxYMin)
	case "xMinYMid":
		return parsePARAlign(fields, SVGAspectRatioAlignXMinYMid)
	case "xMidYMid":
		return parsePARAlign(fields, SVGAspectRatioAlignXMidYMid)
	case "xMaxYMid":
		return parsePARAlign(fields, SVGAspectRatioAlignXMaxYMid)
	case "xMinYMax":
		return parsePARAlign(fields, SVGAspectRatioAlignXMinYMax)
	case "xMidYMax":
		return parsePARAlign(fields, SVGAspectRatioAlignXMidYMax)
	case "xMaxYMax":
		return parsePARAlign(fields, SVGAspectRatioAlignXMaxYMax)
	default:
		return SVGPreserveAspectRatio{}, fmt.Errorf("svg: unsupported preserveAspectRatio %q", value)
	}
}

func parsePARAlign(fields []string, align SVGAspectRatioAlign) (SVGPreserveAspectRatio, error) {
	out := SVGPreserveAspectRatio{Align: align, MeetOrSlice: SVGMeetOrSliceMeet}
	if len(fields) > 1 {
		switch fields[1] {
		case "meet":
			out.MeetOrSlice = SVGMeetOrSliceMeet
		case "slice":
			out.MeetOrSlice = SVGMeetOrSliceSlice
		default:
			return SVGPreserveAspectRatio{}, fmt.Errorf("svg: unsupported preserveAspectRatio fit %q", fields[1])
		}
	}
	return out, nil
}

func parseURLReference(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "url(") || !strings.HasSuffix(value, ")") {
		return "", fmt.Errorf("svg: invalid url reference %q", value)
	}
	inner := strings.TrimSpace(value[4 : len(value)-1])
	if !strings.HasPrefix(inner, "#") {
		return "", fmt.Errorf("svg: external url references are not supported: %q", value)
	}
	inner = strings.TrimPrefix(inner, "#")
	if inner == "" {
		return "", fmt.Errorf("svg: empty url reference %q", value)
	}
	return inner, nil
}

func useReference(node *svgNode) (string, error) {
	ref := node.Attrs["href"]
	if ref == "" {
		ref = node.Attrs["xlink:href"]
	}
	if ref == "" {
		return "", errors.New("svg: use requires href")
	}
	if strings.HasPrefix(ref, "#") {
		return strings.TrimPrefix(ref, "#"), nil
	}
	if strings.HasPrefix(ref, "url(") {
		return parseURLReference(ref)
	}
	return "", fmt.Errorf("svg: external use reference %q is not supported", ref)
}

func parsePoints(value string) ([]Point, error) {
	nums := parseFloatList(value)
	if len(nums) == 0 {
		return nil, errors.New("svg: points requires coordinates")
	}
	if len(nums)%2 != 0 {
		return nil, fmt.Errorf("svg: points requires even coordinate count, got %d", len(nums))
	}
	pts := make([]Point, 0, len(nums)/2)
	for i := 0; i < len(nums); i += 2 {
		pts = append(pts, Point{X: nums[i], Y: nums[i+1]})
	}
	return pts, nil
}

func parseFloatList(value string) []float32 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	out := make([]float32, 0, 4)
	i := 0
	for i < len(value) {
		for i < len(value) && isSeparator(value[i]) {
			i++
		}
		if i >= len(value) {
			break
		}
		start := i
		if value[i] == '+' || value[i] == '-' {
			i++
		}
		digits := 0
		for i < len(value) && isDigit(value[i]) {
			i++
			digits++
		}
		if i < len(value) && value[i] == '.' {
			i++
			for i < len(value) && isDigit(value[i]) {
				i++
				digits++
			}
		}
		if digits == 0 {
			break
		}
		if i < len(value) && (value[i] == 'e' || value[i] == 'E') {
			j := i + 1
			if j < len(value) && (value[j] == '+' || value[j] == '-') {
				j++
			}
			expDigits := 0
			for j < len(value) && isDigit(value[j]) {
				j++
				expDigits++
			}
			if expDigits > 0 {
				i = j
			}
		}
		v, err := strconv.ParseFloat(value[start:i], 32)
		if err != nil {
			break
		}
		out = append(out, float32(v))
		for i < len(value) && isSeparator(value[i]) {
			i++
		}
	}
	return out
}

func isSeparator(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r' || b == ','
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func parseTransformAttr(value string) Transform {
	value = strings.TrimSpace(value)
	if value == "" {
		return Identity()
	}
	t := Identity()
	for len(value) > 0 {
		value = strings.TrimSpace(value)
		if value == "" {
			break
		}
		switch {
		case strings.HasPrefix(value, "matrix("):
			args, rest := parseFunctionArgs(value, "matrix")
			if len(args) == 6 {
				t = t.Multiply(Transform{A: args[0], B: args[2], C: args[1], D: args[3], TX: args[4], TY: args[5]})
			}
			value = rest
		case strings.HasPrefix(value, "translate("):
			args, rest := parseFunctionArgs(value, "translate")
			if len(args) == 1 {
				t = t.Multiply(Translation(args[0], 0))
			} else if len(args) >= 2 {
				t = t.Multiply(Translation(args[0], args[1]))
			}
			value = rest
		case strings.HasPrefix(value, "scale("):
			args, rest := parseFunctionArgs(value, "scale")
			if len(args) == 1 {
				t = t.Multiply(Scale(args[0], args[0]))
			} else if len(args) >= 2 {
				t = t.Multiply(Scale(args[0], args[1]))
			}
			value = rest
		case strings.HasPrefix(value, "rotate("):
			args, rest := parseFunctionArgs(value, "rotate")
			if len(args) == 1 {
				t = t.Multiply(Rotation(args[0] * math.Pi / 180))
			} else if len(args) >= 3 {
				t = t.Multiply(Translation(args[1], args[2])).Multiply(Rotation(args[0] * math.Pi / 180)).Multiply(Translation(-args[1], -args[2]))
			}
			value = rest
		case strings.HasPrefix(value, "skewX("):
			args, rest := parseFunctionArgs(value, "skewX")
			if len(args) >= 1 {
				t = t.Multiply(Transform{A: 1, B: float32(math.Tan(float64(args[0] * math.Pi / 180))), D: 1})
			}
			value = rest
		case strings.HasPrefix(value, "skewY("):
			args, rest := parseFunctionArgs(value, "skewY")
			if len(args) >= 1 {
				t = t.Multiply(Transform{A: 1, C: float32(math.Tan(float64(args[0] * math.Pi / 180))), D: 1})
			}
			value = rest
		default:
			return t
		}
	}
	return t
}

func parseFunctionArgs(value, name string) ([]float32, string) {
	prefix := name + "("
	if !strings.HasPrefix(value, prefix) {
		return nil, value
	}
	rest := value[len(prefix):]
	end := strings.IndexByte(rest, ')')
	if end < 0 {
		return nil, ""
	}
	args := parseFloatList(rest[:end])
	return args, rest[end+1:]
}
