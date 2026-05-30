package assets

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// APKAsset abstracts the bundled APK asset stream used during extraction.
type APKAsset interface {
	io.Reader
	io.Closer
	Length() int64
}

// AndroidExtractionContext provides the narrow services needed to extract assets.pak.
type AndroidExtractionContext interface {
	FilesDir() string
	OpenAPKAsset(name string) (APKAsset, error)
	SetExtractionProgress(progress float32)
}

// ExtractPakIfNeeded extracts assets.pak to internal storage when the bundled asset changed.
func ExtractPakIfNeeded(ctx AndroidExtractionContext) error {
	if ctx == nil {
		return fmt.Errorf("extract pak: nil context")
	}
	internalDir := ctx.FilesDir()
	if internalDir == "" {
		return fmt.Errorf("extract pak: empty files dir")
	}
	destPath := filepath.Join(internalDir, "assets.pak")

	bundledHash, err := hashAPKAsset(ctx, "assets.pak")
	if err != nil {
		return fmt.Errorf("hash bundled pak: %w", err)
	}
	if extractedHash, err := hashFile(destPath); err == nil && extractedHash == bundledHash {
		return nil
	}

	src, err := ctx.OpenAPKAsset("assets.pak")
	if err != nil {
		return fmt.Errorf("open bundled pak: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create internal dir: %w", err)
	}
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create extracted pak: %w", err)
	}
	defer dst.Close()

	total := src.Length()
	if total <= 0 {
		total = 1
	}
	buf := make([]byte, 512*1024)
	var copied int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return fmt.Errorf("write extracted pak: %w", err)
			}
			copied += int64(n)
			ctx.SetExtractionProgress(float32(copied) / float32(total))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read bundled pak: %w", readErr)
		}
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync extracted pak: %w", err)
	}
	return nil
}

// OpenAndroidPak extracts assets.pak from the APK (if changed) and returns
// an AssetSource backed by the extracted file. Pass the result to NewManager
// to create the runtime-facing asset access surface.
func OpenAndroidPak(ctx AndroidExtractionContext) (*PakFS, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open android pak: nil context")
	}
	if err := ExtractPakIfNeeded(ctx); err != nil {
		return nil, fmt.Errorf("open android pak: %w", err)
	}
	pakPath := filepath.Join(ctx.FilesDir(), "assets.pak")
	return NewPakFS(pakPath)
}

func hashAPKAsset(ctx AndroidExtractionContext, name string) ([32]byte, error) {
	var zero [32]byte
	src, err := ctx.OpenAPKAsset(name)
	if err != nil {
		return zero, err
	}
	defer src.Close()
	h := sha256.New()
	buf := make([]byte, 512*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return zero, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return zero, readErr
		}
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func hashFile(path string) ([32]byte, error) {
	var zero [32]byte
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return zero, err
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}
