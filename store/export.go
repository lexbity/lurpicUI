package store

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// ExportClass describes whether an observation belongs to application domain
// state or application view state. Derived exports are observations of the
// last computed value and retain the class of their source contract.
type ExportClass uint8

const (
	DomainState ExportClass = iota + 1
	ViewState
)

func (c ExportClass) String() string {
	switch c {
	case DomainState:
		return "domain"
	case ViewState:
		return "view"
	default:
		return ""
	}
}

// ExportKind is the read behavior generated into the observation catalog.
type ExportKind uint8

const (
	ValueExportKind ExportKind = iota + 1
	CollectionExportKind
	DerivedExportKind
	DetailExportKind
)

func (k ExportKind) String() string {
	switch k {
	case ValueExportKind:
		return "value"
	case CollectionExportKind:
		return "collection"
	case DerivedExportKind:
		return "derived"
	case DetailExportKind:
		return "detail"
	default:
		return ""
	}
}

// ReadExport is an explicit, typed-at-declaration read capability. Its
// capture closure returns immutable JSON bytes while called at a runtime cut;
// workers never receive a store or application pointer.
type ReadExport struct {
	name              string
	field             string
	class             ExportClass
	kind              ExportKind
	schema            string
	maxBytes          int
	version           func() Version
	capture           func(offset, limit int) (json.RawMessage, uint64, uint64, bool, error)
	captureCollection func() ([]json.RawMessage, uint64, error)
	collectionLen     func() int
}

// ExportInfo is the immutable public catalog description of an export.
type ExportInfo struct {
	Name     string
	Field    string
	Class    ExportClass
	Kind     ExportKind
	Schema   string
	MaxBytes int
}

// Info returns the schema-visible metadata without exposing the source store.
func (e ReadExport) Info() ExportInfo {
	return ExportInfo{Name: e.name, Field: e.field, Class: e.class, Kind: e.kind, Schema: e.schema, MaxBytes: e.maxBytes}
}

// Valid reports whether this value was created by one of the typed
// constructors and has a complete public contract.
func (e ReadExport) Valid() bool {
	i := e.Info()
	return (i.Name != "") != (i.Field != "") && i.Class.String() != "" && i.Kind.String() != "" && i.Schema != "" && i.MaxBytes > 0 && e.capture != nil && e.version != nil
}

// Version captures the current source version. Calls are runtime-cut owned.
func (e ReadExport) Version() Version {
	if e.version == nil {
		return 0
	}
	return e.version()
}

// Capture freezes one value or bounded collection page. The bool reports a
// derived export's dirty state; it is always false for other export kinds.
func (e ReadExport) Capture(offset, limit int) (json.RawMessage, uint64, uint64, bool, error) {
	if e.capture == nil {
		return nil, 0, 0, false, fmt.Errorf("store: invalid read export")
	}
	if offset < 0 || limit < 1 || limit > 256 {
		return nil, 0, 0, false, fmt.Errorf("store: export page requires offset >= 0 and limit in [1,256]")
	}
	return e.capture(offset, limit)
}

// CaptureCollection freezes a complete declared collection at one source
// version. Callers must account the returned immutable DTO bytes before
// retaining them. This is the acquisition boundary used by paged protocol
// reads so later pages never combine values from different collection
// versions.
func (e ReadExport) CaptureCollection() ([]json.RawMessage, uint64, error) {
	if e.kind != CollectionExportKind || e.captureCollection == nil {
		return nil, 0, fmt.Errorf("store: export %q does not support immutable collection snapshots", e.name+e.field)
	}
	return e.captureCollection()
}

// CollectionLength reports the current item count without copying values. It
// lets a caller reserve retained snapshot capacity before freezing the DTOs.
func (e ReadExport) CollectionLength() (int, error) {
	if e.kind != CollectionExportKind || e.collectionLen == nil {
		return 0, fmt.Errorf("store: export %q is not a collection", e.name+e.field)
	}
	return e.collectionLen(), nil
}

// ReadExportProvider explicitly declares mark-owned export capabilities.
// Field is a stable public capability name and is never a Go field lookup.
type ReadExportProvider interface {
	ReadExports() []ReadExport
}

