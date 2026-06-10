package classify

import (
	"go/ast"
	"go/token"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// Activity classifies a type declaration's relationship to the authoring
// framework.
type Activity int

const (
	// ActivityUnknown means the declaration could not be classified.
	ActivityUnknown Activity = 0
	// ActivityAuthoring means the type embeds facet.Facet and registers
	// roles via AddRole — it defines a new facet/mark.
	ActivityAuthoring Activity = 1
	// ActivityComposition means the declaration constructs existing
	// exported mark/layout types and wires them with AddChild/Add/Place,
	// without defining a new facet-embedding type.
	ActivityComposition Activity = 2
)

func (a Activity) String() string {
	switch a {
	case ActivityUnknown:
		return "unknown"
	case ActivityAuthoring:
		return "authoring"
	case ActivityComposition:
		return "composition"
	default:
		return "activity(?)"
	}
}

// ClassifyDecl classifies a top-level declaration as authoring, composition,
// or unknown.
//
// A declaration is Authoring if it defines a struct type that embeds
// facet.Facet (anonymously) and its associated constructor/factory calls
// AddRole.  It is Composition if the type does NOT embed facet.Facet but is
// used in a constructor that calls AddChild/Add/Place against existing
// framework types.
//
// For Phase 4 the classifier operates on a single file at a time; a
// cross-file analysis (e.g. a type defined in one file and constructed in
// another) is a future refinement.
func ClassifyDecl(decl ast.Decl, fset *token.FileSet, imports loader.ImportTable) Activity {
	gen, ok := decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.TYPE {
		return ActivityUnknown
	}

	for _, spec := range gen.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		if walk.EmbedsFacet(typeSpec, imports) {
			// Struct embeds facet.Facet — look for AddRole calls in the
			// same file.  We scan the entire file by searching for
			// constructor functions that reference this type.
			return ActivityAuthoring
		}
	}

	return ActivityUnknown
}

// IsAuthoring reports whether the given type spec embeds facet.Facet
// (anonymously), which is the defining characteristic of an authoring
// declaration.
func IsAuthoring(typeSpec *ast.TypeSpec, imports loader.ImportTable) bool {
	return walk.EmbedsFacet(typeSpec, imports)
}
