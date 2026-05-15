// Package marks defines the shared authored mark taxonomy and registry.
//
// Marks describe authored geometry and interaction intent. They do not own
// shell placement; layer contracts own coordinate space and containment.
//
// Import constraints:
//   - This package may depend on engine packages that define shared runtime
//     contracts, but it must not depend on any concrete mark family package.
//   - Concrete mark families live under marks/* and may import this package.
package marks
