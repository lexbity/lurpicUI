package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

func (rt *Runtime) registerTreeReadExports(root facet.FacetImpl) error {
	if root == nil || root.Base() == nil {
		return nil
	}
	type node struct{ impl facet.FacetImpl }
	stack := []node{{impl: root}}
	seen := make(map[facet.FacetID]struct{})
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.impl == nil || current.impl.Base() == nil || current.impl.Base().State() == facet.StateDisposed {
			continue
		}
		base := current.impl.Base()
		if _, exists := seen[base.ID()]; exists {
			continue
		}
		seen[base.ID()] = struct{}{}
		if provider, ok := current.impl.(store.ReadExportProvider); ok {
			if err := rt.registerFacetReadExports(base.ID(), provider.ReadExports()); err != nil {
				return fmt.Errorf("facet %s read exports: %w", base.TestID(), err)
			}
		}
		children := rt.visibleChildren(current.impl)
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, node{impl: children[i]})
		}
	}
	return nil
}

func (rt *Runtime) registerFacetReadExports(id facet.FacetID, exports []store.ReadExport) error {
	if len(exports) == 0 {
		return nil
	}
	if rt.readExportsFrozen {
		return fmt.Errorf("read export catalog is frozen")
	}
	if rt.facetReadExports[id] == nil {
		rt.facetReadExports[id] = make(map[string]store.ReadExport, len(exports))
	}
	for _, export := range exports {
		if !export.Valid() {
			return fmt.Errorf("invalid export declaration")
		}
		if owner := export.OwnerScope(); owner != nil && !rt.config.StoreScope.Owns(owner) {
			return fmt.Errorf("export source scope is outside the configured application scope")
		}
		info := export.Info()
		if info.Name != "" {
			if owner, exists := rt.namedExportOwners[info.Name]; exists {
				if owner != id {
					return fmt.Errorf("named export %q has ambiguous owners %d and %d", info.Name, owner, id)
				}
				registered, _ := rt.namedStores.Lookup(info.Name)
				if !sameExportInfo(registered.Info(), info) {
					return fmt.Errorf("named export %q changed its declaration", info.Name)
				}
				continue
			}
			if _, exists := rt.namedStores.Lookup(info.Name); exists {
				return fmt.Errorf("named export %q conflicts with an app-level declaration", info.Name)
			}
			if err := rt.namedStores.Register(export); err != nil {
				return err
			}
			rt.namedExportOwners[info.Name] = id
			continue
		}
		if existing, exists := rt.facetReadExports[id][info.Field]; exists {
			if !sameExportInfo(existing.Info(), info) {
				return fmt.Errorf("field capability %q changed its declaration", info.Field)
			}
			continue
		}
		rt.facetReadExports[id][info.Field] = export
	}
	return nil
}

func sameExportInfo(a, b store.ExportInfo) bool {
	return a.Name == b.Name && a.Field == b.Field && a.Class == b.Class && a.Kind == b.Kind && a.Schema == b.Schema && a.MaxBytes == b.MaxBytes
}

// RegisterReadExport adds a named app capability before the contract catalog
// is frozen. The listener calls FreezeReadExports before it starts.
func (rt *Runtime) RegisterReadExport(export store.ReadExport) error {
	if !export.Valid() || export.Info().Name == "" || export.Info().Field != "" {
		return fmt.Errorf("runtime: expected a valid named read export")
	}
	if rt.readExportsFrozen {
		return fmt.Errorf("runtime: read export catalog is frozen")
	}
	if _, exists := rt.namedStores.Lookup(export.Info().Name); exists {
		return fmt.Errorf("runtime: named export %q is already declared", export.Info().Name)
	}
	if err := rt.namedStores.Register(export); err != nil {
		return err
	}
	rt.namedExportOwners[export.Info().Name] = 0
	return nil
}

// FreezeReadExports freezes the public names and schemas before an external
// listener publishes the app contract.
func (rt *Runtime) FreezeReadExports() {
	if rt.readExportsFrozen {
		return
	}
	rt.namedStores.Freeze()
	rt.readExportsFrozen = true
	if rt.drivePort != nil {
		rt.drivePort.setExportSchemaDigest(rt.exportSchemaDigest())
	}
}

// exportSchemaDigest fingerprints only declared export contracts. Facet
// instances, TestIDs, values, versions, and current visibility are excluded;
// structural capabilities are represented by descriptor kind plus their
// explicit author-declared field name.
func (rt *Runtime) exportSchemaDigest() string {
	contracts := make([]string, 0, len(rt.namedStores.Names())+len(rt.facetReadExports)*2)
	for _, name := range rt.namedStores.Names() {
		export, _ := rt.namedStores.Lookup(name)
		contracts = append(contracts, exportSchemaEntry("named", name, export.Info()))
	}
	patterns := make(map[string]struct{})
	for _, exports := range rt.facetReadExports {
		for field, export := range exports {
			// Structural field addresses are declared capabilities, independent
			// of which currently materialized facet instance supplies them.
			key := field + "\x00" + export.Info().Schema + "\x00" + export.Info().Kind.String()
			if _, exists := patterns[key]; exists {
				continue
			}
			patterns[key] = struct{}{}
			contracts = append(contracts, exportSchemaEntry("facet:*", field, export.Info()))
		}
	}
	sort.Strings(contracts)
	digest := sha256.Sum256([]byte("lurpic-export-schema-v1\x00" + strings.Join(contracts, "\n")))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func exportSchemaEntry(scope, address string, info store.ExportInfo) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d", scope, address, info.Class.String(), info.Kind.String(), info.Schema, info.Name, info.MaxBytes)
}

func (rt *Runtime) readExportFor(target facet.FacetID, field string) (store.ReadExport, bool) {
	exports := rt.facetReadExports[target]
	export, ok := exports[field]
	return export, ok
}
