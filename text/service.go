package text

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
)

const (
	DefaultMaxSourceBytes = 64 << 10
	DefaultMaxGlyphs      = 65536
	DefaultMaxLines       = 4096
	MaxFontFamilyBytes    = 256
)

type Overflow uint8

const (
	OverflowNone Overflow = iota
	OverflowWrap
	OverflowTruncate
)

// ShapeRequest contains all request-specific shaping state. Scale and
// direction are values, never mutable shaper settings.
type ShapeRequest struct {
	Content   string
	Style     TextStyle
	MaxWidth  float32
	Scale     float32
	Direction di.Direction
	Overflow  Overflow
	Locale    string
	Alignment TextAlignment
	Paragraph *Paragraph
}

type Limits struct {
	MaxSourceBytes    int
	MaxGlyphs         int
	MaxLines          int
	MaxParagraphSpans int
	MaxLayouts        int
	MaxGlyphEntries   int
	MaxFontFaces      int
}

func DefaultLimits(profile membudget.Profile) Limits {
	if profile == membudget.Compact {
		return Limits{MaxSourceBytes: DefaultMaxSourceBytes, MaxGlyphs: DefaultMaxGlyphs, MaxLines: DefaultMaxLines, MaxParagraphSpans: 4096, MaxLayouts: 2048, MaxGlyphEntries: 8192, MaxFontFaces: 16}
	}
	return Limits{MaxSourceBytes: DefaultMaxSourceBytes, MaxGlyphs: DefaultMaxGlyphs, MaxLines: DefaultMaxLines, MaxParagraphSpans: 4096, MaxLayouts: 8192, MaxGlyphEntries: 32768, MaxFontFaces: 64}
}

type ResourceError struct {
	Resource string
	Limit    int
	Actual   uint64
}

func (e *ResourceError) Error() string {
	return fmt.Sprintf("text: %s exceeds limit %d (got %d)", e.Resource, e.Limit, e.Actual)
}

type cacheEntry struct {
	key    string
	layout *TextLayout
	bytes  uint64
	glyphs int
	pins   int
	elem   *list.Element
	charge *membudget.Reservation
}

// Service serializes access to one reusable shaping engine and owns its
// immutable layout cache. Callers release leases when replacing a layout.
type Service struct {
	mu                 sync.Mutex
	registry           *FontRegistry
	shaper             *Shaper
	wallet             *membudget.Wallet
	limits             Limits
	entries            map[string]*cacheEntry
	lru                list.List
	cacheBytes         uint64
	cacheGlyphEntries  int
	fontRegistryClosed bool
	closed             bool
	runtimeBound       bool
	hits, misses       uint64
}

// ShapeSession binds one immutable content scale to a runtime-owned service.
// It is a value request context; it does not change shared service state.
type ShapeSession struct {
	service    *Service
	scale      float32
	leaseOwner LayoutLeaseOwner
}

// LayoutLeaseOwner retains service-produced layouts for the same lifetime as
// their runtime owner. It is implemented by the facet root without coupling
// text to the runtime package.
type LayoutLeaseOwner interface {
	KeepTextLease(*LayoutLease)
}

func (s *Service) AtScale(scale float32, owner LayoutLeaseOwner) (*ShapeSession, error) {
	if s == nil {
		return nil, errors.New("text: nil service")
	}
	if !finite(scale) || scale <= 0 {
		return nil, errors.New("text: scale must be finite and positive")
	}
	return &ShapeSession{service: s, scale: scale, leaseOwner: owner}, nil
}

func (s *ShapeSession) ShapeSimple(content string, style TextStyle) *TextLayout {
	return s.shape(ShapeRequest{Content: content, Style: style, Scale: s.scale})
}
func (s *ShapeSession) ShapeTruncated(content string, style TextStyle, width float32) *TextLayout {
	return s.shape(ShapeRequest{Content: content, Style: style, MaxWidth: width, Scale: s.scale, Overflow: OverflowTruncate})
}
func (s *ShapeSession) Shape(p Paragraph) *TextLayout {
	return s.shape(ShapeRequest{Paragraph: &p, Scale: s.scale, MaxWidth: p.MaxWidth, Direction: p.Direction, Alignment: p.Alignment, Overflow: OverflowWrap})
}

