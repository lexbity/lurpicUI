//go:build linux && cgo

package vulkan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The FFI codegen drift gate. `src/ffi_inventory.rs` is the single source of
// truth for the C ABI. This test regenerates ffi_linux.c (function-pointer
// declarations, dlsym table, wrapper functions, unload resets) and the Go cgo
// declaration block in ffi_linux.go from the inventory, and fails CI on drift.
//
// Regenerate with:
//   LURPIC_FFI_REGENERATE=1 go test -run TestFFISymbols_InSync ./render/vulkan/

type ffiSymbol struct {
	name     string
	ret      string
	args     string // "const unsigned char *data, uintptr_t len"
	platform string
	testOnly bool
}

const (
	ffiGenStart = "// GEN-FFI-START "
	ffiGenEnd   = "// GEN-FFI-END "
)

// parseInventory mirrors build.rs's extractor over src/ffi_inventory.rs.
func parseInventory(t *testing.T) []ffiSymbol {
	t.Helper()
	path := filepath.Join("crates", "lurpic_render", "src", "ffi_inventory.rs")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	src := string(data)
	var out []ffiSymbol
	rest := src
	for {
		rel := strings.Index(rest, `name: "`)
		if rel < 0 {
			break
		}
		rest = rest[rel+len(`name: "`):]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			break
		}
		name := rest[:end]
		rest = rest[end:]

		rel = strings.Index(rest, `ret: "`)
		if rel < 0 {
			break
		}
		rest = rest[rel+len(`ret: "`):]
		end = strings.IndexByte(rest, '"')
		if end < 0 {
			break
		}
		ret := rest[:end]
		rest = rest[end:]

		rel = strings.Index(rest, `args: "`)
		if rel < 0 {
			break
		}
		rest = rest[rel+len(`args: "`):]
		end = strings.IndexByte(rest, '"')
		if end < 0 {
			break
		}
		args := rest[:end]
		rest = rest[end:]

		rel = strings.Index(rest, `platform: "`)
		if rel < 0 {
			break
		}
		rest = rest[rel+len(`platform: "`):]
		end = strings.IndexByte(rest, '"')
		if end < 0 {
			break
		}
		platform := rest[:end]
		rest = rest[end:]

		rel = strings.Index(rest, `test_only: `)
		if rel < 0 {
			break
		}
		rest = rest[rel+len(`test_only: `):]
		end = strings.IndexByte(rest, ',')
		if end < 0 {
			break
		}
		testOnly := strings.TrimSpace(rest[:end]) == "true"
		rest = rest[end:]

		out = append(out, ffiSymbol{name: name, ret: ret, args: args, platform: platform, testOnly: testOnly})
	}
	if len(out) == 0 {
		t.Fatalf("inventory parse produced no symbols")
	}
	return out
}

func linuxSymbols(symbols []ffiSymbol) []ffiSymbol {
	var out []ffiSymbol
	for _, s := range symbols {
		if s.platform == "" || s.platform == "linux" {
			out = append(out, s)
		}
	}
	return out
}

