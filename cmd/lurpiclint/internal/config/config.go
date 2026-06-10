// Package config handles .lurpiclint.toml loading, merging, and the
// precedence chain: defaults < config file < CLI flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// RuleConfig overrides a single rule's severity.
type RuleConfig struct {
	Severity string `toml:"severity"`
}

// PathsConfig controls which files are excluded from analysis.
type PathsConfig struct {
	Exclude []string `toml:"exclude"`
}

// CapabilitiesConfig controls which packages are introspected for the index.
type CapabilitiesConfig struct {
	Roots []string `toml:"roots"`
}

// LurpiclintConfig holds top-level config options.
type LurpiclintConfig struct {
	FailOn string `toml:"fail_on"`
}

// Config is the root .lurpiclint.toml structure.
type Config struct {
	Lurpiclint   LurpiclintConfig            `toml:"lurpiclint"`
	Rules        map[string]RuleConfig       `toml:"rules"`
	Paths        PathsConfig                 `toml:"paths"`
	Capabilities CapabilitiesConfig          `toml:"capabilities"`
}

// DefaultConfig returns a Config with the defaults.
func DefaultConfig() Config {
	return Config{
		Lurpiclint: LurpiclintConfig{
			FailOn: "error",
		},
		Rules: map[string]RuleConfig{},
		Paths: PathsConfig{},
	}
}

// LoadFile reads and parses a .lurpiclint.toml file.
func LoadFile(path string) (Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

// Discover walks upward from dir looking for .lurpiclint.toml.
// Returns empty path and nil if not found (not an error).
func Discover(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(abs, ".lurpiclint.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", nil
		}
		abs = parent
	}
}

// PathExcluded reports whether the given file path matches any of the
// exclude globs in the config.  Glob matching is simple: checks if the path
// contains the pattern as a substring, or matches **/pattern.
func (c *Config) PathExcluded(path string) bool {
	for _, pattern := range c.Paths.Exclude {
		if matchGlob(path, pattern) {
			return true
		}
	}
	return false
}

// matchGlob implements a simple glob matcher for exclude patterns.
func matchGlob(path, pattern string) bool {
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	// **/ prefix — match the remainder against the basename.
	if strings.HasPrefix(pattern, "**/") {
		rest := pattern[3:]
		base := filepath.Base(path)
		matched, err := filepath.Match(rest, base)
		if err == nil && matched {
			return true
		}
		// Also try matching against the full path relative parts.
		matched, err = filepath.Match(rest, path)
		return err == nil && matched
	}

	// /** suffix — match the prefix as a directory prefix.
	if strings.HasSuffix(pattern, "/**") {
		prefix := pattern[:len(pattern)-3]
		return strings.HasPrefix(path, prefix)
	}

	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// RuleSeverity returns the configured severity for a rule, or empty string
// if not overridden.
func (c *Config) RuleSeverity(ruleID string) string {
	if c == nil {
		return ""
	}
	r, ok := c.Rules[ruleID]
	if !ok {
		return ""
	}
	return r.Severity
}
