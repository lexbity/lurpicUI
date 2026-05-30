package assets

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// PakMeta is a sidecar file stored alongside the extracted assets.pak.
// It carries the content hash of the pak so the next cold start can decide
// whether to re-extract without hashing hundreds of megabytes.
type PakMeta struct {
	Version int    `json:"v"`
	Hash    string `json:"bh"` // hex-encoded SHA-256 of the pak content
}

// sidecarFileName is the name of the metadata file bundled alongside assets.pak.
const sidecarFileName = "assets.pak.meta"

// sidecarPath returns the path to the sidecar file alongside destPath.
func sidecarPath(destPath string) string {
	dir := filepath.Dir(destPath)
	return filepath.Join(dir, sidecarFileName)
}

// readPakMeta reads a PakMeta sidecar file. A missing or unparseable file
// returns an error, which callers use to signal "fall back to full hash".
func readPakMeta(metaPath string) (PakMeta, error) {
	f, err := os.Open(metaPath)
	if err != nil {
		return PakMeta{}, err
	}
	defer f.Close()
	var m PakMeta
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return PakMeta{}, err
	}
	if m.Version != 1 || m.Hash == "" {
		return PakMeta{}, fmt.Errorf("invalid sidecar: version=%d hash=%q", m.Version, m.Hash)
	}
	return m, nil
}

// writePakMeta writes the sidecar alongside the pak at destPath.
func writePakMeta(destPath string, hash [32]byte) error {
	m := PakMeta{
		Version: 1,
		Hash:    hex.EncodeToString(hash[:]),
	}
	metaPath := sidecarPath(destPath)
	tmpPath := metaPath + extractPakTempSuffix()
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create sidecar temp: %w", err)
	}
	if err := json.NewEncoder(f).Encode(m); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode sidecar: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close sidecar: %w", err)
	}
	if err := os.Rename(tmpPath, metaPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename sidecar: %w", err)
	}
	return nil
}

// sidecarHash reads the sidecar and returns the stored hash as raw bytes.
func sidecarHash(metaPath string) ([32]byte, error) {
	m, err := readPakMeta(metaPath)
	if err != nil {
		return [32]byte{}, err
	}
	raw, err := hex.DecodeString(m.Hash)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, fmt.Errorf("invalid sidecar hash: %w", err)
	}
	var h [32]byte
	copy(h[:], raw)
	return h, nil
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

	// Fast path: check the sidecar instead of hashing the entire pak.
	if needsExtract, err := sidecarGate(ctx, destPath); err == nil && !needsExtract {
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

	// Write the sidecar so the next cold start avoids a full-pak hash.
	// The hash comes from the APK sidecar (cheap) or is computed from the
	// extracted file (fallback when no APK sidecar exists).
	if apkHash, err := apkSidecarHash(ctx); err == nil {
		writePakMeta(destPath, apkHash)
	} else if h, err := hashFile(destPath); err == nil {
		writePakMeta(destPath, h)
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

// APKFDSource is an optional interface for AndroidExtractionContext
// implementations that support direct file-descriptor access to bundled
// APK assets. When available, uncompressed assets can be mmap'd directly
// from the APK zip entry, avoiding a copy to internal storage.
type APKFDSource interface {
	OpenAPKAssetFD(name string) (fd int, offset, length int64, err error)
}

// OpenAndroidPak opens assets.pak from the APK and returns an AssetSource.
// It tries direct-fd mmap first (for uncompressed paks). If the pak is
// compressed or the fd path is unavailable, it falls back to extracting
// to internal storage and opening the extracted file.
func OpenAndroidPak(ctx AndroidExtractionContext) (*PakFS, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open android pak: nil context")
	}
	// Try direct-fd mmap first — avoids a copy for uncompressed paks.
	if fdSrc, ok := ctx.(APKFDSource); ok {
		fd, offset, length, err := fdSrc.OpenAPKAssetFD("assets.pak")
		if err == nil && fd >= 0 {
			pak, pakErr := NewPakFSFromFD(fd, offset, length)
			if pakErr == nil {
				return pak, nil
			}
			// fd mmap failed; close fd and fall back to extraction.
		}
	}
	// Fall back: extract to internal storage, then open the file.
	if err := ExtractPakIfNeeded(ctx); err != nil {
		return nil, fmt.Errorf("open android pak: %w", err)
	}
	pakPath := filepath.Join(ctx.FilesDir(), "assets.pak")
	return NewPakFS(pakPath)
}

// sidecarGate checks whether extraction is needed. Returns false (no extract
// needed) when the APK sidecar hash matches the stored sidecar hash and the
// extracted file exists. Returns true when extraction is required. Returns an
// error when the check cannot be performed (no sidecar in APK → caller must
// fall back to full content hashing).
func sidecarGate(ctx AndroidExtractionContext, destPath string) (needsExtract bool, _ error) {
	apkHash, err := apkSidecarHash(ctx)
	if err != nil {
		// No sidecar in APK — caller must fall back to full content hash.
		return true, err
	}
	storedHash, err := sidecarHash(sidecarPath(destPath))
	if err != nil {
		return true, nil // no stored sidecar → extract
	}
	if storedHash != apkHash {
		return true, nil // hash mismatch → extract
	}
	// Hashes match — verify the extracted file actually exists.
	if _, err := os.Stat(destPath); err != nil {
		return true, nil // file missing → extract
	}
	return false, nil // up to date
}

// apkSidecarHash reads the APK-bundled assets.pak.meta and returns the hash.
func apkSidecarHash(ctx AndroidExtractionContext) ([32]byte, error) {
	src, err := ctx.OpenAPKAsset(sidecarFileName)
	if err != nil {
		return [32]byte{}, err
	}
	defer src.Close()
	var m PakMeta
	if err := json.NewDecoder(src).Decode(&m); err != nil {
		return [32]byte{}, err
	}
	if m.Version != 1 || m.Hash == "" {
		return [32]byte{}, fmt.Errorf("invalid APK sidecar: version=%d hash=%q", m.Version, m.Hash)
	}
	raw, err := hex.DecodeString(m.Hash)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, fmt.Errorf("invalid APK sidecar hash: %w", err)
	}
	var h [32]byte
	copy(h[:], raw)
	return h, nil
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
