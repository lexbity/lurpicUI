package assets

import "testing"

func TestDecodeImageHeader_validKTX2(t *testing.T) {
	data := make([]byte, 80)
	copy(data, ktx2Identifier)
	data[20] = 64 // pixelWidth = 64
	data[24] = 32 // pixelHeight = 32

	w, h, err := decodeImageHeader(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != 64 {
		t.Fatalf("expected width 64, got %d", w)
	}
	if h != 32 {
		t.Fatalf("expected height 32, got %d", h)
	}
}

func TestDecodeImageHeader_tooShort(t *testing.T) {
	_, _, err := decodeImageHeader([]byte{0xAB, 0x4B})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestDecodeImageHeader_invalidIdentifier(t *testing.T) {
	data := make([]byte, 80)
	data[0] = 0xFF // wrong first byte
	_, _, err := decodeImageHeader(data)
	if err == nil {
		t.Fatal("expected error for invalid identifier")
	}
}

func TestDecodeImageHeader_zeroWidth(t *testing.T) {
	data := make([]byte, 80)
	copy(data, ktx2Identifier)
	// Width stays 0, height = 32
	data[24] = 32
	_, _, err := decodeImageHeader(data)
	if err == nil {
		t.Fatal("expected error for zero width")
	}
}

func TestCompressedSizeForTarget_rgba8(t *testing.T) {
	size := compressedSizeForTarget(64, 32, 0)
	if size != 64*32*4 {
		t.Fatalf("expected %d, got %d", 64*32*4, size)
	}
}

func TestCompressedSizeForTarget_astc4x4(t *testing.T) {
	// 64x32 → 16x8 = 128 blocks → 128 * 8 = 1024 bytes
	size := compressedSizeForTarget(64, 32, 1)
	if size != 1024 {
		t.Fatalf("expected 1024, got %d", size)
	}
}

func TestCompressedSizeForTarget_astc4x4NonAligned(t *testing.T) {
	// 65x33 → rounded to 68x36 = 17x9 = 153 blocks → 153 * 8 = 1224 bytes
	size := compressedSizeForTarget(65, 33, 1)
	if size != 1224 {
		t.Fatalf("expected 1224, got %d", size)
	}
}

func TestCompressedSizeForTarget_bc7(t *testing.T) {
	// 64x32 → 16x8 = 128 blocks → 128 * 16 = 2048 bytes
	size := compressedSizeForTarget(64, 32, 2)
	if size != 2048 {
		t.Fatalf("expected 2048, got %d", size)
	}
}

func TestCompressedSizeForTarget_zeroDimensions(t *testing.T) {
	if size := compressedSizeForTarget(0, 32, 0); size != 0 {
		t.Fatalf("expected 0 for zero width, got %d", size)
	}
	if size := compressedSizeForTarget(64, 0, 1); size != 0 {
		t.Fatalf("expected 0 for zero height, got %d", size)
	}
}
