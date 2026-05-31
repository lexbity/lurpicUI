package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	metaHash string // hex-encoded SHA-256 of bundle; empty means no sidecar in APK
	progress []float32
	mu       sync.Mutex
}

func (c *fakeAndroidExtractionContext) FilesDir() string { return c.dir }

func (c *fakeAndroidExtractionContext) OpenAPKAsset(name string) (APKAsset, error) {
	switch name {
	case "assets.pak":
		return &fakeAPKAsset{Reader: bytes.NewReader(append([]byte(nil), c.bundle...))}, nil
	case "assets.pak.meta":
		if c.metaHash == "" {
			return nil, os.ErrNotExist
		}
		m := pakMetaJSON{V: 1, Bh: c.metaHash}
		data, _ := json.Marshal(m)
		return &fakeAPKAsset{Reader: bytes.NewReader(data)}, nil
	default:
		return nil, os.ErrNotExist
	}
}

// pakMetaJSON is the JSON sidecar structure used in tests.
type pakMetaJSON struct {
	V  int    `json:"v"`
	Bh string `json:"bh"`
}

func (c *fakeAndroidExtractionContext) SetExtractionProgress(progress float32) {
	c.mu.Lock()
	c.progress = append(c.progress, progress)
	c.mu.Unlock()
}

// fdExtractionContext is like fakeAndroidExtractionContext but also
// implements APKFDSource, returning an fd to the bundled data.
type fdExtractionContext struct {
	fakeAndroidExtractionContext
	pakPath string // real file path for fd access
}

func (c *fdExtractionContext) OpenAPKAssetFD(name string) (int, int64, int64, error) {
	if name != "assets.pak" || c.pakPath == "" {
		return -1, 0, 0, os.ErrNotExist
	}
	f, err := os.Open(c.pakPath)
	if err != nil {
		return -1, 0, 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return -1, 0, 0, err
	}
	return int(f.Fd()), 0, fi.Size(), nil
}

func TestOpenAndroidPakFDFallback(t *testing.T) {
	dir := t.TempDir()
	// A completely empty bundle — extraction succeeds but NewPakFS rejects it.
	payload := []byte{}

	ctx := &fakeAndroidExtractionContext{
		dir:    dir,
		bundle: payload,
	}
	// Does NOT implement APKFDSource — must fall back to extraction.
	// The extraction creates an empty file, which NewPakFS rejects.
	_, err := OpenAndroidPak(ctx)
	if err == nil {
		t.Fatal("expected error for empty pak data (extraction should still have run)")
	}
	// Verify extraction actually happened (file was created).
	dest := filepath.Join(dir, "assets.pak")
	if _, statErr := os.Stat(dest); statErr != nil {
		t.Fatalf("extracted file not found: %v", statErr)
	}
}

func TestCheckFreeSpace_acceptsSufficientSpace(t *testing.T) {
	dir := t.TempDir()
	// Small needed bytes should pass on any writable filesystem.
	if err := checkFreeSpace(dir, 1024); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckFreeSpace_rejectsAbsurdDemand(t *testing.T) {
	dir := t.TempDir()
	// Request more space than any filesystem could reasonably have.
	err := checkFreeSpace(dir, 1<<60) // 1 exabyte
	if err == nil {
		t.Fatal("expected error for absurd space demand")
	}
	if !errors.Is(err, ErrExtractionNoSpace) {
		t.Fatalf("expected ErrExtractionNoSpace, got %v", err)
	}
}

func TestExtractDir_prefersCacheDir(t *testing.T) {
	ctx := &fakeAndroidExtractionContext{dir: "/data/files"}
	if got := extractDir(ctx); got != "/data/files" {
		t.Fatalf("extractDir without CacheDirProvider = %q, want /data/files", got)
	}

	// Add CacheDir support.
	cacheCtx := &fakeExtractionWithCache{
		fakeAndroidExtractionContext: fakeAndroidExtractionContext{dir: "/data/files"},
		cacheDir: "/data/cache",
	}
	if got := extractDir(cacheCtx); got != "/data/cache" {
		t.Fatalf("extractDir with CacheDirProvider = %q, want /data/cache", got)
	}
}

type fakeExtractionWithCache struct {
	fakeAndroidExtractionContext
	cacheDir string
}

func (c *fakeExtractionWithCache) CacheDir() string { return c.cacheDir }

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

	// Verify no stray temp files remain (sidecar is expected).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "assets.pak" && e.Name() != "assets.pak.meta" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}

func TestExtractPakSidecarGateFastPath(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("this-is-the-pak-content")
	wantHash := sha256.Sum256(payload)

	// First extraction: no sidecar stored yet, should extract.
	ctx := &fakeAndroidExtractionContext{
		dir:      dir,
		bundle:   payload,
		metaHash: hex.EncodeToString(wantHash[:]),
	}
	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("first extract: %v", err)
	}
	if len(ctx.progress) == 0 {
		t.Fatal("expected progress updates during first extraction")
	}

	// Second call: sidecar matches, extracted file exists → fast path.
	progressCount := len(ctx.progress)
	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("second extract (fast path): %v", err)
	}
	if len(ctx.progress) != progressCount {
		t.Fatalf("expected no progress on fast path: got %d progress events, want %d",
			len(ctx.progress), progressCount)
	}

	// Verify the pak content is correct.
	got, err := os.ReadFile(filepath.Join(dir, "assets.pak"))
	if err != nil {
		t.Fatalf("read pak: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("pak content = %q, want %q", got, payload)
	}
}

func TestExtractPakSidecarGateMismatch(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("correct-content")
	correctHash := sha256.Sum256(payload)
	wrongHash := sha256.Sum256([]byte("wrong-content"))

	// Store a sidecar with the WRONG hash to simulate an update.
	if err := writePakMeta(filepath.Join(dir, "assets.pak"), wrongHash); err != nil {
		t.Fatalf("write mismatched sidecar: %v", err)
	}
	// Also place a file with the wrong content.
	if err := os.WriteFile(filepath.Join(dir, "assets.pak"), []byte("old-content"), 0o644); err != nil {
		t.Fatalf("write old pak: %v", err)
	}

	ctx := &fakeAndroidExtractionContext{
		dir:      dir,
		bundle:   payload,
		metaHash: hex.EncodeToString(correctHash[:]),
	}
	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("extract after mismatch: %v", err)
	}
	if len(ctx.progress) == 0 {
		t.Fatal("expected progress during re-extraction after hash mismatch")
	}

	got, err := os.ReadFile(filepath.Join(dir, "assets.pak"))
	if err != nil {
		t.Fatalf("read extracted pak: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("pak content = %q, want %q", got, payload)
	}
}

func TestExtractPakSidecarGateMissingFile(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("pak-with-missing-file")
	wantHash := sha256.Sum256(payload)

	// Store a sidecar but NO pak file.
	if err := writePakMeta(filepath.Join(dir, "assets.pak"), wantHash); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	ctx := &fakeAndroidExtractionContext{
		dir:      dir,
		bundle:   payload,
		metaHash: hex.EncodeToString(wantHash[:]),
	}
	if err := ExtractPakIfNeeded(ctx); err != nil {
		t.Fatalf("extract when pak missing: %v", err)
	}
	if len(ctx.progress) == 0 {
		t.Fatal("expected progress when pak file was missing")
	}

	got, err := os.ReadFile(filepath.Join(dir, "assets.pak"))
	if err != nil {
		t.Fatalf("read extracted pak: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("pak content = %q, want %q", got, payload)
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
