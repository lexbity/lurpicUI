package cook

import (
	"bytes"
	"encoding/gob"
	"testing"
)

type layoutConfig struct {
	Name    string   `toml:"name" json:"name"`
	Columns int      `toml:"columns" json:"columns"`
	Ratios  []string `toml:"ratios" json:"ratios"`
	Enabled bool     `toml:"enabled" json:"enabled"`
}

func TestConfigCompilerCompileTOML(t *testing.T) {
	compiler := &ConfigCompiler{SourcePath: "assets/config/theme.toml"}
	compiler.Register("assets/config/*.toml", layoutConfig{})

	src := []byte(`
name = "dashboard"
columns = 3
ratios = ["1:1", "16:9"]
enabled = true
`)
	lods, err := compiler.Compile(src, PlatformLinux)
	if err != nil {
		t.Fatalf("compile toml: %v", err)
	}
	if len(lods) != 1 || lods[0].Level != 0 || len(lods[0].Data) == 0 {
		t.Fatalf("unexpected lods: %+v", lods)
	}

	var got layoutConfig
	if err := gob.NewDecoder(bytes.NewReader(lods[0].Data)).Decode(&got); err != nil {
		t.Fatalf("decode gob: %v", err)
	}
	if got.Name != "dashboard" || got.Columns != 3 || len(got.Ratios) != 2 || got.Ratios[0] != "1:1" || got.Ratios[1] != "16:9" || !got.Enabled {
		t.Fatalf("unexpected decoded config: %+v", got)
	}
}

func TestConfigCompilerCompileJSON(t *testing.T) {
	compiler := &ConfigCompiler{SourcePath: "assets/config/theme.json"}
	compiler.Register("assets/config/*.json", layoutConfig{})

	src := []byte(`{"name":"dashboard","columns":4,"ratios":["4:3"],"enabled":false}`)
	lods, err := compiler.Compile(src, PlatformLinux)
	if err != nil {
		t.Fatalf("compile json: %v", err)
	}
	if len(lods) != 1 || lods[0].Level != 0 || len(lods[0].Data) == 0 {
		t.Fatalf("unexpected lods: %+v", lods)
	}

	var got layoutConfig
	if err := gob.NewDecoder(bytes.NewReader(lods[0].Data)).Decode(&got); err != nil {
		t.Fatalf("decode gob: %v", err)
	}
	if got.Name != "dashboard" || got.Columns != 4 || len(got.Ratios) != 1 || got.Ratios[0] != "4:3" || got.Enabled {
		t.Fatalf("unexpected decoded config: %+v", got)
	}
}

func TestConfigCompilerRejectsUnregisteredPattern(t *testing.T) {
	compiler := &ConfigCompiler{SourcePath: "assets/config/theme.toml"}
	if _, err := compiler.Compile([]byte(`name = "x"`), PlatformLinux); err == nil {
		t.Fatal("expected error for missing registration")
	}
}
