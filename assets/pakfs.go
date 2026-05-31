package assets

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

var errPakFSClosed = errors.New("pakfs closed")

// PakFS implements fs.FS and AssetSource for release builds backed by a
// memory-mapped pak file. It provides no network, scheduling, or cache
// logic — those live in ManagerImpl, which wraps a PakFS as an AssetSource.
//
// Concurrency: ReadLOD increments an in-flight counter so Close can wait
// for all active reads to finish before munmap. Every ReadLOD call copies
// the requested bytes out of the mmap before returning, so jobs that hold
// the returned slice do not keep a reference to the mapping.
type PakFS struct {
	mu       sync.RWMutex
	data     []byte
	header   *PakHeader
	toc      []PakTOCEntry
	deps     []AssetID
	closed   bool
	inFlight sync.WaitGroup
}

type pakFile struct {
	name    string
	data    []byte
	modTime time.Time
	offset  int
}

type pakDir struct {
	name    string
	entries []fs.DirEntry
}

type pakDirEntry struct {
	name string
	dir  bool
	size int64
}

type pakFileInfo struct {
	name    string
	size    int64
	dir     bool
	modTime time.Time
}

// NewPakFSFromFD memory-maps a file descriptor at the given offset and
// length and returns an AssetSource + fs.FS backed by the mapping. The
// caller must close the PakFS (which munmaps) and close the fd separately.
// This is used on Android to mmap uncompressed APK assets directly.
func NewPakFSFromFD(fd int, offset, length int64) (*PakFS, error) {
	if length <= 0 {
		return nil, fmt.Errorf("PakFS fd: invalid length %d", length)
	}
	data, err := syscall.Mmap(fd, offset, int(length), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("PakFS fd mmap: %w", err)
	}
	p := &PakFS{data: data}
	if err := p.init(); err != nil {
		_ = syscall.Munmap(data)
		return nil, err
	}
	return p, nil
}

