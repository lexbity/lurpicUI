package assets

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// extractPakTempSuffix returns a unique temp file suffix incorporating the
// PID and a random uint32 so concurrent callers within the same process do
// not collide.
func extractPakTempSuffix() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return fmt.Sprintf(".tmp.%d.%08x", os.Getpid(), buf)
	}
	return fmt.Sprintf(".tmp.%d", os.Getpid())
}

// ExtractPakIfNeeded extracts assets.pak atomically from the APK to internal
// storage. It writes to a pid-tagged temp file, syncs, then atomically renames
// to the final path. Stale temp files from previous crashes are cleaned up
// before extraction begins.
func ExtractPakIfNeeded(ctx AndroidExtractionContext) error {
	if ctx == nil {
		return fmt.Errorf("extract pak: nil context")
	}
	internalDir := ctx.FilesDir()
	if internalDir == "" {
		return fmt.Errorf("extract pak: empty files dir")
	}
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		return fmt.Errorf("extract pak: mkdir: %w", err)
	}

	destPath := filepath.Join(internalDir, "assets.pak")
	tmpPath := destPath + extractPakTempSuffix()

	// Clean up stale temp files from previous crashed extractions.
	cleanStaleTempFiles(destPath)

	// Fast path: already up to date.
	bundledHash, err := hashAPKAsset(ctx, "assets.pak")
	if err != nil {
		return fmt.Errorf("hash bundled pak: %w", err)
	}
	if extractedHash, err := hashFile(destPath); err == nil && extractedHash == bundledHash {
		return nil
	}

	// Open bundled asset from APK.
	src, err := ctx.OpenAPKAsset("assets.pak")
	if err != nil {
		return fmt.Errorf("open bundled pak: %w", err)
	}
	defer src.Close()

	// Write to a unique temp file (pid-suffixed) so concurrent extractions
	// from different processes do not share a temp path.
	dst, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	closeFailed := true
	defer func() {
		dst.Close()
		if closeFailed {
			os.Remove(tmpPath)
		}
	}()

	total := src.Length()
	if total <= 0 {
		total = 1
	}
	buf := make([]byte, 512*1024)
	var copied int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, wErr := dst.Write(buf[:n]); wErr != nil {
				return fmt.Errorf("write temp: %w", wErr)
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
		return fmt.Errorf("sync temp: %w", err)
	}

	// Atomic rename — on the same filesystem this is a metadata operation.
	// Readers see either the old file or the complete new file, never a
	// partial write.
	closeFailed = false
	if err := dst.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename pak: %w", err)
	}

	return nil
}

// cleanStaleTempFiles removes temp files from processes that are no longer
// alive. Files from the current process (recognised by matching PID) are
// preserved because a concurrent in-process extraction may be using them.
func cleanStaleTempFiles(destPath string) {
	dir := filepath.Dir(destPath)
	base := filepath.Base(destPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	myPID := os.Getpid()
	prefix := base + ".tmp."
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		// Extract PID from suffix: .tmp.<PID>.<random>
		rest := strings.TrimPrefix(e.Name(), prefix)
		pidStr, _, _ := strings.Cut(rest, ".")
		pid, err := parseInt(pidStr)
		if err != nil || pid == myPID {
			continue // current process, could be in-flight
		}
		// Try to check if the process is still alive.
		if processExists(pid) {
			continue
		}
		os.Remove(filepath.Join(dir, e.Name()))
	}
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func processExists(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check liveness.
	return p.Signal(os.Signal(syscall.Signal(0))) == nil
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
