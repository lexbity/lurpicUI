package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileCache caches parsed Go files keyed by (path, mtime, size) so that
// unchanged files are not re-parsed on subsequent Load calls.
type FileCache struct {
	entries map[cacheKey]*cacheEntry
}

type cacheKey struct {
	Path    string
	ModTime int64
	Size    int64
}

type cacheEntry struct {
	AST  *ast.File
	Data []byte
}

// NewFileCache creates an empty file cache.
func NewFileCache() *FileCache {
	return &FileCache{entries: make(map[cacheKey]*cacheEntry)}
}

// lookup checks the cache for a valid entry for the given file path.
// Returns the AST and file data if found, or nil if not cached/stale.
func (fc *FileCache) lookup(path string) (*ast.File, []byte, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, false
	}
	key := cacheKey{
		Path:    path,
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
	}
	entry, ok := fc.entries[key]
	if !ok {
		return nil, nil, false
	}
	return entry.AST, entry.Data, true
}

// store adds a parsed file to the cache.
func (fc *FileCache) store(path string, astFile *ast.File, data []byte) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	fc.entries[cacheKey{
		Path:    path,
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
	}] = &cacheEntry{AST: astFile, Data: data}
}

// Config controls loader behaviour.
type Config struct {
	// IncludeTests, when true, includes _test.go files in the parsed output.
	// By default test files are excluded.
	IncludeTests bool

	// Root is the module root directory used for resolving relative patterns.
	// If empty, the current working directory is used.
	Root string

	// Cache optionally caches parsed files to avoid re-parsing.
	Cache *FileCache
}

// Load discovers, parses, and organises Go source files matching the given
// patterns.  It returns a LoadResult containing all parsed files grouped by
// package, or an error if any file cannot be read or parsed.
//
// Patterns may be:
//   - Go package directory paths (relative or absolute)
//   - Directory paths suffixed with /... to recursively include sub-packages
//   - Specific .go file paths
//
// Results are deterministic: files within each package and the top-level
// Files slice are sorted by path.
func Load(patterns []string, cfg Config) (*LoadResult, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}

	fset := token.NewFileSet()

	dirs, err := resolveDirs(patterns, cfg.Root)
	if err != nil {
		return nil, err
	}

	sort.Strings(dirs)

	var allFiles []*ParsedFile
	pkgMap := make(map[string]*Package, len(dirs))

	for _, dir := range dirs {
		pkg, err := loadPackage(dir, fset, cfg)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", dir, err)
		}
		if pkg == nil {
			continue
		}
		pkgMap[dir] = pkg
		allFiles = append(allFiles, pkg.Files...)
	}

	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Path < allFiles[j].Path
	})

	return &LoadResult{
		Files:    allFiles,
		Packages: pkgMap,
		Fset:     fset,
	}, nil
}

// resolveDirs expands a list of patterns into a set of unique package
// directories.
func resolveDirs(patterns []string, root string) ([]string, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	var dirs []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		if strings.HasSuffix(pattern, "/...") {
			base := strings.TrimSuffix(pattern, "/...")
			if base == "" {
				base = "."
			}
			// If base is relative and root is set, join them.
			absBase := resolvePath(base, root)

			info, err := os.Stat(absBase)
			if err != nil {
				return nil, fmt.Errorf("pattern %s: %w", pattern, err)
			}
			if !info.IsDir() {
				return nil, fmt.Errorf("pattern %s: not a directory", pattern)
			}

			err = filepath.WalkDir(absBase, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					return nil
				}
				// Skip hidden directories (except the root) and vendor/testdata.
				if path != absBase {
					baseName := d.Name()
					if strings.HasPrefix(baseName, ".") {
						return filepath.SkipDir
					}
					if baseName == "vendor" || baseName == "testdata" {
						return filepath.SkipDir
					}
				}
				hasGo, err := hasGoFiles(path)
				if err != nil {
					return err
				}
				if hasGo && !seen[path] {
					seen[path] = true
					dirs = append(dirs, path)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walking %s: %w", absBase, err)
			}
		} else {
			p := resolvePath(pattern, root)
			info, err := os.Stat(p)
			if err != nil {
				return nil, fmt.Errorf("pattern %s: %w", pattern, err)
			}
			if info.IsDir() {
				if !seen[p] {
					seen[p] = true
					dirs = append(dirs, p)
				}
			} else if strings.HasSuffix(pattern, ".go") {
				dir := filepath.Dir(p)
				if !seen[dir] {
					seen[dir] = true
					dirs = append(dirs, dir)
				}
			} else {
				return nil, fmt.Errorf("unsupported pattern: %s (not a directory or .go file)", pattern)
			}
		}
	}

	sort.Strings(dirs)
	return dirs, nil
}

// resolvePath resolves a potentially relative pattern against the given root.
func resolvePath(pattern, root string) string {
	if filepath.IsAbs(pattern) {
		return filepath.Clean(pattern)
	}
	return filepath.Clean(filepath.Join(root, pattern))
}

// hasGoFiles reports whether dir contains at least one .go file.
func hasGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

// loadPackage reads and parses all Go source files in a single directory,
// returning a Package or nil if the directory contains no parseable Go files.
func loadPackage(dir string, fset *token.FileSet, cfg Config) (*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var (
		pkgName string
		files   []*ParsedFile
	)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") && !cfg.IncludeTests {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		pf, err := parseFile(filePath, fset, cfg.Cache)
		if err != nil {
			return nil, err
		}

		if pf.Pkg == "" {
			continue // file with no package declaration (e.g. empty or assembly)
		}

		if pkgName == "" {
			pkgName = pf.Pkg
		} else if pf.Pkg != pkgName {
			return nil, fmt.Errorf(
				"%s: package name %q does not match %q declared in %s",
				filePath, pf.Pkg, pkgName, files[0].Path)
		}

		files = append(files, pf)
	}

	if len(files) == 0 {
		return nil, nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return &Package{
		Name:  pkgName,
		Path:  dir,
		Files: files,
	}, nil
}

// parseFile reads and parses a single Go source file, preserving comments.
// If cache is non-nil, it is checked for a previously-parsed result and
// updated after parsing.
func parseFile(path string, fset *token.FileSet, cache *FileCache) (*ParsedFile, error) {
	var data []byte
	var astFile *ast.File

	// Check cache first.
	if cache != nil {
		if cachedAST, _, ok := cache.lookup(path); ok {
			astFile = cachedAST
		}
	}

	// Parse or re-parse.
	if astFile == nil {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		astFile, err = parser.ParseFile(fset, path, data, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if cache != nil {
			cache.store(path, astFile, data)
		}
	}

	pkgName := ""
	if astFile.Name != nil {
		pkgName = astFile.Name.Name
	}

	return &ParsedFile{
		Fset:    fset,
		AST:     astFile,
		Path:    path,
		Pkg:     pkgName,
		Imports: BuildImportTable(astFile.Imports),
	}, nil
}
