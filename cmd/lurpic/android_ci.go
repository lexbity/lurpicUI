package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"strings"
)

func cmdAndroidCI(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: android-ci subcommand required (frame, replay)")
		return 1
	}

	switch args[0] {
	case "frame":
		return cmdAndroidFrameCheck(args[1:])
	case "replay":
		return cmdAndroidReplayCheck(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown android-ci subcommand %q (supported: frame, replay)\n", args[0])
		return 1
	}
}

func cmdAndroidFrameCheck(args []string) int {
	fs := flag.NewFlagSet("android-ci frame", flag.ExitOnError)
	screenshot := fs.String("screenshot", "", "Path to captured PNG screenshot")
	maxNonBlackRatio := fs.Float64("max-nonblack-ratio", 0.05, "Maximum ratio of non-black pixels allowed")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}
	if *screenshot == "" {
		fmt.Fprintln(os.Stderr, "Error: --screenshot is required")
		return 1
	}

	if err := verifyScreenshotLaunchState(*screenshot, *maxNonBlackRatio); err != nil {
		fmt.Fprintf(os.Stderr, "Frame check failed: %v\n", err)
		return 1
	}

	fmt.Printf("Frame check passed: %s\n", *screenshot)
	return 0
}

func cmdAndroidReplayCheck(args []string) int {
	fs := flag.NewFlagSet("android-ci replay", flag.ExitOnError)
	logcat := fs.String("logcat", "", "Path to captured logcat output")
	var required []string
	fs.Func("require", "Required substring in logcat (may be repeated)", func(value string) error {
		required = append(required, value)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}
	if *logcat == "" {
		fmt.Fprintln(os.Stderr, "Error: --logcat is required")
		return 1
	}
	if len(required) == 0 {
		required = []string{"[TOUCH] Down", "[TOUCH] Up"}
	}

	if err := verifyLogcatSequence(*logcat, required); err != nil {
		fmt.Fprintf(os.Stderr, "Replay check failed: %v\n", err)
		return 1
	}

	fmt.Printf("Replay check passed: %s\n", *logcat)
	return 0
}

func verifyScreenshotLaunchState(path string, maxNonBlackRatio float64) error {
	if maxNonBlackRatio < 0 || maxNonBlackRatio > 1 {
		return fmt.Errorf("max-nonblack-ratio must be between 0 and 1")
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open screenshot: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decode png: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		return fmt.Errorf("screenshot is empty")
	}

	total := bounds.Dx() * bounds.Dy()
	var nonBlack int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !isNearlyBlack(img.At(x, y)) {
				nonBlack++
			}
		}
	}

	ratio := float64(nonBlack) / float64(total)
	if ratio > maxNonBlackRatio {
		return fmt.Errorf("non-black pixel ratio %.4f exceeds limit %.4f", ratio, maxNonBlackRatio)
	}
	return nil
}

func verifyLogcatSequence(path string, required []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read logcat: %w", err)
	}

	text := string(data)
	offset := 0
	for _, needle := range required {
		idx := strings.Index(text[offset:], needle)
		if idx < 0 {
			return fmt.Errorf("missing required log entry %q", needle)
		}
		offset += idx + len(needle)
	}
	return nil
}

func isNearlyBlack(c color.Color) bool {
	r, g, b, a := c.RGBA()
	if a < 0xf000 {
		return false
	}
	return r <= 0x1010 && g <= 0x1010 && b <= 0x1010
}
