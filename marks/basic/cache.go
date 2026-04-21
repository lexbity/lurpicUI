package basic

import (
	"fmt"
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

var (
	textRegistryMu      sync.RWMutex
	defaultTextRegistry *text.FontRegistry
	textLayoutCacheMu   sync.Mutex
	textLayoutCache     = make(map[string]*text.TextLayout)
	imageFitCacheMu     sync.Mutex
	imageFitCache       = make(map[string]fitRect)
)

type fitRect struct {
	content gfx.Rect
	hit     gfx.Rect
}

// SetTextRegistry installs the package default font registry used by Text marks.
func SetTextRegistry(reg *text.FontRegistry) {
	textRegistryMu.Lock()
	defaultTextRegistry = reg
	textRegistryMu.Unlock()
}

func currentTextRegistry() *text.FontRegistry {
	textRegistryMu.RLock()
	defer textRegistryMu.RUnlock()
	return defaultTextRegistry
}

func textLayoutCacheKey(paragraph text.Paragraph, style text.TextStyle, maxWidth float32, selectable bool, registry *text.FontRegistry) string {
	return fmt.Sprintf("%p|%q|%g|%d|%g|%g|%d|%t|%p",
		registry,
		paragraphKey(paragraph),
		style.Size,
		style.Weight,
		style.LineHeight,
		maxWidth,
		paragraph.Alignment,
		selectable,
		registry,
	)
}

func paragraphKey(p text.Paragraph) string {
	if len(p.Spans) == 0 {
		return ""
	}
	out := ""
	for _, span := range p.Spans {
		out += span.Text + "\x00" + span.Style.Family + "\x01"
		out += fmt.Sprintf("%g/%d/%d/%g/%g/%d;", span.Style.Size, span.Style.Weight, span.Style.Style, span.Style.LineHeight, span.Style.LetterSpacing, span.Style.TabWidth)
	}
	return out
}

func cachedTextLayout(key string, build func() *text.TextLayout) *text.TextLayout {
	textLayoutCacheMu.Lock()
	defer textLayoutCacheMu.Unlock()
	if layout := textLayoutCache[key]; layout != nil {
		return layout
	}
	layout := build()
	textLayoutCache[key] = layout
	return layout
}

func lookupTextLayout(key string) (*text.TextLayout, bool) {
	textLayoutCacheMu.Lock()
	defer textLayoutCacheMu.Unlock()
	layout, ok := textLayoutCache[key]
	return layout, ok
}

func cachedImageFit(key string, build func() fitRect) fitRect {
	imageFitCacheMu.Lock()
	defer imageFitCacheMu.Unlock()
	if rect, ok := imageFitCache[key]; ok {
		return rect
	}
	rect := build()
	imageFitCache[key] = rect
	return rect
}