// NewPakFS memory-maps pakPath and returns an AssetSource + fs.FS backed by it.
func NewPakFS(pakPath string) (*PakFS, error) {
	f, err := os.Open(pakPath)
	if err != nil {
		return nil, fmt.Errorf("PakFS open: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("PakFS stat: %w", err)
	}
	if fi.Size() == 0 {
		return nil, fmt.Errorf("PakFS: empty pak file")
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("PakFS mmap: %w", err)
	}

	p := &PakFS{data: data}
	if err := p.init(); err != nil {
		_ = syscall.Munmap(data)
		return nil, err
	}
	return p, nil
}

func (p *PakFS) init() error {
	if len(p.data) < int(unsafe.Sizeof(PakHeader{})) {
		return fmt.Errorf("PakFS: truncated header")
	}
	hdr := &PakHeader{
		TOCOffset:  binary.LittleEndian.Uint64(p.data[8:16]),
		DepsOffset: binary.LittleEndian.Uint64(p.data[20:28]),
		Magic:      [4]byte{p.data[0], p.data[1], p.data[2], p.data[3]},
		Version:    binary.LittleEndian.Uint32(p.data[4:8]),
		TOCCount:   binary.LittleEndian.Uint32(p.data[16:20]),
		DepsCount:  binary.LittleEndian.Uint32(p.data[28:32]),
	}
	if hdr.Magic != [4]byte{'L', 'U', 'R', 'P'} {
		return fmt.Errorf("PakFS: invalid magic bytes")
	}
	if hdr.Version != 2 {
		return fmt.Errorf("PakFS: unsupported version %d", hdr.Version)
	}
	if hdr.TOCOffset > uint64(len(p.data)) || hdr.DepsOffset > uint64(len(p.data)) {
		return fmt.Errorf("PakFS: invalid table offsets")
	}
	if hdr.TOCCount > 0 {
		tocBytes := uint64(hdr.TOCCount) * uint64(unsafe.Sizeof(PakTOCEntry{}))
		if hdr.TOCOffset+tocBytes > uint64(len(p.data)) {
			return fmt.Errorf("PakFS: truncated toc table")
		}
	}
	if hdr.DepsCount > 0 {
		depsBytes := uint64(hdr.DepsCount) * uint64(unsafe.Sizeof(AssetID{}))
		if hdr.DepsOffset+depsBytes > uint64(len(p.data)) {
			return fmt.Errorf("PakFS: truncated deps table")
		}
	}

	p.header = hdr
	if hdr.TOCCount > 0 {
		p.toc = unsafe.Slice((*PakTOCEntry)(unsafe.Pointer(&p.data[hdr.TOCOffset])), hdr.TOCCount)
	}
	if hdr.DepsCount > 0 {
		p.deps = unsafe.Slice((*AssetID)(unsafe.Pointer(&p.data[hdr.DepsOffset])), hdr.DepsCount)
	}
	return nil
}

// Close releases the mmap backing the filesystem. It waits for any in-flight
// ReadLOD calls to complete before munmap. Callers should first quiesce the
// ManagerImpl (drain jobs, cancel pending) before closing the underlying source.
func (p *PakFS) Close() error {
	if p == nil {
		return nil
	}

	// Mark closed under write lock so new ReadLOD calls see the closed flag.
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Wait for any in-flight ReadLOD calls to finish. After this returns,
	// no ReadLOD call is touching the mmap and no new one can start
	// (they all check p.closed under the read lock).
	p.inFlight.Wait()

	if p.data == nil {
		return nil
	}
	return syscall.Munmap(p.data)
}

func assetTypeForPath(path string) AssetType {
	switch strings.ToLower(filepathExt(path)) {
	case ".svg":
		return AssetTypeSVG
	case ".png", ".jpg", ".jpeg":
		return AssetTypeImage
	case ".ttf", ".otf":
		return AssetTypeFont
	case ".toml", ".json":
		return AssetTypeConfig
	default:
		return AssetTypeImage
	}
}

func filepathExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

// ReadLOD returns the raw compressed bytes for the requested asset LOD.
// The returned slice is a copy of the mmap data, safe to use after Close.
func (p *PakFS) ReadLOD(id AssetID, lod int) ([]byte, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, errPakFSClosed
	}
	// Add in-flight while still under the read lock so Close's write-lock
	// + Wait sequence cannot see a closed-but-in-flight state.
	p.inFlight.Add(1)
	p.mu.RUnlock()

	defer p.inFlight.Done()

	entry, err := p.findEntry(id, lod)
	if err != nil {
		return nil, err
	}
	raw := p.readBlock(entry)
	if raw == nil {
		return nil, fs.ErrNotExist
	}
	// Copy out of the mmap so the caller can safely use the bytes after the
	// RUnlock and even after the mmap is closed. The copy is small compared
	// to the downstream decompression work.
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

// Open implements fs.FS. Files are addressed by UUID string and optional `.lodN` suffix.
func (p *PakFS) Open(name string) (fs.File, error) {
	if name == "." || name == "" {
		return p.openRoot(), nil
	}
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	id, lod, err := parsePakFSName(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	decoded, err := p.readDecodedLOD(id, lod)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &pakFile{name: name, data: decoded, modTime: time.Unix(0, 0)}, nil
}

// ReadDir implements fs.ReadDirFS for the root asset listing.
func (p *PakFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." && name != "" {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil, errPakFSClosed
	}
	entries := p.rootEntries()
	return entries, nil
}

func (p *PakFS) openRoot() fs.File {
	return &pakDir{name: ".", entries: p.rootEntries()}
}

func (p *PakFS) rootEntries() []fs.DirEntry {
	seen := make(map[AssetID]struct{})
	entries := make([]fs.DirEntry, 0)
	for _, entry := range p.toc {
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		seen[entry.ID] = struct{}{}
		entries = append(entries, pakDirEntry{name: entry.ID.String(), size: int64(entry.UncompressedSize)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries
}

func (p *PakFS) findEntry(id AssetID, lod int) (*PakTOCEntry, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil, errPakFSClosed
	}
	if len(p.toc) == 0 {
		return nil, fs.ErrNotExist
	}

	idx, ok := tocSearch(p.toc, id)
	if !ok {
		return nil, fs.ErrNotExist
	}
	key := p.toc[idx].ID
	prefix := keyPrefix(key)
	left := idx
	for left > 0 && keyPrefix(p.toc[left-1].ID) == prefix {
		left--
	}
	right := idx + 1
	for right < len(p.toc) && keyPrefix(p.toc[right].ID) == prefix {
		right++
	}
	for i := left; i < right; i++ {
		if p.toc[i].ID == id && int(p.toc[i].LODLevel) == lod {
			return &p.toc[i], nil
		}
	}
	return nil, fs.ErrNotExist
}

func (p *PakFS) readDecodedLOD(id AssetID, lod int) ([]byte, error) {
	entry, err := p.findEntry(id, lod)
	if err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil, fs.ErrClosed
	}
	raw := p.readBlock(entry)
	return decodeLODBytes(raw, entry)
}

// readBlock returns the raw compressed bytes for a TOC entry.
// The returned slice points into the mmap'd region.
func (p *PakFS) readBlock(entry *PakTOCEntry) []byte {
	if entry == nil || len(p.data) == 0 {
		return nil
	}
	start := entry.Offset
	end := start + uint64(entry.CompressedSize)
	if start > uint64(len(p.data)) || end > uint64(len(p.data)) || end < start {
		return nil
	}
	return p.data[start:end]
}

func decodeLODBytes(raw []byte, entry *PakTOCEntry) ([]byte, error) {
	switch PakCompression(entry.Compression) {
	case PakCompressionNone:
		return append([]byte(nil), raw...), nil
	case PakCompressionLZ4:
		if entry.UncompressedSize == 0 {
			return nil, nil
		}
		dst := make([]byte, entry.UncompressedSize)
		n, err := lz4.UncompressBlock(raw, dst)
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), dst[:n]...), nil
	case PakCompressionZstd:
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return dec.DecodeAll(raw, nil)
	default:
		return nil, fmt.Errorf("unsupported compression %d", entry.Compression)
	}
}

