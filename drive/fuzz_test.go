package drive_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFuzzCorpusCommitted is the G10.5 corpus-maturity gate: the live LDP
// fuzz targets retain committed seeds so scheduled runs exercise requests,
// locators, and subscription filters from their first iteration.
func TestFuzzCorpusCommitted(t *testing.T) {
	for _, target := range []string{
		filepath.Join("proto", "testdata", "fuzz", "FuzzEnvelope"),
		filepath.Join("locate", "testdata", "fuzz", "FuzzLocator"),
	} {
		entries, err := os.ReadDir(target)
		if err != nil {
			t.Fatalf("read fuzz corpus %s: %v", target, err)
		}
		if len(entries) < 2 {
			t.Errorf("fuzz corpus %s has %d entries, want at least 2 seeds", target, len(entries))
		}
	}
}