func (s *ShapeSession) shape(request ShapeRequest) *TextLayout {
	if s == nil || s.service == nil {
		panic("text: shape session is nil")
	}
	lease, err := s.service.Shape(request)
	if err != nil {
		panic(err)
	}
	if s.leaseOwner != nil {
		s.leaseOwner.KeepTextLease(lease)
		return lease.Layout
	}
	layout := lease.Layout
	lease.Release()
	return layout
}

func NewService(registry *FontRegistry, limits Limits) (*Service, error) {
	if registry == nil {
		return nil, errors.New("text: font registry is required")
	}
	if limits.MaxSourceBytes <= 0 || limits.MaxGlyphs <= 0 || limits.MaxLines <= 0 || limits.MaxParagraphSpans <= 0 || limits.MaxLayouts <= 0 || limits.MaxGlyphEntries <= 0 || limits.MaxFontFaces <= 0 {
		return nil, errors.New("text: all service limits must be positive")
	}
	if registry.FaceCount() > limits.MaxFontFaces {
		return nil, &ResourceError{"font faces", limits.MaxFontFaces, nonnegativeUint64(registry.FaceCount())}
	}
	if err := registry.setFaceLimit(limits.MaxFontFaces); err != nil {
		return nil, err
	}
	return &Service{registry: registry, shaper: newShaper(registry), wallet: registry.wallet, limits: limits, entries: make(map[string]*cacheEntry)}, nil
}

// LayoutLease pins one immutable cached layout until Release.
type LayoutLease struct {
	Layout   *TextLayout
	service  *Service
	entry    *cacheEntry
	released bool
}

func (l *LayoutLease) Release() {
	if l == nil || l.service == nil {
		return
	}
	s := l.service
	s.mu.Lock()
	defer s.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	if l.entry != nil && l.entry.pins > 0 {
		l.entry.pins--
		if s.closed && l.entry.pins == 0 {
			delete(s.entries, l.entry.key)
			s.lru.Remove(l.entry.elem)
			s.cacheBytes -= l.entry.bytes
			s.cacheGlyphEntries -= l.entry.glyphs
			l.entry.layout = nil
			_ = l.entry.charge.Release()
			s.releaseFontChargeIfClosedLocked()
		}
	}
	l.Layout = nil
	l.entry = nil
	l.service = nil
}

