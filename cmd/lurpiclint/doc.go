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
// bug; render-import contracts (LL010),
// goroutine discipline (LL011), domain state (LL012), token capture (LL013),
// overlay contracts (LL014), stability evidence (LL015), store mutation in
// layout callbacks (LL016), asset misuse (LL017), and unmounted overlay
// detection (LL018).
//
// Usage:
//
//	lurpiclint check [flags] [packages...]   # run rules, the build gate
//	lurpiclint capabilities [flags]          # emit the uxauthoring index
//	lurpiclint explain <rule-id>             # print a rule's rationale + fix
//	lurpiclint version                       # print version information
package main
