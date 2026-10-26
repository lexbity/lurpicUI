package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/store"

	drivedesc "codeburg.org/lexbit/lurpicui/drive/describe"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// TestCoverage_testids is the Debt-6 gate: every author identity the studio
// documents as its interactive-chrome contract resolves against the live
// tree through the LDP locator engine. A missing TestID fails with the
// facet's kind; the enumerated list is the studio's driver surface.
func TestCoverage_testids(t *testing.T) {
	scope := store.NewScope()
	fontRegistry := testkit.TestFontRegistry(t)
	textService := testkit.TestTextService(t, fontRegistry)
	reg, err := StudioLayerRegistry()
	if err != nil {
		t.Fatalf("layer registry: %v", err)
	}
	themeCtx := StudioThemeContext().WithFontRegistry(fontRegistry)
	root := BuildRoot(app.BuildContext{
		StoreScope:   scope,
		FontRegistry: fontRegistry,
		TextService:  textService,
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        themeCtx,
	}, NewDirtySink(10), mustSeed(t), reg)

	cfg := testkit.StandardHarnessConfig(t, 1280, 800)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = themeCtx.Resolver
	cfg.Drive = true
	h := testkit.NewHarness(t, scope, cfg, root)
	h.RunFrame()

	rt := h.Runtime()
	resolve := func(l locate.Locator) (locate.Match, *locate.Error) {
		m, _, lerr := locate.Resolve(rt.Root(), l, drivedesc.Describe, nil, drivedesc.HostedChildren)
		return m, lerr
	}

	// Interactive chrome (Debt-6): every entry MUST resolve to exactly one
	// live facet.
	byTestID := []string{
		"studio.root",
		"shell.cmdk",
		"shell.theme",
		"shell.status.light",
		"shell.status.bar",
		"shell.status.ring",
		"shell.status.badge",
		"shell.status.driving",
		"index.tree",
		"e1.controls.card",
		"e1.controls.live",
		"e1.controls.chart",
		"e1.controls.color",
		"e1.controls.opacity",
		"e1.controls.grid",
		"e1.controls.ymax",
		"e1.controls.range",
		"e1.controls.jump",
		"e1.reshape",
		"e1.canvas",
		"e1.grid.editor",
		"e6.action.split",
		"e6.action.menu",
		"e6.action.bar",
		"e6.action.toolbar",
		"e6.selection.checkbox",
		"e6.selection.toggle",
		"e6.selection.slider",
		"e6.selection.dial",
		"e6.selection.radio",
		"e6.selection.dropdown",
		"e6.selection.segments",
		"e6.selection.item",
		"e6.input.field",
		"e6.input.number",
		"e6.input.color",
		"e6.navigation.rail",
		"e6.feedback.alert-trigger",
		"e6.feedback.alert-clear",
		"e6.feedback.dialog-open",
		"e6.feedback.toast",
		"e6.feedback.tooltip",
		"e6.status.tick",
		"e6.status.online",
		"e6.status.slider",
	}
	for _, id := range byTestID {
		m, lerr := resolve(locate.Locator{TestID: id})
		if lerr != nil {
			t.Errorf("TestID %q unresolved: %v", id, lerr)
			continue
		}
		if m.Base.TestID() != id {
			t.Errorf("TestID %q resolved to %q", id, m.Base.TestID())
		}
	}

	// Stage exhibit roots: stage.<exhibit> for every catalog entry.
	for _, e := range exhibitCatalog {
		m, lerr := resolve(locate.Locator{TestID: "stage." + string(e.id)})
		if lerr != nil {
			t.Errorf("exhibit root stage.%s unresolved: %v", e.id, lerr)
			continue
		}
		_ = m
	}

	// Keyed addressing: index tree rows are hosted ListItems keyed by node
	// path ("group/leaf"); E1 grid rows are keyed by collection ItemID.
	for _, group := range indexTreeNodes(shellActiveDefault()) {
		for _, leaf := range group.Children {
			if _, lerr := resolve(locate.Locator{Key: group.Key + "/" + leaf.Key}); lerr != nil {
				t.Errorf("index item key %q unresolved: %v", group.Key+"/"+leaf.Key, lerr)
			}
		}
	}
	// The seed feeds at least three grid rows; ItemIDs count from 1.
	for _, key := range []string{"1", "2", "3"} {
		if _, lerr := resolve(locate.Locator{Key: key}); lerr != nil {
			t.Errorf("grid row key %q unresolved: %v", key, lerr)
		}
	}
}

// shellActiveDefault returns the default active exhibit for tree enumeration.
func shellActiveDefault() ExhibitID { return ExhibitRealtime }

func mustSeed(t *testing.T) []dataset.Row {
	t.Helper()
	return seedRows(t)
}