func (s *Service) Shape(r ShapeRequest) (*LayoutLease, error) {
	if s == nil {
		return nil, errors.New("text: nil service")
	}
	if len(r.Content) > s.limits.MaxSourceBytes {
		return nil, &ResourceError{"source bytes", s.limits.MaxSourceBytes, nonnegativeUint64(len(r.Content))}
	}
	if !utf8.ValidString(r.Content) {
		return nil, errors.New("text: content is not valid UTF-8")
	}
	if len(r.Locale) > 128 {
		return nil, errors.New("text: locale identifier exceeds 128 bytes")
	}
	if !finite(r.Scale) || r.Scale <= 0 {
		return nil, errors.New("text: scale must be finite and positive")
	}
	if !finite(r.MaxWidth) || r.MaxWidth < 0 {
		return nil, errors.New("text: max width must be finite and nonnegative")
	}
	if err := validateStyle(r.Style); err != nil {
		return nil, err
	}
	if r.Paragraph != nil {
		if len(r.Paragraph.Spans) > s.limits.MaxParagraphSpans {
			return nil, &ResourceError{"paragraph spans", s.limits.MaxParagraphSpans, nonnegativeUint64(len(r.Paragraph.Spans))}
		}
		if !finite(r.Paragraph.MaxWidth) || r.Paragraph.MaxWidth < 0 {
			return nil, errors.New("text: paragraph max width must be finite and nonnegative")
		}
		total := 0
		for _, span := range r.Paragraph.Spans {
			if !utf8.ValidString(span.Text) {
				return nil, errors.New("text: paragraph span is not valid UTF-8")
			}
			if err := validateStyle(span.Style); err != nil {
				return nil, err
			}
			if len(span.Text) > s.limits.MaxSourceBytes-total {
				total = s.limits.MaxSourceBytes + 1
				return nil, &ResourceError{"source bytes", s.limits.MaxSourceBytes, nonnegativeUint64(total)}
			}
			total += len(span.Text)
		}
		paragraph := cloneParagraph(*r.Paragraph)
		r.Paragraph = &paragraph
	}
	key := s.key(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("text: service is closed")
	}
	if entry := s.entries[key]; entry != nil {
		entry.pins++
		s.lru.MoveToFront(entry.elem)
		s.hits++
		return &LayoutLease{Layout: entry.layout, service: s, entry: entry}, nil
	}
	s.misses++
	scratchBytes, err := shapeScratchBytes(s.limits)
	if err != nil {
		return nil, err
	}
	var scratchReservation *membudget.Reservation
	for scratchReservation, err = s.wallet.Reserve(scratchBytes); err != nil; scratchReservation, err = s.wallet.Reserve(scratchBytes) {
		if !s.evictOne() {
			cap, _, _, _ := s.wallet.Snapshot()
			return nil, &ResourceError{"shaping scratch", nonnegativeInt(cap), scratchBytes}
		}
	}
	defer func() { _ = scratchReservation.Cancel() }()
	s.shaper.contentScale = r.Scale
	p := Paragraph{Spans: []TextSpan{{Text: r.Content, Style: r.Style, Direction: r.Direction}}, MaxWidth: r.MaxWidth, Direction: r.Direction, Alignment: r.Alignment}
	if r.Locale != "" {
		p.Language = language.NewLanguage(r.Locale)
		p.Spans[0].Language = p.Language
	}
	if r.Paragraph != nil {
		p = cloneParagraph(*r.Paragraph)
	}
	var shaped *TextLayout
	switch r.Overflow {
	case OverflowWrap:
		shaped = s.shaper.Shape(p)
	case OverflowTruncate:
		if len(p.Spans) == 1 {
			shaped = s.shaper.ShapeTruncated(p.Spans[0].Text, p.Spans[0].Style, p.MaxWidth)
		} else {
			shaped = s.shaper.Shape(p)
		}
	case OverflowNone:
		shaped = s.shaper.Shape(p)
	default:
		return nil, errors.New("text: invalid overflow mode")
	}
	if shaped == nil {
		return nil, errors.New("text: shaping failed")
	}
	glyphs, lines := layoutCounts(shaped)
	if glyphs > s.limits.MaxGlyphs {
		return nil, &ResourceError{"output glyphs", s.limits.MaxGlyphs, nonnegativeUint64(glyphs)}
	}
	if lines > s.limits.MaxLines {
		return nil, &ResourceError{"output lines", s.limits.MaxLines, nonnegativeUint64(lines)}
	}
	cacheBytes, err := estimateLayoutBytes(shaped, uint64(len(key)))
	if err != nil {
		return nil, err
	}
	if cacheBytes == 0 {
		cacheBytes = 1
	}
	if err := s.admit(key, shaped, cacheBytes); err != nil {
		return nil, err
	}
	entry := s.entries[key]
	entry.pins++
	return &LayoutLease{Layout: entry.layout, service: s, entry: entry}, nil
}

func (s *Service) ShapeSimple(content string, style TextStyle, scale float32) (*TextLayout, error) {
	lease, err := s.Shape(ShapeRequest{Content: content, Style: style, Scale: scale})
	if err != nil {
		return nil, err
	}
	layout := lease.Layout
	lease.Release()
	return layout, nil
}

func (s *Service) ShapeTruncated(content string, style TextStyle, maxWidth, scale float32) (*TextLayout, error) {
	lease, err := s.Shape(ShapeRequest{Content: content, Style: style, MaxWidth: maxWidth, Scale: scale, Overflow: OverflowTruncate})
	if err != nil {
		return nil, err
	}
	layout := lease.Layout
	lease.Release()
	return layout, nil
}

func (s *Service) ShapeParagraph(p Paragraph, scale float32) (*TextLayout, error) {
	lease, err := s.Shape(ShapeRequest{Scale: scale, Paragraph: &p, MaxWidth: p.MaxWidth, Direction: p.Direction, Alignment: p.Alignment, Overflow: OverflowWrap})
	if err != nil {
		return nil, err
	}
	layout := lease.Layout
	lease.Release()
	return layout, nil
}