// argNames returns the comma-joined parameter identifiers from an args spec.
func argNames(args string) string {
	if strings.TrimSpace(args) == "" || strings.TrimSpace(args) == "void" {
		return ""
	}
	parts := strings.Split(args, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		tokens := strings.Fields(strings.TrimSpace(p))
		name := strings.TrimLeft(tokens[len(tokens)-1], "*")
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func cFnDeclsBlock(symbols []ffiSymbol) string {
	var b strings.Builder
	b.WriteString(ffiGenStart + "fn-declarations\n")
	for _, s := range symbols {
		fmt.Fprintf(&b, "static %s (*%s_fn)(%s) = NULL;\n", s.ret, s.name, s.args)
	}
	b.WriteString(ffiGenEnd + "fn-declarations")
	return b.String()
}

func cDlsymBlock(symbols []ffiSymbol) string {
	var b strings.Builder
	b.WriteString(ffiGenStart + "dlsym\n")
	for _, s := range symbols {
		macro := "LOAD_SYM"
		if s.testOnly {
			macro = "LOAD_SYM_OPTIONAL"
		}
		fmt.Fprintf(&b, "  %s(%s_fn, \"%s\", %s(*)(%s));\n",
			macro, s.name, s.name, s.ret, s.args)
	}
	b.WriteString(ffiGenEnd + "dlsym")
	return b.String()
}

func cWrapper(s ffiSymbol) string {
	sig := fmt.Sprintf("%s %s(%s)", s.ret, s.name, s.args)
	call := fmt.Sprintf("%s_fn(%s)", s.name, argNames(s.args))
	fnNullCheck := fmt.Sprintf("  if (%s_fn == NULL) {\n    set_error(\"vulkan: Rust library not loaded\");", s.name)

	switch s.ret {
	case "int32_t":
		return fmt.Sprintf(`%s {
%s
    return -1;
  }
  int32_t result = %s;
  if (result == 0) {
    set_error("");
  } else if (lurpic_render_last_error_fn != NULL) {
    const char *msg = lurpic_render_last_error_fn();
    if (msg != NULL) {
      set_error(msg);
    }
  }
  return result;
}`, sig, fnNullCheck, call)
	case "const char *":
		return fmt.Sprintf(`%s {
%s
    return NULL;
  }
  set_error("");
  return %s;
}`, sig, fnNullCheck, call)
	case "uint64_t", "uintptr_t":
		return fmt.Sprintf(`%s {
%s
    return 0;
  }
  set_error("");
  return %s;
}`, sig, fnNullCheck, call)
	case "void":
		return fmt.Sprintf(`void %s(%s) {
  if (%s_fn != NULL) {
    %s_fn(%s);
  }
}`, s.name, s.args, s.name, s.name, argNames(s.args))
	default:
		return fmt.Sprintf(`// %s: wrapper for return type %s not covered by codegen`, s.name, s.ret)
	}
}

func cWrappersBlock(symbols []ffiSymbol) string {
	var b strings.Builder
	b.WriteString(ffiGenStart + "wrappers\n")
	for i, s := range symbols {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(cWrapper(s))
	}
	b.WriteString("\n" + ffiGenEnd + "wrappers")
	return b.String()
}

func cUnloadBlock(symbols []ffiSymbol) string {
	var b strings.Builder
	b.WriteString(ffiGenStart + "unload\n")
	for _, s := range symbols {
		fmt.Fprintf(&b, "  %s_fn = NULL;\n", s.name)
	}
	b.WriteString(ffiGenEnd + "unload")
	return b.String()
}

func cGoDeclBlock(symbols []ffiSymbol) string {
	var b strings.Builder
	b.WriteString(ffiGenStart + "declarations\n")
	for _, s := range symbols {
		fmt.Fprintf(&b, "%s %s(%s);\n", s.ret, s.name, s.args)
	}
	b.WriteString(ffiGenEnd + "declarations")
	return b.String()
}

const cHeader = `#include <dlfcn.h>
#include <inttypes.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static void *lurpic_render_handle = NULL;

`

const cTail = `
static char lurpic_render_error[512];

static void set_error(const char *message) {
  if (message == NULL || message[0] == '\0') {
    lurpic_render_error[0] = '\0';
    return;
  }
  strncpy(lurpic_render_error, message, sizeof(lurpic_render_error) - 1);
  lurpic_render_error[sizeof(lurpic_render_error) - 1] = '\0';
}

#define LOAD_SYM(field, symbol_name, fn_type) \
  do { \
    void *symbol = dlsym(handle, symbol_name); \
    const char *sym_err = dlerror(); \
    if (sym_err != NULL || symbol == NULL) { \
      if (sym_err != NULL) { \
        set_error(sym_err); \
      } else { \
        snprintf(lurpic_render_error, sizeof(lurpic_render_error), "vulkan: missing symbol %s", symbol_name); \
      } \
      lurpic_render_reset_fns(); \
      dlclose(handle); \
      return -3; \
    } \
    field = (fn_type)symbol; \
  } while (0)

#define LOAD_SYM_OPTIONAL(field, symbol_name, fn_type) \
  do { \
    dlerror(); \
    void *symbol = dlsym(handle, symbol_name); \
    if (symbol != NULL) { \
      field = (fn_type)symbol; \
    } \
  } while (0)

int lurpic_render_load(const char *library_path) {
  if (lurpic_render_handle != NULL) {
    set_error("");
    return 0;
  }
  if (library_path == NULL || library_path[0] == '\0') {
    set_error("vulkan: library path is empty");
    return -1;
  }

  dlerror();
  void *handle = dlopen(library_path, RTLD_NOW | RTLD_LOCAL);
  if (handle == NULL) {
    const char *err = dlerror();
    set_error(err != NULL ? err : "vulkan: dlopen failed");
    return -2;
  }

  dlerror();
`

const cUnloadSkeleton = `void lurpic_render_unload(void) {
  if (lurpic_render_handle != NULL) {
    dlclose(lurpic_render_handle);
  }
  lurpic_render_handle = NULL;
  lurpic_render_reset_fns();
  set_error("");
}
`

func generateCFile(symbols []ffiSymbol) string {
	fnDecls := cFnDeclsBlock(symbols)
	dlsym := cDlsymBlock(symbols)
	wrappers := cWrappersBlock(symbols)
	unload := cUnloadBlock(symbols)
	return cHeader + fnDecls + "\n" +
		"static void lurpic_render_reset_fns(void) {\n" + unload + "\n}\n\n" +
		cTail +
		dlsym + "\n#undef LOAD_SYM\n#undef LOAD_SYM_OPTIONAL\n\n  lurpic_render_handle = handle;\n  set_error(\"\");\n  return 0;\n}\n\n" +
		wrappers + "\n" +
		cUnloadSkeleton
}

func regenerateEnv() bool {
	return os.Getenv("LURPIC_FFI_REGENERATE") == "1"
}

func TestFFISymbols_InSync(t *testing.T) {
	symbols := linuxSymbols(parseInventory(t))

	// The Go cgo declaration block must match the inventory.
	goDecls := cGoDeclBlock(symbols)
	if err := assertBlock("ffi_linux.go", "declarations", goDecls); err != nil {
		if regenerateEnv() {
			writeBlock(t, "ffi_linux.go", "declarations", goDecls)
			t.Log("regenerated ffi_linux.go declarations")
		} else {
			t.Errorf("ffi_linux.go declarations out of sync: %v", err)
		}
	}

	// ffi_linux.c must be regenerable from the inventory in full.
	generated := generateCFile(symbols)
	actual, err := os.ReadFile("ffi_linux.c")
	if err != nil {
		t.Fatalf("read ffi_linux.c: %v", err)
	}
	if string(actual) != generated {
		if regenerateEnv() {
			if err := os.WriteFile("ffi_linux.c", []byte(generated), 0o644); err != nil {
				t.Fatalf("write ffi_linux.c: %v", err)
			}
			t.Log("regenerated ffi_linux.c")
		} else {
			t.Errorf("ffi_linux.c is out of sync with the FFI inventory.\n"+
				"Run: LURPIC_FFI_REGENERATE=1 go test -run TestFFISymbols_InSync ./render/vulkan/")
		}
	}
}

// assertBlock verifies a GEN-FFI-marked block in a file matches `want`.
func assertBlock(file, id, want string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	start := strings.Index(string(data), ffiGenStart+id+"\n")
	end := strings.Index(string(data), ffiGenEnd+id)
	if start < 0 || end < 0 {
		return fmt.Errorf("missing GEN-FFI block %q in %s", id, file)
	}
	actual := string(data)[start : end+len(ffiGenEnd+id)]
	if actual != want {
		return fmt.Errorf("block %q mismatch\n--- want ---\n%s\n--- got ---\n%s", id, want, actual)
	}
	return nil
}

func writeBlock(t *testing.T, file, id, content string) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	start := strings.Index(string(data), ffiGenStart+id+"\n")
	end := strings.Index(string(data), ffiGenEnd+id)
	if start < 0 || end < 0 {
		t.Fatalf("missing GEN-FFI block %q in %s", id, file)
	}
	end += len(ffiGenEnd + id)
	out := string(data[:start]) + content + string(data[end:])
	if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

// TestFFISymbols_WrappersExist checks the load-bearing cross-side mirror: every
// inventory symbol has a C wrapper in ffi_linux.c, and every exported Go
// wrapper in ffi_linux.go maps to an inventory symbol (with known renames).
func TestFFISymbols_WrappersExist(t *testing.T) {
	symbols := linuxSymbols(parseInventory(t))
	c, err := os.ReadFile("ffi_linux.c")
	if err != nil {
		t.Fatalf("read ffi_linux.c: %v", err)
	}
	g, err := os.ReadFile("ffi_linux.go")
	if err != nil {
		t.Fatalf("read ffi_linux.go: %v", err)
	}
	inventory := map[string]bool{}
	for _, s := range symbols {
		short := strings.TrimPrefix(s.name, "lurpic_render_")
		inventory[short] = true
		if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(s.ret) + ` lurpic_render_` + short + `\(`).Match(c) {
			t.Errorf("ffi_linux.c missing C wrapper for %s", s.name)
		}
	}
	// Go wrappers whose exported name does not mechanically map to the symbol
	// name (UploadImage wraps lurpic_render_create_image; ForceSwappedRendering
	// wraps the test-only lurpic_render_test_force_swapped_rendering).
	goRenames := map[string]bool{
		"upload_image":            true,
		"force_swapped_rendering": true,
	}
	re := regexp.MustCompile(`(?m)^func ([A-Z][A-Za-z0-9_]*)\(`)
	for _, m := range re.FindAllSubmatch(g, -1) {
		name := string(m[1])
		if name == "Test" {
			continue
		}
		if !inventory[camelToSnake(name)] && !goRenames[camelToSnake(name)] {
			t.Errorf("ffi_linux.go has Go function %s not in the FFI inventory", name)
		}
	}
}

// camelToSnake converts an exported Go wrapper name (CreateXcbSurface) to the
// inventory's snake_case symbol name (create_xcb_surface).
func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteByte(byte(r) + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
