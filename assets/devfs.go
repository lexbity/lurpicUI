package assets

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// DevFS is the development-mode asset manager.
//
// It watches a source tree for changes and enqueues invalidation work for the
// runtime thread to drain at the start of the next frame.
type DevFS struct {
	root      fs.FS
	rootDir   string
	registry  *AssetRegistryStore
	scheduler JobScheduler
	idReg     PathIDRegistry
	source    AssetSource
	manager   *managerImpl

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

// WithDevSource supplies an AssetSource used for loads.
func WithDevSource(source AssetSource) DevFSOption {
	return func(d *DevFS) {
		d.source = source
	}
}

// NewDevFS constructs a development asset manager.
func NewDevFS(root fs.FS, registry *AssetRegistryStore, scheduler JobScheduler, idReg PathIDRegistry, opts ...DevFSOption) (*DevFS, error) {
	d := &DevFS{
		root:      root,
		registry:  registry,
		scheduler: scheduler,
		idReg:     idReg,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
	if d.source == nil {
		if src, ok := root.(AssetSource); ok {
			d.source = src
		}
	}
	if registry == nil {
		registry = NewAssetRegistryStore()
		d.registry = registry
	}
	if d.scheduler == nil {
		d.scheduler = NewAsyncJobScheduler(nil)
	}
	if d.source != nil {
		d.manager = NewManagerImpl(registry, d.source, BackendSoftware, d.scheduler)
	}
	if d.rootDir != "" {
		if err := d.startWatcher(); err != nil {
			_ = d.Close()
			return nil, err
		}
	}
	return d, nil
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

// LoadSVG schedules progressive LODs for an SVG asset.
func (d *DevFS) LoadSVG(path string) Handle { return d.loadByPath(path, AssetTypeSVG) }

// LoadImage schedules progressive LODs for a raster image asset.
func (d *DevFS) LoadImage(path string) Handle { return d.loadByPath(path, AssetTypeImage) }

// LoadTexture schedules progressive LODs for a material texture asset.
func (d *DevFS) LoadTexture(path string) Handle { return d.loadByPath(path, AssetTypeImage) }

// LoadFont schedules progressive LODs for a font asset.
func (d *DevFS) LoadFont(path string) Handle { return d.loadByPath(path, AssetTypeFont) }

// LoadConfig schedules a config asset.
func (d *DevFS) LoadConfig(path string, _ any) Handle { return d.loadByPath(path, AssetTypeConfig) }

// Prefetch queues load work ahead of time.
func (d *DevFS) Prefetch(paths ...string) {
	for _, path := range paths {
		d.loadByPath(path, assetTypeForPath(path))
	}
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
		if id := d.lookupID(path); id != (AssetID{}) && d.registry != nil {
			d.registry.Invalidate(id)
			count++
		}
	}
	return count
}

// Stats returns a snapshot of the registry and load queue.
func (d *DevFS) Stats() ManagerStats {
	if d == nil {
		return ManagerStats{}
	}
	if d.manager != nil {
		return d.manager.Stats()
	}
	return ManagerStats{}
}

// DrainCompleted commits any jobs that have completed since the last drain.
func (d *DevFS) DrainCompleted() int {
	if d == nil || d.manager == nil {
		return 0
	}
	return d.manager.DrainCompleted()
}

func (d *DevFS) loadByPath(path string, typ AssetType) Handle {
	if d == nil || d.manager == nil {
		return Handle{}
	}
	id := d.lookupID(path)
	if id == (AssetID{}) {
		return Handle{}
	}
	d.manager.scheduleAllLODs(id, path, typ)
	return NewHandle(id, d.manager.registry)
}

func (d *DevFS) lookupID(path string) AssetID {
	if d == nil || d.idReg == nil {
		return AssetID{}
	}
	return d.idReg.Lookup(d.canonicalPath(path))
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
