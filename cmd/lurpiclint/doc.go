// lurpiclint is the static analyzer and capability-awareness tool for lurpicUI
// applications.
//
// It detects reinvention (hand-rolled layout where framework marks exist),
// enforces framework contracts via machine-checkable rules, and supplies
// awareness through the uxauthoring index.
//
// Usage:
//
//	lurpiclint check [flags] [packages...]   # run rules, the build gate
//	lurpiclint capabilities [flags]          # emit the uxauthoring index
//	lurpiclint explain <rule-id>             # print a rule's rationale + fix
//	lurpiclint version                       # print version information
package main
