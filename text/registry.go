package text

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-text/typesetting/font"
)

// FontRegistry manages loaded font sources and face resolution.
type FontRegistry struct {
	mu         sync.RWMutex
	faceMu     sync.Mutex
	faces      []*fontFaceRecord
	generation uint64
	maxFaces   int
}

// NewFontRegistry creates an empty registry.
func NewFontRegistry() (*FontRegistry, error) {
	return &FontRegistry{}, nil
}

// LoadFontFile loads a font from disk and stores it in the registry.
func (r *FontRegistry) LoadFontFile(path string) error {
	if r == nil {
		return errors.New("text: nil registry")
	}
	if path == "" {
		return errors.New("text: empty font path")
	}
	data, err := os.ReadFile(path) //nolint:gosec // path from user config
	if err != nil {
		return fmt.Errorf("text: load font %q: %w", filepath.Clean(path), err)
	}
	return r.LoadFontBytes(data, filepath.Base(path))
}

// LoadFontBytes loads a font from memory and stores it in the registry.
func (r *FontRegistry) LoadFontBytes(data []byte, name string) error {
	if r == nil {
		return errors.New("text: nil registry")
	}
	if len(data) == 0 {
		return errors.New("text: empty font data")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("text: font data requires name")
	}
	ownedData := append([]byte(nil), data...)
	faces, err := font.ParseTTC(bytes.NewReader(ownedData))
	if err != nil {
		return fmt.Errorf("text: parse font %q: %w", name, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	validFaces := 0
	for _, face := range faces {
		if face != nil && face.Font != nil {
			validFaces++
		}
	}
	if r.maxFaces > 0 && len(r.faces)+validFaces > r.maxFaces {
		return fmt.Errorf("text: font face limit %d exceeded", r.maxFaces)
	}
	for i, face := range faces {
		if face == nil || face.Font == nil {
			continue
		}
		rec := &fontFaceRecord{
			face:     face,
			desc:     face.Describe(),
			registry: r,
			source:   FontSource{Name: name, Data: ownedData},
			cacheKey: computeFontCacheKey(ownedData, i),
		}
		r.faces = append(r.faces, rec)
	}
	r.generation++
	return nil
}

func (r *FontRegistry) setFaceLimit(limit int) error {
	if r == nil || limit <= 0 {
		return errors.New("text: invalid font face limit")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.faces) > limit {
		return fmt.Errorf("text: loaded %d faces exceeds limit %d", len(r.faces), limit)
	}
	if r.maxFaces == 0 || limit < r.maxFaces {
		r.maxFaces = limit
	}
	return nil
}

// Generation changes whenever the ordered face set changes.
func (r *FontRegistry) Generation() uint64 {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.generation
}

func (r *FontRegistry) FaceCount() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.faces)
}

// Sources returns a copy of the loaded font sources.
func (r *FontRegistry) Sources() []FontSource {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FontSource, 0, len(r.faces))
	for _, face := range r.faces {
		if face == nil {
			continue
		}
		source := face.source
		source.Data = append([]byte(nil), source.Data...)
		out = append(out, source)
	}
	return out
}

// Resolve finds the best available face for the given style.
func (r *FontRegistry) Resolve(style TextStyle) FontFace {
	if r == nil {
		return FontFace{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if face := r.resolveLocked(style); face != nil {
		return FontFace{face: face}
	}
	return FontFace{}
}

// FirstFamily returns the family name of the first registered face, or "" if empty.
func (r *FontRegistry) FirstFamily() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.faces) > 0 && r.faces[0] != nil && r.faces[0].face != nil {
		return r.faces[0].desc.Family
	}
	return ""
}

func (r *FontRegistry) resolveLocked(style TextStyle) *fontFaceRecord {
	if r == nil {
		return nil
	}
	targetFamily := font.NormalizeFamily(style.Family)
	if targetFamily == "" {
		return nil
	}
	var (
		best      *fontFaceRecord
		bestScore int
	)
	for _, face := range r.faces {
		if face == nil || face.face == nil {
			continue
		}
		if font.NormalizeFamily(face.desc.Family) != targetFamily {
			continue
		}
		score := faceMatchScore(face.desc.Aspect, style)
		if best == nil || score < bestScore {
			best = face
			bestScore = score
		}
	}
	return best
}

// IsZero reports whether the face wraps an unresolved record.
func (f FontFace) IsZero() bool {
	return f.face == nil
}

// GoFace returns the underlying go-text font face.
func (f FontFace) GoFace() *font.Face {
	if f.face == nil {
		return nil
	}
	return f.face.face
}

// CacheKey returns a stable identifier suitable for glyph atlas keys.
func (f FontFace) CacheKey() uint64 {
	if f.face == nil {
		return 0
	}
	return f.face.cacheKey
}

func computeFontCacheKey(data []byte, index int) uint64 {
	sum := sha256.Sum256(append(append([]byte(nil), data...), byte(index>>24), byte(index>>16), byte(index>>8), byte(index))) //nolint:gosec // integer overflow conversion
	return binaryToUint64(sum[:8])
}

func binaryToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

func faceMatchScore(aspect font.Aspect, style TextStyle) int {
	score := 0
	wantStyle := font.StyleNormal
	if style.Style != StyleNormal {
		wantStyle = font.StyleItalic
	}
	if aspect.Style != wantStyle {
		score += 1000
	}
	wantWeight := font.Weight(style.Weight)
	if wantWeight == 0 {
		wantWeight = font.WeightNormal
	}
	diff := aspect.Weight - wantWeight
	if diff < 0 {
		diff = -diff
	}
	score += int(diff)
	return score
}