func (s *Service) admit(key string, layout *TextLayout, bytes uint64) error {
	glyphs, _ := layoutCounts(layout)
	if glyphs > s.limits.MaxGlyphEntries {
		return &ResourceError{"cached glyph entries", s.limits.MaxGlyphEntries, nonnegativeUint64(glyphs)}
	}
	for len(s.entries) >= s.limits.MaxLayouts || s.cacheBytes > math.MaxUint64-bytes || glyphs > s.limits.MaxGlyphEntries-s.cacheGlyphEntries {
		if !s.evictOne() {
			return &ResourceError{"layout cache entries", s.limits.MaxLayouts, nonnegativeUint64(len(s.entries) + 1)}
		}
	}
	reservation, err := s.wallet.Reserve(bytes)
	for err != nil {
		if !s.evictOne() {
			cap, _, _, _ := s.wallet.Snapshot()
			return &ResourceError{"layout cache bytes", nonnegativeInt(cap), bytes}
		}
		reservation, err = s.wallet.Reserve(bytes)
	}
	if err := reservation.Publish(); err != nil {
		_ = reservation.Cancel()
		return err
	}
	entry := &cacheEntry{key: key, layout: layout, bytes: bytes, glyphs: glyphs, pins: 0, charge: reservation}
	entry.elem = s.lru.PushFront(entry)
	s.entries[key] = entry
	s.cacheBytes += bytes
	s.cacheGlyphEntries += glyphs
	return nil
}

func (s *Service) evictOne() bool {
	for el := s.lru.Back(); el != nil; el = el.Prev() {
		e := el.Value.(*cacheEntry)
		if e.pins != 0 {
			continue
		}
		delete(s.entries, e.key)
		s.lru.Remove(el)
		s.cacheBytes -= e.bytes
		s.cacheGlyphEntries -= e.glyphs
		e.layout = nil
		_ = e.charge.Release()
		return true
	}
	return false
}

func (s *Service) Trim(targetBytes uint64) uint64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.cacheBytes
	for s.cacheBytes > targetBytes && s.evictOne() {
	}
	return before - s.cacheBytes
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	for el := s.lru.Back(); el != nil; {
		prev := el.Prev()
		e := el.Value.(*cacheEntry)
		if e.pins == 0 {
			delete(s.entries, e.key)
			s.lru.Remove(el)
			s.cacheBytes -= e.bytes
			s.cacheGlyphEntries -= e.glyphs
			e.layout = nil
			if err := e.charge.Release(); err != nil {
				return err
			}
		}
		el = prev
	}
	s.releaseFontChargeIfClosedLocked()
	return nil
}

func (s *Service) releaseFontChargeIfClosedLocked() {
	if !s.closed || len(s.entries) != 0 || s.fontRegistryClosed {
		return
	}
	if err := s.registry.close(); err != nil {
		panic(fmt.Errorf("text: close font registry: %w", err))
	}
	s.fontRegistryClosed = true
}

func (s *Service) Stats() (hits, misses uint64, entries int, cacheBytes uint64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits, s.misses, len(s.entries), s.cacheBytes
}

func (s *Service) Registry() *FontRegistry {
	if s == nil {
		return nil
	}
	return s.registry
}

func (s *Service) BindRuntime() error {
	if s == nil {
		return errors.New("text: nil service")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("text: cannot bind a closed service")
	}
	if s.runtimeBound {
		return errors.New("text: service is already bound to a runtime")
	}
	s.runtimeBound = true
	return nil
}

func (s *Service) key(r ShapeRequest) string {
	var key bytes.Buffer
	put := func(v uint64) { var b [8]byte; binary.LittleEndian.PutUint64(b[:], v); _, _ = key.Write(b[:]) }
	writeString := func(v string) { put(uint64(len(v))); _, _ = key.WriteString(v) }
	put(s.registry.Generation())
	writeString(r.Content)
	writeString(r.Locale)
	put(uint64(math.Float32bits(r.MaxWidth)))
	put(uint64(math.Float32bits(r.Scale)))
	put(uint64(r.Direction))
	put(uint64(r.Overflow))
	put(uint64(r.Alignment))
	writeStyle := func(st TextStyle) {
		writeString(st.Family)
		put(uint64(math.Float32bits(st.Size)))
		put(uint64(st.Weight))
		put(uint64(st.Style))
		put(uint64(math.Float32bits(st.LineHeight)))
		put(uint64(math.Float32bits(st.LetterSpacing)))
		writeString(strconv.Itoa(st.TabWidth))
	}
	writeStyle(r.Style)
	if r.Paragraph != nil {
		put(1)
		put(uint64(math.Float32bits(r.Paragraph.MaxWidth)))
		put(uint64(r.Paragraph.Alignment))
		put(uint64(r.Paragraph.Direction))
		writeString(fmt.Sprint(r.Paragraph.Language))
		writeString(fmt.Sprint(r.Paragraph.Script))
		put(uint64(len(r.Paragraph.Spans)))
		for _, sp := range r.Paragraph.Spans {
			writeString(sp.Text)
			writeStyle(sp.Style)
			put(uint64(sp.Direction))
			writeString(fmt.Sprint(sp.Language))
			writeString(fmt.Sprint(sp.Script))
		}
	} else {
		put(0)
	}
	return key.String()
}

