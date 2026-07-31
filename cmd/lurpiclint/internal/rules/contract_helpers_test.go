package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// ---------------------------------------------------------------------------
// TestPackageFiles
// ---------------------------------------------------------------------------

func TestPackageFiles_returnsOnlyCategory(t *testing.T) {
	dirA := ruleTestdataDir(t, "contract_helpers", "helpers_pkgfiles", "pkg_a")
	dirB := ruleTestdataDir(t, "contract_helpers", "helpers_pkgfiles", "pkg_b")
	result, err := loader.Load([]string{dirA, dirB}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	// packageFiles(ctx, "pkg_a") should only return files from pkg_a.
	files := packageFiles(ctx, "pkg_a")
	if len(files) == 0 {
		t.Fatal("expected at least 1 file from pkg_a")
	}
	for _, f := range files {
		if !strings.Contains(filepath.ToSlash(f.Path), "/pkg_a/") {
			t.Errorf("file %s does not belong to pkg_a", f.Path)
		}
	}

	// packageFiles(ctx, "nonexistent") should return empty.
	none := packageFiles(ctx, "nonexistent")
	if len(none) != 0 {
		t.Errorf("expected 0 files for nonexistent category, got %d", len(none))
	}
}

// ---------------------------------------------------------------------------
// TestReceiverTypeName
// ---------------------------------------------------------------------------

func parseRecv(t *testing.T, src string) *ast.FieldList {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv != nil {
			return fn.Recv
		}
	}
	t.Fatal("no method with receiver found")
	return nil
}

func TestReceiverTypeName_Pointer(t *testing.T) {
	recv := parseRecv(t, "package p; func (t *Table) Foo() {}")
	if got := receiverTypeName(recv); got != "Table" {
		t.Errorf("receiverTypeName(*Table) = %q, want %q", got, "Table")
	}
}

func TestReceiverTypeName_Value(t *testing.T) {
	recv := parseRecv(t, "package p; func (t Table) Foo() {}")
	if got := receiverTypeName(recv); got != "Table" {
		t.Errorf("receiverTypeName(Table) = %q, want %q", got, "Table")
	}
}

func TestReceiverTypeName_GenericPointer(t *testing.T) {
	recv := parseRecv(t, "package p; func (t *Table[T]) Foo() {}")
	if got := receiverTypeName(recv); got != "Table" {
		t.Errorf("receiverTypeName(*Table[T]) = %q, want %q", got, "Table")
	}
}

func TestReceiverTypeName_MultiGenericPointer(t *testing.T) {
	recv := parseRecv(t, "package p; func (t *Table[T, U]) Foo() {}")
	if got := receiverTypeName(recv); got != "Table" {
		t.Errorf("receiverTypeName(*Table[T, U]) = %q, want %q", got, "Table")
	}
}

func TestReceiverTypeName_Nil(t *testing.T) {
	if got := receiverTypeName(nil); got != "" {
		t.Errorf("receiverTypeName(nil) = %q, want %q", got, "")
	}
}

func TestReceiverTypeName_Empty(t *testing.T) {
	if got := receiverTypeName(&ast.FieldList{}); got != "" {
		t.Errorf("receiverTypeName(empty) = %q, want %q", got, "")
	}
}

// ---------------------------------------------------------------------------
// TestPackageHasMethod
// ---------------------------------------------------------------------------

func TestPackageHasMethod_Found(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hasmeth",
	}
	if !packageHasMethod(ctx, cap, "ExportAnchors") {
		t.Error("packageHasMethod should find ExportAnchors on Table")
	}
}

func TestPackageHasMethod_NotFound(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hasmeth",
	}
	if packageHasMethod(ctx, cap, "BoundData") {
		t.Error("packageHasMethod should NOT find BoundData on Table")
	}
}

func TestPackageHasMethod_DifferentType(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Other",
		Category: "helpers_hasmeth",
	}
	if !packageHasMethod(ctx, cap, "BoundData") {
		t.Error("packageHasMethod should find BoundData on Other")
	}
	if packageHasMethod(ctx, cap, "ExportAnchors") {
		t.Error("packageHasMethod should NOT find ExportAnchors on Other")
	}
}

