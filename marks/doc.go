// Package marks defines the shared authored mark taxonomy and registry.
//
// Import constraints:
//   - This package may depend on engine packages that define shared runtime
//     contracts, but it must not depend on any concrete mark family package.
//   - Concrete mark families live under marks/* and may import this package.
package marks
