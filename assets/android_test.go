package assets

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type fakeAPKAsset struct {
	*bytes.Reader
}

func (a *fakeAPKAsset) Close() error { return nil }
func (a *fakeAPKAsset) Length() int64 {
	if a == nil || a.Reader == nil {
		return 0
	}
	return int64(a.Size())
}

type fakeAndroidExtractionContext struct {
	dir      string
	bundle   []byte
	progress []float32
	mu       sync.Mutex
}

func (c *fakeAndroidExtractionContext) FilesDir() string { return c.dir }

func (c *fakeAndroidExtractionContext) OpenAPKAsset(name string) (APKAsset, error) {
	if name != "assets.pak" {
		return nil, os.ErrNotExist
	}
	return &fakeAPKAsset{Reader: bytes.NewReader(append([]byte(nil), c.bundle...))}, nil
}

func (c *fakeAndroidExtractionContext) SetExtractionProgress(progress float32) {
	c.mu.Lock()
	c.progress = append(c.progress, progress)
	c.mu.Unlock()
}

func TestExtractPakIfNeededHashesAndCopiesBundledPak(t *testing.T) {
	dir := t.TempDir()
	ctx := &fakeAndroidExtractionContext{
		dir:    dir,
		bundle: []byte("pak-one"),
	}

	hash, err := hashAPKAsset(ctx, "assets.pak")
	if err != nil {
		t.Fatalf("hash apk asset: %v", err)
	}
	wantHash := sha256.Sum256([]byte("pak-one"))
	if hash != wantHash {
		t.Fatalf("hash = %x, want %x", hash, wantHash)
	}

	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("extract pak: %v", err)
	}
	dest := filepath.Join(dir, "assets.pak")
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted pak: %v", err)
	}
	if !bytes.Equal(got, []byte("pak-one")) {
		t.Fatalf("extracted pak = %q, want pak-one", got)
	}
	if len(ctx.progress) == 0 || ctx.progress[len(ctx.progress)-1] != 1 {
		t.Fatalf("unexpected progress updates: %#v", ctx.progress)
	}

	ctx.bundle = []byte("pak-two")
	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("re-extract pak: %v", err)
	}
	got, err = os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read re-extracted pak: %v", err)
	}
	if !bytes.Equal(got, []byte("pak-two")) {
		t.Fatalf("re-extracted pak = %q, want pak-two", got)
	}
}

func TestExtractPakKillMidExtractRecovers(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "assets.pak")

	// Simulate a crash mid-extract: write a truncated file directly.
	if err := os.WriteFile(destPath, []byte("truncated"), 0o644); err != nil {
		t.Fatalf("write truncated: %v", err)
	}
	// Also leave a stale temp file.
	staleTemp := destPath + ".tmp.99999"
	if err := os.WriteFile(staleTemp, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}

	ctx := &fakeAndroidExtractionContext{
		dir:    dir,
		bundle: []byte("complete-pak-content"),
	}

	// Extraction should replace the truncated file with the complete bundle.
	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("extract pak after simulated crash: %v", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted pak: %v", err)
	}
	if !bytes.Equal(got, []byte("complete-pak-content")) {
		t.Fatalf("extracted pak = %q, want %q", got, "complete-pak-content")
	}

	// Stale temp file should have been cleaned up.
	if _, err := os.Stat(staleTemp); err == nil {
		t.Fatal("stale temp file was not removed")
	}
}

func TestExtractPakConcurrentSingleWriter(t *testing.T) {
	dir := t.TempDir()

	bundle := make([]byte, 1_000_000) // 1 MB to exercise the copy loop
	for i := range bundle {
		bundle[i] = byte(i)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := &fakeAndroidExtractionContext{
				dir:    dir,
				bundle: bundle,
			}
			errs <- ExtractPakIfNeeded(ctx)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent extract error: %v", err)
		}
	}

	dest := filepath.Join(dir, "assets.pak")
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted pak after concurrent extract: %v", err)
	}
	if !bytes.Equal(got, bundle) {
		t.Fatal("extracted content mismatch after concurrent extraction")
	}

	// Verify only one final file exists (no stray temp files).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "assets.pak" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}

func TestExtractPakProgressMonotonic(t *testing.T) {
	dir := t.TempDir()
	ctx := &fakeAndroidExtractionContext{
		dir:    dir,
		bundle: []byte("progress-test-content"),
	}

	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(ctx.progress) == 0 {
		t.Fatal("expected at least one progress update")
	}

	var last float32
	for i, p := range ctx.progress {
		if p < last {
			t.Fatalf("progress decreased: %f -> %f at index %d", last, p, i)
		}
		if p < 0 || p > 1 {
			t.Fatalf("progress out of range [0,1]: %f at index %d", p, i)
		}
		last = p
	}
	if last != 1 {
		t.Fatalf("final progress = %f, want 1", last)
	}
}
