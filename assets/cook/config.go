package cook

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// ConfigCompiler encodes registered Go types to gob binaries.
type ConfigCompiler struct {
	// SourcePath is used for extension and glob matching.
	SourcePath string

	types map[string]reflect.Type
}

// Extensions reports the handled source file extensions.
func (c *ConfigCompiler) Extensions() []string {
	return []string{".toml", ".json"}
}

// Register associates a filename glob with a prototype Go value.
func (c *ConfigCompiler) Register(glob string, proto any) {
	if c.types == nil {
		c.types = make(map[string]reflect.Type)
	}
	gob.Register(proto)
	c.types[glob] = reflect.TypeOf(proto)
}

// Compile parses src into the registered target type and returns gob encoded output.
func (c *ConfigCompiler) Compile(src []byte, target Platform) ([]CompiledLOD, error) {
	_ = target

	targetType, err := c.resolveTargetType()
	if err != nil {
		return nil, err
	}

	val := reflect.New(targetType)
	if err := c.decodeConfig(src, val.Interface()); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val.Interface()); err != nil {
		return nil, fmt.Errorf("gob encode: %w", err)
	}

	return []CompiledLOD{{Level: 0, Data: append([]byte(nil), buf.Bytes()...)}}, nil
}

func (c *ConfigCompiler) decodeConfig(src []byte, dst any) error {
	switch strings.ToLower(filepath.Ext(c.SourcePath)) {
	case ".json":
		if err := json.Unmarshal(src, dst); err != nil {
			return fmt.Errorf("json decode: %w", err)
		}
		return nil
	case ".toml", "":
		if _, err := toml.Decode(string(src), dst); err != nil {
			return fmt.Errorf("toml decode: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported config extension %q", filepath.Ext(c.SourcePath))
	}
}

func (c *ConfigCompiler) resolveTargetType() (reflect.Type, error) {
	if len(c.types) == 0 {
		return nil, fmt.Errorf("config compiler has no registered types")
	}
	path := filepath.ToSlash(c.SourcePath)
	var patterns []string
	for pattern := range c.types {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool { return patterns[i] < patterns[j] })
	for _, pattern := range patterns {
		ok, err := filepath.Match(pattern, path)
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", pattern, err)
		}
		if ok {
			return c.types[pattern], nil
		}
	}
	return nil, fmt.Errorf("no config type registered for %q", c.SourcePath)
}
