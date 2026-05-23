package assets

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
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
}

func (c *fakeAndroidExtractionContext) FilesDir() string { return c.dir }

func (c *fakeAndroidExtractionContext) OpenAPKAsset(name string) (APKAsset, error) {
	if name != "assets.pak" {
		return nil, os.ErrNotExist
	}
	return &fakeAPKAsset{Reader: bytes.NewReader(append([]byte(nil), c.bundle...))}, nil
}

func (c *fakeAndroidExtractionContext) SetExtractionProgress(progress float32) {
	c.progress = append(c.progress, progress)
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
