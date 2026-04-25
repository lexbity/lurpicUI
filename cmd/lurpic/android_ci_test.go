package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyScreenshotLaunchState_accepts_black_image(t *testing.T) {
	path := writePNG(t, 8, 8, color.NRGBA{A: 255})

	if err := verifyScreenshotLaunchState(path, 0.01); err != nil {
		t.Fatalf("verifyScreenshotLaunchState() error = %v", err)
	}
}

func TestVerifyScreenshotLaunchState_rejects_nonblack_image(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	path := writePNGImage(t, img)

	if err := verifyScreenshotLaunchState(path, 0.01); err == nil {
		t.Fatal("verifyScreenshotLaunchState() expected error for non-black image")
	}
}

func TestVerifyLogcatSequence_acceptsOrderedEvents(t *testing.T) {
	path := writeText(t, "noise\n[TOUCH] Down seq=1\nother\n[TOUCH] Move seq=1\n[TOUCH] Up seq=1\n")

	if err := verifyLogcatSequence(path, []string{"[TOUCH] Down", "[TOUCH] Move", "[TOUCH] Up"}); err != nil {
		t.Fatalf("verifyLogcatSequence() error = %v", err)
	}
}

func TestVerifyLogcatSequence_rejectsOutOfOrderEvents(t *testing.T) {
	path := writeText(t, "[TOUCH] Up seq=1\n[TOUCH] Down seq=1\n")

	if err := verifyLogcatSequence(path, []string{"[TOUCH] Down", "[TOUCH] Up"}); err == nil {
		t.Fatal("verifyLogcatSequence() expected error for out-of-order events")
	}
}

func writePNG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return writePNGImage(t, img)
}

func writePNGImage(t *testing.T, img image.Image) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create() error = %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return path
}

func writeText(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "logcat.txt")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
