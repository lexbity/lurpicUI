package text

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"github.com/go-text/typesetting/font"
)

const maxFontSourceNameBytes = 1024

// FontRegistry manages loaded font sources and face resolution.
type FontRegistry struct {
	mu          sync.RWMutex
	faceMu      sync.Mutex
	faces       []*fontFaceRecord
	generation  uint64
	sourceBytes uint64
	sourceCount int
	maxFaces    int
	wallet      *membudget.Wallet
	charges     []*membudget.Reservation
	closed      bool
}

// NewFontRegistry creates a registry whose retained source bytes are charged
// to the owning text wallet before the registry copies or parses them.
func NewFontRegistry(wallet *membudget.Wallet, maxFaces int) (*FontRegistry, error) {
	if wallet == nil || maxFaces <= 0 || maxFaces > math.MaxUint32 {
		return nil, errors.New("text: font registry requires a wallet and positive face limit")
	}
	return &FontRegistry{wallet: wallet, maxFaces: maxFaces}, nil
}

// LoadFontFile loads a font from disk and stores it in the registry.
func (r *FontRegistry) LoadFontFile(path string) error {
	if r == nil {
		return errors.New("text: nil registry")
	}
	if err := r.ensureOpen(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("text: empty font path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("text: load font %q: %w", filepath.Clean(path), err)
	}
	if info.Size() <= 0 {
		return errors.New("text: empty font data")
	}
	if uint64(info.Size()) > uint64(maxAddressableInt()) { //nolint:gosec // info.Size was checked positive above; the conversion is exact.
		return fmt.Errorf("text: font file size %d exceeds addressable memory", info.Size())
	}
	reservation, err := r.wallet.Reserve(uint64(info.Size())) //nolint:gosec // info.Size was checked positive above; the conversion is exact.
	if err != nil {
		capacity, _, _, _ := r.wallet.Snapshot()
		return fmt.Errorf("text: font file admission exceeds text wallet capacity %d: %w", capacity, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = reservation.Cancel()
		}
	}()
	file, err := os.Open(path) //nolint:gosec // path from user config
	if err != nil {
		return fmt.Errorf("text: load font %q: %w", filepath.Clean(path), err)
	}
	defer file.Close()
	data := make([]byte, int(info.Size()))
	if _, err := io.ReadFull(file, data); err != nil {
		return fmt.Errorf("text: read font %q: %w", filepath.Clean(path), err)
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || err != io.EOF {
		return fmt.Errorf("text: font %q changed size while being read", filepath.Clean(path))
	}
	if err := r.registerFontData(data, filepath.Base(path), reservation); err != nil {
		return err
	}
	committed = true
	return nil
}

// LoadFontBytes loads a font from memory and stores it in the registry.
func (r *FontRegistry) LoadFontBytes(data []byte, name string) error {
	if r == nil {
		return errors.New("text: nil registry")
	}
	if err := r.ensureOpen(); err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("text: empty font data")
	}
	if len(name) > maxFontSourceNameBytes {
		return fmt.Errorf("text: font source name exceeds %d bytes", maxFontSourceNameBytes)
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("text: font data requires name")
	}
	reservation, err := r.wallet.Reserve(uint64(len(data)))
	if err != nil {
		capacity, _, _, _ := r.wallet.Snapshot()
		return fmt.Errorf("text: font source admission exceeds text wallet capacity %d: %w", capacity, err)
	}
	ownedData := append([]byte(nil), data...)
	if err := r.registerFontData(ownedData, name, reservation); err != nil {
		_ = reservation.Cancel()
		return err
	}
	return nil
}

func (r *FontRegistry) registerFontData(ownedData []byte, name string, reservation *membudget.Reservation) error {
	faces, err := font.ParseTTC(bytes.NewReader(ownedData))
	if err != nil {
		return fmt.Errorf("text: parse font %q: %w", name, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("text: font registry is closed")
	}
	validFaces := 0
	for _, face := range faces {
		if face != nil && face.Font != nil {
			validFaces++
		}
	}
	if r.maxFaces > 0 && len(r.faces)+validFaces > r.maxFaces {
		return fmt.Errorf("text: font face limit %d exceeded", r.maxFaces)
	}
	if r.sourceBytes > ^uint64(0)-uint64(len(ownedData)) {
		return errors.New("text: registered font source bytes overflow uint64")
	}
	if err := reservation.Publish(); err != nil {
		return err
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
	r.charges = append(r.charges, reservation)
	r.generation++
	r.sourceBytes += uint64(len(ownedData))
	r.sourceCount++
	return nil
}

func maxAddressableInt() int { return int(^uint(0) >> 1) }

// Close releases retained font source charges after all service layout leases
// have ended. Repeated calls are harmless.
func (r *FontRegistry) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeLocked()
}

func (r *FontRegistry) close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeLocked()
}

func (r *FontRegistry) closeLocked() error {
	if r.closed {
		return nil
	}
	for _, charge := range r.charges {
		if err := charge.Release(); err != nil {
			return err
		}
	}
	r.charges = nil
	r.faces = nil
	r.sourceBytes = 0
	r.sourceCount = 0
	r.closed = true
	r.generation++
	return nil
}

// SourceBytes reports retained raw font bytes once per registered source,
// independent of the number of faces in a collection.
func (r *FontRegistry) SourceBytes() uint64 {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sourceBytes
}

// WalletCapacity reports the capacity assigned to this registry's text owner.
func (r *FontRegistry) WalletCapacity() uint64 {
	if r == nil || r.wallet == nil {
		return 0
	}
	capacity, _, _, _ := r.wallet.Snapshot()
	return capacity
}

// SourceCount reports registered source blobs independent of collection face
// count.
func (r *FontRegistry) SourceCount() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sourceCount
}

func (r *FontRegistry) setFaceLimit(limit int) error {
	if r == nil || limit <= 0 {
		return errors.New("text: invalid font face limit")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("text: font registry is closed")
	}
	if len(r.faces) > limit {
		return fmt.Errorf("text: loaded %d faces exceeds limit %d", len(r.faces), limit)
	}
	if r.maxFaces == 0 || limit < r.maxFaces {
		r.maxFaces = limit
	}
	return nil
}

func (r *FontRegistry) ensureOpen() error {
	r.mu.RLock()
	closed := r.closed
	r.mu.RUnlock()
	if closed {
		return errors.New("text: font registry is closed")
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
	var indexBytes [4]byte
	indexBytes[0] = byte(index >> 24) //nolint:gosec // MaxFontFaces bounds the collection index.
	indexBytes[1] = byte(index >> 16) //nolint:gosec // MaxFontFaces bounds the collection index.
	indexBytes[2] = byte(index >> 8)  //nolint:gosec // MaxFontFaces bounds the collection index.
	indexBytes[3] = byte(index)       //nolint:gosec // MaxFontFaces bounds the collection index.
	h := sha256.New()
	_, _ = h.Write(data)
	_, _ = h.Write(indexBytes[:])
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
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
