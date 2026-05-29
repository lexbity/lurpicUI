package main

import (
	"testing"
)

func TestArchitectureByABI_x86_64(t *testing.T) {
	arch, ok := ArchitectureByABI("x86_64")
	if !ok {
		t.Fatal("expected x86_64 to be found")
	}
	if arch.ABI != "x86_64" {
		t.Fatalf("ABI = %q, want x86_64", arch.ABI)
	}
	if arch.GOARCH != "amd64" {
		t.Fatalf("GOARCH = %q, want amd64", arch.GOARCH)
	}
	if arch.GOARM != "" {
		t.Fatalf("GOARM = %q, want empty", arch.GOARM)
	}
	if arch.NDKTriple != "x86_64-linux-android" {
		t.Fatalf("NDKTriple = %q, want x86_64-linux-android", arch.NDKTriple)
	}
	if arch.CargoTarget != "x86_64-linux-android" {
		t.Fatalf("CargoTarget = %q, want x86_64-linux-android", arch.CargoTarget)
	}
	if arch.EmulatorABI != "x86_64" {
		t.Fatalf("EmulatorABI = %q, want x86_64", arch.EmulatorABI)
	}
}

func TestArchitectureByABI_arm64(t *testing.T) {
	arch, ok := ArchitectureByABI("arm64-v8a")
	if !ok {
		t.Fatal("expected arm64-v8a to be found")
	}
	if arch.ABI != "arm64-v8a" {
		t.Fatalf("ABI = %q, want arm64-v8a", arch.ABI)
	}
	if arch.GOARCH != "arm64" {
		t.Fatalf("GOARCH = %q, want arm64", arch.GOARCH)
	}
	if arch.GOARM != "" {
		t.Fatalf("GOARM = %q, want empty", arch.GOARM)
	}
	if arch.NDKTriple != "aarch64-linux-android" {
		t.Fatalf("NDKTriple = %q, want aarch64-linux-android", arch.NDKTriple)
	}
	if arch.CargoTarget != "aarch64-linux-android" {
		t.Fatalf("CargoTarget = %q, want aarch64-linux-android", arch.CargoTarget)
	}
	if arch.EmulatorABI != "arm64-v8a" {
		t.Fatalf("EmulatorABI = %q, want arm64-v8a", arch.EmulatorABI)
	}
}

func TestArchitectureByABI_armeabi_v7a(t *testing.T) {
	arch, ok := ArchitectureByABI("armeabi-v7a")
	if !ok {
		t.Fatal("expected armeabi-v7a to be found")
	}
	if arch.ABI != "armeabi-v7a" {
		t.Fatalf("ABI = %q, want armeabi-v7a", arch.ABI)
	}
	if arch.GOARCH != "arm" {
		t.Fatalf("GOARCH = %q, want arm", arch.GOARCH)
	}
	if arch.GOARM != "7" {
		t.Fatalf("GOARM = %q, want 7", arch.GOARM)
	}
	if arch.NDKTriple != "armv7a-linux-androideabi" {
		t.Fatalf("NDKTriple = %q, want armv7a-linux-androideabi", arch.NDKTriple)
	}
	if arch.CargoTarget != "armv7-linux-androideabi" {
		t.Fatalf("CargoTarget = %q, want armv7-linux-androideabi", arch.CargoTarget)
	}
	if arch.EmulatorABI != "" {
		t.Fatalf("EmulatorABI = %q, want empty", arch.EmulatorABI)
	}
}

func TestArchitectureByABI_unknown(t *testing.T) {
	_, ok := ArchitectureByABI("riscv64")
	if ok {
		t.Fatal("expected riscv64 to not be found")
	}
}

func TestDefaultEmulatorArchitecture_is_x86_64(t *testing.T) {
	arch := DefaultEmulatorArchitecture()
	if arch.ABI != "x86_64" {
		t.Fatalf("default emulator arch ABI = %q, want x86_64", arch.ABI)
	}
	if arch.GOARCH != "amd64" {
		t.Fatalf("default emulator arch GOARCH = %q, want amd64", arch.GOARCH)
	}
}

func TestReleaseArchitectures_containsAll(t *testing.T) {
	arches := ReleaseArchitectures()
	if len(arches) != 3 {
		t.Fatalf("expected 3 release architectures, got %d", len(arches))
	}

	found := make(map[string]bool)
	for _, a := range arches {
		found[a.ABI] = true
	}

	if !found["x86_64"] {
		t.Fatal("release architectures missing x86_64")
	}
	if !found["arm64-v8a"] {
		t.Fatal("release architectures missing arm64-v8a")
	}
	if !found["armeabi-v7a"] {
		t.Fatal("release architectures missing armeabi-v7a")
	}
}

func TestArchitectureByABI_mappingTableComplete(t *testing.T) {
	// Every known ABI must be resolvable and self-consistent
	for abi := range architectures {
		arch, ok := ArchitectureByABI(abi)
		if !ok {
			t.Fatalf("architecture %q should be resolvable", abi)
		}
		if arch.ABI != abi {
			t.Fatalf("arch.ABI = %q, want %q", arch.ABI, abi)
		}
	}
}
