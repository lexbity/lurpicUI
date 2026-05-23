package cook

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/assets"
)

// AssetSource is one cooked source asset presented to the resolver.
type AssetSource struct {
	Path string
	Type assets.AssetType
	LODs []CompiledLOD
}

// AssetNode is a resolved asset with a stable ID.
type AssetNode struct {
	ID   assets.AssetID
	Path string
	Type assets.AssetType
	LODs []CompiledLOD
}

// ConfigNode is a config asset with resolved leaf dependencies.
type ConfigNode struct {
	AssetNode
	Deps []assets.AssetID
}

// DependencyTree is the phase-9 flat dependency tree.
type DependencyTree struct {
	Leaves  []AssetNode
	Configs []ConfigNode
}

// ResolveDependencyTree assigns IDs, partitions assets into leaves/configs, and validates config dependencies.
func ResolveDependencyTree(manifest *Manifest, registry *UUIDRegistry, sources []AssetSource) (*DependencyTree, error) {
	if registry == nil {
		registry = NewUUIDRegistry()
	}

	sourceByPath := make(map[string]AssetSource, len(sources))
	nodes := make(map[string]AssetNode, len(sources))
	for _, src := range sources {
		canonical, err := canonicalizePath(src.Path)
		if err != nil {
			return nil, err
		}
		src.Path = canonical
		if _, exists := sourceByPath[canonical]; exists {
			return nil, fmt.Errorf("duplicate source asset %q", canonical)
		}
		if src.Type == 0 {
			src.Type = inferAssetType(canonical)
		}
		if src.Type == assets.AssetTypeConfig && !isConfigPath(canonical) {
			return nil, fmt.Errorf("config asset must use a config extension: %q", canonical)
		}
		sourceByPath[canonical] = src
	}

	paths := make([]string, 0, len(sourceByPath))
	for path := range sourceByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	tree := &DependencyTree{}
	for _, path := range paths {
		src := sourceByPath[path]
		id, err := registry.Assign(path)
		if err != nil {
			return nil, fmt.Errorf("assign id for %q: %w", path, err)
		}
		node := AssetNode{
			ID:   id,
			Path: path,
			Type: src.Type,
			LODs: append([]CompiledLOD(nil), src.LODs...),
		}
		nodes[path] = node
		if src.Type == assets.AssetTypeConfig {
			tree.Configs = append(tree.Configs, ConfigNode{AssetNode: node})
		} else {
			tree.Leaves = append(tree.Leaves, node)
		}
	}

	configDeps := map[string][]assets.AssetID{}
	for _, cfg := range tree.Configs {
		rule := ConfigRule{}
		if manifest != nil && manifest.Config != nil {
			if found, ok := manifest.Config[cfg.Path]; ok {
				rule = found
			}
		}
		deps, err := resolveConfigDeps(cfg.Path, rule.Deps, sourceByPath, nodes)
		if err != nil {
			return nil, err
		}
		configDeps[cfg.Path] = deps
	}

	for i := range tree.Configs {
		tree.Configs[i].Deps = append([]assets.AssetID(nil), configDeps[tree.Configs[i].Path]...)
	}

	if manifest != nil {
		if regPath := manifest.RegistryPath(); regPath != "" {
			if err := registry.SaveTo(regPath); err != nil {
				return nil, fmt.Errorf("save uuid registry: %w", err)
			}
		}
	}

	sort.Slice(tree.Leaves, func(i, j int) bool { return tree.Leaves[i].Path < tree.Leaves[j].Path })
	sort.Slice(tree.Configs, func(i, j int) bool { return tree.Configs[i].Path < tree.Configs[j].Path })
	return tree, nil
}

func resolveConfigDeps(configPath string, deps []string, sources map[string]AssetSource, nodes map[string]AssetNode) ([]assets.AssetID, error) {
	if len(deps) == 0 {
		return nil, nil
	}
	out := make([]assets.AssetID, 0, len(deps))
	for _, depPath := range deps {
		canonical, err := canonicalizePath(depPath)
		if err != nil {
			return nil, err
		}
		src, ok := sources[canonical]
		if !ok {
			return nil, fmt.Errorf("config %q depends on missing asset %q", configPath, canonical)
		}
		if src.Type == assets.AssetTypeConfig || isConfigPath(canonical) {
			return nil, fmt.Errorf("config %q depends on nested config %q", configPath, canonical)
		}
		node, ok := nodes[canonical]
		if !ok {
			return nil, fmt.Errorf("config %q dependency %q has no resolved node", configPath, canonical)
		}
		out = append(out, node.ID)
	}
	return out, nil
}

func inferAssetType(path string) assets.AssetType {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		return assets.AssetTypeSVG
	case ".png", ".jpg", ".jpeg":
		return assets.AssetTypeImage
	case ".ttf", ".otf":
		return assets.AssetTypeFont
	case ".toml", ".json":
		return assets.AssetTypeConfig
	default:
		return assets.AssetTypeImage
	}
}

func isConfigPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml", ".json":
		return true
	default:
		return false
	}
}
