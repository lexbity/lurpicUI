package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// BundleConfig is the JSON structure for bundletool's BundleConfig.
type BundleConfig struct {
	Bundletool  BundleConfigTool  `json:"bundletool"`
	Compression BundleCompression `json:"compression"`
	Optimizations BundleOptimizations `json:"optimizations,omitempty"`
}

type BundleConfigTool struct {
	Version string `json:"version"`
}

type BundleCompression struct {
	UncompressedGlob []string `json:"uncompressedGlob"`
}

type BundleOptimizations struct {
	SplitsConfig *SplitConfig `json:"splitsConfig,omitempty"`
}

type SplitConfig struct {
	SplitDimension []SplitDimension `json:"splitDimension"`
}

type SplitDimension string

const (
	SplitABI      SplitDimension = "ABI"
	SplitLanguage SplitDimension = "LANGUAGE"
	SplitDensity  SplitDimension = "SCREEN_DENSITY"
)

// defaultBundleConfig returns the standard BundleConfig for lurpicUI apps.
func defaultBundleConfig() BundleConfig {
	return BundleConfig{
		Bundletool: BundleConfigTool{
			Version: "1.16.0",
		},
		Compression: BundleCompression{
			// .so files must remain uncompressed so they can be page-aligned
			// during install. This matches the APK assembly behaviour.
			UncompressedGlob: []string{"lib/**/*.so"},
		},
		Optimizations: BundleOptimizations{
			SplitsConfig: &SplitConfig{
				SplitDimension: defaultSplitDimensions(),
			},
		},
	}
}

// defaultSplitDimensions returns the recommended split dimensions for
// optimized Play Store delivery. ABI + density ensures users download
// only the APK matching their device architecture and screen density.
func defaultSplitDimensions() []SplitDimension {
	return []SplitDimension{
		SplitABI,
		SplitDensity,
	}
}

// assembleAAB produces a signed Android App Bundle for Play Store submission.
// It reuses the build pipeline up to the point of generating the module files,
// then runs bundletool to create the .aab, and signs it with apksigner.
func (b *androidBuilder) assembleAAB() error {
	fmt.Println("=== Assembling Android App Bundle ===")

	// Step 1: Generate the base module directory.
	moduleDir, err := b.buildBundleModule()
	if err != nil {
		return fmt.Errorf("bundle module: %w", err)
	}

	// Step 2: Write BundleConfig.json.
	config := defaultBundleConfig()
	configPath := filepath.Join(moduleDir, "BundleConfig.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("bundle config json: %w", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("write BundleConfig.json: %w", err)
	}
	fmt.Printf("  BundleConfig: %s\n", configPath)

	// Step 3: Run bundletool build-bundle.
	unsignedAab := filepath.Join(b.buildDir, "unsigned.aab")
	bundleTool, err := findBundleTool(b.sdk)
	if err != nil {
		return fmt.Errorf("bundletool: %w", err)
	}

	if err := b.runner.Run(CommandSpec{
		Path:   "java",
		Args:   []string{"-jar", bundleTool, "build-bundle", "--modules=" + moduleDir, "--output=" + unsignedAab, "--config=" + configPath},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return fmt.Errorf("bundletool build-bundle failed: %w", err)
	}
	fmt.Printf("  Built: %s\n", unsignedAab)

	// Step 4: Align and sign the AAB (zipalign + apksigner).
	alignedAab := filepath.Join(b.buildDir, "aligned.aab")
	if err := b.alignAPK(unsignedAab, alignedAab); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: zipalign AAB failed: %v\n", err)
		alignedAab = unsignedAab
	}

	if err := b.signFile(alignedAab); err != nil {
		return fmt.Errorf("sign AAB: %w", err)
	}
	fmt.Printf("  Signed AAB: %s\n", b.outputPath)
	return nil
}

