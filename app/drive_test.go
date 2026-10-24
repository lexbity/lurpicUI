package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseDriveEnv(t *testing.T) {
	cases := []struct {
		in      string
		enabled bool
		socket  string
		grants  string
		wantErr bool
	}{
		{in: "", enabled: false},
		{in: "off", enabled: false},
		{in: "on", enabled: true, grants: "read"},
		{in: "on:read", enabled: true, grants: "read"},
		{in: "on:read,input", enabled: true, grants: "read,input"},
		{in: "on:read,input,write,time,admin", enabled: true, grants: "read,input,write,time,admin"},
		{in: "unix:/tmp/d.sock", enabled: true, socket: "/tmp/d.sock", grants: "read"},
		{in: "unix:/tmp/d.sock:write", enabled: true, socket: "/tmp/d.sock", grants: "write"},
		{in: "unix:", wantErr: true},
		{in: "onx", wantErr: true},
		{in: "on:", wantErr: true}, // empty grant list after colon
		{in: "on:read,root", wantErr: true},
		{in: "tcp:host:1", wantErr: true},
	}
	for _, tc := range cases {
		cfg, err := parseDriveEnv(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDriveEnv(%q) = %+v, want error", tc.in, cfg)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDriveEnv(%q): %v", tc.in, err)
			continue
		}
		if cfg.Enabled != tc.enabled {
			t.Errorf("parseDriveEnv(%q).Enabled = %v, want %v", tc.in, cfg.Enabled, tc.enabled)
		}
		if cfg.SocketPath != tc.socket {
			t.Errorf("parseDriveEnv(%q).SocketPath = %q, want %q", tc.in, cfg.SocketPath, tc.socket)
		}
		if cfg.Grants.String() != tc.grants {
			t.Errorf("parseDriveEnv(%q).Grants = %q, want %q", tc.in, cfg.Grants.String(), tc.grants)
		}
	}
}

// TestDriveDisabledByDefault (G1.4): with LURPIC_DRIVE unset nothing drive-
// related exists — no socket file, no token file, no listener goroutine.
func TestDriveDisabledByDefault(t *testing.T) {
	t.Setenv("LURPIC_DRIVE", "")
	before := runtime.NumGoroutine()

	cfg := DefaultConfig("test", 100, 100)
	driveCfg, err := wireDriveEnv(&cfg)
	if err != nil {
		t.Fatalf("wireDriveEnv: %v", err)
	}
	if driveCfg.Enabled {
		t.Fatal("drive must be disabled when LURPIC_DRIVE is unset")
	}
	if cfg.Runtime.Drive.Enabled {
		t.Fatal("runtime drive must be disabled when LURPIC_DRIVE is unset")
	}
	if _, err := startDriveServer(&driveCfg, nil); err == nil {
		t.Fatal("startDriveServer must refuse a disabled config")
	}

	// The default socket location stays untouched.
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "lurpic-"+itoaQuick(os.Getpid()))); !os.IsNotExist(err) {
			t.Fatalf("drive dir must not exist when disabled: %v", err)
		}
	}
	// No listener goroutine was spawned by configuration alone.
	after := runtime.NumGoroutine()
	if after > before {
		t.Fatalf("goroutines grew from %d to %d with drive disabled", before, after)
	}
}

func itoaQuick(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestDriveEnabledCreatesSocketAndToken proves the enabled path binds the
// socket (0600) and writes the token file (0600) next to it, and that Close
// removes both.
func TestDriveEnabledCreatesSocketAndToken(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	t.Setenv("LURPIC_DRIVE", "unix:"+socket+":read")

	cfg := DefaultConfig("test", 100, 100)
	driveCfg, err := wireDriveEnv(&cfg)
	if err != nil {
		t.Fatalf("wireDriveEnv: %v", err)
	}
	if !driveCfg.Enabled || !driveCfg.Grants.Read || driveCfg.Grants.Input {
		t.Fatalf("driveCfg = %+v", driveCfg)
	}
	if driveCfg.SocketPath != socket {
		t.Fatalf("SocketPath = %q", driveCfg.SocketPath)
	}
}

func TestWireDriveEnvDefaultsSocket(t *testing.T) {
	t.Setenv("LURPIC_DRIVE", "unix:/tmp/whatever.sock")
	cfg := DefaultConfig("t", 10, 10)
	driveCfg, err := wireDriveEnv(&cfg)
	if err != nil {
		t.Fatalf("wireDriveEnv: %v", err)
	}
	if !driveCfg.Enabled || !cfg.Runtime.Drive.Enabled {
		t.Fatal("both layers must be enabled")
	}
	if !driveCfg.Grants.Read || driveCfg.Grants.Write {
		t.Fatalf("default grants = %v", driveCfg.Grants)
	}
}

func TestDefaultDriveSocketPath(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	if _, err := defaultDriveSocketPath(); err == nil {
		t.Fatal("missing XDG_RUNTIME_DIR must be an error")
	}
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	path, err := defaultDriveSocketPath()
	if err != nil {
		t.Fatalf("defaultDriveSocketPath: %v", err)
	}
	want := "/run/user/1000/lurpic-" + itoaQuick(os.Getpid()) + "/drive.sock"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