// ---------------------------------------------------------------------------
// TestPackageTestCallsHelper
// ---------------------------------------------------------------------------

func TestPackageTestCallsHelper_Found(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hashelper")
	result, err := loader.Load([]string{dir}, loader.Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hashelper",
	}
	if !packageTestCallsHelper(ctx, cap, "AssertAnchorExport") {
		t.Error("packageTestCallsHelper should find AssertAnchorExport call in test file")
	}
}

func TestPackageTestCallsHelper_NotFound(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hashelper")
	result, err := loader.Load([]string{dir}, loader.Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hashelper",
	}
	if packageTestCallsHelper(ctx, cap, "AssertDataBound") {
		t.Error("packageTestCallsHelper should NOT find AssertDataBound call")
	}
}

func TestPackageTestCallsHelper_GenericCall(t *testing.T) {
	// AssertDataBound[Item](...) parses as a generic instantiation; the
	// helper must unwrap the IndexExpr and still match the helper name.
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasgeneric")
	result, err := loader.Load([]string{dir}, loader.Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hasgeneric",
	}
	if !packageTestCallsHelper(ctx, cap, "AssertDataBound") {
		t.Error("packageTestCallsHelper should detect generic AssertDataBound[Item](...) call")
	}
}

// ---------------------------------------------------------------------------
// TestIsNolint
// ---------------------------------------------------------------------------

func TestIsNolint_Found(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_nolint")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_nolint",
	}
	if !isNolint(ctx, cap, "LL030") {
		t.Error("isNolint should find //nolint:LL030 on Table")
	}
}

func TestIsNolint_WrongRule(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_nolint")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_nolint",
	}
	if isNolint(ctx, cap, "LL029") {
		t.Error("isNolint should NOT find LL029 on a LL030-only nolint")
	}
}

func TestIsNolint_NoNolint(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hasmeth",
	}
	if isNolint(ctx, cap, "LL030") {
		t.Error("isNolint should be false when no nolint directive exists")
	}
}

// ---------------------------------------------------------------------------
// TestNolintReason
// ---------------------------------------------------------------------------

func TestNolintReason_Found(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_nolint")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_nolint",
	}
	if got := nolintReason(ctx, cap, "LL030"); got != "deliberate opt-out" {
		t.Errorf("nolintReason = %q, want %q", got, "deliberate opt-out")
	}
}

func TestNolintReason_NotFound(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_nolint")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_nolint",
	}
	if got := nolintReason(ctx, cap, "LL029"); got != "" {
		t.Errorf("nolintReason for LL029 = %q, want empty", got)
	}
}

func TestNolintReason_NoNolint(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hasmeth",
	}
	if got := nolintReason(ctx, cap, "LL030"); got != "" {
		t.Errorf("nolintReason without directive = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// TestCapabilityCheck — fired when helper missing
// ---------------------------------------------------------------------------

func TestCapabilityCheck_firesMissingHelper(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "unwired")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "unwired",
		Path:     "unwired.Table",
	}

	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	d := capabilityCheck(ctx, rule, cap, []string{"ExportAnchors"}, "AssertAnchorExport")
	if d == nil {
		t.Fatal("capabilityCheck should return a diagnostic when helper is missing")
	}
	if d.RuleID != "LL030" {
		t.Errorf("diagnostic RuleID = %q, want %q", d.RuleID, "LL030")
	}
	if d.Severity != diag.SeverityWarn {
		t.Errorf("diagnostic Severity = %s, want warn", d.Severity)
	}
}

// fakeRule implements Rule for testing.
type fakeRule struct {
	id  string
	sev diag.Severity
}

func (r *fakeRule) ID() string                            { return r.id }
func (r *fakeRule) DefaultSeverity() diag.Severity        { return r.sev }
func (r *fakeRule) Description() string                   { return "test rule" }
func (r *fakeRule) Check(ctx *Context) []*diag.Diagnostic { return nil }

// TestCapabilityCheck — silent when helper is wired
// ---------------------------------------------------------------------------

