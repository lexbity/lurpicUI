package assets

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// DevFS is a development-mode AssetSource + fs.FS that watches a source tree
// for changes and enqueues invalidation work for the runtime thread to drain
// at the start of the next frame. It does not implement Manager — wrap it in
// NewManager to create the runtime-facing asset access surface.
type DevFS struct {
	root     fs.FS
	rootDir  string
	registry *AssetRegistryStore
	idReg    PathIDRegistry
	source   AssetSource // optional override for ReadLOD

	mu          sync.Mutex
	pending     []string
	watcher     *fsnotify.Watcher
	watchClosed bool
	closeOnce   sync.Once
}

// DevFSOption configures a DevFS instance.
type DevFSOption func(*DevFS)

// WithDevWatchRoot enables fsnotify watching for rootDir.
func WithDevWatchRoot(rootDir string) DevFSOption {
	return func(d *DevFS) {
		d.rootDir = rootDir
	}
}

// WithDevSource supplies an AssetSource override used by ReadLOD.
// When not provided, ReadLOD reads from the root filesystem via the registry.
func WithDevSource(source AssetSource) DevFSOption {
	return func(d *DevFS) {
		d.source = source
	}
}

// NewDevFS constructs a DevFS source. The registry is used for reverse
// path lookups in ReadLOD. Pass the same registry to NewManager so both
// share the same asset state.
func NewDevFS(root fs.FS, registry *AssetRegistryStore, idReg PathIDRegistry, opts ...DevFSOption) (*DevFS, error) {
	if registry == nil {
		registry = NewAssetRegistryStore()
	}
	d := &DevFS{
		root:     root,
		registry: registry,
		idReg:    idReg,
	}
	if d.source == nil {
		if src, ok := root.(AssetSource); ok {
			d.source = src
		}
	}
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
	if d.rootDir != "" {
		if err := d.startWatcher(); err != nil {
			_ = d.Close()
			return nil, err
		}
	}
	return d, nil
}

// ReadLOD implements AssetSource. It reads the asset file from the root
// filesystem using the path stored in the registry.
func (d *DevFS) ReadLOD(id AssetID, lod int) ([]byte, error) {
	if d == nil {
		return nil, fs.ErrNotExist
	}
	if d.source != nil {
		return d.source.ReadLOD(id, lod)
	}
	entry := d.registry.Get(id)
	if entry == nil || entry.Path == "" {
		return nil, fmt.Errorf("devfs: unknown asset %s", id.String())
	}
	name := entry.Path
	if lod > 0 {
		name = fmt.Sprintf("%s.lod%d", name, lod)
	}
	return fs.ReadFile(d.root, name)
}

func (d *DevFS) startWatcher() error {
	if d.rootDir == "" {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("devfs watcher: %w", err)
	}
	if err := addWatchedTree(watcher, d.rootDir); err != nil {
		_ = watcher.Close()
		return err
	}
	d.watcher = watcher
	go d.watchLoop()
	return nil
}

func addWatchedTree(w *fsnotify.Watcher, root string) error {
	if err := w.Add(root); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d != nil && d.IsDir() && path != root {
			return w.Add(path)
		}
		return nil
	})
}

func (d *DevFS) watchLoop() {
	if d == nil || d.watcher == nil {
		return
	}
	for {
		select {
		case ev, ok := <-d.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) != 0 {
				d.Invalidate(ev.Name)
			}
		case _, ok := <-d.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// Close stops any active watcher.
func (d *DevFS) Close() error {
	if d == nil {
		return nil
	}
	var err error
	d.closeOnce.Do(func() {
		d.mu.Lock()
		d.watchClosed = true
		watcher := d.watcher
		d.mu.Unlock()
		if watcher != nil {
			err = watcher.Close()
		}
	})
	return err
}

// Open implements fs.FS by delegating to the root filesystem.
func (d *DevFS) Open(name string) (fs.File, error) {
	if d == nil || d.root == nil {
		return nil, fs.ErrNotExist
	}
	return d.root.Open(name)
}

// Invalidate queues a path for runtime-thread invalidation.
func (d *DevFS) Invalidate(path string) {
	if d == nil || path == "" {
		return
	}
	canonical := d.canonicalPath(path)
	d.mu.Lock()
	if d.watchClosed {
		d.mu.Unlock()
		return
	}
	d.pending = append(d.pending, canonical)
	d.mu.Unlock()
}

// DrainInvalidations applies queued invalidations on the runtime thread.
func (d *DevFS) DrainInvalidations() int {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	pending := d.pending
	d.pending = nil
	d.mu.Unlock()
	if len(pending) == 0 {
		return 0
	}
	count := 0
	for _, path := range pending {
		if d.idReg == nil || d.registry == nil {
			continue
		}
		if id := d.idReg.Lookup(path); id != (AssetID{}) {
			d.registry.Invalidate(id)
			count++
		}
	}
	return count
}

func (d *DevFS) canonicalPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if d != nil && d.rootDir != "" {
		if rel, err := filepath.Rel(d.rootDir, cleaned); err == nil && rel != "." {
			cleaned = rel
		}
	}
	return filepath.ToSlash(cleaned)
}
