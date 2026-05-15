package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/marks"
)

func TestStructureDescriptors_areRegisteredAndComposed(t *testing.T) {
	cases := []struct {
		name string
		typ  marks.TypeName
	}{
		{name: "group", typ: "structure:group"},
		{name: "transform", typ: "structure:transform"},
		{name: "viewporthost", typ: "structure:viewporthost"},
		{name: "layermount", typ: "structure:layermount"},
		{name: "clip", typ: "structure:clip"},
		{name: "anchorproxy", typ: "structure:anchorproxy"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desc, ok := marks.DescriptorFor(tc.typ)
			if !ok {
				t.Fatalf("descriptor %q is not registered", tc.typ)
			}
			if desc.Family != marks.FamilyStructure {
				t.Fatalf("descriptor family = %v, want structure", desc.Family)
			}
			if desc.ConstructionClass != marks.ConstructionComposed {
				t.Fatalf("descriptor construction = %v, want composed", desc.ConstructionClass)
			}
			if !desc.ChildHosting {
				t.Fatalf("descriptor %q should host children", tc.typ)
			}
			if !desc.AnchorExporting {
				t.Fatalf("descriptor %q should export anchors", tc.typ)
			}
		})
	}
}
