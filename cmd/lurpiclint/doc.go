// lurpiclint is the static analyzer and capability-awareness tool for lurpicUI
// applications.
//
// It detects reinvention (hand-rolled layout where framework marks exist),
// enforces framework contracts via machine-checkable rules, and supplies
// awareness through the uxauthoring index.
//
// The ruleset covers layout encapsulation (LL001), which flags raw
// facet.LayoutRole OnMeasure/OnArrange population — both via composite
// literals and via field assignment — outside the layout/ or marks/
// packages; coordinate reinvention (LL002), container detection (LL003)
// which now detects both composite-literal and field-assigned child-arranging
// LayoutRoles, including arrangement hidden behind same-package helper
// functions (e.g.  arrangeChildAtCtx); shape-matching suggestions (LL004);
// missing-layout-role detection (LL019), which flags types that embed
// facet.Facet and add children but register no LayoutRole — the blank-canvas
// bug; direct callback invocation and field-write detection (LL020), which
// flags direct OnMeasure/OnArrange calls and ArrangedBounds/MeasuredSize
// writes outside the framework; render-import contracts (LL010),
// goroutine discipline (LL011), domain state (LL012), token capture (LL013),
// overlay contracts (LL014), stability evidence (LL015), store mutation in
// layout callbacks (LL016), asset misuse (LL017), and unmounted overlay
// detection (LL018); sibling-overlay detection (LL021), which flags overlays
// mounted as plain children without layer/ZPriority; unbounded-fixed-stack
// detection (LL022), which flags all-Fixed ColumnLayout/RowLayout without a
// ScrollRegion; and reactive-binding overwrite detection (LL023), which flags
// marks.Const assignments that sever reactive FromStore/FromDerived bindings.
//
// The lint gate is green-by-default at HEAD (modulo baselined debt in
// lurpiclint-baseline.json).  A red gate means new debt introduced by the
// change under review, never inherited noise.
//
// Package verifylayout provides runtime layout-tree assertions backed by
// layout.System.  It drives one measure+arrange pass and walks the facet
// tree checking structural soundness: rendered facets have non-empty bounds,
// siblings don't overlap, and children don't overflow their parent (with
// sanctioned exemptions).
//
// Library mode (recommended) — import verifylayout in your app's _test.go:
//
//	import vl "codeburg.org/lexbit/lurpicui/cmd/lurpiclint/verifylayout"
//
//	func TestVerifyLayout(t *testing.T) {
//	    root := BuildRoot()
//	    vl.Assert(t, root, vl.Options{Size: gfx.Size{W: 1280, H: 800}})
//	}
//
// CLI mode — generates a transient lurpiclint_verifylayout_test.go in the
// target package and runs it via go test.  The generated file calls the
// builder with app.BuildContext{WindowSize: <size>, ContentScale: 1} and
// cleans up on exit.  Requires the go toolchain.  The builder must accept
// a minimal BuildContext (nil FontRegistry/Theme is safe for builders that
// do not dereference them).
//
//	lurpiclint verify-layout -builder BuildRoot -size 1280x800 <package>
//
// Usage:
//
//	lurpiclint check [flags] [packages...]         # run rules, the build gate
//	lurpiclint capabilities [flags]                # emit the uxauthoring index
//	lurpiclint explain <rule-id>                   # print a rule's rationale + fix
//	lurpiclint baseline generate [flags] [pkgs]    # generate baseline JSON from current findings
//	lurpiclint verify-layout [flags] <package>     # run layout-tree assertion (CLI mode)
//	lurpiclint version                             # print version information
package main
