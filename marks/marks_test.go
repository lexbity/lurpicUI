package marks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

type packageInfo struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func TestFamily_string_roundtrip(t *testing.T) {
	for _, family := range []Family{
		FamilyBasic,
		FamilyStructure,
		FamilyAnnotation,
		FamilyUIInput,
		FamilyUINav,
		FamilyUINotification,
		FamilyChart,
	} {
		got, ok := ParseFamily(family.String())
		if !ok || got != family {
			t.Fatalf("family roundtrip failed: %v -> %v %v", family, got, ok)
		}
	}
}

func TestConstructionClass_string_roundtrip(t *testing.T) {
	for _, class := range []ConstructionClass{
		ConstructionPrimitive,
		ConstructionComposed,
		ConstructionGenerated,
	} {
		got, ok := ParseConstructionClass(class.String())
		if !ok || got != class {
			t.Fatalf("construction roundtrip failed: %v -> %v %v", class, got, ok)
		}
	}
}

func TestDescriptor_registry_lookup(t *testing.T) {
	resetDescriptorRegistryForTest()
	d := Descriptor{
		Family:            FamilyAnnotation,
		ConstructionClass: ConstructionComposed,
		Type:              TypeName("annotation:label"),
		Focusable:         false,
		HitTestable:       true,
		Customizable:      true,
	}
	RegisterDescriptor(d)
	got, ok := DescriptorFor(d.Type)
	if !ok || got != d {
		t.Fatalf("descriptor lookup = %#v %v", got, ok)
	}
}

func TestDescriptor_registry_duplicate_panics(t *testing.T) {
	resetDescriptorRegistryForTest()
	d := Descriptor{
		Family:            FamilyBasic,
		ConstructionClass: ConstructionPrimitive,
		Type:              TypeName("basic:rect"),
	}
	RegisterDescriptor(d)
	mustPanic(t, func() { RegisterDescriptor(d) })
}

func TestAllDescriptors_sorted_stable(t *testing.T) {
	resetDescriptorRegistryForTest()
	RegisterDescriptor(Descriptor{Family: FamilyChart, ConstructionClass: ConstructionPrimitive, Type: TypeName("chart:zeta")})
	RegisterDescriptor(Descriptor{Family: FamilyBasic, ConstructionClass: ConstructionPrimitive, Type: TypeName("basic:alpha")})
	RegisterDescriptor(Descriptor{Family: FamilyAnnotation, ConstructionClass: ConstructionComposed, Type: TypeName("annotation:beta")})
	got := AllDescriptors()
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Type != "annotation:beta" || got[1].Type != "basic:alpha" || got[2].Type != "chart:zeta" {
		t.Fatalf("sorted order = %#v", got)
	}
}

func TestMarkDescriptor_flags_consistent(t *testing.T) {
	resetDescriptorRegistryForTest()
	mustPanic(t, func() {
		RegisterDescriptor(Descriptor{
			Family:            FamilyBasic,
			ConstructionClass: ConstructionPrimitive,
			Type:              TypeName("basic:line"),
			ChildHosting:      true,
		})
	})
}

func TestNoEnginePackageImportsMarks(t *testing.T) {
	pkgs := mustLoadPackages(t, "../...")
	for _, pkg := range pkgs {
		if strings.HasPrefix(pkg.ImportPath, "codeburg.org/lexbit/lurpicui/marks") {
			continue
		}
		for _, imp := range append(append([]string(nil), pkg.Imports...), append(pkg.TestImports, pkg.XTestImports...)...) {
			if strings.HasPrefix(imp, "codeburg.org/lexbit/lurpicui/marks") {
				t.Fatalf("%s imports %s", pkg.ImportPath, imp)
			}
		}
	}
}

func TestMarksPackageGraphIsAcyclic(t *testing.T) {
	t.Skip("standard mark families intentionally import sibling mark packages")
}

func TestChartDoesNotImportUIFamilies(t *testing.T) {
	pkgs := mustLoadPackages(t, "./...")
	for _, pkg := range pkgs {
		if pkg.ImportPath != "codeburg.org/lexbit/lurpicui/marks/chart" {
			continue
		}
		for _, imp := range append(append([]string(nil), pkg.Imports...), append(pkg.TestImports, pkg.XTestImports...)...) {
			switch imp {
			case "codeburg.org/lexbit/lurpicui/marks/uiinput",
				"codeburg.org/lexbit/lurpicui/marks/uinav",
				"codeburg.org/lexbit/lurpicui/marks/uinotification":
				t.Fatalf("chart package imports UI family %s", imp)
			}
		}
	}
}

func resetDescriptorRegistryForTest() {
	descriptorsMu.Lock()
	descriptorsByType = make(map[TypeName]Descriptor)
	descriptorsMu.Unlock()
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func mustLoadPackages(t *testing.T, pattern string) []packageInfo {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", pattern)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list %s: %v\n%s", pattern, err, stdout.String())
	}
	dec := json.NewDecoder(&stdout)
	var pkgs []packageInfo
	for {
		var pkg packageInfo
		if err := dec.Decode(&pkg); err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("decode package: %v", err)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

func (p packageInfo) String() string {
	return fmt.Sprintf("%s (%d imports)", p.ImportPath, len(p.Imports))
}
