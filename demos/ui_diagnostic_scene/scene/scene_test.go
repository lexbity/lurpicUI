package scene

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestNewRegistry_empty(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Count() != 0 {
		t.Fatalf("expected empty registry, got %d scenes", r.Count())
	}
}

func TestRegistry_Register_and_Get(t *testing.T) {
	r := NewRegistry()
	def := Definition{
		ID:          "test-scene",
		DisplayName: "Test Scene",
		Description: "A test scene",
		Families:    []string{"basic"},
		Factory:     func() Scene { return nil },
	}

	r.Register(def)

	got, ok := r.Get("test-scene")
	if !ok {
		t.Fatal("expected to find registered scene")
	}
	if got.ID != "test-scene" {
		t.Fatalf("expected ID test-scene, got %s", got.ID)
	}
	if got.DisplayName != "Test Scene" {
		t.Fatalf("expected DisplayName 'Test Scene', got %s", got.DisplayName)
	}
}

func TestRegistry_Register_duplicate_ignored(t *testing.T) {
	r := NewRegistry()
	def1 := Definition{
		ID:          "test",
		DisplayName: "First",
		Factory:     func() Scene { return nil },
	}
	def2 := Definition{
		ID:          "test",
		DisplayName: "Second",
		Factory:     func() Scene { return nil },
	}

	r.Register(def1)
	r.Register(def2)

	got, _ := r.Get("test")
	if got.DisplayName != "First" {
		t.Fatal("expected first registration to be preserved")
	}
}

func TestRegistry_GetAll_order(t *testing.T) {
	r := NewRegistry()
	ids := []string{"scene-a", "scene-b", "scene-c"}
	for _, id := range ids {
		r.Register(Definition{
			ID:          id,
			DisplayName: id,
			Factory:     func() Scene { return nil },
		})
	}

	all := r.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 scenes, got %d", len(all))
	}
	for i, def := range all {
		if def.ID != ids[i] {
			t.Fatalf("expected %s at position %d, got %s", ids[i], i, def.ID)
		}
	}
}

func TestRegistry_Create(t *testing.T) {
	r := NewRegistry()

	r.Register(Definition{
		ID:          "factory-test",
		DisplayName: "Factory Test",
		Factory: func() Scene {
			return &testScene{id: "factory-test"}
		},
	})

	sc, ok := r.Create("factory-test")
	if !ok {
		t.Fatal("expected to create scene")
	}
	if sc == nil {
		t.Fatal("expected non-nil scene")
	}

	// Verify it's the right type
	ts, ok := sc.(*testScene)
	if !ok {
		t.Fatal("expected scene to be *testScene")
	}
	if ts.id != "factory-test" {
		t.Fatalf("expected scene id factory-test, got %s", ts.id)
	}
}

// testScene implements the Scene interface for testing
type testScene struct {
	id string
}

func (t *testScene) SceneID() string             { return t.id }
func (t *testScene) DisplayName() string         { return t.id }
func (t *testScene) BuildRoot() facet.FacetImpl  { return nil }
func (t *testScene) Reset()                      {}
func (t *testScene) ApplyTheme(theme.Context)    {}
func (t *testScene) ApplyDensity(float32)        {}
func (t *testScene) Capabilities() CapabilitySet { return CapabilitySet{} }
func (t *testScene) ExportState() map[string]any { return nil }
func (t *testScene) ImportState(map[string]any)  {}

func TestRegistry_Create_unknown(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Create("unknown")
	if ok {
		t.Fatal("expected Create to return false for unknown scene")
	}
}

func TestRegistry_Create_nil_factory(t *testing.T) {
	r := NewRegistry()
	r.Register(Definition{
		ID:          "nil-factory",
		DisplayName: "Nil Factory",
		Factory:     nil,
	})

	_, ok := r.Create("nil-factory")
	if ok {
		t.Fatal("expected Create to return false for nil factory")
	}
}