type exportFreezer[T any] func(T) (json.RawMessage, error)

func newExport(name, field string, class ExportClass, kind ExportKind, schema string, maxBytes int) ReadExport {
	if (name == "") == (field == "") || (name != "" && !utf8.ValidString(name)) || class.String() == "" || schema == "" || maxBytes < 1 || maxBytes > 16<<20 {
		panic("store: invalid read export declaration")
	}
	if field != "" && (!utf8.ValidString(field) || len(field) > 128) {
		panic("store: invalid read export field capability")
	}
	return ReadExport{name: name, field: field, class: class, kind: kind, schema: schema, maxBytes: maxBytes}
}

// ValueExport declares a named scalar/DTO store read. freeze must copy all
// mutable members into bounded JSON bytes before returning.
func ValueExport[T any](name string, source *ValueStore[T], class ExportClass, schema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	return valueExport(name, "", source, class, schema, maxBytes, freeze)
}

// FacetValueExport declares a mark field capability without exposing or
// reflecting the mark's Go fields.
func FacetValueExport[T any](field string, source *ValueStore[T], class ExportClass, schema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	return valueExport("", field, source, class, schema, maxBytes, freeze)
}

func valueExport[T any](name, field string, source *ValueStore[T], class ExportClass, schema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	e := newExport(name, field, class, ValueExportKind, schema, maxBytes)
	if source == nil || freeze == nil {
		panic("store: value export requires a non-nil store and freezer")
	}
	e.version = func() Version { return source.Version() }
	e.capture = func(offset, limit int) (json.RawMessage, uint64, uint64, bool, error) {
		if offset != 0 {
			return nil, 0, 0, false, fmt.Errorf("store: value export offset must be zero")
		}
		value := source.Get()
		payload, err := freeze(value)
		if err != nil {
			return nil, 0, 0, false, err
		}
		if len(payload) > e.maxBytes {
			return nil, 0, 0, false, fmt.Errorf("store: value export exceeds %d bytes", e.maxBytes)
		}
		return append(json.RawMessage(nil), payload...), uint64(source.Version()), 1, false, nil
	}
	return e
}

// CollectionExport declares a named, paged collection. The store copies only
// the requested page while holding its read lock; each item freezer must
// produce an immutable, bounded DTO.
func CollectionExport[T any](name string, source *CollectionStore[T], class ExportClass, itemSchema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	return collectionExport(name, "", source, class, itemSchema, maxBytes, freeze)
}

// FacetCollectionExport declares a paged mark-owned collection capability.
func FacetCollectionExport[T any](field string, source *CollectionStore[T], class ExportClass, itemSchema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	return collectionExport("", field, source, class, itemSchema, maxBytes, freeze)
}

func collectionExport[T any](name, field string, source *CollectionStore[T], class ExportClass, itemSchema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	e := newExport(name, field, class, CollectionExportKind, "array<"+itemSchema+">", maxBytes)
	if source == nil || freeze == nil {
		panic("store: collection export requires a non-nil store and freezer")
	}
	e.version = func() Version { return source.Version() }
	e.collectionLen = source.Len
	e.capture = func(offset, limit int) (json.RawMessage, uint64, uint64, bool, error) {
		items, total, version := source.Page(offset, limit)
		frozen := make([]json.RawMessage, len(items))
		totalBytes := 2
		for i, item := range items {
			payload, err := freeze(item)
			if err != nil {
				return nil, 0, 0, false, err
			}
			totalBytes += len(payload)
			if i > 0 {
				totalBytes++
			}
			if totalBytes > e.maxBytes {
				return nil, 0, 0, false, fmt.Errorf("store: collection page exceeds %d bytes", e.maxBytes)
			}
			frozen[i] = append(json.RawMessage(nil), payload...)
		}
		payload, err := json.Marshal(struct {
			Items  []json.RawMessage `json:"items"`
			Offset int               `json:"offset"`
			Total  string            `json:"total"`
		}{frozen, offset, strconv.Itoa(total)})
		if err != nil {
			return nil, 0, 0, false, err
		}
		if len(payload) > e.maxBytes {
			return nil, 0, 0, false, fmt.Errorf("store: collection page exceeds %d bytes", e.maxBytes)
		}
		if total < 0 {
			return nil, 0, 0, false, fmt.Errorf("store: collection reported a negative item count")
		}
		return payload, uint64(version), nonnegativeInt(total), false, nil
	}
	e.captureCollection = func() ([]json.RawMessage, uint64, error) {
		items, version, err := source.AllVersionedLimit(16384)
		if err != nil {
			return nil, 0, err
		}
		frozen := make([]json.RawMessage, len(items))
		totalBytes := 2
		for i, item := range items {
			payload, err := freeze(item)
			if err != nil {
				return nil, 0, err
			}
			totalBytes += len(payload)
			if i > 0 {
				totalBytes++
			}
			if totalBytes > e.maxBytes {
				return nil, 0, fmt.Errorf("store: immutable collection snapshot exceeds %d bytes", e.maxBytes)
			}
			frozen[i] = append(json.RawMessage(nil), payload...)
		}
		return frozen, uint64(version), nil
	}
	return e
}