// buildBundleModule creates the base module directory structure for bundletool.
// It returns the path to the module directory root.
func (b *androidBuilder) buildBundleModule() (string, error) {
	moduleRoot := filepath.Join(b.buildDir, "bundle", "base")
	if err := os.MkdirAll(moduleRoot, 0755); err != nil {
		return "", err
	}

	// ── Manifest ──
	// AAB requires the manifest in proto-binary format, produced by
	// aapt2 link --proto-format. We do a separate aapt2 link pass.
	aapt2, err := findSDKTool(b.sdk, "aapt2")
	if err != nil {
		return "", fmt.Errorf("aapt2 not found: %w", err)
	}

	// Create a minimal compiled resource set for the manifest.
	manifestDir := filepath.Join(moduleRoot, "manifest")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return "", err
	}
	manifestPB, err := b.compileManifestProto(aapt2)
	if err != nil {
		return "", fmt.Errorf("manifest proto: %w", err)
	}
	// Copy the proto manifest to the module manifest directory.
	manifestDest := filepath.Join(manifestDir, "AndroidManifest.xml")
	if err := copyFile(manifestPB, manifestDest); err != nil {
		return "", err
	}
	fmt.Printf("  Manifest: %s\n", manifestDest)

	// ── Dex (from Java compilation) ──
	dexDir := filepath.Join(moduleRoot, "dex")
	if err := os.MkdirAll(dexDir, 0755); err != nil {
		return "", err
	}
	dexPath := filepath.Join(b.buildDir, "classes.dex")
	if _, err := os.Stat(dexPath); err == nil {
		dest := filepath.Join(dexDir, "classes.dex")
		if err := copyFile(dexPath, dest); err != nil {
			return "", err
		}
		fmt.Printf("  Dex: %s\n", dest)
	}

	// ── Native libs ──
	libRoot := filepath.Join(b.buildDir, "lib")
	if _, err := os.Stat(libRoot); err == nil {
		entries, err := os.ReadDir(libRoot)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			abi := entry.Name()
			libDir := filepath.Join(moduleRoot, "lib", abi)
			if err := os.MkdirAll(libDir, 0755); err != nil {
				return "", err
			}
			srcLibDir := filepath.Join(libRoot, abi)
			libs, err := filepath.Glob(filepath.Join(srcLibDir, "*.so"))
			if err != nil {
				return "", err
			}
			for _, lib := range libs {
				dest := filepath.Join(libDir, filepath.Base(lib))
				if err := copyFile(lib, dest); err != nil {
					return "", err
				}
			}
			fmt.Printf("  Libs [%s]: %s\n", abi, libDir)
		}
	}

	// ── Assets ──
	assetsSrc := filepath.Join(b.buildDir, "assets")
	if _, err := os.Stat(assetsSrc); err == nil {
		assetsDir := filepath.Join(moduleRoot, "assets")
		if err := copyDir(assetsSrc, assetsDir); err != nil {
			return "", fmt.Errorf("assets: %w", err)
		}
		fmt.Printf("  Assets: %s\n", assetsDir)
	}

	// The module root for bundletool is the parent directory of "base".
	return filepath.Dir(moduleRoot), nil
}

// compileManifestProto runs aapt2 link with --proto-format to produce a
// proto-binary AndroidManifest.xml, which is required for AAB. Returns the
// path to the output file.
func (b *androidBuilder) compileManifestProto(aapt2 string) (string, error) {
	androidJar := filepath.Join(b.sdk, "platforms", fmt.Sprintf("android-%d", b.config.Android.TargetSDK), "android.jar")
	if _, err := os.Stat(androidJar); os.IsNotExist(err) {
		return "", fmt.Errorf("android.jar not found at %s", androidJar)
	}

	output := filepath.Join(b.buildDir, "manifest-proto.apk")
	if err := os.Remove(output); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	manifestPath := filepath.Join(b.buildDir, "AndroidManifest.xml")
	if _, err := os.Stat(manifestPath); err != nil {
		return "", fmt.Errorf("manifest not found at %s: %w", manifestPath, err)
	}

	if err := b.runner.Run(CommandSpec{
		Path: aapt2,
		Args: []string{
			"link",
			"--proto-format",
			"-o", output,
			"-I", androidJar,
			"--manifest", manifestPath,
			"--auto-add-overlay",
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		return "", fmt.Errorf("aapt2 link --proto-format failed: %w", err)
	}

	// The manifest proto is inside the output APK — extract it.
	tmpDir, err := os.MkdirTemp("", "manifest-proto-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if err := unzipFile(output, "AndroidManifest.xml", tmpDir); err != nil {
		return "", fmt.Errorf("extract proto manifest: %w", err)
	}

	extracted := filepath.Join(tmpDir, "AndroidManifest.xml")
	manifestProto := filepath.Join(b.buildDir, "AndroidManifest.protobin.xml")
	if err := copyFile(extracted, manifestProto); err != nil {
		return "", err
	}
	return manifestProto, nil
}

// unzipFile extracts a single entry from a zip archive to destDir.
func unzipFile(zipPath, entryName, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", zipPath, err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != entryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		outPath := filepath.Join(destDir, filepath.Base(entryName))
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, rc)
		return err
	}
	return fmt.Errorf("entry %q not found in %s", entryName, zipPath)
}

// copyZipData copies from a zip file reader to a writer.
func copyZipData(w *os.File, r io.ReadCloser) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// findBundleTool locates the bundletool jar, either from the SDK extras
// or from a well-known path.
func findBundleTool(sdk string) (string, error) {
	candidates := []string{
		filepath.Join(sdk, "extras", "google", "bundletool"),
		filepath.Join(sdk, "cmdline-tools", "latest", "bin", "bundletool"),
	}
	// Also check if bundletool is already a jar in the SDK build-tools.
	buildToolsDir := filepath.Join(sdk, "build-tools")
	if entries, err := os.ReadDir(buildToolsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidate := filepath.Join(buildToolsDir, entry.Name(), "lib", "bundletool.jar")
				candidates = append(candidates, candidate)
			}
		}
	}
	// Fallback: check PATH for bundletool jar
	candidates = append(candidates, "bundletool.jar")

	for _, candidate := range candidates {
		// Check for a jar file
		jarPath := candidate
		if !strings.HasSuffix(jarPath, ".jar") {
			jarPath = filepath.Join(candidate, "bundletool.jar")
		}
		if _, err := os.Stat(jarPath); err == nil {
			return jarPath, nil
		}
	}
	return "", fmt.Errorf("bundletool.jar not found; install via sdkmanager \"extras;google;bundletool\" or place bundletool.jar on PATH")
}