func TestCapabilityCheck_silentWhenWired(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "wired")
	result, err := loader.Load([]string{dir}, loader.Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "wired",
		Path:     "wired.Table",
	}

	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	d := capabilityCheck(ctx, rule, cap, []string{"ExportAnchors"}, "AssertAnchorExport")
	if d != nil {
		t.Errorf("capabilityCheck should be nil when helper is wired, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// TestCapabilityCheck — silent when nolint with reason
// ---------------------------------------------------------------------------

func TestCapabilityCheck_silentWhenNolintWithReason(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "nolint_ok")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "nolint_ok",
		Path:     "nolint_ok.Table",
	}

	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	d := capabilityCheck(ctx, rule, cap, []string{"ExportAnchors"}, "AssertAnchorExport")
	if d != nil {
		t.Errorf("capabilityCheck should be nil when nolint has a substantive reason, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// TestCapabilityCheck — warns on empty/placeholder nolint
// ---------------------------------------------------------------------------

func TestCapabilityCheck_warnsOnEmptyNolint(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "nolint_bad")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "nolint_bad",
		Path:     "nolint_bad.Table",
	}

	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	d := capabilityCheck(ctx, rule, cap, []string{"ExportAnchors"}, "AssertAnchorExport")
	if d == nil {
		t.Fatal("capabilityCheck should return a warning diagnostic when nolint has placeholder reason")
	}
	if d.RuleID != "LL030" {
		t.Errorf("diagnostic RuleID = %q, want %q", d.RuleID, "LL030")
	}
	if d.Severity != diag.SeverityWarn {
		t.Errorf("diagnostic Severity = %s, want warn", d.Severity)
	}
	if !strings.Contains(d.Message, "nolint") {
		t.Errorf("diagnostic message should mention nolint, got: %s", d.Message)
	}
}

// ---------------------------------------------------------------------------
// TestCapabilityCheck — silent when capability not declared
// ---------------------------------------------------------------------------

func TestCapabilityCheck_silentWhenNotDeclared(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	// Table has ExportAnchors, not BoundData
	cap := capindex.Capability{
		Kind:     capindex.KindMark,
		TypeName: "Table",
		Category: "helpers_hasmeth",
	}

	rule := &fakeRule{id: "LL029", sev: diag.SeverityWarn}
	d := capabilityCheck(ctx, rule, cap, []string{"BoundData"}, "AssertDataBound")
	if d != nil {
		t.Errorf("capabilityCheck should be nil when capability not declared, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// TestCapabilityRule — missing capindex
// ---------------------------------------------------------------------------

func TestCapabilityRule_missingCapindex(t *testing.T) {
	ctx := &Context{
		Files: nil,
		Pkgs:  nil,
		Fset:  token.NewFileSet(),
		// Index is nil
	}
	rule := &fakeRule{id: "LL029", sev: diag.SeverityWarn}
	diags := capabilityRule(ctx, rule, []string{"BoundData"}, "AssertDataBound", nil)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for missing capindex, got %d", len(diags))
	}
	if diags[0].Severity != diag.SeverityError {
		t.Errorf("missing capindex diag severity = %s, want error", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "capindex not populated") {
		t.Errorf("missing capindex diag should mention capindex not populated, got: %s", diags[0].Message)
	}
}

func TestCapabilityRule_wrongIndexType(t *testing.T) {
	ctx := &Context{
		Index: "not a []capindex.Capability",
	}
	rule := &fakeRule{id: "LL029", sev: diag.SeverityWarn}
	diags := capabilityRule(ctx, rule, []string{"BoundData"}, "AssertDataBound", nil)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for invalid capindex, got %d", len(diags))
	}
}

func TestCapabilityRule_firesForUnwiredMark(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "unwired")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: []capindex.Capability{
			{
				Kind:     capindex.KindMark,
				TypeName: "Table",
				Category: "unwired",
				Path:     "unwired.Table",
			},
		},
	}
	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	diags := capabilityRule(ctx, rule, []string{"ExportAnchors"}, "AssertAnchorExport", nil)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for unwired mark, got %d", len(diags))
	}
	if diags[0].RuleID != "LL030" {
		t.Errorf("diagnostic RuleID = %q, want %q", diags[0].RuleID, "LL030")
	}
}

