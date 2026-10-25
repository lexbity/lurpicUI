package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// DriveConfig is the app-layer drive configuration. The environment is the
// single source (parseDriveEnv); LDP is disabled unless LURPIC_DRIVE is set.
type DriveConfig struct {
	Enabled    bool
	SocketPath string
	GrantsSpec string
	Grants     drive.Grants
	// Instance is the app launch's random identity; Sessions is the table
	// it issues. Both are created by wireDriveEnv when enabled and shared
	// by the runtime (admission) and the server (transport).
	Instance Instance
	Sessions *drive.SessionTable
	// Stats is the shared drive counter set.
	Stats *drive.Stats
}

// Instance is the drive.Instance type at app scope (the app layer owns the
// lifecycle: one identity per launch, created in wireDriveEnv).
type Instance = drive.Instance

// driveDefaultGrants is the grant set for a bare "on": least privilege —
// reads only. Automation that needs more asks for it explicitly
// (on:read,input).
const driveDefaultGrants = "read"

// parseDriveEnv parses LURPIC_DRIVE. Grammar:
//
//	off                          → disabled
//	on[:grant,grant...]          → default socket, optional grants
//	unix:<path>[:grant,grant...] → explicit absolute socket, optional grants
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
		if !strings.HasPrefix(path, "/") {
			return DriveConfig{}, fmt.Errorf("app: LURPIC_DRIVE socket path %q must be absolute and private", path)
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
	grants, err := drive.ParseGrants(cfg.GrantsSpec)
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

// startDriveServer boots the LDP v2 server for the configured app and
// runtime: it generates the 32-byte launcher token, binds the socket under
// the ownership rules, and installs the explicitly-constructed ports. The
// caller owns Close.
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
	if cfg.Sessions == nil {
		return nil, errors.New("app: drive session table missing (wireDriveEnv)")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("app: drive token: %w", err)
	}
	port := rt.DrivePort()
	if port == nil {
		return nil, errors.New("app: runtime drive port missing (Runtime.Config.Drive.Enabled)")
	}
	return server.Listen(server.Config{
		SocketPath: cfg.SocketPath,
		Token:      hex.EncodeToString(raw),
		Grants:     cfg.Grants,
		Sessions:   cfg.Sessions,
		Port:       port,
		Stats:      rt.DriveStats(),
	})
}

// wireDriveEnv applies the single LURPIC_DRIVE parse to the app and runtime
// configurations. When enabled it creates the instance identity, the
// session table, and the shared counter set, and installs them on the
// runtime configuration BEFORE the runtime is constructed — both sides of
// the seam are explicit at startup.
func wireDriveEnv(cfg *Config) (DriveConfig, error) {
	driveCfg, err := parseDriveEnv(os.Getenv("LURPIC_DRIVE"))
	if err != nil {
		return DriveConfig{}, err
	}
	if driveCfg.Enabled {
		// Android release builds refuse LDP at startup (the build-tag gate).
		if err := androidDriveGate(); err != nil {
			return DriveConfig{}, err
		}
		inst, err := drive.NewInstance()
		if err != nil {
			return DriveConfig{}, fmt.Errorf("app: drive instance: %w", err)
		}
		driveCfg.Instance = inst
		driveCfg.Sessions = drive.NewSessionTable(inst)
		driveCfg.Stats = &drive.Stats{}
	}
	cfg.Drive = driveCfg
	cfg.Runtime.Drive.Enabled = driveCfg.Enabled
	cfg.Runtime.Drive.Sessions = driveCfg.Sessions
	cfg.Runtime.Drive.Stats = driveCfg.Stats
	return driveCfg, nil
}

// logDriveSocket announces the bound socket so a driver can find it.
func logDriveSocket(logger log.Logger, cfg DriveConfig) {
	line := fmt.Sprintf("drive: socket=%s grants=%s instance=%s", cfg.SocketPath, cfg.Grants.String(), cfg.Instance.ID)
	if logger != nil {
		logger.Info(line)
		return
	}
	fmt.Fprintln(os.Stderr, line)
}
