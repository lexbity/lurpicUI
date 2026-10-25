package testevidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// write persists the machine-readable evidence. The summary lives at the run
// root; each gate keeps its own immutable record beside its artifacts so a
// consumer can follow one gate without reading an unrelated suite. Nothing in
// the report includes environment dumps, credentials or request payloads.
func (r *Report) write(dir string) error {
	if r == nil {
		return fmt.Errorf("testevidence: nil report")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("testevidence: evidence dir: %w", err)
	}
	for i := range r.Gates {
		gate := &r.Gates[i]
		if gate.EvidenceDir == "" {
			continue
		}
		if err := writeJSON(filepath.Join(gate.EvidenceDir, "gate.json"), gate); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(dir, "summary.json"), r); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("testevidence: encode %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil { //nolint:gosec // evidence is intentionally readable build output
		return fmt.Errorf("testevidence: write %s: %w", path, err)
	}
	return nil
}
