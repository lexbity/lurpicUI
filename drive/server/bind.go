// Socket and token file ownership (RFC §Finite resources and trust
// boundaries): the socket directory is private and owned by the app UID;
// the token inode is created exclusively with no-follow semantics at 0600;
// existing socket/token paths are a clear initialization failure — the
// server never unlinks an arbitrary file, replaces a live listener, or
// chmods a shared directory to make a path pass validation. Cleanup
// removes only inodes this server instance created.
package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// boundSocket carries the listener plus the identities of the inodes this
// Listen created (for owned cleanup).
type boundSocket struct {
	listener  net.Listener
	socketDev uint64
	socketIno uint64
	tokenDev  uint64
	tokenIno  uint64
}

// bind validates and creates the socket directory, the token file, and the
// listening socket.
func bind(socketPath, token string) (boundSocket, error) {
	dir := filepath.Dir(socketPath)
	if err := ensurePrivateDir(dir); err != nil {
		return boundSocket{}, err
	}
	tokenPath := socketPath + ".token"
	// Refuse collisions: an existing token file — regular, symlink, or
	// otherwise — is a startup failure, never an overwrite.
	if st, err := os.Lstat(tokenPath); err == nil {
		return boundSocket{}, fmt.Errorf("drive: token path %s already exists (%s); remove it explicitly before launch", tokenPath, describeFile(st))
	} else if !os.IsNotExist(err) {
		return boundSocket{}, fmt.Errorf("drive: token path %s: %w", tokenPath, err)
	}
	// Same refusal for a stale socket path.
	if st, err := os.Lstat(socketPath); err == nil {
		return boundSocket{}, fmt.Errorf("drive: socket path %s already exists (%s); stale-path cleanup is an operator action", socketPath, describeFile(st))
	} else if !os.IsNotExist(err) {
		return boundSocket{}, fmt.Errorf("drive: socket path %s: %w", socketPath, err)
	}
	// Exclusive no-follow token creation: O_CREAT|O_EXCL cannot follow a
	// symlink that appears after the Lstat check — the open fails instead.
	f, err := os.OpenFile(tokenPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // 0600 by policy; path is launcher-supplied
	if err != nil {
		return boundSocket{}, fmt.Errorf("drive: create token: %w", err)
	}
	if _, err := f.WriteString(token); err != nil {
		_ = f.Close()
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: write token: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: close token: %w", err)
	}
	tokenSt, err := os.Lstat(tokenPath)
	if err != nil {
		return boundSocket{}, fmt.Errorf("drive: token stat: %w", err)
	}
	if !tokenSt.Mode().IsRegular() || tokenSt.Mode().Perm() != 0o600 {
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: token inode is %s with mode %s, want regular 0600", describeFile(tokenSt), tokenSt.Mode().Perm())
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: listen: %w", err)
	}
	// net.Listen applies the process umask; the socket inode itself must be
	// 0600 regardless (this is our own new inode, never a shared file).
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: socket mode: %w", err)
	}
	socketSt, err := os.Lstat(socketPath)
	if err != nil {
		_ = ln.Close()
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: socket stat: %w", err)
	}
	if socketSt.Mode()&os.ModeSocket == 0 || socketSt.Mode().Perm() != 0o600 {
		_ = ln.Close()
		_ = os.Remove(tokenPath)
		return boundSocket{}, fmt.Errorf("drive: socket inode is %s with mode %s, want socket 0600", describeFile(socketSt), socketSt.Mode().Perm())
	}
	sd, si := fileIdentity(socketSt)
	td, ti := fileIdentity(tokenSt)
	return boundSocket{
		listener:  ln,
		socketDev: sd, socketIno: si,
		tokenDev: td, tokenIno: ti,
	}, nil
}

// ensurePrivateDir requires the socket directory to exist (or creates it
// 0700), and validates ownership: a directory owned by another user, or
// group/world accessible, is refused. A symlinked directory segment is
// rejected via Lstat.
func ensurePrivateDir(dir string) error {
	st, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("drive: socket dir: %w", err)
		}
		st, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("drive: socket dir: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("drive: socket dir %s is %s, want a directory", dir, describeFile(st))
	}
	if st.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("drive: socket dir %s has mode %s, want 0700 (private)", dir, st.Mode().Perm())
	}
	if uid, ok := currentUID(); ok {
		if ownerUID(st) != uid {
			return fmt.Errorf("drive: socket dir %s is owned by uid %d, want uid %d", dir, ownerUID(st), uid)
		}
	}
	return nil
}

// describeFile renders an Lstat result for an initialization failure.
func describeFile(st os.FileInfo) string {
	switch {
	case st.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case st.Mode()&os.ModeSocket != 0:
		return "socket"
	case st.Mode().IsRegular():
		return "regular file"
	case st.IsDir():
		return "directory"
	default:
		return "file"
	}
}

// fileIdentity extracts (dev, ino) on Unix.
func fileIdentity(st os.FileInfo) (uint64, uint64) {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return uint64(sys.Dev), uint64(sys.Ino) //nolint:unconvert // platform-dependent field widths
}

// ownerUID extracts the file owner.
func ownerUID(st os.FileInfo) uint32 {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return sys.Uid
}

// currentUID reports the effective uid (non-negative on success).
func currentUID() (uint32, bool) {
	uid := os.Geteuid()
	if uid < 0 {
		return 0, false
	}
	return uint32(uid), true //nolint:gosec // the negative case is excluded above
}
