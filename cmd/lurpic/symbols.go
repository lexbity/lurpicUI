package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type symbolSet struct {
	// ABIMap maps ABI (e.g. "arm64-v8a") to a list of absolute .so paths
	// for that architecture.
	ABIMap map[string][]string
	// BuildDir is the project build directory where the symbols were found.
	BuildDir string
}

// findSymbolSet locates unstripped .so files in the build directory for all
// ABIs. It checks two locations:
//
//  1. build/android/lib/<abi>/*.so  — staged native libs (potentially stripped
//     in release builds but present for debug builds).
//  2. build/android/native-debug-symbols/<abi>/*.so — unstripped copies
//     retained from a prior build.
//
// When both exist for the same ABI, the debug-symbols copy takes precedence.
func findSymbolSet(buildDir string) (*symbolSet, error) {
	s := &symbolSet{
		ABIMap:   make(map[string][]string),
		BuildDir: buildDir,
	}

	libRoot := filepath.Join(buildDir, "android", "lib")
	if _, err := os.Stat(libRoot); err == nil {
		entries, err := os.ReadDir(libRoot)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", libRoot, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			abi := entry.Name()
			abis, err := collectSOFiles(filepath.Join(libRoot, abi))
			if err != nil {
				return nil, fmt.Errorf("collect .so for %s: %w", abi, err)
			}
			if len(abis) > 0 {
				s.ABIMap[abi] = abis
			}
		}
	}

	symbolRoot := filepath.Join(buildDir, "android", "native-debug-symbols")
	if _, err := os.Stat(symbolRoot); err == nil {
		entries, err := os.ReadDir(symbolRoot)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", symbolRoot, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			abi := entry.Name()
			abis, err := collectSOFiles(filepath.Join(symbolRoot, abi))
			if err != nil {
				return nil, fmt.Errorf("collect debug symbols for %s: %w", abi, err)
			}
			if len(abis) > 0 {
				s.ABIMap[abi] = abis
			}
		}
	}

	if len(s.ABIMap) == 0 {
		return nil, fmt.Errorf("no .so files found in %s (run 'lurpic build android' first)", buildDir)
	}

	return s, nil
}

// collectSOFiles walks dir and returns all .so files found, sorted.
func collectSOFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".so") {
			result = append(result, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(result)
	return result, nil
}

// symbolDirForNDKStack returns the directory to pass to ndk-stack -sym.
// ndk-stack requires a single directory containing the unstripped .so files
// for the crashing ABI. If the debug-symbols directory exists for the given
// ABI, it is returned; otherwise the lib/<abi> directory.
func (s *symbolSet) symbolDirForNDKStack(abi string) string {
	symbolRoot := filepath.Join(s.BuildDir, "android", "native-debug-symbols", abi)
	if _, err := os.Stat(symbolRoot); err == nil {
		return symbolRoot
	}
	libDir := filepath.Join(s.BuildDir, "android", "lib", abi)
	return libDir
}

// ABIs returns the sorted list of available ABIs.
func (s *symbolSet) ABIs() []string {
	var abis []string
	for abi := range s.ABIMap {
		abis = append(abis, abi)
	}
	sort.Strings(abis)
	return abis
}

func (s *symbolSet) String() string {
	var b strings.Builder
	b.WriteString("Symbol sets:\n")
	for _, abi := range s.ABIs() {
		b.WriteString(fmt.Sprintf("  %s:\n", abi))
		for _, so := range s.ABIMap[abi] {
			b.WriteString(fmt.Sprintf("    %s\n", so))
		}
	}
	return b.String()
}