func nonnegativeInt(value int) uint64 {
	if value < 0 {
		panic("store: nonnegativeInt called with a negative value")
	}
	return uint64(value)
}

// DerivedExport declares a named derived Peek observation.
func DerivedExport[T any](name string, source *Derived[T], class ExportClass, schema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	return derivedExport(name, "", source, class, schema, maxBytes, freeze)
}

// FacetDerivedExport declares a mark-owned derived Peek capability.
func FacetDerivedExport[T any](field string, source *Derived[T], class ExportClass, schema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	return derivedExport("", field, source, class, schema, maxBytes, freeze)
}

// FacetDetailExport declares an immutable, bounded custom observation. The
// capture function is authored with the mark and is called only at a runtime
// cut; it must not evaluate derived values or mutate app state.
func FacetDetailExport[T any](field, schema string, maxBytes int, capture func() T, freeze exportFreezer[T]) ReadExport {
	e := newExport("", field, ViewState, DetailExportKind, schema, maxBytes)
	if capture == nil || freeze == nil {
		panic("store: detail export requires a capture function and freezer")
	}
	e.version = func() Version { return 0 }
	e.capture = func(offset, limit int) (json.RawMessage, uint64, uint64, bool, error) {
		if offset != 0 {
			return nil, 0, 0, false, fmt.Errorf("store: detail export offset must be zero")
		}
		payload, err := freeze(capture())
		if err != nil {
			return nil, 0, 0, false, err
		}
		if len(payload) > e.maxBytes {
			return nil, 0, 0, false, fmt.Errorf("store: detail export exceeds %d bytes", e.maxBytes)
		}
		return append(json.RawMessage(nil), payload...), 0, 1, false, nil
	}
	return e
}

func derivedExport[T any](name, field string, source *Derived[T], class ExportClass, schema string, maxBytes int, freeze exportFreezer[T]) ReadExport {
	e := newExport(name, field, class, DerivedExportKind, schema, maxBytes)
	if source == nil || freeze == nil {
		panic("store: derived export requires a non-nil store and freezer")
	}
	e.version = func() Version { return source.Version() }
	e.capture = func(offset, limit int) (json.RawMessage, uint64, uint64, bool, error) {
		if offset != 0 {
			return nil, 0, 0, false, fmt.Errorf("store: derived export offset must be zero")
		}
		value, dirty := source.Peek()
		payload, err := freeze(value)
		if err != nil {
			return nil, 0, 0, false, err
		}
		if len(payload) > e.maxBytes {
			return nil, 0, 0, false, fmt.Errorf("store: derived export exceeds %d bytes", e.maxBytes)
		}
		return append(json.RawMessage(nil), payload...), uint64(source.Version()), 1, dirty, nil
	}
	return e
}

// JSONFreezer is a typed convenience for small immutable DTOs and scalar
// values. Collection/freezer declarations for mutable payloads should copy
// explicitly instead of returning shared slices or maps.
func JSONFreezer[T any](value T) (json.RawMessage, error) {
	payload, err := json.Marshal(value)
	return json.RawMessage(payload), err
}
