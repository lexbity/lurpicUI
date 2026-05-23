package cook

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
)

// AssetIDRecord is one persisted registry entry.
type AssetIDRecord struct {
	ID            assets.AssetID `json:"id"`
	CreatedAt     int64          `json:"created_at"`
	CanonicalPath string         `json:"canonical_path"`
}

type uuidGenerator func() (assets.AssetID, error)

type registryFile struct {
	Version int             `json:"version"`
	Records []AssetIDRecord `json:"records"`
}

// UUIDRegistry manages stable asset UUID assignments.
type UUIDRegistry struct {
	mu        sync.RWMutex
	records   map[string]AssetIDRecord
	byID      map[assets.AssetID]string
	filePath  string
	dirty     bool
	generator uuidGenerator
}

// NewUUIDRegistry returns an empty UUID registry backed by a cryptographic v4 generator.
func NewUUIDRegistry() *UUIDRegistry {
	return &UUIDRegistry{
		records:   make(map[string]AssetIDRecord),
		byID:      make(map[assets.AssetID]string),
		generator: generateUUIDv4,
	}
}

// LoadUUIDRegistry loads an existing registry file or creates an empty registry if the file is missing.
func LoadUUIDRegistry(path string) (*UUIDRegistry, error) {
	r := NewUUIDRegistry()
	r.filePath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return r, nil
	}

	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.Version != 0 && file.Version != 1 {
		return nil, fmt.Errorf("unsupported registry version %d", file.Version)
	}
	for _, record := range file.Records {
		canonical, err := canonicalizePath(record.CanonicalPath)
		if err != nil {
			return nil, err
		}
		record.CanonicalPath = canonical
		if record.ID.IsZero() {
			return nil, fmt.Errorf("registry entry %q has zero asset id", canonical)
		}
		if _, exists := r.records[canonical]; exists {
			return nil, fmt.Errorf("duplicate canonical path %q in registry", canonical)
		}
		if otherPath, exists := r.byID[record.ID]; exists {
			return nil, fmt.Errorf("duplicate asset id %s for %q and %q", record.ID, otherPath, canonical)
		}
		r.records[canonical] = record
		r.byID[record.ID] = canonical
	}
	r.dirty = false
	return r, nil
}

// Assign returns the stable AssetID for canonicalPath, creating one if necessary.
func (r *UUIDRegistry) Assign(canonicalPath string) (assets.AssetID, error) {
	canonical, err := canonicalizePath(canonicalPath)
	if err != nil {
		return assets.AssetID{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if record, ok := r.records[canonical]; ok {
		return record.ID, nil
	}

	const maxAttempts = 1024
	for attempt := 0; attempt < maxAttempts; attempt++ {
		id, err := r.generator()
		if err != nil {
			return assets.AssetID{}, err
		}
		if id.IsZero() {
			continue
		}
		if otherPath, exists := r.byID[id]; exists && otherPath != canonical {
			continue
		}
		record := AssetIDRecord{
			ID:            id,
			CreatedAt:     time.Now().Unix(),
			CanonicalPath: canonical,
		}
		r.records[canonical] = record
		r.byID[id] = canonical
		r.dirty = true
		return id, nil
	}

	return assets.AssetID{}, fmt.Errorf("unable to allocate unique asset id for %q after %d attempts", canonical, maxAttempts)
}

// Lookup returns the AssetID for canonicalPath, or zero if it is not registered.
func (r *UUIDRegistry) Lookup(canonicalPath string) assets.AssetID {
	canonical, err := canonicalizePath(canonicalPath)
	if err != nil {
		return assets.AssetID{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.records[canonical]
	if !ok {
		return assets.AssetID{}
	}
	return record.ID
}

// Save writes the registry to its configured file path.
func (r *UUIDRegistry) Save() error {
	r.mu.RLock()
	path := r.filePath
	r.mu.RUnlock()
	if path == "" {
		return errors.New("uuid registry has no file path")
	}
	return r.SaveTo(path)
}

// SaveTo writes the registry to path and remembers that path for future Save calls.
func (r *UUIDRegistry) SaveTo(path string) error {
	if path == "" {
		return errors.New("uuid registry path is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.filePath = path
	records := make([]AssetIDRecord, 0, len(r.records))
	for _, record := range r.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CanonicalPath < records[j].CanonicalPath
	})

	payload, err := json.MarshalIndent(registryFile{
		Version: 1,
		Records: records,
	}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	r.dirty = false
	return nil
}

// Records returns a copy of the registry records, sorted by canonical path.
func (r *UUIDRegistry) Records() []AssetIDRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	records := make([]AssetIDRecord, 0, len(r.records))
	for _, record := range r.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CanonicalPath < records[j].CanonicalPath
	})
	return records
}

func canonicalizePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == string(filepath.Separator) || cleaned == "" {
		return "", fmt.Errorf("invalid canonical path %q", path)
	}
	return filepath.ToSlash(cleaned), nil
}

func generateUUIDv4() (assets.AssetID, error) {
	var id assets.AssetID
	if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
		return assets.AssetID{}, err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}
