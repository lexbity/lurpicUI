// Package mcptemplate carries the embedded scaffolding for per-app MCP
// bridge binaries (LDP Phase 9 / FR-22): the framework carries no MCP
// dependency — the generated bridge's module belongs to the app project and
// declares the MCP SDK itself.
package mcptemplate

import _ "embed"

// MainGo is the generated bridge's main.go template. Placeholders:
// {{.AppName}} — the app name; {{.Tools}} — one RegisterTool block per LDP
// method (rendered by cmd/lurpic/mcp.go from the schema); {{.ToolCalls}} —
// the per-tool typed dispatch.
//
//go:embed main.go.tmpl
var MainGo string

// GoMod is the generated bridge's go.mod. The module path and MCP SDK
// requirement belong to the app project.
//
//go:embed go.mod.tmpl
var GoMod string