func parsePakFSName(name string) (AssetID, int, error) {
	lod := 0
	base := name
	if idx := strings.LastIndex(name, ".lod"); idx >= 0 {
		if parsed, err := strconv.Atoi(name[idx+4:]); err == nil {
			lod = parsed
			base = name[:idx]
		}
	}
	id, err := ParseAssetID(base)
	if err != nil {
		return AssetID{}, 0, err
	}
	if lod < 0 {
		return AssetID{}, 0, fmt.Errorf("negative lod")
	}
	return id, lod, nil
}

func keyPrefix(id AssetID) uint64 {
	return uint64(id[0])<<56 |
		uint64(id[1])<<48 |
		uint64(id[2])<<40 |
		uint64(id[3])<<32 |
		uint64(id[4])<<24 |
		uint64(id[5])<<16 |
		uint64(id[6])<<8 |
		uint64(id[7])
}

func (f *pakFile) Read(p []byte) (int, error) {
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.offset:])
	f.offset += n
	if f.offset >= len(f.data) {
		return n, io.EOF
	}
	return n, nil
}

func (f *pakFile) Close() error { return nil }

func (f *pakFile) Stat() (fs.FileInfo, error) {
	return pakFileInfo{name: f.name, size: int64(len(f.data)), modTime: f.modTime}, nil
}

func (d *pakDir) Read([]byte) (int, error) { return 0, io.EOF }

func (d *pakDir) Close() error { return nil }

func (d *pakDir) Stat() (fs.FileInfo, error) {
	return pakFileInfo{name: d.name, dir: true, modTime: time.Unix(0, 0)}, nil
}

func (d *pakDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if count <= 0 || count >= len(d.entries) {
		out := append([]fs.DirEntry(nil), d.entries...)
		d.entries = nil
		return out, nil
	}
	out := append([]fs.DirEntry(nil), d.entries[:count]...)
	d.entries = d.entries[count:]
	return out, nil
}

func (d pakDirEntry) Name() string { return d.name }
func (d pakDirEntry) IsDir() bool  { return d.dir }
func (d pakDirEntry) Type() fs.FileMode {
	if d.dir {
		return fs.ModeDir
	}
	return 0
}
func (d pakDirEntry) Info() (fs.FileInfo, error) { return pakFileInfo{name: d.name, size: d.size}, nil }

func (f pakFileInfo) Name() string { return f.name }
func (f pakFileInfo) Size() int64  { return f.size }
func (f pakFileInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (f pakFileInfo) ModTime() time.Time { return f.modTime }
func (f pakFileInfo) IsDir() bool        { return f.dir }
func (f pakFileInfo) Sys() any           { return nil }