func finite(v float32) bool                { return !float32NaN(v) && !float32Inf(v) }
func float32NaN(v float32) bool            { return v != v }
func float32Inf(v float32) bool            { return math.IsInf(float64(v), 0) }
func cloneParagraph(p Paragraph) Paragraph { p.Spans = append([]TextSpan(nil), p.Spans...); return p }
func nonnegativeUint64(v int) uint64 {
	if v <= 0 {
		return 0
	}
	return uint64(v)
}
func nonnegativeInt(v uint64) int {
	max := uint64(^uint(0) >> 1)
	if v > max {
		return int(max)
	}
	return int(v)
}
func validateStyle(style TextStyle) error {
	if len(style.Family) > MaxFontFamilyBytes {
		return &ResourceError{"font family bytes", MaxFontFamilyBytes, nonnegativeUint64(len(style.Family))}
	}
	if !finite(style.Size) || !finite(style.LineHeight) || !finite(style.LetterSpacing) {
		return errors.New("text: style metrics must be finite")
	}
	return nil
}
func shapeScratchBytes(l Limits) (uint64, error) {
	const sourceMultiplier, glyphMultiplier, lineMultiplier = uint64(8), uint64(40), uint64(256)
	parts := [][2]uint64{
		{nonnegativeUint64(l.MaxSourceBytes), sourceMultiplier},
		{nonnegativeUint64(l.MaxGlyphs), glyphMultiplier},
		{nonnegativeUint64(l.MaxLines), lineMultiplier},
	}
	total := uint64(64 << 10)
	for _, part := range parts {
		if part[0] > math.MaxUint64/part[1] {
			return 0, errors.New("text: shaping scratch limit overflows uint64")
		}
		amount := part[0] * part[1]
		if total > math.MaxUint64-amount {
			return 0, errors.New("text: shaping scratch limit overflows uint64")
		}
		total += amount
	}
	return total, nil
}
func layoutCounts(l *TextLayout) (glyphs, lines int) {
	if l == nil {
		return
	}
	lines = len(l.Lines)
	for _, line := range l.Lines {
		for _, run := range line.Runs {
			glyphs += len(run.Glyphs)
		}
	}
	return
}
func estimateLayoutBytes(l *TextLayout, keyBytes uint64) (uint64, error) {
	if l == nil {
		return 0, nil
	}
	n := keyBytes
	add := func(amount uint64) error {
		if n > math.MaxUint64-amount {
			return errors.New("text: layout reservation overflows uint64")
		}
		n += amount
		return nil
	}
	if err := add(512); err != nil {
		return 0, err
	}
	if err := add(nonnegativeUint64(len(l.Source))); err != nil {
		return 0, err
	}
	if err := add(nonnegativeUint64(len(l.source))); err != nil {
		return 0, err
	}
	for _, line := range l.Lines {
		lineBytes := uint64(192)
		for _, part := range [][2]uint64{{nonnegativeUint64(len(line.Runs)), 128}, {nonnegativeUint64(len(line.clusterMap)), 4}} {
			if part[0] > math.MaxUint64/part[1] {
				return 0, errors.New("text: line reservation overflows uint64")
			}
			if lineBytes > math.MaxUint64-part[0]*part[1] {
				return 0, errors.New("text: line reservation overflows uint64")
			}
			lineBytes += part[0] * part[1]
		}
		if err := add(lineBytes); err != nil {
			return 0, err
		}
		for _, run := range line.Runs {
			runBytes := uint64(192)
			for _, part := range [][2]uint64{{nonnegativeUint64(len(run.Text)), 1}, {nonnegativeUint64(cap(run.Glyphs)), 32}, {nonnegativeUint64(cap(run.GraphemeAdvances)), 4}} {
				if part[0] > math.MaxUint64/part[1] {
					return 0, errors.New("text: glyph-run reservation overflows uint64")
				}
				if runBytes > math.MaxUint64-part[0]*part[1] {
					return 0, errors.New("text: glyph-run reservation overflows uint64")
				}
				runBytes += part[0] * part[1]
			}
			if err := add(runBytes); err != nil {
				return 0, err
			}
		}
	}
	return n, nil
}
