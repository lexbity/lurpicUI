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

// PakFS implements fs.FS and raw LOD access for release builds.
type PakFS struct {
	mu          sync.RWMutex
	data        []byte
	header      *PakHeader
	toc         []PakTOCEntry
	deps        []AssetID
	closed      bool
	manager     *managerImpl
	idReg       PathIDRegistry
	cache       *assetCache
	backendType BackendType
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

// NewPakFS memory-maps pakPath and exposes its contents.
// Extra arguments are accepted for forward compatibility with later runtime phases.
func NewPakFS(pakPath string, extras ...any) (*PakFS, error) {
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

	fs := &PakFS{data: data, backendType: BackendSoftware}
	var registry *AssetRegistryStore
	var scheduler JobScheduler
	for _, extra := range extras {
		switch v := extra.(type) {
		case *AssetRegistryStore:
			registry = v
		case JobScheduler:
			scheduler = v
		case BackendType:
			fs.backendType = v
		case PathIDRegistry:
			fs.idReg = v
		case *assetCache:
			fs.cache = v
		}
	}
	if err := fs.init(); err != nil {
		_ = syscall.Munmap(data)
		return nil, err
	}
	if registry == nil {
		registry = NewAssetRegistryStore()
	}
	fs.manager = NewManagerImpl(registry, fs, fs.backendType, scheduler)
	return fs, nil
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

// LoadSVG schedules progressive LOD streaming for an SVG path.
func (p *PakFS) LoadSVG(path string) Handle {
	return p.loadByPath(path, AssetTypeSVG)
}

// LoadImage schedules progressive LOD streaming for a raster image path.
func (p *PakFS) LoadImage(path string) Handle {
	return p.loadByPath(path, AssetTypeImage)
}

// LoadTexture schedules progressive LOD streaming for a material texture path.
func (p *PakFS) LoadTexture(path string) Handle {
	return p.loadByPath(path, AssetTypeImage)
}

// LoadFont schedules progressive LOD streaming for a font path.
func (p *PakFS) LoadFont(path string) Handle {
	return p.loadByPath(path, AssetTypeFont)
}

// LoadConfig schedules loading for a config path.
func (p *PakFS) LoadConfig(path string, _ any) Handle {
	return p.loadByPath(path, AssetTypeConfig)
}

// Prefetch schedules assets without changing the caller's control flow.
func (p *PakFS) Prefetch(paths ...string) {
	for _, path := range paths {
		p.loadByPath(path, assetTypeForPath(path))
	}
}

// Invalidate marks an asset stale and clears any ready LODs from the registry.
func (p *PakFS) Invalidate(path string) {
	if p == nil || p.registry() == nil {
		return
	}
	if id := p.lookupID(path); id != (AssetID{}) {
		p.registry().Invalidate(id)
	}
}

// DrainCompleted commits any jobs that have completed since the last drain.
func (p *PakFS) DrainCompleted() int {
	if p == nil || p.manager == nil {
		return 0
	}
	return p.manager.DrainCompleted()
}

// Stats returns a snapshot of the registry and cache state.
func (p *PakFS) Stats() ManagerStats {
	if p == nil {
		return ManagerStats{}
	}
	stats := ManagerStats{}
	if reg := p.registry(); reg != nil {
		reg.mu.RLock()
		stats.TotalEntries = len(reg.entries)
		for _, entry := range reg.entries {
			switch entry.State {
			case AssetStateLoading:
				stats.LoadingEntries++
			case AssetStateReady:
				stats.ReadyEntries++
			case AssetStateFailed:
				stats.FailedEntries++
			}
			if entry.HighestReadyLOD >= 0 && entry.HighestReadyLOD < len(entry.LODHandles) {
				if entry.HighestReadyLOD > 0 {
					stats.PartialEntries++
				}
			}
			stats.Entries = append(stats.Entries, AssetDiagEntry{
				ID:              entry.ID,
				Path:            entry.Path,
				State:           entry.State,
				HighestReadyLOD: entry.HighestReadyLOD,
				RefCounts:       entry.LODRefCounts,
				SizeBytes:       entry.SizeBytes,
				LoadTimeNs:      entry.LoadTimeNs,
				LastUsedFrame:   entry.LastUse,
			})
		}
		reg.mu.RUnlock()
	}
	if p.cache != nil {
		stats.CPUUsedBytes = p.cache.usedBytes
		stats.CPUBudgetBytes = p.cache.budgetBytes
		stats.GPUUsedBytes = p.cache.gpuUsed
		stats.GPUBudgetBytes = p.cache.gpuBudget
		stats.EvictionsThisFrame = p.cache.evictionsThisFrame
	}
	if p.manager != nil && p.manager.scheduler != nil {
		stats.JobsInFlight = len(p.manager.results)
	}
	return stats
}

// Close releases the mmap backing the filesystem.
func (p *PakFS) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if p.data == nil {
		return nil
	}
	return syscall.Munmap(p.data)
}

func (p *PakFS) registry() *AssetRegistryStore {
	if p == nil || p.manager == nil {
		return nil
	}
	return p.manager.registry
}

func (p *PakFS) loadByPath(path string, typ AssetType) Handle {
	if p == nil || p.manager == nil {
		return Handle{}
	}
	id := p.lookupID(path)
	if id == (AssetID{}) {
		return Handle{}
	}
	p.manager.scheduleAllLODs(id, path, typ)
	return NewHandle(id, p.manager.registry)
}

func (p *PakFS) lookupID(path string) AssetID {
	if p == nil || p.idReg == nil {
		return AssetID{}
	}
	return p.idReg.Lookup(path)
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
func (p *PakFS) ReadLOD(id AssetID, lod int) ([]byte, error) {
	entry, err := p.findEntry(id, lod)
	if err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil, errPakFSClosed
	}
	return p.readBlock(entry), nil
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
