package facets

import (
	"os/exec"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/testkit"
)

func TestMeterFacetSubscribesAndReleases(t *testing.T) {
	svc := testkit.NewFakeVoiceService()
	f := NewMeterFacet(svc)
	facet.Attach(f, facet.AttachContext{})

	if got := f.Subs().Len(); got == 0 {
		t.Fatal("expected subscriptions after attach")
	}
	svc.Stores().Params.Set(voiceux.AudioParamsView{RMS: 1})
	if got := f.DirtyFlags(); got == 0 {
		t.Fatal("expected invalidation after params change")
	}
	facet.Dispose(f)
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected subscriptions released, got %d", got)
	}
}

func TestVoiceUXImportFirewall(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "codeburg.org/lexbit/lurpicui/voiceux/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, out)
	}
	forbidden := []string{
		"codeburg.org/lexbit/voicedsp/internal/",
		"codeburg.org/lexbit/lurpicui/demos/",
		"codeburg.org/lexbit/lurpicui/cmd/",
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, needle := range forbidden {
			if strings.Contains(line, needle) {
				t.Fatalf("forbidden dependency %q found in %q", needle, line)
			}
		}
	}
}