func TestCapabilityRule_silentForWiredMark(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "wired")
	result, err := loader.Load([]string{dir}, loader.Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: []capindex.Capability{
			{
				Kind:     capindex.KindMark,
				TypeName: "Table",
				Category: "wired",
				Path:     "wired.Table",
			},
		},
	}
	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	diags := capabilityRule(ctx, rule, []string{"ExportAnchors"}, "AssertAnchorExport", nil)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for wired mark, got %d: %v", len(diags), diags)
	}
}

func TestCapabilityRule_skipsNonMarkCapabilities(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "unwired")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// A KindLayout capability that happens to point at the unwired Table type
	// must NOT fire the rule — only KindMark capabilities are in scope.
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: []capindex.Capability{
			{
				Kind:     capindex.KindLayout,
				TypeName: "Table",
				Category: "unwired",
				Path:     "unwired.Table",
			},
		},
	}
	rule := &fakeRule{id: "LL030", sev: diag.SeverityWarn}
	diags := capabilityRule(ctx, rule, []string{"ExportAnchors"}, "AssertAnchorExport", nil)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for non-mark capability, got %d: %v", len(diags), diags)
	}
}

func TestCapabilityRule_gateSkipsNonMatching(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_capcheck", "unwired")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: []capindex.Capability{
			{
				Kind:     capindex.KindMark,
				TypeName: "Table",
				Category: "unwired",
				Path:     "unwired.Table",
				Fingerprint: capindex.Fingerprint{
					IsContainer: false,
				},
			},
		},
	}
	rule := &fakeRule{id: "LL031", sev: diag.SeverityWarn}
	// A gate that requires IsContainer must skip the unwired Table even though
	// it is a KindMark declaring ExportAnchors.
	gate := func(c capindex.Capability) bool { return c.Fingerprint.IsContainer }
	diags := capabilityRule(ctx, rule, []string{"ExportAnchors"}, "AssertAnchorExport", gate)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics when gate rejects the capability, got %d: %v", len(diags), diags)
	}

	// Without the gate, the same capability fires (unwired helper).
	diags = capabilityRule(ctx, rule, []string{"ExportAnchors"}, "AssertAnchorExport", nil)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic without gate, got %d", len(diags))
	}
}

// ---------------------------------------------------------------------------
// TestCapabilityDeclared
// ---------------------------------------------------------------------------

func TestCapabilityDeclared_AllMethodsPresent(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		TypeName: "Other",
		Category: "helpers_hasmeth",
	}
	if !capabilityDeclared(ctx, cap, []string{"BoundData"}) {
		t.Error("capabilityDeclared should find BoundData on Other")
	}
}

func TestCapabilityDeclared_MethodMissing(t *testing.T) {
	dir := ruleTestdataDir(t, "contract_helpers", "helpers_hasmeth")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	cap := capindex.Capability{
		TypeName: "Table",
		Category: "helpers_hasmeth",
	}
	if capabilityDeclared(ctx, cap, []string{"BoundData"}) {
		t.Error("capabilityDeclared should NOT find BoundData on Table")
	}
}

func TestCapabilityDeclared_MultipleMethods_AllPresent(t *testing.T) {
	// Create a fixture with a type that has both AccessibilityRole and AccessibleName
	src := `package p
type Button struct {}
func (b *Button) AccessibilityRole() string { return "button" }
func (b *Button) AccessibleName() string { return "name" }`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "button.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pf := &loader.ParsedFile{
		Fset: fset,
		AST:  f,
		Path: "/test/button.go",
		Pkg:  "p",
	}
	ctx := &Context{
		Files: []*loader.ParsedFile{pf},
		Pkgs: map[string]*loader.Package{
			"/test": {
				Name:  "p",
				Path:  "/test",
				Files: []*loader.ParsedFile{pf},
			},
		},
		Fset: fset,
	}
	cap := capindex.Capability{
		TypeName: "Button",
		Category: "test",
	}
	if !capabilityDeclared(ctx, cap, []string{"AccessibilityRole", "AccessibleName"}) {
		t.Error("capabilityDeclared should find both methods on Button")
	}
}

// ---------------------------------------------------------------------------
// TestExtractNolintBody / TestExtractNolintReason (pure unit tests)
// ---------------------------------------------------------------------------

