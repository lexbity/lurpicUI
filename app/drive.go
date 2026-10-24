package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// DriveConfig is the app-layer drive configuration. The environment is the
// single source (parseDriveEnv); LDP is disabled unless LURPIC_DRIVE is set
// (FR-4).
type DriveConfig struct {
	Enabled    bool
	SocketPath string
	GrantsSpec string
	Grants     server.Grants
}

// driveDefaultGrants is the grant set for a bare "on": least privilege —
// reads only. Automation that needs more asks for it explicitly
// (on:read,input).
const driveDefaultGrants = "read"

// parseDriveEnv parses LURPIC_DRIVE. Grammar (QP-4):
//
//	off                          → disabled
//	on[:grant,grant...]          → default socket, optional grants
//	unix:<path>[:grant,grant...] → explicit socket, optional grants
//
// Empty string disables. Anything else is a startup error — fail-loud, no
// best-effort guessing. This is the only LURPIC_DRIVE parse site in the
// tree.
func parseDriveEnv(v string) (DriveConfig, error) {
	if v == "" || v == "off" {
		return DriveConfig{}, nil
	}
	cfg := DriveConfig{Enabled: true, GrantsSpec: driveDefaultGrants}
	switch {
	case strings.HasPrefix(v, "unix:"):
		rest := strings.TrimPrefix(v, "unix:")
		path, grants, hasGrants := strings.Cut(rest, ":")
		if path == "" {
			return DriveConfig{}, fmt.Errorf("app: LURPIC_DRIVE=unix:<path> requires a path")
		}
		cfg.SocketPath = path
		if hasGrants {
			cfg.GrantsSpec = grants
		}
	case strings.HasPrefix(v, "on"):
		rest := strings.TrimPrefix(v, "on")
		switch {
		case rest == "":
		case strings.HasPrefix(rest, ":"):
			cfg.GrantsSpec = strings.TrimPrefix(rest, ":")
		default:
			return DriveConfig{}, fmt.Errorf("app: LURPIC_DRIVE=%q is invalid (want off, on[:grants], or unix:<path>[:grants])", v)
		}
	default:
		return DriveConfig{}, fmt.Errorf("app: LURPIC_DRIVE=%q is invalid (want off, on[:grants], or unix:<path>[:grants])", v)
	}
	if cfg.GrantsSpec == "" {
		return DriveConfig{}, fmt.Errorf("app: LURPIC_DRIVE=%q has an empty grant list", v)
	}
	grants, err := server.ParseGrants(cfg.GrantsSpec)
	if err != nil {
		return DriveConfig{}, fmt.Errorf("app: LURPIC_DRIVE grants: %w", err)
	}
	cfg.Grants = grants
	return cfg, nil
}

// defaultDriveSocketPath returns $XDG_RUNTIME_DIR/lurpic-<pid>/drive.sock.
func defaultDriveSocketPath() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("app: LURPIC_DRIVE=on requires XDG_RUNTIME_DIR (or use LURPIC_DRIVE=unix:<path>)")
	}
	return dir + "/lurpic-" + strconv.Itoa(os.Getpid()) + "/drive.sock", nil
}

// startDriveServer boots the LDP server for the configured app and runtime.
// It generates the 32-byte token, binds the socket, and shares the runtime's
// drive counters so both sides count into one surface. The caller owns
// Close.
func startDriveServer(cfg *DriveConfig, rt *runtime.Runtime) (*server.Server, error) {
	if !cfg.Enabled {
		return nil, errors.New("app: drive disabled")
	}
	if cfg.SocketPath == "" {
		path, err := defaultDriveSocketPath()
		if err != nil {
			return nil, err
		}
		cfg.SocketPath = path
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("app: drive token: %w", err)
	}
	sink := rt.DriveSink()
	if sink == nil {
		return nil, errors.New("app: runtime drive sink missing (Runtime.Config.Drive.Enabled)")
	}
	return server.Listen(server.Config{
		SocketPath: cfg.SocketPath,
		Token:      hex.EncodeToString(raw),
		Grants:     cfg.Grants,
		Sink:       sink,
		Stats:      rt.DriveStats(),
	})
}

// wireDriveEnv applies the single LURPIC_DRIVE parse to the app and runtime
// configurations. Returns the parsed config so Run can log and start the
// server after runtime construction.
func wireDriveEnv(cfg *Config) (DriveConfig, error) {
	driveCfg, err := parseDriveEnv(os.Getenv("LURPIC_DRIVE"))
	if err != nil {
		return DriveConfig{}, err
	}
	cfg.Drive = driveCfg
	cfg.Runtime.Drive.Enabled = driveCfg.Enabled
	return driveCfg, nil
}

// logDriveSocket announces the bound socket so a driver can find it.
func logDriveSocket(logger log.Logger, cfg DriveConfig) {
	line := fmt.Sprintf("drive: socket=%s grants=%s", cfg.SocketPath, cfg.Grants.String())
	if logger != nil {
		logger.Info(line)
		return
	}
	fmt.Fprintln(os.Stderr, line)
}
