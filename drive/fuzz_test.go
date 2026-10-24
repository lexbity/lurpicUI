package drive_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFuzzCorpusCommitted is the G10.5 corpus-maturity gate: the fuzz seed
// corpus under drive/proto/testdata/fuzz must exist and cover the seed
// shapes (envelope requests, responses, notifications). The scheduled
// 10-minute fuzz runs live in CI (weekly cron); this test keeps the corpus
// from silently disappearing.
func TestFuzzCorpusCommitted(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("proto", "testdata", "fuzz", "FuzzEnvelope"))
	if err != nil {
		t.Fatalf("read fuzz corpus: %v — commit the seed corpus (go test ./drive/proto -fuzz FuzzEnvelope -fuzztime=10m writes it)", err)
	}
	if len(entries) < 5 {
		t.Fatalf("fuzz corpus has %d entries, want >= 5 seeds", len(entries))
	}
}
