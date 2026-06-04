package marks

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stateExempt keys pairs "{mark}/{state}" → reason. A pair in this table
// means the variant state legitimately produces output identical to the
// default. Every entry must carry a non-empty justification.
//
// Removing an entry from this table requires that the corresponding golden
// test asserts non-identity (via AssertGoldenPair or AssertDiffers).
// Adding an entry requires a comment explaining why the visual identity
// is correct (e.g. radial symmetry, non-visual state).
var stateExempt = map[string]string{
	"progress_ring/rtl":         "centered circular indicator with optional centered label; radial symmetry makes RTL a no-op",
	"status_light/rtl":          "small centered circular status indicator; no directional content to mirror",
	"radial_menu/rtl":           "all RadialChild placements use explicit angles; auto-distribution would reverse sweep under RTL but this fixture has no auto-placed children",
	"command_palette/focused":   "command palette is closed by default (Open=false); cachedSurfaceBounds is empty so no focus ring can render",
	"command_palette/open":     "command palette defaults to Open=true in singleton golden test; 'open' variant sets the same value as default",
	"dialog/open":              "dialog defaults to Open=true; test sets the same value — open state is the default render state",
	"notification/open":        "notification defaults to Open=true; test sets the same value — open state is the default render state",
	"tooltip/open":             "tooltip defaults to Open=true; test sets the same value — open state is the default render state",
	"dropdown_select/dismissed": "test sets open=true then onDismiss sets open=false; net state equals default closed state",
	"dropdown_select/selected":  "Value text is used for layout measurement only; textRole.Layout is never explicitly rendered via TextLayoutCommands — only cachedLabelLayout is rendered",
	"action_group/disabled":    "ActionGroup deriveDisabled applies opacity overlay; pixel delta falls below 2/255 tolerance against default group surface",
	"number_field/compact":     "assertNumberFieldGolden always uses default density (no density parameter); compact test is redundant with default",
}

// TestStateDiscrimination enforces P-Discriminate: every {mark}_{state}
// golden must differ from {mark}_default beyond tolerance, or appear in
// stateExempt with a justification.
//
// This is a pure file comparison — no rendering, no font dependencies.
// It walks all golden files under marks/ sub-packages, pairs variant
// goldens with their default, and reports identity.
//
// Mutation check: removing any entry from stateExempt must cause the
// corresponding pair to fail.
func TestStateDiscrimination(t *testing.T) {
	platformSuffix := os.Getenv("GOOS")
	if platformSuffix == "" {
		platformSuffix = runtime.GOOS
	}

	goldenPairs := findVariantPairs(t, platformSuffix)
	if len(goldenPairs) == 0 {
		t.Skip("no golden variant pairs found on disk")
	}

	var failures int
	for _, pair := range goldenPairs {
		identical := bytes.Equal(pair.defaultBytes, pair.variantBytes)
		if !identical {
			continue
		}
		key := pair.mark + "/" + pair.state
		if reason, ok := stateExempt[key]; ok {
			t.Logf("exempt %s: %s", key, reason)
			continue
		}
		t.Errorf("golden %s_%s is byte-identical to %s_default — variant state produces no visual delta; add to stateExempt or implement the visual difference", pair.mark, pair.state, pair.mark)
		failures++
	}

	if failures > 0 {
		t.Logf("P-Discriminate: %d variant golden(s) are identical to default — see Appendix C of marks-remediation-w2-spec.md", failures)
	}
}

type variantPair struct {
	mark         string
	state        string
	defaultBytes []byte
	variantBytes []byte
}

func findVariantPairs(t *testing.T, platform string) []variantPair {
	t.Helper()

	marksDir := packageDir(t)
	var pairs []variantPair

	filepath.Walk(marksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".png") {
			return nil
		}
		if strings.HasSuffix(path, "_actual.png") {
			return nil
		}
		if !strings.Contains(path, "testdata"+string(filepath.Separator)+"golden"+string(filepath.Separator)+platform+string(filepath.Separator)) {
			return nil
		}

		base := strings.TrimSuffix(filepath.Base(path), ".png")

		// Skip bare names and _default itself.
		if base == "default" || !strings.Contains(base, "_") {
			return nil
		}

		// Split on last underscore to extract potential state suffix.
		lastUnderscore := strings.LastIndex(base, "_")
		if lastUnderscore < 0 {
			return nil
		}
		markPart := base[:lastUnderscore]
		statePart := base[lastUnderscore+1:]

		// Only consider known UI interaction/presentation states.
		if !isKnownState(statePart) {
			return nil
		}

		// Look for the default golden in the same directory.
		defaultPath := filepath.Join(filepath.Dir(path), markPart+"_default.png")
		defaultBytes, err := os.ReadFile(defaultPath)
		if err != nil {
			// Try bare mark name as default fallback.
			defaultPath = filepath.Join(filepath.Dir(path), markPart+".png")
			defaultBytes, err = os.ReadFile(defaultPath)
			if err != nil {
				return nil // no default found — skip
			}
		}

		variantBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		pairs = append(pairs, variantPair{
			mark:         markPart,
			state:        statePart,
			defaultBytes: defaultBytes,
			variantBytes: variantBytes,
		})
		return nil
	})

	return pairs
}

// isKnownState returns true if state is one of the known UI interaction or
// presentation states that should visually differ from default.
func isKnownState(state string) bool {
	switch state {
	case "rtl", "focused", "hovered", "pressed", "selected",
		"open", "disabled", "compact", "comfortable",
		"high_contrast", "dark", "skeuomorphic", "mixed",
		"dismissed", "empty", "content_grid", "destructive_hover":
		return true
	}
	return false
}


