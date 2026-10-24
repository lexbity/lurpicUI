package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpic/mcptemplate"
)

// cmdMCP implements `lurpic mcp init <app> --schema <path> --out dir`
// (LDP Phase 9 / FR-22): scaffold a per-app MCP bridge binary from the LDP
// schema — one MCP tool per method the schema grants. The framework carries
// no MCP dependency; the generated module belongs to the app project.
func cmdMCP(args []string) int {
	if len(args) == 0 || args[0] != "init" {
		fmt.Fprintln(os.Stderr, "mcp: only `init` is supported (lurpic mcp init <app> --schema <path> --out dir)")
		return 1
	}
	rest := args[1:] // drop "init"
	// --out is REQUIRED with no default: a silent default of "." would
	// overwrite the framework module's go.mod when run from the repo root
	// (this exact accident drove the requirement).
	schemaPath, outDir, appName := scanMCPArgs(rest, "drive/schema/schema.json", "", "")
	if appName == "" {
		fmt.Fprintln(os.Stderr, "mcp: app name is required (lurpic mcp init <app> --schema <path> --out dir)")
		return 1
	}
	if outDir == "" {
		fmt.Fprintln(os.Stderr, "mcp: --out dir is required (the scaffold writes its own go.mod — never point it at the framework module)")
		return 1
	}
	if err := mcpInit(appName, schemaPath, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "mcp: %v\n", err)
		return 1
	}
	return 0
}

// scanMCPArgs extracts --schema/--out values and the app-name positional
// from any order. Unknown flags are an error (fail-loud: a typo'd flag must
// not silently write the scaffold elsewhere).
func scanMCPArgs(args []string, defaultSchema, defaultOut, _ string) (schemaPath, outDir, appName string) {
	schemaPath, outDir = defaultSchema, defaultOut
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--schema" && i+1 < len(args):
			i++
			schemaPath = args[i]
		case strings.HasPrefix(a, "--schema="):
			schemaPath = strings.TrimPrefix(a, "--schema=")
		case a == "--out" && i+1 < len(args):
			i++
			outDir = args[i]
		case strings.HasPrefix(a, "--out="):
			outDir = strings.TrimPrefix(a, "--out=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "mcp: unknown flag %q\n", a)
			os.Exit(1)
		default:
			appName = a
		}
	}
	return schemaPath, outDir, appName
}

// schemaMethod is one method entry in the committed schema.json.
type schemaMethod struct {
	Name     string `json:"name"`
	Grant    string `json:"grant"`
	Mutating bool   `json:"mutating"`
}

// mcpInit renders the bridge scaffold from the schema.
func mcpInit(appName, schemaPath, outDir string) error {
	raw, err := os.ReadFile(schemaPath) //nolint:gosec // operator-supplied path by design
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	var schema struct {
		Methods []schemaMethod `json:"methods"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	// appName is operator-supplied and joins a path — reject separators so
	// the scaffold stays under outDir (fail-loud, G703).
	if filepath.Base(appName) != appName || appName == "." || appName == ".." {
		return fmt.Errorf("app name %q must be a bare name (no path separators)", appName)
	}
	// appName is validated above (filepath.Base equivalence + dot-name
	// rejection); G703's flow analysis cannot prove the sanitizer, so the
	// annotation marks a false positive, not an unvalidated path.
	bridgeDir := filepath.Join(outDir, "cmd", appName+"-mcp")
	if err := os.MkdirAll(bridgeDir, 0o750); err != nil { //nolint:gosec // appName validated above (Base(appName)==appName); flow analysis cannot prove it
		return fmt.Errorf("bridge dir: %w", err)
	}

	mainGo, err := renderMain(appName, schema.Methods)
	if err != nil {
		return err
	}
	mainPath := filepath.Join(bridgeDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainGo), 0o644); err != nil { //nolint:gosec // generated source file
		return fmt.Errorf("write main.go: %w", err)
	}
	goMod := mcptemplate.GoMod
	goMod = strings.ReplaceAll(goMod, "{{.AppName}}", appName)
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(goMod), 0o644); err != nil { //nolint:gosec // generated module file
		return fmt.Errorf("write go.mod: %w", err)
	}
	fmt.Printf("mcp: scaffolded %s (bridge: %s)\n", appName, mainPath)
	return nil
}

// renderMain renders the main.go template with one MCP tool per schema
// method and validates the result parses as Go.
func renderMain(appName string, methods []schemaMethod) (string, error) {
	var tools strings.Builder
	for _, m := range methods {
		if m.Grant == "" || m.Name == "sys.hello" {
			continue // handshake is the transport's job
		}
		title := m.Name
		tools.WriteString(fmt.Sprintf(`
	// %s forwards to the LDP method of the same name.
	mcp.AddTool(server, &mcp.Tool{Name: %q, Description: "LDP %s (grant: %s)"},
		func(ctx context.Context, req *mcp.CallToolRequest, in map[string]any) (*mcp.CallToolResult, map[string]any, error) {
			res, err := t.call(%q, in)
			if err != nil {
				return nil, nil, err
			}
			var out map[string]any
			if err := json.Unmarshal(res, &out); err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(res)}}}, out, nil
		})
`, m.Name, title, m.Name, m.Grant, m.Name))
	}

	main := mcptemplate.MainGo
	main = strings.ReplaceAll(main, "{{.AppName}}", appName)
	main = strings.ReplaceAll(main, "{{.Tools}}", tools.String())

	// Parse-level validation (no MCP SDK in the framework module — a full
	// compile check belongs to the generated module).
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "main.go", main, parser.AllErrors); err != nil {
		return "", fmt.Errorf("generated main.go does not parse: %w", err)
	}
	return main, nil
}
