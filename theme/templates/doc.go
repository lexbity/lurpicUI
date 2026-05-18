// Package templates defines the canonical template-theme schema.
//
// The package freezes the template-theme contract: semantic token groups,
// font fallback policy, density scaling, chart inheritance rules, recipe
// bundle catalogs, shipped theme factories, validation reports, and the shape
// of a template-theme record. It is intentionally separate from the older
// theme package so the new contract can stabilize without breaking callers.
//
// Authoring guidance lives in docs/theme-template-authoring.md.
package templates