func TestExtractNolintBody_Simple(t *testing.T) {
	if got := extractNolintBody("//nolint:LL030"); got != "LL030" {
		t.Errorf("extractNolintBody = %q, want %q", got, "LL030")
	}
}

func TestExtractNolintBody_WithReasonDoubleSlash(t *testing.T) {
	if got := extractNolintBody("//nolint:LL030 // deliberate"); got != "LL030" {
		t.Errorf("extractNolintBody = %q, want %q", got, "LL030")
	}
}

func TestExtractNolintBody_WithReasonDash(t *testing.T) {
	if got := extractNolintBody("//nolint:LL030 -- deliberate"); got != "LL030" {
		t.Errorf("extractNolintBody = %q, want %q", got, "LL030")
	}
}

func TestExtractNolintBody_MultiRule(t *testing.T) {
	if got := extractNolintBody("//nolint:LL030,LL031 // reason"); got != "LL030,LL031" {
		t.Errorf("extractNolintBody = %q, want %q", got, "LL030,LL031")
	}
}

func TestExtractNolintBody_NotNolint(t *testing.T) {
	if got := extractNolintBody("// just a comment"); got != "" {
		t.Errorf("extractNolintBody = %q, want empty", got)
	}
}

func TestExtractNolintBody_SpaceForm(t *testing.T) {
	// gofmt normalises standalone //nolint: directives with unknown rule IDs
	// to "// nolint:" (with a space); both forms must be recognised.
	if got := extractNolintBody("// nolint:LL031"); got != "LL031" {
		t.Errorf("extractNolintBody(space form) = %q, want %q", got, "LL031")
	}
	if got := extractNolintBody("// nolint:LL031 // reason"); got != "LL031" {
		t.Errorf("extractNolintBody(space form + reason) = %q, want %q", got, "LL031")
	}
}

func TestExtractNolintReason_SpaceForm(t *testing.T) {
	if got := extractNolintReason("// nolint:LL031 // deliberate opt-out"); got != "deliberate opt-out" {
		t.Errorf("extractNolintReason(space form) = %q, want %q", got, "deliberate opt-out")
	}
}

func TestExtractNolintReason_DoubleSlash(t *testing.T) {
	if got := extractNolintReason("//nolint:LL030 // deliberate opt-out"); got != "deliberate opt-out" {
		t.Errorf("extractNolintReason = %q, want %q", got, "deliberate opt-out")
	}
}

func TestExtractNolintReason_Dash(t *testing.T) {
	if got := extractNolintReason("//nolint:LL030 -- deliberate"); got != "deliberate" {
		t.Errorf("extractNolintReason = %q, want %q", got, "deliberate")
	}
}

func TestExtractNolintReason_NoReason(t *testing.T) {
	if got := extractNolintReason("//nolint:LL030"); got != "" {
		t.Errorf("extractNolintReason = %q, want empty", got)
	}
}

func TestExtractNolintReason_NotNolint(t *testing.T) {
	if got := extractNolintReason("// just a comment"); got != "" {
		t.Errorf("extractNolintReason = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// TestContractTestSuffix
// ---------------------------------------------------------------------------

func TestContractTestSuffix_KnownHelpers(t *testing.T) {
	cases := map[string]string{
		"AssertDataBound":     "databound",
		"AssertAnchorExport":  "anchor_export",
		"AssertGroupChildren": "group_children",
		"AssertAccessible":    "accessible",
		"AssertFocusable":     "focusable",
	}
	for helper, want := range cases {
		if got := contractTestSuffix(helper); got != want {
			t.Errorf("contractTestSuffix(%s) = %q, want %q", helper, got, want)
		}
	}
}

func TestContractTestSuffix_UnknownHelperFallback(t *testing.T) {
	if got := contractTestSuffix("AssertSomeCapability"); got != "some_capability" {
		t.Errorf("contractTestSuffix(AssertSomeCapability) = %q, want %q", got, "some_capability")
	}
}

func TestContractTestSuffix_NonAssert(t *testing.T) {
	if got := contractTestSuffix("anchor_export"); got != "anchor_export" {
		t.Errorf("contractTestSuffix(anchor_export) = %q, want %q", got, "anchor_export")
	}
}
